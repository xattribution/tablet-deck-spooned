package dev.xattribution.tabletdeck

import android.content.Context
import android.database.ContentObserver
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.Message
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import android.provider.Settings
import android.view.HapticFeedbackConstants
import android.view.View

/**
 * Haptics for the deck. Uses the view's haptic feedback (the device's tuned effects)
 * when system touch feedback is on, and the vibrator otherwise, so the deck still
 * buzzes for players who turned touch feedback off for the rest of the tablet.
 * Calls arrive on the JavaBridge thread; they hop to the UI thread through pooled
 * Messages, so nothing is allocated per tap.
 */
class Haptics(context: Context, private val view: View) : Handler.Callback {
    private val main = Handler(Looper.getMainLooper(), this)
    private val resolver = context.contentResolver

    private val vibrator: Vibrator? = if (Build.VERSION.SDK_INT >= 31) {
        context.getSystemService(VibratorManager::class.java)?.defaultVibrator
    } else {
        @Suppress("DEPRECATION")
        context.getSystemService(Vibrator::class.java)
    }?.takeIf { it.hasVibrator() }

    private val effects: Array<VibrationEffect> = Array(KINDS) { effectFor(it) }

    @Volatile private var systemOn = readSetting()
    private val observer = object : ContentObserver(main) {
        override fun onChange(selfChange: Boolean) { systemOn = readSetting() }
    }

    init {
        resolver.registerContentObserver(Settings.System.getUriFor(Settings.System.HAPTIC_FEEDBACK_ENABLED), false, observer)
    }

    fun release() {
        resolver.unregisterContentObserver(observer)
        main.removeCallbacksAndMessages(null)
    }

    /** Any thread. Unknown kinds are ignored. */
    fun perform(kind: String?) {
        val k = when (kind) {
            "tick" -> TICK
            "tap" -> TAP
            "press" -> PRESS
            "heavy" -> HEAVY
            "error" -> ERROR
            "long" -> LONG
            else -> return
        }
        main.sendMessageAtFrontOfQueue(Message.obtain(main, k))
    }

    override fun handleMessage(msg: Message): Boolean {
        play(msg.what)
        return true
    }

    private fun play(k: Int) {
        // "heavy" has no matching view constant; the vibrator's heavy click is the real thing.
        if (k != HEAVY) {
            val flags = HapticFeedbackConstants.FLAG_IGNORE_VIEW_SETTING
            if (Build.VERSION.SDK_INT < 33) {
                // Before Android 13 any app may override the global switch.
                @Suppress("DEPRECATION")
                if (view.performHapticFeedback(constantFor(k), flags or HapticFeedbackConstants.FLAG_IGNORE_GLOBAL_SETTING)) return
            } else if (systemOn) {
                // From 13 on the override is ignored for normal apps, so only use the view
                // path when touch feedback is on; otherwise fall through to the vibrator.
                view.performHapticFeedback(constantFor(k), flags)
                return
            }
        }
        vibrator?.vibrate(effects[k])
    }

    private fun readSetting(): Boolean =
        Settings.System.getInt(resolver, Settings.System.HAPTIC_FEEDBACK_ENABLED, 1) != 0

    private fun constantFor(k: Int): Int = when (k) {
        TICK -> HapticFeedbackConstants.CLOCK_TICK
        TAP -> HapticFeedbackConstants.KEYBOARD_TAP
        PRESS -> if (Build.VERSION.SDK_INT >= 30) HapticFeedbackConstants.CONFIRM else HapticFeedbackConstants.VIRTUAL_KEY
        ERROR -> if (Build.VERSION.SDK_INT >= 30) HapticFeedbackConstants.REJECT else HapticFeedbackConstants.LONG_PRESS
        else -> HapticFeedbackConstants.LONG_PRESS
    }

    private fun effectFor(k: Int): VibrationEffect {
        if (Build.VERSION.SDK_INT >= 29) {
            return VibrationEffect.createPredefined(
                when (k) {
                    TICK -> VibrationEffect.EFFECT_TICK
                    TAP -> VibrationEffect.EFFECT_CLICK
                    ERROR -> VibrationEffect.EFFECT_DOUBLE_CLICK
                    else -> VibrationEffect.EFFECT_HEAVY_CLICK
                }
            )
        }
        // Android 8–9: short one-shots scaled to match.
        return when (k) {
            TICK -> VibrationEffect.createOneShot(8, 70)
            TAP -> VibrationEffect.createOneShot(14, 140)
            PRESS -> VibrationEffect.createOneShot(20, 200)
            ERROR -> VibrationEffect.createWaveform(longArrayOf(0, 18, 70, 18), intArrayOf(0, 220, 0, 220), -1)
            else -> VibrationEffect.createOneShot(32, 255)
        }
    }

    private companion object {
        const val TICK = 0
        const val TAP = 1
        const val PRESS = 2
        const val HEAVY = 3
        const val ERROR = 4
        const val LONG = 5
        const val KINDS = 6
    }
}
