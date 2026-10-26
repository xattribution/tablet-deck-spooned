package dev.xattribution.tabletdeck

import android.content.Context
import androidx.core.content.edit
import java.util.UUID

/** The install id and the last PC, in SharedPreferences. */
class Prefs(context: Context) {
    private val sp = context.getSharedPreferences("tabletdeck", Context.MODE_PRIVATE)

    /** A random id made once per install, sent to the PC as `did`. */
    val installId: String = sp.getString(K_INSTALL, null) ?: UUID.randomUUID().toString().also {
        sp.edit { putString(K_INSTALL, it) }
    }

    fun lastHost(): Host? {
        val ip = sp.getString(K_IP, null) ?: return null
        val port = sp.getInt(K_PORT, 0).takeIf { it in 1..65535 } ?: return null
        return Host(sp.getString(K_ID, "") ?: "", sp.getString(K_NAME, ip) ?: ip, ip, port)
    }

    fun saveHost(h: Host) {
        sp.edit { putString(K_ID, h.id).putString(K_NAME, h.name).putString(K_IP, h.ip).putInt(K_PORT, h.port) }
    }

    fun forgetHost() {
        sp.edit { remove(K_ID).remove(K_NAME).remove(K_IP).remove(K_PORT) }
    }

    private companion object {
        const val K_INSTALL = "install_id"
        const val K_ID = "host_id"
        const val K_NAME = "host_name"
        const val K_IP = "host_ip"
        const val K_PORT = "host_port"
    }
}
