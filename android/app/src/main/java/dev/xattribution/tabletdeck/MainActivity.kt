package dev.xattribution.tabletdeck

import android.annotation.SuppressLint
import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Intent
import android.graphics.Bitmap
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import android.webkit.RenderProcessGoneDetail
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.window.OnBackInvokedCallback
import android.window.OnBackInvokedDispatcher
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import org.json.JSONObject
import java.net.InetSocketAddress
import java.net.Socket

/**
 * One full-screen WebView that shows either the bundled launch page (find a PC, enter
 * the pairing code) or the deck served by the PC.
 */
class MainActivity : Activity() {
    lateinit var prefs: Prefs
        private set
    lateinit var discovery: Discovery
        private set
    lateinit var haptics: Haptics
        private set

    private val main = Handler(Looper.getMainLooper())
    private lateinit var root: FrameLayout
    private var web: WebView? = null
    private val bridge by lazy { Bridge(this) }

    private enum class Mode { LAUNCH, LOADING, DECK }

    private var mode = Mode.LAUNCH
    /** What to reload if the renderer dies. */
    private var currentUrl = LAUNCH_URL
    /** The PC being connected to or shown. */
    private var target: Host? = null
    private var loadFailed = false
    /** Bumped to cancel an in-flight reachability probe. */
    @Volatile private var attempt = 0
    private var ready = false
    private var backCallback: Any? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        val splash = installSplashScreen()
        super.onCreate(savedInstanceState)
        splash.setKeepOnScreenCondition { !ready }
        main.postDelayed({ ready = true }, 1200)

        prefs = Prefs(this)
        discovery = Discovery(this) { hosts -> main.post { onHosts(hosts) } }

        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        if (Build.VERSION.SDK_INT >= 30) {
            window.attributes = window.attributes.apply {
                layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_ALWAYS
            }
        } else if (Build.VERSION.SDK_INT >= 28) {
            window.attributes = window.attributes.apply {
                layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
            }
        }

        root = FrameLayout(this).apply { setBackgroundColor(BG) }
        setContentView(root)
        haptics = Haptics(this, root)
        // Edge-to-edge turns off adjustResize, so make room for the keyboard by hand.
        ViewCompat.setOnApplyWindowInsetsListener(root) { v, insets ->
            v.setPadding(0, 0, 0, insets.getInsets(WindowInsetsCompat.Type.ime()).bottom)
            insets
        }

        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
        newWebView()
        registerBack()

        val last = prefs.lastHost()
        if (last != null) {
            showLaunch("connecting=" + Uri.encode(last.name) + "&addr=" + Uri.encode(last.address))
            probeAndLoad(last, null)
        } else {
            showLaunch(null)
        }
    }

    // ---------- WebView ----------

    @SuppressLint("SetJavaScriptEnabled")
    private fun newWebView() {
        val w = WebView(this)
        w.setBackgroundColor(BG)
        w.overScrollMode = View.OVER_SCROLL_NEVER
        w.isVerticalScrollBarEnabled = false
        w.isHorizontalScrollBarEnabled = false
        w.isHapticFeedbackEnabled = false
        w.isLongClickable = false
        w.setOnLongClickListener { true } // no text-selection or link menus
        w.defaultFocusHighlightEnabled = false
        w.setRendererPriorityPolicy(WebView.RENDERER_PRIORITY_IMPORTANT, false)

        w.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            @Suppress("DEPRECATION")
            databaseEnabled = true
            mediaPlaybackRequiresUserGesture = false
            mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
            cacheMode = WebSettings.LOAD_DEFAULT
            setSupportZoom(false)
            builtInZoomControls = false
            displayZoomControls = false
            textZoom = 100
            useWideViewPort = true
            loadWithOverviewMode = false
            allowFileAccess = false
            allowContentAccess = false
            setGeolocationEnabled(false)
            javaScriptCanOpenWindowsAutomatically = false
            setSupportMultipleWindows(false)
            userAgentString = "$userAgentString TabletDeckAndroid/${BuildConfig.VERSION_NAME}"
        }
        w.addJavascriptInterface(bridge, "TabletDeckNative")
        w.webChromeClient = WebChromeClient()
        w.webViewClient = Client()
        root.addView(w, FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT))
        web = w
    }

    private inner class Client : WebViewClient() {
        override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
            val url = request.url
            if (url.scheme == "file" && url.toString().startsWith(LAUNCH_URL)) return false
            val t = target
            if (t != null && (url.scheme == "http" || url.scheme == "https") && sameHost(url, t)) return false
            // Anything else (docs, the website) opens in the browser.
            try {
                startActivity(Intent(Intent.ACTION_VIEW, url).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            } catch (_: ActivityNotFoundException) {
            }
            return true
        }

        override fun onPageStarted(view: WebView, url: String, favicon: Bitmap?) {
            loadFailed = false
        }

        override fun onPageFinished(view: WebView, url: String) {
            if (url.startsWith(LAUNCH_URL)) {
                ready = true
                view.clearHistory()
                return
            }
            if (mode == Mode.LOADING && !loadFailed) {
                mode = Mode.DECK
                target?.let { prefs.saveHost(it); currentUrl = deckUrl(it, null) }
                discovery.stop()
                view.clearHistory()
            }
        }

        override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
            if (!request.isForMainFrame || mode == Mode.LAUNCH) return
            loadFailed = true
            val t = target ?: return showLaunch(null)
            if (mode == Mode.LOADING) {
                // The port answered but the page didn't load: say so rather than retrying forever.
                showLaunch("error=" + Uri.encode(t.name) + "&addr=" + Uri.encode(t.address))
                return
            }
            // The deck lost the PC (it closed or moved): go back to the launch page and keep trying it.
            showLaunch("connecting=" + Uri.encode(t.name) + "&addr=" + Uri.encode(t.address))
            probeAndLoad(t, null)
        }

        override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
            // The renderer crashed or was killed for memory: start a new WebView and reload.
            root.removeView(view)
            if (web === view) web = null
            view.destroy()
            newWebView()
            web?.loadUrl(currentUrl)
            return true
        }
    }

    private fun sameHost(url: Uri, h: Host): Boolean {
        val host = url.host ?: return false
        val port = if (url.port == -1) (if (url.scheme == "https") 443 else 80) else url.port
        return host.trim('[', ']').equals(h.ip.trim('[', ']'), ignoreCase = true) && port == h.port
    }

    // ---------- launch page and connecting ----------

    private fun showLaunch(hash: String?) {
        mode = Mode.LAUNCH
        currentUrl = if (hash.isNullOrEmpty()) LAUNCH_URL else "$LAUNCH_URL#$hash"
        web?.loadUrl(currentUrl)
        discovery.start()
    }

    /** Called by the launch page once the player picked a PC (and typed the code). */
    fun connect(h: Host, code: String?) {
        if (mode != Mode.LAUNCH) showLaunch("connecting=" + Uri.encode(h.name) + "&addr=" + Uri.encode(h.address))
        probeAndLoad(h, code)
    }

    /**
     * Checks the PC answers on its port before handing the address to the WebView, so a
     * PC that's off shows a clear message instead of a browser error page. Retries for a
     * while, and follows the PC to a new address if its beacon shows up somewhere else.
     */
    private fun probeAndLoad(h: Host, code: String?) {
        val my = ++attempt
        target = h
        discovery.start()
        Thread({
            val deadline = SystemClock.elapsedRealtime() + PROBE_WINDOW_MS
            var t = h
            while (attempt == my) {
                if (t.id.isNotEmpty()) discovery.find(t.id)?.let { seen ->
                    if (seen.ip != t.ip || seen.port != t.port) t = t.copy(ip = seen.ip, port = seen.port)
                }
                if (reachable(t)) {
                    val found = t
                    main.post { if (attempt == my) loadDeck(found, code) }
                    return@Thread
                }
                if (SystemClock.elapsedRealtime() > deadline) {
                    val lost = t
                    main.post { if (attempt == my) connectFailed(lost) }
                    return@Thread
                }
                try { Thread.sleep(700) } catch (_: InterruptedException) { return@Thread }
            }
        }, "td-probe").apply { isDaemon = true }.start()
    }

    private fun reachable(h: Host): Boolean = try {
        Socket().use { it.connect(InetSocketAddress(h.ip, h.port), PROBE_TIMEOUT_MS); true }
    } catch (_: Exception) {
        false
    }

    private fun loadDeck(h: Host, code: String?) {
        target = h
        mode = Mode.LOADING
        currentUrl = deckUrl(h, code)
        web?.loadUrl(currentUrl)
    }

    /** The code is only sent on first pairing; after that the page uses its stored device token. */
    private fun deckUrl(h: Host, code: String?): String {
        val q = StringBuilder()
        if (code != null) q.append("t=").append(Uri.encode(code)).append('&')
        q.append("device=tablet&native=android&did=").append(Uri.encode(prefs.installId))
        return "http://${h.address}/?$q"
    }

    private fun connectFailed(h: Host) {
        val msg = JSONObject().put("name", h.name).put("addr", h.address).toString()
        if (mode == Mode.LAUNCH) {
            web?.evaluateJavascript("window.onConnectFailed&&window.onConnectFailed($msg)", null)
        } else {
            showLaunch("error=" + Uri.encode(h.name) + "&addr=" + Uri.encode(h.address))
        }
    }

    fun disconnect(forget: Boolean) {
        attempt++
        if (forget) prefs.forgetHost()
        if (mode != Mode.LAUNCH) {
            web?.stopLoading()
            showLaunch(null)
        }
    }

    private fun onHosts(hosts: List<Host>) {
        // Follow the remembered PC if its address changed.
        prefs.lastHost()?.let { last ->
            val seen = hosts.firstOrNull { it.id == last.id && last.id.isNotEmpty() }
                ?: hosts.firstOrNull { last.id.startsWith("manual:") && it.ip == last.ip && it.port == last.port }
            if (seen != null && (seen.ip != last.ip || seen.port != last.port || seen.name != last.name || seen.id != last.id)) {
                prefs.saveHost(seen)
            }
        }
        if (mode == Mode.LAUNCH) {
            web?.evaluateJavascript("window.onHosts&&window.onHosts(${Beacon.toJson(hosts)})", null)
        }
    }

    fun setKeepAwake(on: Boolean) {
        if (on) window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        else window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
    }

    // ---------- back, immersive, lifecycle ----------

    private fun registerBack() {
        if (Build.VERSION.SDK_INT >= 33) {
            val cb = OnBackInvokedCallback { handleBack() }
            onBackInvokedDispatcher.registerOnBackInvokedCallback(OnBackInvokedDispatcher.PRIORITY_DEFAULT, cb)
            backCallback = cb
        }
    }

    @Deprecated("Android 12 and older; 13+ uses OnBackInvokedCallback")
    @Suppress("DEPRECATION")
    override fun onBackPressed() = handleBack()

    /** The page gets the first say (close a sheet, leave a sub-screen); otherwise go to the background. */
    private fun handleBack() {
        val w = web ?: return run { moveTaskToBack(true) }
        w.evaluateJavascript(BACK_JS) { r -> if (r != "true") moveTaskToBack(true) }
    }

    private fun hideBars() {
        WindowInsetsControllerCompat(window, window.decorView).apply {
            hide(WindowInsetsCompat.Type.systemBars())
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus) hideBars()
    }

    override fun onResume() {
        super.onResume()
        hideBars()
        web?.onResume()
        if (mode == Mode.LAUNCH) discovery.start()
    }

    override fun onPause() {
        web?.onPause()
        discovery.stop()
        super.onPause()
    }

    override fun onDestroy() {
        attempt++
        discovery.stop()
        haptics.release()
        if (Build.VERSION.SDK_INT >= 33) {
            (backCallback as? OnBackInvokedCallback)?.let { onBackInvokedDispatcher.unregisterOnBackInvokedCallback(it) }
        }
        web?.let { root.removeView(it); it.destroy() }
        web = null
        super.onDestroy()
    }

    private companion object {
        const val LAUNCH_URL = "file:///android_asset/launch/index.html"
        const val BG = 0xFF0A111C.toInt()
        const val PROBE_TIMEOUT_MS = 2500
        const val PROBE_WINDOW_MS = 12_000L
        const val BACK_JS = "(function(){try{var a=window.TabletDeckApp;return !!(a&&a.back&&a.back())}catch(e){return false}})()"
    }
}
