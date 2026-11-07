# The Stream Deck connection

SpoolUp for Stream Deck (the plugin, icon pack and profiles) is a separate paid
product, sold on the [Elgato Marketplace](https://marketplace.elgato.com). Its
code lives in a private repository, `xattribution/SpoolUp-StreamDeck`. This page
describes SpoolUp's side of the connection: what `server.js` offers the plugin and
what the plugin's build reads from this repository. Keep it working, and keep the
two repositories in step when it changes.

The free Stream Deck pack that came with SpoolUp 2.0 and earlier talks to the
same interface, so changes here must keep it working too.

## Connecting

The plugin connects like a tablet, over the same WebSocket and through the same
pairing check:

```
ws://127.0.0.1:<port>/ws?t=<pairing code>&device=streamdeck&layout=layouts/hud.json
```

- **Port and pairing code:** the plugin reads `port` and `token` from
  `%APPDATA%\TabletDeck\config.json` (the installed app's settings) when it can,
  and otherwise uses the port and code the player set in its settings (host
  127.0.0.1, port 8765). Keep those two keys and that folder name.
- **Pairing:** `checkPairing()` treats the plugin like any other device: a wrong
  code gets `403`, too many wrong codes from one address get `429`.
- **`device=streamdeck`** only changes the log line ("Stream Deck connected") and
  which clients receive `panel` messages (below). Presses go through the same
  focus check, press scheduling and release rules as a tablet's.
- **`layout`** picks the layout whose buttons the plugin presses by id. The
  plugin's keys name buttons by their `id` in `layouts/hud.json`, so renaming or
  removing a button id there breaks the keys that use it.

## Messages

JSON text frames, the same ones the deck uses (`connect()` in `public/app.js`).

Plugin to server:

| message | what it does |
|---|---|
| `{t:"hb", n}` | heartbeat, every 1.5 s; the server answers `{t:"hb", n}` with the same `n`. A client that sends nothing for 5 s is dropped and its held keys are released |
| `{t:"tap", id}` | one press of a button |
| `{t:"down", id}`, `{t:"up", id}` | a hold button pressed and released. The plugin sends `up` for every `down`, also when a key disappears, the device disconnects or the PC wakes; the server releases everything a client holds when it disconnects |
| `{t:"voice", on}` | hold to talk |
| `{t:"reset-states"}` | clears the lit buttons on every client |
| `{t:"scene", scene, left, right}` | a scene picked on a Stream Deck (below) |

Server to plugin: `{t:"hello", voice, voiceStatus, states, soundStatus, tapMs}`
on connecting, `{t:"state", group, on}` and `{t:"states", states}` for lit
buttons, `{t:"blocked", id}` when a press was skipped because Star Citizen
wasn't in front, `{t:"unbound", id}` when it did nothing because the player
cleared that bind in Star Citizen (the button's `unbound: true` in `/layout.json`),
`{t:"released", id, reason}` when the server let go of a hold by itself
(`reason` is `"focus"`: Star Citizen lost focus, or `"limit"`: held 2 minutes; a
later `up` for it is ignored), `{t:"listening", on}`, `{t:"heard", id, label, text}`,
`{t:"unheard"}`, `{t:"voice-off", reason}` and `{t:"voice-status", ...}` for
voice, `{t:"reload"}` when the layouts changed, and `{t:"panel", side, panel}`
(below).

One press per tap holds for the plugin as for the deck: a key press is one `tap`,
and a dial notch is one `tap` (at most 5 per turn).

## Scenes both ways

Scenes are relayed in both directions, and neither direction answers the other,
so a tablet and a Stream Deck can't page each other back and forth.

- **Stream Deck to tablets:** when a device's scene changes and the plugin's
  **Tablets follow scenes** is on, it sends `{t:"scene", scene, left, right}`,
  where `left` and `right` are the ids of the panels that scene shows on a
  tablet. The server passes it to every other client. Tablets with **Follow
  Stream Deck scenes** on page their HUD halves to those panels (`showPanels` in
  `public/canvas.js`), skip panels their layout doesn't have, and send nothing
  back for it.
- **Tablets to Stream Decks:** a tablet sends `{t:"panel", side, panel}` when its
  player pages one HUD half (its arrows, a two-finger swipe), once paging has
  stopped for 400 ms and the half moved (not for a scene, a layout loading or a
  reconnect, nor from a grid layout). The server passes it to `device=streamdeck`
  clients. The plugin then takes
  the device to the scene with that panel, if exactly one scene has it.

The server checks both: `scene` and panel ids must match `^[a-z0-9-]{1,40}$`,
`side` must be `left` or `right`, and the server passes on just those fields.
The scene ids and the panel ids the plugin's scenes follow come from
`layouts/hud.json` and the phone layout, so a renamed panel there needs a change
in the plugin too.

## What the plugin's build reads

The private repository's build links in a SpoolUp checkout and reads:

- `public/themes.css` (the 12 themes' tokens, in the order of `THEMES` in
  `public/app.js`) and `public/icons.js` (`DECK_ICONS`) for the key art;
- `layouts/hud.json` for the buttons, their ids, keys, modes and `scAction`s;
- `keybinds/TabletDeck-StarCitizen.xml`, which the profiles ship beside them;
- `server.js`, which its tests start with `--dry-run`.

Changing the theme tokens, an icon's name or a button's id or mode reaches the
Stream Deck products only when they are rebuilt there.
