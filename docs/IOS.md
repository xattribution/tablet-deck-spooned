# SpoolUp for iPhone and iPad

The iOS app is a small native shell around the deck. The deck itself is the same web
page Safari gets from the PC; the app adds what a browser tab can't: it finds the PC by
itself, pairs with a QR code, plays real haptics, keeps the screen on, runs full screen,
remembers PCs and includes an offline demo. The sources are in [`ios/`](../ios).

> **Status:** written before the Apple developer account was approved, on a machine
> without a Mac. None of the Swift has been compiled or run yet. The first step on a Mac
> is to build it, run the unit tests and fix whatever the compiler finds (see
> [First build](#first-build)). Treat everything below as the plan until then.

## What's in `ios/`

| file | what it does |
|---|---|
| `project.yml` | the [XcodeGen](https://github.com/yonaskolb/XcodeGen) spec; the `.xcodeproj` is generated from it and never committed |
| `bundle-deck.js` | copies `public/` (without the editor and showcase) and `layouts/` into `ios/Deck/` for the offline demo; Xcode runs it before each build when it can find `node` |
| `SpoolUp/SpoolUpApp.swift` | the app, the root view and the full-screen deck screen (screen kept on) |
| `SpoolUp/HomeView.swift` | the list of PCs: found on the network, saved, Scan QR code, Type the address, Try the demo, About |
| `SpoolUp/DeckView.swift` | the `WKWebView` that shows the deck; sends other sites to Safari and comes back to the list if the PC doesn't answer |
| `SpoolUp/Bridge.swift` | `window.SpoolUpApp` for the deck (haptics, Switch PC) |
| `SpoolUp/Discovery.swift` | Bonjour: browses for `_spoolup._tcp` with `NWBrowser` and resolves each PC to its IPv4 address |
| `SpoolUp/PCAddress.swift` | reads typed addresses and QR codes; accepts only home-network addresses |
| `SpoolUp/SavedPCs.swift` | the saved PCs, in `UserDefaults` on the device |
| `SpoolUp/QRScanner.swift` | the camera view (AVFoundation) that reads the PC's QR code |
| `SpoolUp/DemoScheme.swift` | serves the bundled deck from `spoolup-demo://deck/` for the offline demo |
| `SpoolUp/Haptics.swift` | the Taptic Engine kinds |
| `SpoolUp/Info.plist`, `PrivacyInfo.xcprivacy` | permissions, App Transport Security, the privacy manifest |
| `SpoolUp/{es,fr,de}.lproj/` | Spanish, French and German text (`Localizable.strings`, `InfoPlist.strings`); the English text is the key |
| `SpoolUpTests/` | unit tests for address parsing, saved PCs and the bridge messages |

- Bundle ID `dev.xattribution.spoolup`, iOS and iPadOS 16 or newer, iPhone and iPad.
- Permissions: Local Network (Bonjour and talking to the PC) and Camera (to scan the QR code).
  No entitlements: Bonjour through `NWBrowser` doesn't need the multicast entitlement.
- Colors are the house style from [DESIGN.md](DESIGN.md) (the RSI look), in dark mode
  only; text uses the system font.

## How it works

| step | what happens |
|---|---|
| First launch | The list of PCs. iOS asks for Local Network access as soon as the app starts browsing for `_spoolup._tcp`. PCs running SpoolUp show up by name as they resolve. |
| Pairing | Tap a found PC, scan the QR code in the PC's window, or type `192.168.1.20:8765`. The app opens `http://192.168.1.20:8765/` and the deck asks for the pairing code, as in Safari. A QR code already holds it (`?t=1a2b3c4d`), so it's passed to the deck that one time. The deck keeps the code in its own storage, per address. |
| Next launches | The app opens the PC used last straight away. If it doesn't answer within 15 seconds, the list comes back with the reason. If Bonjour finds a saved PC at a new address, the saved address follows it (the deck then asks for the code once, because its storage is per address). |
| Deck | Full screen, status bar and home indicator hidden, screen kept on (`isIdleTimerDisabled`) while the deck is up, no bounce, no link previews. Links to other sites open in Safari. If iOS ends the page's process, the deck reloads. Once the deck has loaded, it handles a lost connection itself, as in Safari. |
| Switch PC | Deck settings gets a **Switch PC** row inside the app (public/app.js), which goes back to the list. |
| Demo | **Try the demo** opens the bundled deck in demo mode (`index.html?demo`, the website's "Try it" page), offline; presses play their animations. |

The user agent ends with ` SpoolUpiOS/<version>`. Debug builds can be inspected from
Safari on a Mac (`isInspectable`); release builds can't.

### The bridge: `window.SpoolUpApp`

The same two calls as the Android app. The iOS app defines them in a user script that
runs before the deck's own scripts, in the top frame only, and posts to the
`spoolUp` script message handler; the Android app adds a JavaScript interface of the
same name. The deck checks for it before use, since a browser doesn't have it.

| call | what it does |
|---|---|
| `SpoolUpApp.haptic(kind)` | `tick`, `tap`, `press`, `heavy` or `error`; unknown kinds are ignored. The deck's `buzz()` maps its vibration lengths onto these: a short tap is `tap`, 30 ms or more is `press`, the denied pattern is `error`. |
| `SpoolUpApp.switchPC()` | Leaves the deck for the app's list of PCs. |

The app acts on messages from the top frame of the PC it opened (or the demo).
iPhones have a Taptic Engine; iPads don't, so on an iPad `haptic` does nothing.

### What the PC advertises

The server's discovery module (`discovery.js`, shared with the Android app) answers
multicast DNS on the home network with:

- the service `_spoolup._tcp.local`, with the PC's name as the instance name;
- an SRV record whose port is the deck's HTTP port (`config.port`, 8765 by default) and
  whose target, `spoolup-<pc name>.local`, it also answers **A** queries for with the
  PC's address on that network;
- a **TXT** record with `v=<version>` and `name=<PC name>`:
  Apple's resolver waits for both SRV and TXT before it reports a service as resolved.

The "Let the SpoolUp app find this PC" switch in the app window's Settings turns it off.

The app doesn't read the TXT record. It resolves the service with a TCP connect over
IPv4, reads the address, hangs up and opens the deck by IP address, the same address the
PC's QR code holds.

## Why plain http is allowed

The deck is plain http on the home network. App Transport Security doesn't block
connections to IP addresses, `.local` names or single-label names, and
`NSAllowsLocalNetworking` states that intent in `Info.plist`. It isn't one of the
exceptions App Review asks a justification for, unlike `NSAllowsArbitraryLoads`, which
the app doesn't use. `PCAddress` takes private IPv4 addresses (10/8, 172.16/12,
192.168/16, 169.254/16), `.local` names and single-label names. (Apple:
[Preventing insecure network connections](https://developer.apple.com/documentation/security/preventing-insecure-network-connections).)

## Once the account is approved

### 1. Account and certificates

1. In [developer.apple.com](https://developer.apple.com/account) > Membership, note
   the **Team ID**. Put it in `ios/project.yml` (`DEVELOPMENT_TEAM`); it isn't a secret.
2. Sign in to Xcode with the Apple ID (Xcode > Settings > Accounts). With automatic
   signing (`CODE_SIGN_STYLE: Automatic`), Xcode creates the **Apple Development** and
   **Apple Distribution** certificates and the provisioning profiles itself.
3. Register the **App ID** `dev.xattribution.spoolup` (Certificates, Identifiers &
   Profiles > Identifiers > +, App IDs, App). No capabilities need turning on. Automatic
   signing also registers it on the first device build, but registering it by hand
   reserves the ID.
4. Register the iPad and iPhone for development: plug them in and let Xcode add them,
   and turn on **Developer Mode** on each (Settings > Privacy & Security).

### 2. First build

On a Mac with **Xcode 26 or newer** (App Store Connect only accepts builds made with
Xcode 26 and the iOS 26 SDK since 28 Apr 2026) and Node 18 or newer:

```sh
brew install xcodegen
git clone https://github.com/xattribution/SpoolUp && cd SpoolUp
node ios/bundle-deck.js        # the deck for the offline demo, into ios/Deck/
cd ios && xcodegen generate    # writes SpoolUp.xcodeproj
open SpoolUp.xcodeproj
```

Then:

1. Build (⌘B) and fix what the compiler reports. The code was written to compile in Swift
   5 language mode with Xcode 26; concurrency warnings are expected to be few.
   Check that `PrivacyInfo.xcprivacy` and the `Deck` folder are under the target's Copy
   Bundle Resources (XcodeGen should put them there).
2. Run the tests (⌘U) on an iPad simulator. `SpoolUpTests` covers the address rules, the
   saved PCs and the bridge messages.
3. Run on the iPad (pick it as the destination, ⌘R). Check, with SpoolUp running on the
   PC:
   - Local Network prompt on first launch; the PC appears by name within a few seconds.
   - Tap it and type the pairing code the PC shows (like `1a2b3c4d`); the deck loads full
     screen with no status bar; presses reach the PC (try `node server.js --dry-run` and
     the showcase first).
   - Scan the QR code in the PC's window: the deck opens already paired.
   - Type the address; try a wrong one (`example.com`, `8.8.8.8`) and see the message.
   - Deck settings > Switch PC returns to the list; the PC is under Saved PCs.
   - Quit and relaunch: it opens the last PC. Stop the server and relaunch: the list
     comes back after 15 s with the reason.
   - Try the demo with Wi-Fi off.
   - Leave the deck open for 10 minutes: the screen stays on.
   - On an iPhone: haptics on presses, and the denied buzz when Star Citizen isn't in
     front.
   - Turn off Local Network for SpoolUp in Settings: the list says so, with Open Settings.
4. Write down anything that differs in [`docs/IOS.md`](IOS.md) and fix it.

### 3. TestFlight

1. In [App Store Connect](https://appstoreconnect.apple.com) > Apps > +, **New App**:
   platform iOS, name **SpoolUp** (if it's taken, *SpoolUp Deck*), primary language
   English (U.S.), bundle ID `dev.xattribution.spoolup`, SKU `spoolup-ios`.
2. In Xcode: select **Any iOS Device**, Product > Archive, then Distribute App > App
   Store Connect > Upload. `ITSAppUsesNonExemptEncryption` is already `false` (the app
   uses no encryption of its own), so no export compliance questions come up.
3. The build appears under TestFlight after processing (minutes to an hour). Add
   yourself as an **internal tester**; internal testing needs no review.
4. For outside testers, create an external group and add the build: the first build of
   each version gets a short **Beta App Review**. Give it the same review notes as below.

`MARKETING_VERSION` in `project.yml` is SpoolUp's version and `CURRENT_PROJECT_VERSION`
counts up with it (2.2.0 is 20200, like the Android version code). Set both with each
release, and raise `CURRENT_PROJECT_VERSION` again for a second upload of one version.

### 4. App Store listing

| field | value |
|---|---|
| Name | SpoolUp |
| Subtitle | Control deck for your gaming PC |
| Category | Utilities (secondary: Entertainment) |
| Price | Free |
| Age rating | Answer *None* to every content question (the app has no user content, web browsing or chat); expect 4+ |
| Keywords | `control deck,tablet,hotas,macro pad,button box,flight sim,space sim,gaming pc,stream,keyboard` (no game or company names: guideline 2.3.7) |
| Support URL | `https://github.com/xattribution/SpoolUp/issues` |
| Marketing URL | the website |
| Privacy Policy URL | required for every app; publish the text below on the website, e.g. `privacy.html` |
| Copyright | `2026 xattribution` |

**Description** (keep game and company names out of the name, subtitle, keywords and
the first lines):

> SpoolUp turns your iPad or iPhone into a touch control deck for your gaming PC: big,
> clear buttons for landing gear, power, shields, quantum travel and more, each one
> sending a single key press to the free SpoolUp app for Windows over your home network.
>
> - Finds your PC by itself, or pair with the QR code on the PC
> - Haptic feedback on iPhone, screen kept on while you play
> - Layouts and themes for flying, mining, salvage and hauling; edit them on the PC
> - Spanish, French and German
> - Try the demo without a PC
>
> Requires the free SpoolUp app on a Windows PC on the same network. One tap sends one
> key press.
>
> SpoolUp is an unofficial fan project, not affiliated with or endorsed by any game
> developer or publisher.

**Screenshots:** the deck on an iPad 13-inch (2064 x 2752 or 2048 x 2732, landscape works
too) and an iPhone 6.9-inch (1320 x 2868 or 1290 x 2796); App Store Connect scales them
for smaller devices. Take them from the demo or a dry-run server, with a theme whose name
isn't a game manufacturer's, and no game footage. Add one of the list of PCs (Bonjour) and
one of the QR scan, so the native parts are visible.

**App Privacy:** answer **Data Not Collected**, matching `PrivacyInfo.xcprivacy`
(`NSPrivacyTracking` false, no collected data types). The deck page loads its fonts from
Google Fonts; Apple counts data as collected when it's kept longer than needed to serve
the request, which the fonts request isn't, so the answer stays *Data Not Collected*.
Bundling the two fonts with the deck would take the question away.

**EU:** App Store Connect asks every developer for a **trader status** under the Digital
Services Act. A free fan app with no income can declare *not a trader*; a trader's
address and phone number are shown on the listing.

**Privacy policy text:**

> SpoolUp for iPhone and iPad keeps the PCs you save (their names and home-network
> addresses) on your device. It connects to the SpoolUp app on your PC over your home
> network and uses the camera to read the QR code on your PC. The deck loads its fonts
> from Google Fonts. Data collected: none. Questions: open an issue at
> https://github.com/xattribution/SpoolUp/issues.

### 5. Review notes

In App Store Connect > the version > App Review Information. No sign-in is needed, so
there's no demo account; say so and point to the demo:

> SpoolUp is a companion control deck for the free SpoolUp app for Windows
> (https://github.com/xattribution/SpoolUp), which runs on the player's own PC and turns
> each tap into one key press. The reviewer can try every screen of the deck without a
> PC: on the first screen tap **Try the demo**. The deck is bundled in the app and runs
> offline.
>
> Native features: Bonjour discovery of the PC on the local network (NWBrowser,
> `_spoolup._tcp`), QR code pairing with the camera, haptic feedback (iPhone), the
> screen kept on while the deck is open, saved PCs, and the bundled offline demo.
>
> Local Network access is used to find and talk to the player's PC; the camera reads
> the PC's QR code. The app connects to private home-network addresses.
>
> A video of the app controlling a PC: <link>

Record that video (a minute: the PC appears in the list, pairing by QR code, presses on
the PC) and link it; it answers most questions before they're asked.

## App Review risks

| guideline | risk | mitigation |
|---|---|---|
| **4.2 Minimum Functionality** (and 4.2.2) | A `WKWebView` app looks like a "repackaged website". This is the most likely rejection. | The app does what Safari can't and the listing, screenshots and review notes say so: **Bonjour discovery** of the PC; **QR pairing** with the camera; **haptics** through the Taptic Engine (Safari on iOS has no vibration API); **keep awake** for long sessions (a web page's wake lock can lapse; the app's can't); **offline demo** bundled in the app; **saved PCs** that follow the PC to a new address. The content isn't a website at all: it's served by the player's own PC on their home network. If it's still rejected, reply in Resolution Center with the list and the video before changing code. Next native steps, if needed: a widget or Shortcuts action to open a saved PC, and a native settings screen. |
| **4.2.3(i)** "work on its own without requiring installation of another app" | The deck needs the Windows app. | The demo mode works with nothing else installed, and the review notes lead with it. Companion apps for PC software (remote controls, Stream Deck–style decks) are common on the store; describe it as a companion and link the free PC app. |
| **2.1 App Completeness** | The reviewer can't test without a PC. | The demo mode shows the full deck. Apple's text allows a built-in demo mode "in lieu of a demo account" with Apple's prior approval; there's no account here at all, so say that plainly and ask in the notes for the demo to be used. Add the video. |
| **4.1 Copycats** (4.1(c)) and **5.2.1 Intellectual Property** | Star Citizen and its ship makers' names are trademarks of Cloud Imperium Games. Using them in the name, icon or screenshots without approval is a classic rejection, and can bring a takedown. | Keep the game's name, logos, ship art and fonts out of the app name, subtitle, keywords, icon and screenshots. The icon is SpoolUp's own (a tablet with HUD keys). Put the fan disclaimer in the app (About) and the description. The bundled deck still says "Star Citizen" in some text and its themes carry ship makers' names (RSI, Drake, Anvil …): pick a neutral theme for screenshots, and check CIG's published rules for fan creations; written permission from CIG is the strongest answer if App Review asks for "authorization upon request". If Review objects to the names inside the app, a build setting could hide the maker names in the bundled demo. |
| **5.2.2 Third-party services** | The app controls a third party's game. | It sends key presses to the player's own PC, one per tap, like a keyboard; say so in the review notes if asked. |
| **2.3.7 Metadata** | Game names in keywords or subtitle. | None in name, subtitle or keywords. |
| **5.1.1 Privacy / permission strings** | Vague purpose strings get rejected. | `NSLocalNetworkUsageDescription` and `NSCameraUsageDescription` say exactly what each is for, translated in `InfoPlist.strings`. The camera is asked for when the player taps Scan; Local Network is asked for when the list starts looking, with a clear message and Open Settings if it's off. |
| **Privacy manifest** | Apps using required-reason APIs without a manifest get an email and can be rejected. | `PrivacyInfo.xcprivacy` declares `UserDefaults` (reason `CA92.1`), `NSPrivacyTracking` false and no collected data types. |
| **ATS** | `NSAllowsArbitraryLoads` needs a justification. | Not used; the app sets `NSAllowsLocalNetworking`. |
| **4.2.7 Remote desktop clients** | Doesn't apply (the app doesn't mirror the PC's screen), but it points the same way: it connects to a PC the player owns, on the home network (`PCAddress`). | Mention it in the notes. |

## Not done yet

- Nothing has been compiled or run: there's no Mac here. Expect small compiler fixes.
- Discovery has been tried against the server with python-zeroconf on Linux, not yet
  with Bonjour on an iPhone or iPad.
- App Store screenshots, the review video and the privacy page on the website.
