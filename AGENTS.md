# Agent guidelines

Instructions for AI coding agents (and a quick start for human contributors)
working on SpoolUp. Read this first, then
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the details it points to.

## What the project is

SpoolUp turns a tablet or phone into a HUD-style control deck for Star
Citizen. The deck runs in the tablet's browser; every button press travels over
the home network to a small server on the gaming PC, which presses the real keys
with Windows' `SendInput`. It's a fan project, not affiliated with Cloud
Imperium Games.

| path | what it is |
|---|---|
| `server.js` | the server: serves the deck, pairs tablets, sends keys, runs voice, the layout editor's API |
| `gamelog.js` | follows the game's `Game.log` for the deck's battle alerts |
| `discovery.js` | answers mDNS for `_spoolup._tcp` so the apps find the PC (name, port, version) |
| `public/` | the deck, the layout editor (`editor.*`) and the showcase |
| `layouts/` | built-in layouts; `hud.json` is the source, most others are generated |
| `keybinds/` | the SpoolUp Keyboard profile (generated) |
| `data/` | Star Citizen 4.10's actions, default keys and action-map contexts, for the profile check and the merged profile |
| `native/` | C# sources: the app window, the key sender, the voice listener, the sound player |
| `tools/`, `scripts/` | generators, the installer build, release notes |
| `site/` | the website on GitHub Pages |
| `android/` | the Android app (Kotlin, Gradle): [docs/ANDROID.md](docs/ANDROID.md) |
| `ios/` | the iPhone and iPad app (SwiftUI, XcodeGen on a Mac): [docs/IOS.md](docs/IOS.md) |
| `docs/STREAMDECK-API.md` | the server's side of the Stream Deck plugin, a separate paid product built in a private repository |

## Ground rules

How the key path and the routes work. Don't change these without the owner's
agreement:

- **One press per tap.** A button presses its key or key combo once, through
  `tapButton()` in `server.js`. A control that is dragged or swiped sends one
  tap per step through the same path, and a swipe that pages the deck sends no
  key.
- **The focus check.** `onlyWhenGameFocused` is on by default, and every way of
  pressing keys goes through `gameFocused()` / `tapButton()` in `server.js`.
- **Releases always go through.** A key that went down comes up, also after
  focus is lost or a tablet disconnects.
- **Pairing on routes.** The WebSocket and every data route check the pairing
  code (`checkPairing()`), and the layout editor's routes also check
  `fromThisPC`. New routes do the same.

## Code conventions

- **Match the surrounding code:** its naming, comment density and idiom.
  Comments explain why, not what.
- **No runtime dependencies.** The server uses Node's built-in modules.
  `devDependencies` are for the installer build only. Running needs Node 18 or
  newer; the build needs 20 or newer.
- **C# must compile with the compiler that ships with Windows**
  (`csc.exe` from .NET Framework 4, i.e. C# 5): no newer language features, no
  NuGet packages.
- **Windows is the target** for everything that runs on the PC. The deck itself
  must keep working in Safari (iPad, iPhone) and Chrome (Android).
- **Translations:** new text on the deck or in the app window gets Spanish,
  French and German entries in `public/i18n.js` or `Lang` in
  `native/launcher.cs` (see [Translations](docs/DEVELOPMENT.md#translations)).
- **Design:** anything visual follows [docs/DESIGN.md](docs/DESIGN.md): theme
  tokens, button colors, the HUD style and the house colors for the editor,
  website and app window.
- **American spelling only**, in every English string, comment, doc and identifier:
  center, color, gray, favorite, behavior, organize, license (noun), canceled,
  labeled, toward, traveling. Data keys renamed for spelling keep reading the
  old one (`recentre`, `follows.centre`).

## Generated files

Never edit these by hand; change the source and rerun the tool:

| generated | from | rerun |
|---|---|---|
| `layouts/hud-phone.json` (a hidden copy of the grid layout) | `layouts/hud.json`, `tools/make-phone-layout.js` | `node tools/make-phone-layout.js` |
| starter and ship layouts and Dashboards (`combat*.json`, `mining-salvage*.json`, `hauling-exploration*.json`, `prospector*.json`, `vulture*.json`, `dashboards.json`) | `hud.json`, `tools/make-starter-layouts.js` | `node tools/make-starter-layouts.js` (after the phone layout) |
| `keybinds/TabletDeck-StarCitizen.xml` | every button's `scAction` and keys | `node tools/make-profile.js` |

A new `scAction` must be added to `ACTIONMAPS` in `tools/make-profile.js`, and
exist in that map in `data/sc-actions-4.10.json`, or the profile build stops. `layouts/hud.json` is a grid layout (2.1) and the
source of every button; `layouts/hud-classic.json` and
`layouts/hud-classic-phone.json` keep the 2.0 HUD as it was and are edited by
hand (see [Grid layouts](docs/DEVELOPMENT.md#grid-layouts)).

## Checking your work

There's no automated test suite, so check by hand before you call a change
done:

1. `node --check` every JavaScript file you changed.
2. `node tools/make-profile.js --check` (the installer build runs it too).
3. Rerun the generators if you touched a layout, make sure their output is
   committed, and run `node tools/check-grid.js`.
4. Run a dry-run server, which presses no keys:
   `node server.js --dry-run --port 8790`. Open the deck (at a phone and a
   tablet size; `&viewport=small|large` forces a grid layout's size class), or
   the showcase at
   `http://localhost:8790/showcase.html?t=<pairing code>`, which shows each
   press, its keys and whether the focus check would have blocked it.
5. For the website or the "Try it" page: `node scripts/build-site.js` and serve
   `_site/`. The deck's demo mode is `public/index.html?demo`.

Say what you couldn't check. Windows-only parts (the key sender, voice, the sound player, the app
window, the installer) can't be run on other systems; never report them as
tested if they weren't. The same goes for the apps: the Android app needs a
phone or tablet for discovery, the camera and haptics, and the iOS app needs a
Mac to build at all.

## Docs and releases

- **Every user-facing change** gets a line in the top section of
  `CHANGELOG.md`, and the README (and `site/index.html` if it lists the
  feature) is updated in the same change.
- **The version** lives in `package.json`. A release is a commit with the new
  version and its `CHANGELOG.md` section, then a `v<version>` tag; the Release
  workflow builds and publishes it. Don't tag or release without the owner.
- **No Stream Deck downloads.** SpoolUp for Stream Deck is sold on the Elgato
  Marketplace, whose price-match rule rules out free copies, so releases and
  the website don't ship Stream Deck files. The plugin still talks to
  `server.js`: keep [docs/STREAMDECK-API.md](docs/STREAMDECK-API.md) true, and
  don't rename button or panel ids in `layouts/hud.json` without saying so.
- **Pushing to `main` publishes the website** when it touches `site/`,
  `public/`, `layouts/` or the images in `docs/`.

## Git

- Short, imperative commit subjects ("Add a Mining panel"), with a body that
  explains what changed and why when it isn't obvious.
- Never force-push, rewrite history or delete branches or tags on GitHub
  without the owner's explicit OK.
- Keep unrelated changes in separate commits.

## Example values

Commits, docs, screenshots and the build use the README's examples, not values
from a real machine: `http://192.168.1.20:8765`, pairing code `1a2b3c4d`, and
made-up user names and paths. `config.json` (pairing code, ports, game paths)
is in `.gitignore`.
