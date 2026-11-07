# Developing SpoolUp

Everything for changing SpoolUp itself: running it from source, editing
layouts by hand, the keybind profile, building the installer, releasing, code
signing, the website and the demo recording. For using SpoolUp, see the
[README](../README.md).

- [Run from source](#run-from-source)
- [Test without the game](#test-without-the-game)
- [Project layout](#project-layout)
- [Editing layouts by hand](#editing-layouts-by-hand)
- [The keybind profile](#the-keybind-profile)
- [The Stream Deck connection](#the-stream-deck-connection)
- [Building the installer](#building-the-installer)
- [Releasing](#releasing)
- [Code signing](#code-signing)
- [The website](#the-website)
- [Recording the demo](#recording-the-demo)

## Run from source

You need Windows 10 or 11, [Node.js](https://nodejs.org) 18 or newer, and a
tablet on the same network. No `npm install` is needed to run it: the server
uses only Node's built-in modules. The key sender (`native/keysender.cs`), the
voice listener (`native/voice.cs`) and the sound player (`native/sound.cs`) compile
themselves on first run with the C# compiler that ships with Windows. The sound
player only starts once a tablet with sounds on opens Deck settings or plays its
sounds on the PC.

1. Clone the repository (or **Code → Download ZIP** and extract it).
2. Double-click `start.bat`, or run `node server.js`.
3. When Windows Firewall asks whether Node.js may use the network, allow
   **Private networks**.
4. Pair the tablet with the address, pairing code or QR code the console shows.

Running from source shows everything in the console instead of the app window,
and keeps `config.json` next to `server.js` instead of in `%APPDATA%`. The
console also prints a link to the layout editor.

Options (they work with `start.bat`, `node server.js` and the installed
`TabletDeck.exe`):

| option | does |
|---|---|
| `--dry-run` | log button presses instead of sending keys; skips the focus check |
| `--port <n>` | use another port than the one in `config.json` |
| `--ui` | status lines for the app window (used by `TabletDeck.exe`) |
| `--set name=value ...` | save settings to `config.json` and exit |
| `--find-game` | print the Star Citizen folder as JSON and exit (see [The Star Citizen folder](#the-star-citizen-folder)) |
| `--merge-profile <out.xml> [actionmaps.xml]` | write the merged keybind profile and exit (see [Your binds kept](#your-binds-kept)) |
| `--game-log <file>` | follow that file for battle alerts instead of the game's `Game.log` (see [Battle alerts](#battle-alerts)) |

## Test without the game

### Showcase / sandbox

To show or test the deck without a phone, and without pressing keys in the
game, run a dry-run copy on another port and open the showcase page on the PC:

```
node server.js --dry-run --port 8790
```

Then open `http://localhost:8790/showcase.html?t=<pairing code>`. It shows the
deck inside an iPhone 15 Pro frame (852×393, landscape) or, with the toggle, an
iPad frame (1024×768). Clicks work as taps. The phone frame passes the iPhone's
safe-area insets (`&safe=0,59,21,59`) so the deck stays clear of the Dynamic
Island and home indicator as on the real phone.

Next to the device, a live log shows each button's label, its keys, and how it
was pressed (tap, long press, hold and release, or voice), voice results, and
whether the focus check would have blocked the press. Mode switches and panel
arrows appear as "screen only", since they send nothing to the PC. Button sounds
from a tablet set to play them on the PC show as "PC sound", with the output; a
dry run plays them too, so you can try an output without the game. The log comes
from `/events`, which only exists in `--dry-run` and needs the pairing code.
Against a normal server the page warns that taps send real keys.

### Battle alerts

`gamelog.js` follows the game's `Game.log` (it opens the file, reads the new
whole lines and closes it again every 250 ms) in the running channel's folder,
else LIVE's, from the same `findInstalls()` as the keybinds, and the server
sends paired tablets
`{ t: "battle", event, state }`. `event` is `sync` (on connect and after the log
is first read), `ship`, `hit`, `clear`, `ship-1` (disabled), `ship-2`
(destroyed), `dead`, `kill` or `reset` (a new game session); `state` is
`{ ship, fire, from, hits, level, dead, by, kills, killed, recent: [{ from, n, at }], now, reading }`.
Stream Decks (`device=streamdeck`) don't get it, and `state` leaves out the
player's own name. A log that's already there when the server starts is read
silently, so old hits don't show as current. The deck shows it in `public/battle.js`
(the strip at the top and the `lasthits` widget). `"gameLog": false` in
`config.json` turns it off.

To try it without the game, follow a sample file and append lines to it (the
line formats are in `gamelog.js`); the dry run logs each event:

```
node server.js --dry-run --port 8790 --game-log sample-Game.log
```

### The apps' discovery

`discovery.js` answers DNS-SD over multicast DNS (224.0.0.251, UDP 5353) for
`_spoolup._tcp`, so the Android and iOS apps can list the PC: PTR (the PC's
name), SRV (the deck's port, target `spoolup-<pc name>.local`), TXT (`v=<version>`,
`name=<pc name>`) and A (the PC's address on the adapter the question came in
on). VPN and virtual adapters (`VIRTUAL_ADAPTER` in `server.js`) are skipped,
questions from outside the PC's own networks get no answer, and answers are rate
limited. It announces at start and says goodbye
(TTL 0) on quit. If UDP 5353 is taken it logs "Discovery is off" and the rest
of the server runs as usual. `"discovery": false` (Settings → Network in the window) turns it
off. To see it from another machine: `avahi-browse -r _spoolup._tcp` or
python-zeroconf.

### Demo mode

`public/index.html?demo` runs the deck with no server at all: it loads the
layout from `layouts/`, and presses only play their animations. The website's
"Try it" page uses it (see [The website](#the-website)).

## Project layout

| path | what it is |
|---|---|
| `server.js` | the server: serves the deck, pairs tablets, sends keys, runs voice, the layout editor's API |
| `qr.js` | QR code generator for the console and the app window |
| `gamelog.js` | follows Star Citizen's `Game.log` for battle alerts |
| `discovery.js` | the mDNS answers the SpoolUp apps find the PC by |
| `public/` | the deck (`index.html`, `app.js`, `canvas.js`, `grid.js`, the widgets in `dial.js`, `clusters.js`, `emotes.js` and `battle.js`, `i18n.js`, themes, icons, animations, `sound.js`), the layout editor (`editor.*`) and the showcase |
| `layouts/` | built-in layouts; `hud.json` is the source, `hud-phone.json` and the starter layouts are generated, `hud-classic*.json` keep the 2.0 HUD as it was |
| `keybinds/` | the SpoolUp Keyboard profile for Star Citizen (generated) |
| `data/` | Star Citizen 4.10's actions and default keys (`sc-actions-4.10.json`) and which action maps are active together (`sc-contexts.json`), for the profile check and the merged profile |
| `native/` | C# sources: `launcher.cs` (the app window), `keysender.cs`, `voice.cs`, `sound.cs` |
| `tools/` | generators for the phone layout, starter layouts and keybind profile, the grid layout check (`check-grid.js`) and the demo recorder |
| `scripts/` | the installer build and release notes |
| `installer/` | the Inno Setup script |
| `site/` | the website (GitHub Pages) |
| `android/` | the Android app (Kotlin): [ANDROID.md](ANDROID.md) |
| `ios/` | the iPhone and iPad app (SwiftUI, XcodeGen): [IOS.md](IOS.md) |

## Editing layouts by hand

The layout editor covers most changes (see the README). For anything else, edit
the JSON. With the installed app, put your layout in `%APPDATA%\TabletDeck\layouts`:
a file there with the same name as a built-in one (e.g. `hud.json`) replaces it,
and one with a new name needs `layout` set in `config.json`.

There are three kinds of layout:

- **Grid layouts** (2.1; `hud.json`, the starters, Dashboards) have a `grid`
  object. Every panel is an arrangement of buttons, controls and widgets on a grid
  of cells that follows the screen, so one file fits phones and tablets. See
  [Grid layouts](#grid-layouts) below.
- **Canvas layouts** (2.0; `hud-classic.json`, `hud-classic-phone.json`, players'
  own 2.0 layouts) have a `canvas` size and place each element at `x`, `y`, `w`,
  `h` in canvas pixels; elements in a panel are relative to their half. They draw
  exactly as in 2.0. The layout editor can convert one into a new grid layout.
- `star-citizen.json` is an older tabbed grid layout (`pages`).

Element types include `button`, `header`, `switch` (with `when` to show elements
per mode), `menu`, `slider`, `grid`, the cockpit controls below and decorations.

Cockpit controls (2.0) hold ordinary button definitions inside them, so the
server, voice commands, the keybind profile and the Stream Deck plugin find
those buttons as usual. Each has its own `id` (no `keys`) so the layout editor
can select it. Every step, pull or turn sends one `tap` of one inner button, through
the same focus check and press scheduling as a tapped button:

| type | fields | what a gesture presses |
|---|---|---|
| `lever` | `label`, `button` | pulling the knob down or pushing it up past halfway taps `button` once; its lamps follow `button.state` |
| `level` | `label`, `icon`, `color`, `steps` (each way, default 5), `up`, `down`, `follows` | each step up or down taps `up` or `down` once, at most 5 per drag and 140 ms apart; `follows` maps `max`, `min`, `center` (2.0 files say `centre`, still read) to button ids whose presses move it |
| `fader` | the same as `level` | the same, lying down |
| `rotary` | `label`, `hub`, `options` | turning the needle to an option and letting go, or tapping it, taps that option once; `hub` is a button in the middle |
| `recenter` | `label` | nothing: puts every `level` and `fader` back in the center (`recentre`, its 2.0 name, still works) |

Levels and faders show the steps sent from the center, not a game value, and keep
them per device with the page positions.

### Grid layouts

A grid layout (SpoolUp 2.1) has a `grid` object and no `canvas` or `regions`;
`chrome` and `panels` keep their names, so the server, voice, the keybind
profile and the Stream Deck plugin find every button as before:

```jsonc
{
  "name": "SpoolUp HUD",
  "grid": {
    "version": 3,
    "density": { "small": "default", "large": "default" },  // compact | default | roomy
    "start": { "small": "full", "large": "halves" },          // optional; these are the defaults
    "halves": { "left": ["power-landing", "..."], "right": ["qt-systems", "..."] },
    "full": ["pilot", "combat-dash"],                         // dashboards, first in the one-panel view
    "primary": "pilot"                                        // optional; a gesture can jump to it
  },
  "chrome": [ { "type": "action", "action": "menu", "icon": "emblem", "bar": "left" }, "..." ],
  "panels": {
    "shields": { "title": "Shields", "cols": 6, "elements": [
      { "type": "shields", "id": "sh-cluster", "span": [6, 6], "size": { "small": [6, 5] }, "...": "..." }
    ] },
    "pilot": { "title": "Pilot", "cols": 12, "wide": true, "elements": [
      { "type": "ref", "id": "pilot-sh-cluster", "ref": "sh-cluster", "span": [4, 3] }
    ] }
  }
}
```

- **Screens.** The deck has two size classes: `small` when the stage's shorter
  side is under 500 px, else `large` (`?viewport=small|large` forces one). Small
  screens open on one panel, large ones on two halves; the pager's view button
  switches, a phone held upright always shows one. Each class has three
  densities (the cell width it aims for); players pick one in Deck settings
  (Button size), else the layout's `grid.density` applies.
- **Panels.** A half panel is authored on 6 columns (`cols`), a dashboard on 12
  with `wide: true` (shown only in the one-panel view). The one-panel view pages
  through `grid.full`, then the left half's panels, then the right's.
- **Items.** Every element needs a unique `id`. `span` is `[columns, rows]` in
  the panel's authored columns (buttons default to `[2, 1]`, widgets to their
  registered size); `size: { "small": [w, h], "large": [w, h] }` wins in that
  class; `at: [col, row]` pins it to a cell; `only: "small"` or `only: "large"`
  shows it on that class alone (the dashboards keep three or four clusters on a
  phone, so they fit without scrolling); `when` works as in 2.0, and hidden
  items are dropped before placement so the rest close up. `x`, `y`, `w`, `h`
  are ignored.
- **Placement** (`DeckGrid.place` in `public/grid.js`): the deck works out how
  many columns fit (an even number, 4 to 12), scales spans down when there are
  fewer columns than authored (never below an item's minimum width: a group
  keeps its own columns), never stretches them when there are more, places pins
  first and then packs everything else in order, first fit. Rows fill the panel
  between the class's smallest and tallest cell; a panel that needs more scrolls
  up and down. Height left over under a panel (a phone held upright) goes to
  the rows that hold clusters alone, so a cluster grows and buttons keep their
  height. Each cell becomes an ordinary box, so 2.0's controls work in it.
- **Labels**: `"labels": "show"` or `"hide"` on a panel, an item (a cluster,
  control, group or ref) or a button overrides Deck settings' Button labels
  (Icons and labels, or Icons only, per device, kept as `labels` in the
  deck's storage); the most specific setting wins (button, then the ref, then
  the item it shows, then the group, then the panel). A hidden label stays
  the button's `aria-label`, title and voice phrase, and a button without an
  `icon` always shows its label. Widgets get the setting as `ctx.labels()`;
  their buttons from `ctx.hudButton` follow it on their own.
- **`group`**: `{ "type": "group", "id", "label", "title", "style": "plain" |
  "danger", "cols": 6, "span", "elements": [...] }`, a framed sub-panel laid out
  on its own columns by the same rules and scaled to its box, so it keeps its
  arrangement. `title` is drawn above it on the deck; `label` only names it in
  the layout editor.
- **`ref`**: `{ "type": "ref", "id", "ref": "<element id>", "span" }` draws that
  element (a cluster, group or control anywhere in `panels`) here too, with the
  same buttons and ids, so nothing is defined twice; both copies share its
  per-device memory.

Per-device state (page positions, the view, the seam, slider steps, dials,
widget memory) is kept as `deck-grid-small` and `deck-grid-large`; on first use
each starts from the 2.0 store of the same screen, which is never removed.

**Widgets** are element types that scripts register, drawn in a cell like any
other control: the mode dial (`public/dial.js`), the cockpit clusters
(`public/clusters.js`) and the emote board (`public/emotes.js`). A registered
type wins over the built-in renderer (the new dial replaces 2.0's `rotary` in
every layout); a type nothing draws shows its buttons as a plain grid.

```js
window.DeckWidgets.register("shields", {
  label: "Shields", category: "shields", icon: "power-shields",  // for the layout editor's library
  spans: { small: [6, 4], large: [6, 6] }, min: [3, 3], max: [12, 8],
  drag: "xy",                       // "x", "y", "xy" or "": what a drag on it claims
  grows: true,                      // false for a row of plain buttons: its rows don't stretch tall
  buttons: (e) => [["front", e.faces.front], "..."],  // its parts, for the editor
  render(e, ctx) { return root; },  // the deck positions the root in its cell
});
```

The built-in widgets and the buttons each one takes (every value is an ordinary
button object, with its own unique `id`, `keys` and `scAction`):

| `type` | fields | gestures (one tap each) |
|---|---|---|
| `rotary` | `hub`, `options` (each a button with `label`), `label` | tap a chip, or drag on the face and let go; the hub taps `hub`. Its shape comes from the maker's `--dial-shape` |
| `shields` | `faces: { front, back, left, right, top, bottom }`, `reset`, `power` | slide toward a face: one tap per 36 px (28 on small screens), at most 5 per slide; tap a face, the ship (`reset`) or the ring (`power`) |
| `targeting` | `next`, `prev`, `corners: { attacker, friendly, any, view }`, `sub` | the four cycles and Prev / Next Hostile are buttons around the reticle; tap its center or swipe right across it for `next`, swipe left for `prev` (once per swipe), tap the inner ring for `sub` |
| `hull` | `lights`, `doors: { toggle, open, close }`, `lock: { lock, unlock }` | tap Lights, Lock or Unlock, tap the drawn doors to toggle them, or tap Open or Close |
| `turrets` | `seats` (a list) | tap a seat's button (T1, T2, ...) |
| `countermeasures` | `decoy`, `noise`, `panic`, `missile`, `fewer`, `pips`, optional `more` and `reset` (beside Fewer) | `panic` sits under a cover between Decoy and Noise: the first tap lifts it and presses nothing |
| `quantum` | `engage` (a hold button), optional `ready` | hold Engage; the HUD keeps Flight Ready with the power switches (`qt-boot`) |
| `mining` | `mode`, `fire`, `laserType`, `up`, `down`, `modules` (a list) | taps; `up` and `down` press as their buttons say |
| `salvage` | `left`, `right`, `wider`, `narrower` | swipe sideways for modifier left or right; drag up or down for spacing (one tap per 32 px, at most 5); tap a chevron |
| `switchbank` | `buttons` (a list) | tap a switch |
| `emotes` | `buttons` (the emotes) | tap plays one; hold stars it and presses nothing; favorites and groups are kept per device in `spoolup-emotes` |

`ctx` gives `D` (the deck: `tapStep`, `bindControl`, `stateOf`, `buzz`, `toast`,
`deckIcon`, `canPress`, `store`), `tr`, `make`, `text`, `keyText`,
`hudButton(def, style)` (a real bound button), `size` (the box in px),
`sizeClass`, `scale()`, `editing` and `place(el, e)` (the layout editor),
`mem(e)` and `save()` (per-device memory by element id), `follow(buttonId, fn)`,
`onRecenter(fn)`, `swiping()`, `steps` (`PER_DRAG` 5, `GAP_MS` 140), 2.0's
`stepper`, `ticks` and `showLevel`, and `renderChild(e, box)` for nested items.
Every press goes through `hudButton` or one `D.tapStep(button)` per gesture
step. A widget ignores a `pointerdown` while `ctx.swiping()` is true, and lets go
on `pointercancel` (the deck sends one when two fingers start a swipe); a tap on
lift checks `ctx.swiping()` and presses nothing then.

**Swipes and gestures.** Two fingers swiping sideways page the half under them
(or the one panel), on by default for grid layouts; one finger swiping sideways
anywhere is an option in Deck settings, off by default, and never starts on an
element marked `data-drag` (sliders, the lever, dials, clusters that are
dragged). While swipes are on a press waits 120 ms: a second finger landing, or
with one-finger swipes the finger setting off sideways, cancels it, so a swipe
never presses a key; a finger that lifts first is still one press, and a finger
already holding a button (hold to talk) isn't part of a swipe. Two fingers up or
down run what the device picked in Deck settings: one tap of `top-mobiglas`, the
Quick actions sheet, the primary panel or a chosen panel.

Check the built-in grid layouts after changing them (unique ids, refs, no
overlaps at five screen sizes, every type drawn):

```
node tools/check-grid.js
```

Button fields:

| field | meaning |
|---|---|
| `id` | unique name for the button |
| `label` | text on the button; also its voice phrase and its name for screen readers |
| `short` | optional shorter text for the button's face (`"Flight Ready"`); `label` stays its name and voice phrase |
| `labels` | `"show"` or `"hide"`: this button's label regardless of Deck settings (see Labels above) |
| `keys` | Star Citizen key names joined with `+`: `r`, `lalt+n`, `np_1`, `f5`, `ralt+y` |
| `mode` | `tap` (quick press), `hold` (held while your finger is down), `timed` (held for `ms`) |
| `color` | `green`, `amber`, `red`, `blue` or blank (uses the theme's accent) |
| `icon` | icon name from `public/icons.js`; defaults to the button `id` |
| `anim` | press animation: `flash`, `ripple`, `gear`, `charge`, `burst`, `glow`, `recoil`, `streak`, `doors`, `lock`, `hazard`, `timed` |
| `voice` | extra voice phrases, or `false` to leave the button out of voice commands |
| `scAction` | Star Citizen action name; if a keybind file in `actionmaps` binds it, that key wins (modifiers are pressed first, whatever order the file writes them in); if the player cleared it there, the button presses nothing and shows a BIND badge |
| `state` | makes the button light up while something is on: `{ "group": "doors" }` flips the group on each press, `"set": true` / `false` turns it on or off (Open/Close Doors), and `"on"` replaces the word shown while lit (default `ON`). Buttons in the same group share the state; the server keeps it (see "Lit buttons" in the README) |

Key names: `a`-`z`, `0`-`9`, `f1`-`f24`, `np_0`-`np_9`, `np_period`, `np_add`,
`np_subtract`, `np_multiply`, `np_divide`, `np_enter`, `lalt`, `ralt`, `lctrl`,
`rctrl`, `lshift`, `rshift`, `space`, `tab`, `enter`, `escape`, `backspace`,
`insert`, `delete`, `home`, `end`, `pgup`, `pgdn`, `up`, `down`, `left`, `right`.

### Generated layouts

The starter and ship layouts and Dashboards are generated from
`layouts/hud.json`, so keys and animations match. Ship layouts copy the HUD's
cluster and buttons into their own panel under new ids (`pr-…`, `vu-…`); to add
a ship, add it to `SHIPS` in `tools/make-starter-layouts.js`. After editing it:

```
node tools/make-phone-layout.js
node tools/make-starter-layouts.js
node tools/check-grid.js
```

Since 2.1 the grid layouts fit every screen, so `hud-phone.json` and each
`*-phone.json` are the same layouts marked `"hidden": true`: settings that name
them keep working, and the layout lists show each once. The page still asks the
server for `device=phone` on screens whose short side is under 500 points, for
2.0 canvas layouts; add `&device=tablet` or `&device=phone` to override that, or
`&viewport=small` / `&viewport=large` to pick a grid layout's size class.

### Themes

Themes are CSS variables in `public/themes.css`, and each maker's own drawing
of the HUD (plates, button materials, widgets, lit looks) is in
`public/makers.css`. Add one by copying a block in each and adding an entry to
`THEMES` in `public/app.js`; [DESIGN.md](DESIGN.md#adding-a-theme) has the
steps. Theme fonts come from Google Fonts.

## Translations

Text is written in English and translated where it's shown, so layouts, keys
and voice phrases never change with the language.

- **The deck:** `public/i18n.js` maps each English text to
  `[Español, Français, Deutsch]`. `t()` translates; `canvas.js`, `fx.js` and
  `app.js` call it for what they draw, and `apply()` translates the fixed text
  in `index.html`. Text without an entry shows in English, which is how players'
  own labels stay as they wrote them.
- **The app window:** `Lang` in `native/launcher.cs` does the same. Buttons,
  switches, links and panel titles translate when they're drawn; other text goes
  through `Lang.T()` or `Lang.F()` (with `{0}` placeholders). Status lines that
  stay on screen are kept as functions, so they follow a language change at once.

When you add a label, button, panel or message, add its translations too. To
check a layout's coverage, every `label`, `title` and `text` in `layouts/`
should have an entry (names like Prospector or mobiGlas stay English). To add a
language, add a column to both tables and an entry to `LANGUAGES` in
`i18n.js` and `Choices` in `launcher.cs`; texts must still fit their buttons,
so check the deck (`?demo`) and the window in the new language.

## The keybind profile

`keybinds/TabletDeck-StarCitizen.xml` is generated from the layouts. After you
add or change a button that has an `scAction`, rebuild it, then copy it into the
game and load it again:

```
node tools/make-profile.js
```

Every `scAction` must be listed with its Star Citizen actionmap in `ACTIONMAPS`
at the top of `tools/make-profile.js`. The script stops if an action is missing
there, if two buttons give the same action different keys, or if two actions
that are active at the same time share a key. It also checks every action
against the game's own list, `data/sc-actions-4.10.json` (every action in Star
Citizen 4.10 with its map and default key, from the game's
`defaultProfile.xml`; it leaves out the menu map `default`): an action or map the
game doesn't have stops it, and a `DEFAULTS` key that isn't the game's default,
or a written key that is another action's default in the same context, is
printed as a warning. Which maps are active together is in
`data/sc-contexts.json`. `node tools/make-profile.js --check` runs the same
checks and reports whether the committed file is up to date (the installer
build runs it).

### The Star Citizen folder

`findInstalls()` in `server.js` lists the channel folders (`SC_CHANNELS`: LIVE,
PTU, EPTU, TECH-PREVIEW, HOTFIX), the same way as the Go rebuild's
`sc.Discover`. Automatically (`gameFolders` empty), the running
`StarCitizen.exe`'s channel comes first (from the key sender's `exe` answer, so
it's found in any folder), then every channel under these folders on each drive
from C to Z: `Program Files\Roberts Space Industries\StarCitizen`,
`Roberts Space Industries\StarCitizen`, `RSI\StarCitizen`,
`Games\Roberts Space Industries\StarCitizen`, `Games\StarCitizen` and
`StarCitizen`. The running channel ranks first, then the order of
`SC_CHANNELS`.

The app window's Settings → Star Citizen shows the copy in use (`gameInUse()`,
from the `ready` status line, or `--find-game` while the server is stopped) and
saves a picked folder as `gameFolders` (`--set gameFolders=<folder>` or
`set gameFolders <folder>` on stdin; `auto` empties it). With a folder picked,
`findInstalls()` lists the channels in it alone: the folder itself if it's a
channel folder, else the channels under it and under its `StarCitizen`
subfolder. The keybinds (`"auto"` in `actionmaps`), `--merge-profile`'s default
source, **Install keybind profile...** (which lists `--find-game`'s `installs`)
and `Game.log` all follow it. Decks get the copy in use as `game` in `hello`
and in `{ t: "game", game }` when it changes, and Deck settings shows it.

### Your binds kept

For a copy of the game with an `actionmaps.xml`, **Install keybind profile...**
writes `SpoolUp-YourBinds.xml` (in-game: **SpoolUp Keyboard (your binds)**)
instead of the plain profile. The window runs
`TabletDeckServer.exe --merge-profile <profile.xml> [actionmaps.xml]`
(`node server.js --merge-profile ...` from source), which copies the player's
whole default profile (keyboard, mouse, joystick and gamepad binds, device
options, binds they cleared) and adds a keyboard bind for each action in the
plain profile that the player has no keyboard bind for in that map. It keeps
the plain profile's key when nothing in the same context uses it (the player's
binds and the game's defaults), else it takes the first free right-hand combo
(RCtrl, RAlt, then with RShift). It keeps every bind the player has, and
writes the new file beside the profile, leaving `actionmaps.xml` as it is. The
first line of its output is
`added <n> moved <m>`, where moved counts binds that aren't the plain
profile's key.

A button that sends its action's own default key (Star Citizen 4.10's
`Data/Libs/Config/defaultProfile.xml`) also gets that key in `DEFAULTS`. The
profile writes nothing for those, and the game's own pairs on one key (F5 tapped
or held, N tapped or held) aren't counted as clashes. Give such buttons an
`scAction` anyway, so a player's own rebind reaches them.

Buttons marked `"verify": true` need a check in-game: the profile gives them a
keyboard key for an action whose game default is on the mouse (Engage Quantum,
Fire Laser, Fracture/Extract, Laser Power +, Speed Range + / −), and it isn't
known yet whether that key replaces the mouse bind, or (Launch Missile) the
held press hasn't been tried in the game.

## The Stream Deck connection

SpoolUp for Stream Deck (the plugin, icon pack and profiles) is a separate paid
product on the Elgato Marketplace, built in a private repository. SpoolUp's side
of it is the server's WebSocket: the plugin connects with `device=streamdeck`,
presses buttons by their id in `layouts/hud.json`, and relays scenes to and from
tablets. [STREAMDECK-API.md](STREAMDECK-API.md) describes the messages, the
pairing and the scene relay, and what the plugin's build reads from this
repository. Renaming a button or panel id in `layouts/hud.json` breaks the
Stream Deck keys and scenes that use it.

## The phone and tablet apps

`android/` and `ios/` are thin native apps around the same deck: they find the
PC (`discovery.js`), take its QR code or address (home network addresses),
keep the screen on and add haptics. The deck talks to them through
`window.SpoolUpApp`, which exists inside an app:

| call | does |
|---|---|
| `SpoolUpApp.haptic(kind)` | `tick`, `tap`, `press`, `heavy` or `error`; `buzz()` in `app.js` maps its vibration lengths onto these |
| `SpoolUpApp.switchPC()` | leaves the deck for the app's list of PCs (Deck settings → Switch PC) |

Inside an app, `awake.js` does nothing and Deck settings hides Keep screen on.
The pairing code goes into the deck's address (`?t=`) once; the deck keeps it
from there. Building, signing and the store steps: [ANDROID.md](ANDROID.md) and
[IOS.md](IOS.md). The Release workflow builds the signed APK when the
`ANDROID_*` secrets are set.

## Building the installer

```
npm install
npm run build
```

This makes `dist/SpoolUp-Setup-<version>.exe` (the version comes from
`package.json`). It needs [Inno Setup 6](https://jrsoftware.org/isinfo.php)
and Node.js 20 or newer. `scripts/build.js`:

1. checks the keybind profile is up to date;
2. makes the icon from `public/icon-512.png`;
3. compiles the app window (`native/launcher.cs` → `TabletDeck.exe`), the key
   sender, the voice listener and the sound player, with version info;
4. packs `server.js`, `qr.js`, `gamelog.js`, `discovery.js`, `public/`, `layouts/`, `keybinds/` and `data/` into
   `TabletDeckServer.exe` with Node's
   [single executable application](https://nodejs.org/api/single-executable-applications.html)
   feature, and writes `THIRD-PARTY-NOTICES.txt` with that Node.js's license (see
   [THIRD-PARTY-NOTICES.md](../THIRD-PARTY-NOTICES.md); set `NODE_LICENSE` if the
   build can't find Node's `LICENSE` file);
5. signs the files when `SIGN_CMD` is set;
6. runs Inno Setup on `installer/TabletDeck.iss` and writes the checksum.

The window runs the server hidden with `--ui` and reads its `@ui {json}` status
lines. `npm run build -- --app`, `-- --installer` and `-- --checksum` run the
stages separately, for signing services.

## Releasing

1. Set `version` in `package.json`.
2. Add a section for it at the top of `CHANGELOG.md`. It becomes the release
   notes, and the release stops without one.
3. Commit, then push a matching tag:

```
git tag v1.0.1
git push origin v1.0.1
```

The **Release** workflow (`.github/workflows/release.yml`) builds the installer
on GitHub and attaches it and its checksum to a new release. Installed copies
offer the update themselves. You can also run the workflow by hand
from the Actions tab to build without publishing. Run by hand with **Publish**
ticked, it publishes the version in `package.json` instead, tagging the commit
it ran on, for when pushing a tag isn't possible. It stops if that version is
already released.

## Code signing

Published releases are always signed: a run with **Publish** (or a version tag)
stops before building when no signing is set up. Test builds stay unsigned, so
Windows SmartScreen warns about an "unknown publisher" there, and antivirus tools
are warier of the key sender, which types keys into other programs. The workflow
signs `TabletDeck.exe`, `TabletDeckServer.exe`, the native helpers, the installer
and its uninstaller, in one of three ways:

- **Azure Artifact Signing** (option C in `release.yml`), the way Quartermaster's
  `tools/sign.sh` signs: [jsign](https://github.com/ebourg/jsign) 7.6, pinned by
  its SHA-256, against an Artifact Signing account, as an app registration with
  the Certificate Profile Signer role on the certificate profile, timestamped by
  `timestamp.acs.microsoft.com`. In the repository's **Settings → Secrets and
  variables → Actions**, add the secrets `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and
  `AZURE_CLIENT_SECRET` and the variables `TD_SIGN_ENDPOINT` (the account's
  endpoint without `https://`, such as `wus2.codesigning.azure.net`) and
  `TD_SIGN_ALIAS` (`<signing account>/<certificate profile>`): the same values as
  Quartermaster's `sign.env` (its `QM_SIGN_ENDPOINT` and `QM_SIGN_ALIAS`). Only the
  two build steps that sign see the secrets.
- **SignPath Foundation** (free for open source): set the `SIGNPATH_API_TOKEN`
  secret and the `SIGNPATH_ORGANIZATION_ID` variable. The build then runs in two
  stages so SignPath signs the app files before they go into the installer, and
  then the installer. The artifact configurations to paste into SignPath are in
  `.signpath/artifact-configurations/`. [CODE_SIGNING.md](../CODE_SIGNING.md) is
  the signing policy, and [docs/SIGNPATH.md](SIGNPATH.md) the setup steps.
- **Your own certificate:** a `.pfx` in the `SIGNING_CERT_PFX_BASE64` and
  `SIGNING_CERT_PASSWORD` secrets. It wins over option C when both are set.

## The website

`site/` is the website at <https://xattribution.github.io/SpoolUp/>. The
**Website** workflow (`.github/workflows/pages.yml`) publishes it to GitHub Pages
on every push to `main` that touches the site, the deck, the layouts or the
images. `scripts/build-site.js` puts it together in `_site/`: the page, the
images from `docs/`, and the deck (`public/`) with the layouts as `try/`, set to
demo mode, so the "Try it" page always runs the current deck.

To preview it, run `node scripts/build-site.js` and serve `_site/` with any
static file server (for example `npx serve _site`).

## Recording the demo

`docs/demo.png` (the animation at the top of the README) is recorded by
`tools/record-demo.js`: it opens the deck in Microsoft Edge or Google Chrome
without a window, taps through a short script with real mouse events, and saves
the result as an animated PNG. Run a dry-run server first so no keys are pressed:

```
node server.js --dry-run --port 8790
node tools/record-demo.js "http://localhost:8790/?t=<pairing code>"
```
