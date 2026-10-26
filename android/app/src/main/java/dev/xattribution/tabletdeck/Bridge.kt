package dev.xattribution.tabletdeck

import android.os.Build
import android.webkit.JavascriptInterface
import org.json.JSONObject

/**
 * `window.TabletDeckNative`, for the launch page and the deck. Methods run on the
 * WebView's JavaBridge thread; anything that touches views hops to the UI thread.
 */
class Bridge(private val activity: MainActivity) {
    private val infoJson: String = JSONObject()
        .put("app", "android")
        .put("version", BuildConfig.VERSION_NAME)
        .put("model", Build.MODEL)
        .put("sdk", Build.VERSION.SDK_INT)
        .toString()

    @JavascriptInterface
    fun haptic(kind: String?) = activity.haptics.perform(kind)

    @JavascriptInterface
    fun deviceId(): String = activity.prefs.installId

    @JavascriptInterface
    fun info(): String = infoJson

    @JavascriptInterface
    fun setKeepAwake(on: Boolean) = activity.runOnUiThread { activity.setKeepAwake(on) }

    @JavascriptInterface
    fun hosts(): String = Beacon.toJson(activity.discovery.snapshot())

    /** The remembered PC as JSON, or "null". Used by the launch page. */
    @JavascriptInterface
    fun lastHost(): String = activity.prefs.lastHost()?.toJson()?.toString() ?: "null"

    @JavascriptInterface
    fun connect(ip: String?, port: Int, code: String?, id: String?, name: String?) {
        val host = ip?.trim().orEmpty()
        if (host.isEmpty() || port !in 1..65535) return
        val h = Host(id?.trim().orEmpty(), name?.trim().orEmpty().ifEmpty { host }.take(40), host, port)
        val c = code?.trim()?.lowercase()?.takeIf { it.isNotEmpty() }
        activity.runOnUiThread { activity.connect(h, c) }
    }

    @JavascriptInterface
    fun disconnect() = activity.runOnUiThread { activity.disconnect(forget = false) }

    @JavascriptInterface
    fun forget() = activity.runOnUiThread { activity.disconnect(forget = true) }
}
