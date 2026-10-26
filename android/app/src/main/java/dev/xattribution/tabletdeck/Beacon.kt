package dev.xattribution.tabletdeck

import org.json.JSONArray
import org.json.JSONObject

/** A Tablet Deck PC: one heard on the network, or the one remembered from last time. */
data class Host(
    val id: String,
    val name: String,
    val ip: String,
    val port: Int,
    val ver: String = "",
    val lastSeen: Long = 0L,
) {
    /** "192.168.1.20:47615", with IPv6 literals bracketed. */
    val address: String get() = (if (':' in ip) "[$ip]" else ip) + ":" + port

    fun toJson(): JSONObject = JSONObject()
        .put("id", id).put("name", name).put("ip", ip).put("port", port)
        .put("ver", ver).put("lastSeen", lastSeen)
}

/**
 * The discovery beacon each PC broadcasts on UDP [PORT] every 5 seconds:
 * `{"app":"tabletdeck","id":"…","name":"…","port":47615,"ver":"2.0.0"}`.
 * It carries no pairing code; the address is the packet's source.
 */
object Beacon {
    const val PORT = 47619
    const val DEFAULT_DECK_PORT = 47615
    private const val MAX_NAME = 40

    /** Parses one packet from [ip]; null when it isn't a well-formed Tablet Deck beacon. */
    fun parse(data: ByteArray, length: Int, ip: String, now: Long): Host? {
        if (length <= 0 || length > data.size || length > 2048) return null
        val o = try {
            JSONObject(String(data, 0, length, Charsets.UTF_8))
        } catch (e: Exception) {
            return null
        }
        if (o.optString("app") != "tabletdeck") return null
        val id = o.optString("id").trim()
        if (id.isEmpty() || id.length > 64) return null
        val port = (o.opt("port") as? Number)?.toInt() ?: return null
        if (port !in 1..65535) return null
        val name = o.optString("name").trim().take(MAX_NAME).ifEmpty { ip }
        return Host(id, name, ip, port, o.optString("ver").trim().take(24), now)
    }

    fun toJson(hosts: Collection<Host>): String {
        val a = JSONArray()
        for (h in hosts) a.put(h.toJson())
        return a.toString()
    }
}
