package dev.xattribution.tabletdeck

import android.content.Context
import android.net.wifi.WifiManager
import java.io.IOException
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetSocketAddress
import java.net.SocketTimeoutException

/**
 * Listens for PC beacons on UDP 47619 while the launch screen is up. It only listens;
 * the tablet never sends. Holds a multicast lock while running, since many Wi-Fi
 * drivers drop broadcast packets to save power without one.
 */
class Discovery(context: Context, private val onChange: (List<Host>) -> Unit) {
    private val wifi = context.applicationContext.getSystemService(WifiManager::class.java)
    private val hosts = LinkedHashMap<String, Host>()
    @Volatile private var worker: Thread? = null
    @Volatile private var socket: DatagramSocket? = null
    private var lock: WifiManager.MulticastLock? = null

    @Synchronized
    fun start() {
        if (worker != null) return
        lock = try {
            wifi?.createMulticastLock("tabletdeck-discovery")?.apply { setReferenceCounted(false); acquire() }
        } catch (e: RuntimeException) {
            null
        }
        worker = Thread(::run, "td-discovery").apply { isDaemon = true; start() }
    }

    @Synchronized
    fun stop() {
        worker = null
        socket?.close()
        socket = null
        try {
            lock?.release()
        } catch (e: RuntimeException) {
            // already released
        }
        lock = null
    }

    /** PCs heard in the last 20 seconds, by name. */
    fun snapshot(): List<Host> = synchronized(hosts) {
        prune(System.currentTimeMillis())
        hosts.values.sortedBy { it.name.lowercase() }
    }

    fun find(id: String): Host? = synchronized(hosts) { hosts[id] }

    private fun run() {
        val me = Thread.currentThread()
        val buf = ByteArray(2048)
        val pkt = DatagramPacket(buf, buf.size)
        while (worker === me) {
            val s = try {
                DatagramSocket(null).apply {
                    reuseAddress = true
                    broadcast = true
                    soTimeout = 2000
                    bind(InetSocketAddress(Beacon.PORT))
                }
            } catch (e: IOException) {
                // Port busy or no network yet: try again shortly.
                try { Thread.sleep(2000) } catch (_: InterruptedException) { return }
                continue
            }
            socket = s
            try {
                while (worker === me) {
                    try {
                        pkt.setLength(buf.size)
                        s.receive(pkt)
                        val ip = pkt.address?.hostAddress ?: continue
                        Beacon.parse(buf, pkt.length, ip.substringBefore('%'), System.currentTimeMillis())?.let(::put)
                    } catch (_: SocketTimeoutException) {
                    }
                    if (synchronized(hosts) { prune(System.currentTimeMillis()) }) publish()
                }
            } catch (_: IOException) {
                // socket closed by stop(), or the network went away
            } finally {
                s.close()
            }
        }
    }

    private fun put(h: Host) {
        val changed = synchronized(hosts) {
            val old = hosts.put(h.id, h)
            old == null || old.ip != h.ip || old.port != h.port || old.name != h.name || old.ver != h.ver
        }
        if (changed) publish()
    }

    /** Drops PCs unheard for 20 s; true when the list changed. Caller holds [hosts]. */
    private fun prune(now: Long): Boolean {
        val it = hosts.values.iterator()
        var removed = false
        while (it.hasNext()) {
            if (now - it.next().lastSeen > STALE_MS) { it.remove(); removed = true }
        }
        return removed
    }

    private fun publish() = onChange(snapshot())

    private companion object {
        const val STALE_MS = 20_000L
    }
}
