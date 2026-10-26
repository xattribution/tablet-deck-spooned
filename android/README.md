# Tablet Deck for Android

A native shell around the deck. The app finds Tablet Deck PCs on the home network,
pairs with one using the code shown on the PC, and then shows the deck that the PC
serves, full screen. The deck itself is the same web app the browser gets. The app
adds discovery, real haptics, immersive full screen and a launcher icon.

- Package: `dev.xattribution.tabletdeck` (the debug build is `…tabletdeck.debug`)
- Android 8.0 (API 26) and newer. Targets API 35.
- Permissions: Internet, network state, Wi-Fi state, Wi-Fi multicast (to hear PC
  beacons), vibrate. Nothing else.

## Install on a tablet

**With USB debugging:**

```sh
adb install -r app/build/outputs/apk/release/app-release.apk
```

**Without a computer:** copy `app-release.apk` to the tablet (USB, cloud drive or
chat), open it in Files, and allow **Install unknown apps** for that app when Android
asks. Updates install over the old version and keep the pairing, as long as they're
signed with the same key (see [keystore/README.md](keystore/README.md)).

## Build

You need JDK 17 or newer and the Android SDK (platform 35, build-tools 35.0.0).

```sh
cd android
echo "sdk.dir=/path/to/android-sdk" > local.properties   # or set ANDROID_HOME
./gradlew assembleRelease      # app/build/outputs/apk/release/app-release.apk (signed)
./gradlew assembleDebug        # app/build/outputs/apk/debug/app-debug.apk
./gradlew testDebugUnitTest lintRelease
```

The release build is minified with R8 and signed with the key in `keystore/`, using
the passwords in `keystore/keystore.properties`. Both files are gitignored. Without
them the release APK is built unsigned.

Setting up the SDK from the command line on Linux:

```sh
mkdir -p ~/android-sdk/cmdline-tools && cd ~/android-sdk/cmdline-tools
curl -LO https://dl.google.com/android/repository/commandlinetools-linux-13114758_latest.zip
unzip commandlinetools-linux-*_latest.zip && mv cmdline-tools latest
yes | latest/bin/sdkmanager --licenses
latest/bin/sdkmanager "platforms;android-35" "build-tools;35.0.0" "platform-tools"
```

## How it works

| step | what happens |
|---|---|
| Launch | The bundled launch page (`app/src/main/assets/launch/`) lists the PCs heard on the network, live. **Enter address** takes `192.168.1.20:47615` by hand (the port defaults to 47615). |
| Discovery | While the launch page is up, the app listens on UDP **47619** with a Wi-Fi multicast lock. PCs broadcast `{"app":"tabletdeck","id":"…","name":"…","port":47615,"ver":"2.0.0"}` every 5 s. The address is the packet's source IP. PCs unheard for 20 s drop off. The tablet never sends. |
| Pairing | After picking a PC you type the 8-character code from the PC. The app checks that the PC answers on its port, then loads `http://<ip>:<port>/?t=<code>&device=tablet&native=android&did=<installId>`. |
| Next launches | The app remembers the last PC (id, name, address) and goes straight to it: `http://<ip>:<port>/?device=tablet&native=android&did=<installId>`, with no code. The deck uses the device token it stored in localStorage on the first visit. If the PC's beacon shows up at a new address, the app follows it. If the PC can't be reached for 12 s, the app goes back to the list. |
| Deck | Full-screen WebView: immersive (swipe from the edge shows the bars), screen kept on, any orientation, no zoom, no long-press menus, no overscroll glow. Rotation doesn't reload the page. If the WebView's renderer crashes, the app makes a new one and reloads. Links to other sites open in the browser. |
| Back | The app calls `window.TabletDeckApp.back()`. If that returns true, the page handled it. Otherwise the app goes to the background without closing. |

The user agent ends with ` TabletDeckAndroid/2.0.0`. Remote debugging
(`chrome://inspect`) works on the debug build only.

Cleartext HTTP is allowed for all hosts (`res/xml/network_security_config.xml`),
because Android can only scope cleartext to host names, not to private IP ranges.
The app only loads the PC the player picked.

## JS bridge: `window.TabletDeckNative`

Present in every page the app shows (the launch page and the deck). Check for it
before use: `window.TabletDeckNative?.haptic('tap')`. Every method is safe to call
often.

| method | returns | what it does |
|---|---|---|
| `haptic(kind)` | — | A haptic tick. Kinds: `tick` (lightest, for detents and swipes), `tap`, `press` (heavier), `heavy`, `error`, `long`. Unknown kinds are ignored. It still works when the player has turned system touch feedback off: then the vibrator plays the matching effect. |
| `deviceId()` | string | A random UUID made once per install. Also sent as `did` in the deck URL. |
| `info()` | JSON string | `{"app":"android","version":"2.0.0","model":"<Build.MODEL>","sdk":35}` |
| `setKeepAwake(on)` | — | Keep the screen on (the default) or let it sleep. |
| `hosts()` | JSON string | PCs heard recently: `[{"id","name","ip","port","ver","lastSeen"}]`, where `lastSeen` is epoch milliseconds. |
| `lastHost()` | JSON string | The remembered PC as `{"id","name","ip","port"}`, or `null`. |
| `connect(ip, port, code, id, name)` | — | Remember this PC and load its deck. Pass `""` as `code` when the device is already paired. |
| `disconnect()` | — | Go back to the launch screen. The remembered PC is kept. |
| `forget()` | — | Forget the remembered PC and go back to the launch screen. |

Calls from the app into the page:

| function | when |
|---|---|
| `window.onHosts(list)` | The list of PCs changed (launch page only). `list` is the array `hosts()` returns. |
| `window.onConnectFailed({name, addr})` | The PC didn't answer (launch page only). |
| `window.TabletDeckApp.back()` | The hardware or gesture back. Return `true` if the page handled it. |

### Haptic mapping

| kind | system touch feedback on | touch feedback off, or `heavy` |
|---|---|---|
| `tick` | `CLOCK_TICK` | `EFFECT_TICK` |
| `tap` | `KEYBOARD_TAP` | `EFFECT_CLICK` |
| `press` | `CONFIRM` (Android 11+), else `VIRTUAL_KEY` | `EFFECT_HEAVY_CLICK` |
| `heavy` | — | `EFFECT_HEAVY_CLICK` |
| `error` | `REJECT` (Android 11+), else `LONG_PRESS` | `EFFECT_DOUBLE_CLICK` |
| `long` | `LONG_PRESS` | `EFFECT_HEAVY_CLICK` |

Before Android 13 the app overrides the system switch directly
(`FLAG_IGNORE_GLOBAL_SETTING`). From Android 13 on, that flag only works for system
apps, so the app uses the vibrator whenever touch feedback is off. On Android 8 and 9,
which have no predefined effects, the vibrator plays short one-shot pulses instead.

## Files

| path | what it is |
|---|---|
| `app/src/main/java/…/MainActivity.kt` | the window, the WebView, connecting, back and immersive mode |
| `app/src/main/java/…/Discovery.kt`, `Beacon.kt` | the beacon listener and parser |
| `app/src/main/java/…/Bridge.kt` | `window.TabletDeckNative` |
| `app/src/main/java/…/Haptics.kt` | haptic kinds to view feedback or vibrator effects |
| `app/src/main/assets/launch/index.html` | the launch page (offline, no external files) |
| `app/src/main/res/drawable/ic_launcher_*.xml` | the adaptive icon, traced from `public/icon.svg` |
| `keystore/` | the sideload signing key (gitignored) and how to remake it |
