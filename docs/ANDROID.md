# SpoolUp for Android

The Android app (`android/`) is a full-screen window onto the deck your PC serves: the same
deck a browser gets, with the PC found by itself, the screen kept on, and real haptics on
every press. Every tap goes to SpoolUp on the PC, with the same pairing code and the same
focus check as the browser deck.

- Package `dev.xattribution.spoolup` (the debug build is `dev.xattribution.spoolup.debug`,
  so both can be installed). App name SpoolUp, the SpoolUp icon.
- Android 8.0 (API 26) and newer. Built against and targeting API 36 (Android 16).
- Version: SpoolUp's, from `package.json` (2.2.0 has version code 20200).
- Permissions: Internet and network state (to reach the PC), Wi-Fi multicast (so the Wi-Fi
  driver passes the PC's discovery answers), vibrate (haptics), camera (for scanning
  the QR code; Android asks the first time).

## Using it

1. Start SpoolUp on the PC. The tablet or phone and the PC need to be on the same network.
2. Open the app. **Choose a PC** lists the SpoolUp PCs it finds on the network, live, and
   the ones opened before. Tap one.
3. The first time, the deck asks for the pairing code shown in the SpoolUp window on the
   PC. Or tap **Scan QR code** and point the camera at the QR code in that window: it
   carries the address and the code. **Enter address** takes `192.168.1.20:8765` by hand
   (the port is 8765 unless you changed it).

After that the app goes straight to the last PC when it starts. If the PC got a new address
from the router, the app follows it (by the name it advertises); the deck may then ask for
the code once more, since it keeps the code per address, as a browser does. The deck's
**Switch PC** button (where the browser deck has Fullscreen) goes back to the list. The
back gesture on the deck sends the app to the background instead of closing the deck.

## How it works

| part | what it does |
|---|---|
| Discovery | SpoolUp's server answers DNS-SD questions over multicast DNS for the service type `_spoolup._tcp` (`discovery.js`): `PTR`, `SRV` (the deck's port), `TXT` (`v=<version>`, `name=<PC name>`) and `A` (the address on the network the question came from). It announces itself at start and says goodbye when it stops. The app browses with Android's `NsdManager` while the list is on screen and resolves each PC one at a time. The iOS app browses for the same service with Bonjour. |
| Networks | mDNS goes to 224.0.0.251 on the PC's network adapters, skipping VPN and virtual ones (the same list the QR code's address skips). Questions from outside the PC's networks get no answer, and answers are rate limited. The app opens home network addresses: private IPv4 ranges, `.local` and single-label names (`PcAddress.isLocal`, the same rules as the iOS app). |
| Turning it off | SpoolUp window: **Settings > Network > Let the SpoolUp app find this PC** (on by default), or `"discovery": false` in `config.json`. The apps then need the address or the QR code. |
| Connecting | The app checks that the PC answers on its port before loading the deck, so a PC that's off shows a clear message; it keeps trying for 12 seconds. Then it loads `http://<address>:<port>/`, adding `?t=<code>` once when the code came from a QR code; after that the deck keeps the code in its own storage, so a newer code typed on the deck stays. |
| Full screen | Immersive: the system bars hide (a swipe from the edge shows them for a moment), the deck stays clear of a camera cutout, any orientation, no zoom, no long-press menus. Rotation doesn't reload the deck. If the WebView's renderer is killed, the app makes a new one and reloads. Links to other sites open in the browser. |
| Screen on | `FLAG_KEEP_SCREEN_ON` on the app's window, the whole time the app is in front. |
| Haptics | The deck calls `window.SpoolUpApp.haptic(kind)` where a browser would call `navigator.vibrate` (Android's WebView ignores that). Kinds: `tick`, `tap`, `press`, `heavy`, `error`. They play the device's tuned effects when system touch feedback is on, and the vibrator otherwise. |
| Saved PCs | Up to 12 names and addresses, kept on the device and out of backups. |

### The bridge the deck sees

`window.SpoolUpApp` exists only inside the apps; the deck checks for it before use
(`public/app.js`), so the browser deck is unchanged. Android and iOS offer the same calls:

| call | what it does |
|---|---|
| `haptic(kind)` | One haptic effect: `tick`, `tap`, `press`, `heavy` or `error`. Unknown kinds are ignored. |
| `switchPC()` | Leaves the deck for the app's list of PCs. |

The launch page (`app/src/main/assets/launch/index.html`) has its own bridge,
`window.SpoolUpLaunch`, which the app adds only while that page is showing and removes
before a deck loads.

### The PC side

- The firewall rule SpoolUp's installer adds is for the server program on private networks,
  so it covers the mDNS answers (UDP 5353) as well as the deck. On a network Windows treats
  as Public, neither works; the SpoolUp window says how to fix that.
- Windows has its own mDNS responder on UDP 5353. SpoolUp shares the port with it and
  answers for its own names (`spoolup-<pc name>.local`; `<pc name>.local` stays Windows'). If
  something holds the port exclusively, the server logs "Discovery is off" and everything
  else works as before.

## Building

You need JDK 17 or newer and the Android SDK with platform 36 and build-tools 36.0.0.
Android Studio has both: open the `android/` folder and run the `app` configuration.

From the command line:

```sh
cd android
echo "sdk.dir=/path/to/android-sdk" > local.properties   # or set ANDROID_HOME
./gradlew assembleDebug         # app/build/outputs/apk/debug/app-debug.apk
./gradlew testDebugUnitTest     # the address and discovery rules
./gradlew lintRelease
./gradlew assembleRelease       # app/build/outputs/apk/release/app-release.apk
./gradlew bundleRelease         # app/build/outputs/bundle/release/app-release.aab (Google Play)
```

The Gradle wrapper downloads Gradle 8.14.3 and checks it against its SHA-256. Release
builds are shrunk with R8. Without a signing key (below) they come out unsigned.

Setting up the SDK without Android Studio, on Linux:

```sh
mkdir -p ~/android-sdk/cmdline-tools && cd ~/android-sdk/cmdline-tools
curl -LO https://dl.google.com/android/repository/commandlinetools-linux-13114758_latest.zip
unzip commandlinetools-linux-*_latest.zip && mv cmdline-tools latest
yes | latest/bin/sdkmanager --licenses
latest/bin/sdkmanager "platforms;android-36" "build-tools;36.0.0" "platform-tools"
```

## Signing

Android only installs an update over an installed app when both are signed with the same
key, so make the key once and keep it. **Back it up** (with its passwords) somewhere safe
and off the repository: if it's lost, players have to uninstall the app before installing a
build signed with a new key, and on Google Play only Google's support can reset an upload
key.

Make the key (keytool comes with the JDK):

```sh
cd android/keystore
keytool -genkeypair -keystore spoolup-upload.jks -storetype PKCS12 -alias spoolup \
  -keyalg RSA -keysize 4096 -validity 10000 -dname "CN=SpoolUp, O=Xattribution"
```

PKCS12 keystores use one password for the store and the key; give the same one to both
settings below.

**On your PC:** write `android/keystore/keystore.properties` (gitignored, like the `.jks`):

```properties
storeFile=keystore/spoolup-upload.jks
storePassword=...
keyAlias=spoolup
keyPassword=...
```

`./gradlew assembleRelease` then signs with it.

**For the Release workflow:** add four repository secrets (GitHub: Settings > Secrets and
variables > Actions > New repository secret):

| secret | value |
|---|---|
| `ANDROID_KEYSTORE` | the keystore, base64-encoded: `base64 -w0 spoolup-upload.jks` (macOS: `base64 -i spoolup-upload.jks`; PowerShell: `[Convert]::ToBase64String([IO.File]::ReadAllBytes("spoolup-upload.jks"))`) |
| `ANDROID_KEYSTORE_PASSWORD` | the store password |
| `ANDROID_KEY_ALIAS` | `spoolup` |
| `ANDROID_KEY_PASSWORD` | the key password |

With all four set, the Release workflow's `android` job runs after the Windows build: unit
tests, lint, a signed APK and bundle, a signature check, and `SpoolUp-<version>.apk` (with
its `.sha256`) attached to the release. The `.aab` for Google Play stays a run artifact
(**SpoolUp-Android**). Without the secrets the job skips with a notice and the release is
Windows only. In the build the key exists only for the one Gradle step, as a temporary
file it deletes afterward.

Gradle reads the same settings from environment variables, which is how the workflow
passes them: `SPOOLUP_KEYSTORE` (a path), `SPOOLUP_KEYSTORE_PASSWORD`, `SPOOLUP_KEY_ALIAS`,
`SPOOLUP_KEY_PASSWORD`.

## Installing the APK (sideloading)

- With USB debugging: `adb install -r SpoolUp-2.2.0.apk`.
- Without a computer: copy the APK to the device, open it in Files, and allow **Install
  unknown apps** for Files when Android asks. Updates install over the old version and keep
  the saved PCs, as long as they're signed with the same key.

## Google Play

1. **Developer account.** Sign up at [play.google.com/console](https://play.google.com/console)
   with a Google account, pay the one-time registration fee, and verify your identity (a
   personal account needs a government ID; an organization account needs a D-U-N-S number).
   New personal accounts must run a **closed test with at least 12 testers for 14 days in a
   row** before they can publish to production, so line up testers early (a Discord or org
   channel works).
2. **Create the app.** Name: SpoolUp. Free. Category: Tools (it isn't a game itself). Accept the declarations.
3. **Play App Signing.** It's required for new apps; Google holds the key that signs what
   devices install. When the console asks for the app signing key, choose to **use your
   own key** and upload the key above with the tool the console offers (PEPK). Then the
   APK on GitHub and the app from Play carry the same signature, so a player can move
   between them without uninstalling. Google recommends a separate upload key for the
   `.aab` files you upload; if you make one, sign the bundle with it by hand (the workflow
   signs both files with the `ANDROID_KEYSTORE` key, which Play also accepts as the upload
   key when you don't register another). If you let Google generate the signing key instead, GitHub and
   Play installs can't update each other; say so on the download page.
4. **Store listing.** Short and full description (say it's a fan project, not affiliated
   with or endorsed by Cloud Imperium Games, and that it needs SpoolUp running on a Windows
   PC); app icon 512 x 512 PNG (`public/icon-512.png`); feature graphic 1024 x 500;
   at least two phone screenshots and, for tablets, 7-inch and 10-inch ones. Don't put
   "Star Citizen" in the app's title (trademark); the description can say what it works with.
5. **Privacy policy.** Google Play asks for its URL in App content > Privacy policy. Publish
   a short page (for example on the website, `site/`) describing what the app does with
   data: it connects to the PC the player picks on their network, uses the camera to read a
   QR code on the device, and keeps its list of PCs on the device. Data collected: none.
6. **Data safety form** (App content > Data safety):
   - Does your app collect or share any of the required user data types? **No.** (Google
     counts data processed on the device, like the camera frames for the QR code, and data
     sent to the player's own PC as not collected.)
   - Is all of the user data collected by your app encrypted in transit? Not applicable.
   - Account creation: none. Data deletion: not applicable; uninstalling removes the saved
     PCs.
7. **The rest of App content.** Ads: no. App access: explain that the app needs SpoolUp
   running on a Windows PC on the same network, and give the reviewers the website link
   and a short video of the app finding a PC and pressing buttons, since they can't run the
   PC side. Content rating questionnaire: a utility with no user-generated content, no
   sharing, no purchases. Target audience: 18 and over (or 13 and over; it isn't made for
   children). News app: no. Government app: no.
8. **Target API level.** Google Play requires new apps and updates to target an API level
   within a year of the latest Android release: from 31 August 2026 that's API 36
   (Android 16), which the app targets. Each August the bar moves up one level; raise
   `compileSdk` and `targetSdk` in `android/app/build.gradle.kts`, check the behavior
   changes for that level, and test before then. Android 17 is expected to add a runtime
   permission for local network access for apps that target it; the app will need to ask
   for it before browsing.
9. **Release.** Upload `SpoolUp-<version>.aab` (from the workflow's **SpoolUp-Android**
   artifact) to Internal testing first, then the closed test, then Production. The version
   code comes from `package.json`, so every SpoolUp release can be uploaded as is; a
   rebuild of the same version needs a new version number.

## Testing

**Automated:** `./gradlew testDebugUnitTest lintRelease` checks the address rules
(`PcAddressTest`: typed addresses, the QR code link, what counts as the home network) and
how a discovered service becomes a PC. On the PC side, `discovery.js` can be exercised with
any mDNS browser: `avahi-browse -rt _spoolup._tcp` (Linux), `dns-sd -B _spoolup._tcp`
(macOS, or Windows with Bonjour), or Python's `zeroconf`.

**By hand**, on a real phone or tablet (the emulator's network is behind NAT and doesn't
see multicast, so discovery can't be tried there; it can open `10.0.2.2:8790`, the host's
dry-run server). Run `node server.js --dry-run --port 8790` on a PC on the same Wi-Fi and
open the showcase (`http://localhost:8790/showcase.html?t=<code>`) to watch the presses.

- [ ] The PC shows up under **Choose a PC** within a few seconds, with its name and
      version; closing SpoolUp takes it off the list.
- [ ] Turning off **Let the SpoolUp app find this PC** takes it off; turning it on brings it back.
- [ ] First open asks for the pairing code; the right code opens the deck; a wrong one is refused.
- [ ] **Scan QR code** asks for the camera once, then opens the deck without asking for
      the code. A QR code for a website is refused with a note.
- [ ] **Enter address** with `192.168.1.20` (port 8765 assumed), with a port, with
      `pc-name.local`; an internet address is refused.
- [ ] Every press gives one haptic tick; a blocked press (game not in front, or not
      connected) gives the error buzz. The browser deck still vibrates as before.
- [ ] The screen stays on with the deck open for longer than the screen timeout.
- [ ] Rotating keeps the deck and its connection; swiping from the edge shows the bars
      for a moment.
- [ ] Back on the deck sends the app to the background; coming back finds the deck
      still connected.
- [ ] **Switch PC** goes back to the list.
- [ ] Quitting SpoolUp on the PC while the deck is open: the deck says it's offline and
      reconnects when SpoolUp starts again.
- [ ] Starting the app with the PC off says it couldn't reach it after 12 seconds.
- [ ] After the PC's address changes (renew its DHCP lease), the app finds it at the new
      address.
- [ ] A new pairing code on the PC: the deck asks for it; the app doesn't put the old one back.
- [ ] Phone and tablet sizes, Android 8 and a current version, gesture and three-button navigation.

The Windows side of discovery (sharing UDP 5353 with Windows' own responder, the firewall
rule) needs a Windows PC to try.
