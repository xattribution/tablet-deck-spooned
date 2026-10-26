# Panels

A panel is a named set of controls and a layout that shows them. Every pane on the
deck is a panel: the built-in ship systems, Comms, panels you write, and panels an
agent writes over MCP. They are all the same JSON, so any of them can be exported,
shared and imported.

This guide is the full format. Agents get it from the `panel_guide` MCP tool or
`GET /api/v1/panels/guide`.

## The smallest panel

```json
{ "name": "Scanning", "controls": ["sc:v_toggle_scan_mode", "sc:v_scanning_trigger_scan"] }
```

That is enough. With no `layout`, the deck shows every control in a grid sized to
fill the pane. Save it and add it to a view (see "Getting it on the deck").

## Panel fields

| Field | Type | What it does |
|---|---|---|
| `id` | string | Lower case letters, digits, `.`, `-`, `_`; up to 48. Made from the name when left out. Saving the same id again replaces the panel. |
| `name` | string | Shown on the pane's plate. Up to 40 characters. Required. |
| `code` | string | 2 to 5 letters on the plate's right. Made from the name when left out. |
| `category` | string | The pane's color: `flight` `nav` `eng` `def` `combat` `ind` `crew` `ui` `emerg`. Default `ui`. |
| `size` | object | `{ "weight": 6, "min": 260 }`. Weight is its share of a column's height; min is its smallest height in px on a tablet. |
| `controls` | list | Controls that already exist, by id. See "Referring to controls". |
| `actions` | list | Controls this panel adds. See "Actions". |
| `layout` | object | How the controls are laid out. See "Layout". Leave it out for an auto grid. |

## Referring to controls

Anywhere a control is named (in `controls` or in a layout), use one of:

- **A game action**: `sc:<action name>`, such as `sc:v_toggle_landing_system`. Every
  action in Star Citizen's keybind list is a control, about 700 of them. The deck
  presses it with the player's own bind.
- **A curated control id**: `landing-gear`, `toggle-flight-ready`, `shields-front`.
- **An action this panel adds**: its `id`.

`find_controls` (MCP) or `GET /api/v1/catalog` lists them all with ids, labels and
keys. Game actions come back as `sc.<name>`; `sc:<name>` and `sc.<name>` both work.

**Unbound actions.** About 440 game actions have no default keyboard key. They still
show on the deck, dimmed with a BIND badge, and a press reports "not bound". Fix it
once on the PC: Keybinds › **Bind every action** writes a profile that keeps the
player's own binds and gives every unbound action a free key. `bind_all_actions`
(MCP) does the same. `GET /api/v1/keybinds/unbound` lists them.

## Actions

Actions add controls that don't exist yet: a key combo, a game action with special
behavior, or a hotkey for another app.

```json
{ "id": "discord-mute", "label": "Discord mute", "keys": "f13", "state": true, "global": true, "icon": "mic-off" }
```

| Field | What it does |
|---|---|
| `id` | Unique within the panel. If another panel already uses it, the deck names it `<panel id>.<id>`; layouts in this panel still use the short id. |
| `label` | Spoken and shown. Required. |
| `keys` | A combo: `ralt+k`, `lctrl+lshift+f5`, `np_add`, `f13`. Modifiers: `lalt ralt lctrl rctrl lshift rshift`. |
| `sc` | A game action instead of keys: `foip_pushtotalk`. Pressed with the player's bind. |
| `mode` | `tap` (default), `hold` (down while the finger is down), `timed` (held for `ms`). |
| `ms` | For `timed`: how long to hold, such as 650 for the game's "medium press". |
| `icon` | An icon name from the deck's set (see "Icons"). |
| `state` | The key lights up and stays lit; each press flips it. |
| `danger` | Needs a deliberate press. Use with the `guard` or `slide` widgets. |
| `global` | Sent to whatever window is in front, not only the game. For Discord, OBS, music. The game's focus check is skipped and a held key isn't let go when focus changes. |

A player can change any control's keys without editing the panel: PC › Comms (for
the Discord keys) or `set_control_keys` (MCP), which writes `rebinds` in config.json.

## Layout

A layout is a tree of nodes. Each node is an object with exactly one widget key, plus
the common options.

```json
{ "row": [ { "toggle": "lights", "size": "hero" }, { "key": "exit-seat" } ] }
```

### Boxes

| Node | What it does |
|---|---|
| `{ "row": [nodes] }` | Side by side. |
| `{ "col": [nodes] }` | Stacked. |
| `{ "flip": [nodes] }` | Stacked in a tall pane, side by side in a wide one. Use this at the top of most panels so one design fits a phone and a tablet. |

### Widgets

| Node | What it does |
|---|---|
| `{ "key": id }` | A key. Tap, or hold for `hold` controls. |
| `{ "toggle": id }` | A key with a lit bar that shows on and off. |
| `{ "lever": id }` | The landing gear lever: drag down or tap. |
| `{ "fader": { "up": id, "down": id, "max": id, "min": id, "label": "Weapons", "icon": "power-weapons", "cat": "combat", "horizontal": true } }` | A slider. Each detent dragged past is one press of up or down; `max` and `min` are end caps. Springs back to the middle because the game doesn't report levels. Only `up` or `down` is required. |
| `{ "pair": [id, id], "names": ["SCM", "NAV"] }` | Two keys as one switch. |
| `{ "dial": [ids], "center": id }` | A rotary selector; `center` is a key in the hub. |
| `{ "shieldpad": { "front": id, "back": id, "left": id, "right": id, "top": id, "bottom": id, "reset": id } }` | The ship from above with a face per side. Any face can be left out. |
| `{ "guard": id }` | A key under a striped cover: lift, then press. |
| `{ "slide": id }` | Slide the handle all the way to fire. For eject. |
| `{ "grid": [ids] }` | Keys in a grid that fits its box. `"grid": "rest"` puts every control the layout hasn't placed here. Add `"small": true` for many small keys. |
| `{ "modes": "store-key", "options": [ { "name": "Landing", "value": "landing", "layout": node }, … ] }` | A switch at the top that swaps the layout below. It changes the deck only, not the game; each tablet remembers its choice under `store-key`. |

### Comms widgets

| Node | What it does |
|---|---|
| `{ "talk": {} }` | The voice key, with a Tap · Hold · Live switch. Tap: talk until you pause. Hold: talk while held. Live: always listening, tap to turn on and off. |
| `{ "route": {} }` | Where voice goes: Commands (deck commands only), Auto (commands, and anything else to the agent) or the agent. |
| `{ "transcript": {} }` | The last lines said to and by the agent. Tap it to type to the agent. |
| `{ "radio": { "discord": id, "game": id } }` | Push to talk with a Discord · Game · Both switch. Hold to transmit; the deck plays a squelch at each end. |

### Common options

Any node can have:

| Option | What it does |
|---|---|
| `flex` | Its share of the space in its box. Default 1. |
| `label` | The text on a key, instead of the control's label. |
| `icon` | The icon on a key. |
| `size` | `"hero"` for the panel's main key, `"wide"` for a key with the label beside the icon. |
| `fx` | The animation on press: `flares` `noise` `missile` `scan` `laser` `lights` `doors` `shield` `lock` `power` `alarm` `quantum` `warp`. |
| `spool` | On a `hold` key: a ring fills while held (quantum spool). |
| `style` | `"hazard"` on a box: yellow-black stripes behind it. |

Every control in the panel always shows. Anything the layout doesn't place lands in
a grid at the end, or where the layout puts `"grid": "rest"`.

## Icons

`power` `power-weapons` `power-engines` `power-shields` `gear` `speed` `ship` `target`
`missile` `decoy` `noise` `shield` `shield-front` `shield-back` `shield-left`
`shield-right` `shield-top` `shield-bottom` `quantum-jump` `quantum-mode` `scanning`
`mining` `laser` `salvage` `module` `turret` `doors` `lock` `unlock` `lights`
`exit-seat` `warning` `eye` `camera` `walk` `emote` `chat` `party` `starmap`
`mobiglas` `inventory` `escape` `swap` `sliders` `chev-up` `chev-down` `mic` `mic-off`
`headset` `headset-off` `radio` `autoland` `docking` `esp` `gforce` `triangle`.

## Examples

### Scanning, with a hero key

```json
{
  "name": "Scanning",
  "category": "nav",
  "size": { "weight": 4, "min": 200 },
  "controls": ["sc:v_toggle_scan_mode", "sc:v_scanning_trigger_scan", "sc:v_inc_scan_focus_level", "sc:v_dec_scan_focus_level"],
  "layout": { "flip": [
    { "key": "sc:v_scanning_trigger_scan", "size": "hero", "fx": "scan", "label": "Ping" },
    { "col": [
      { "toggle": "sc:v_toggle_scan_mode", "label": "Scan mode" },
      { "fader": { "up": "sc:v_inc_scan_focus_level", "down": "sc:v_dec_scan_focus_level", "label": "Angle", "icon": "scanning", "horizontal": true } }
    ] }
  ] }
}
```

### Stream controls for OBS

```json
{
  "name": "Stream",
  "code": "OBS",
  "category": "crew",
  "actions": [
    { "id": "obs-record", "label": "Record", "keys": "rctrl+ralt+f9", "state": true, "global": true, "icon": "camera" },
    { "id": "obs-scene-1", "label": "Cockpit cam", "keys": "rctrl+ralt+f10", "global": true },
    { "id": "obs-scene-2", "label": "Chase cam", "keys": "rctrl+ralt+f11", "global": true },
    { "id": "obs-mute", "label": "Mute mic", "keys": "rctrl+ralt+f12", "state": true, "global": true, "icon": "mic-off" }
  ],
  "layout": { "col": [
    { "toggle": "obs-record", "size": "hero", "flex": 1.4 },
    { "row": [ { "key": "obs-scene-1" }, { "key": "obs-scene-2" }, { "toggle": "obs-mute" } ] }
  ] }
}
```

Bind the same keys in OBS (Settings › Hotkeys).

### The built-in Comms panel

```json
{
  "id": "comms", "name": "Comms", "category": "crew",
  "actions": [
    { "id": "discord-mute", "label": "Discord mute", "keys": "f13", "state": true, "global": true, "icon": "mic-off" },
    { "id": "discord-deafen", "label": "Discord deafen", "keys": "f14", "state": true, "global": true, "icon": "headset-off" },
    { "id": "discord-ptt", "label": "Discord push to talk", "keys": "f15", "mode": "hold", "global": true, "icon": "radio" },
    { "id": "voip-ptt", "label": "In-game push to talk", "sc": "foip_pushtotalk", "mode": "hold", "icon": "radio" },
    { "id": "voip-proximity", "label": "Proximity talk", "sc": "foip_pushtotalk_proximity", "mode": "hold", "icon": "mic" },
    { "id": "hail-target", "label": "Hail target", "sc": "v_target_hail", "icon": "chat" },
    { "id": "voip-channel", "label": "Next channel", "sc": "foip_cyclechannel", "icon": "swap" }
  ],
  "layout": { "flip": [
    { "col": [ { "talk": {}, "flex": 1.6 }, { "route": {}, "flex": 0.45 }, { "transcript": {}, "flex": 0.9 } ], "flex": 1.3 },
    { "col": [
      { "radio": { "discord": "discord-ptt", "game": "voip-ptt" }, "flex": 1.5 },
      { "row": [ { "toggle": "discord-mute" }, { "toggle": "discord-deafen" } ], "flex": 0.9 },
      { "row": [ { "key": "voip-proximity" }, { "key": "hail-target" }, { "key": "voip-channel" } ], "flex": 0.8 }
    ] }
  ] }
}
```

The built-in panels are in the repo's `panels` folder; read them for more patterns.

## Where panels come from

Later sources replace earlier ones with the same id.

1. **Built in**: the `panels` folder in the app.
2. **The panels folder**: drop `.json` files in the `panels` folder next to
   config.json (Windows: `%APPDATA%\TabletDeck\panels`). Each file is one panel or an
   export bundle. The deck picks up changes within a couple of seconds.
3. **Saved**: panels saved over MCP, the API, or by import, kept in state.json.

## Getting it on the deck

A panel shows when a view includes it. Either:

- **Show it now**: `show_view` with `groups: ["scanning"]`.
- **Add it to a view**: `save_view` with the view's groups plus the panel id.
- **On the tablet**: Systems › Find, pick the panel, add it to a view.

## Import and export

An export is one JSON file:

```json
{
  "format": "tabletdeck/1",
  "panels": [ … ],
  "views": [ … ],
  "layout": { "layout-tablet-flight": "…" }
}
```

`panels` and `views` are the player's own (built-in panels and preset views are left
out; every install has them). `layout` is how one tablet arranged its panes, written
by the deck and read back by a deck; the PC ignores it.

- **On the tablet**: Systems › Export downloads the file; Systems › Import reads one.
- **API**: `GET /api/v1/export` (add `?view=<id>` for one view and its panels),
  `POST /api/v1/import` with the file as the body. A single panel on its own also
  imports.
- **MCP**: `export_layout`, `import_layout`.

## API

All under `/api/v1/`, with the API key (PC panel › Agents and integrations).

| Route | What it does |
|---|---|
| `GET panels` | Every panel with its source. |
| `POST panels` | Save a panel (the body is the panel). |
| `GET panels/{id}` | One panel. |
| `PUT panels/{id}` | Save a panel with this id. |
| `DELETE panels/{id}` | Delete a saved panel. Built-in and folder panels stay. |
| `GET panels/guide` | This guide. |
| `GET catalog` | Every control and group. |
| `GET export`, `POST import` | See above. |
| `GET keybinds/unbound` | Game actions with no key. |
| `POST keybinds/bind-all` | Write the bind-everything profile (from this PC only). |
| `POST agent/ask` | `{ "text": "…" }` to the Comms agent (from this PC only). |

## MCP tools

| Tool | What it does |
|---|---|
| `panel_guide` | This guide. |
| `list_panels`, `get_panel` | Read panels. |
| `save_panel` | Create or replace a panel. Returns `missing` (ids that match no control) and `unbound` (controls with no key yet). |
| `delete_panel` | Delete a saved panel. |
| `find_controls` | Search controls by words; returns ids for panels and `press`. |
| `export_layout`, `import_layout` | The export file, as above. |
| `set_control_keys` | Change a control's keys (`{ "id": "discord-ptt", "keys": "f16" }`). |
| `bind_all_actions` | Bind every unbound game action. |
| `show_view`, `save_view` | Put panels on decks. |

### For agents: building a panel from a request

1. `find_controls` for each thing the player wants. Prefer curated ids; use
   `sc:<name>` for the rest.
2. Anything that isn't a game action (another app, a macro key the player set up)
   becomes an `action` with `keys`, and `global: true` if it's for another app.
3. Pick widgets by how the control is used: a main action is a `hero` key, on and off
   is a `toggle`, up and down pairs are a `fader`, a choice of modes is a `dial` or
   `pair`, anything dangerous is a `guard` or `slide`.
4. Wrap the layout in `flip` so it fits phones and tablets.
5. `save_panel`. Fix anything in `missing` and save again.
6. `show_view` with the panel id. Tell the player about anything in `unbound`.

## The agent

The Comms panel talks to an agent through any OpenAI-compatible API. Set it on the
PC under Comms: the API base (xAI, OpenAI, OpenRouter, or a local Ollama or LM Studio),
key, model, name and instructions. The key stays in config.json on the PC.

- **Fast**: short spoken answers and a few tools (press, find, views, release, AFK,
  crew messages), one step.
- **Crew**: every deck tool, the same ones MCP agents get, over several steps. It can
  build panels and views for you.

Replies are spoken by the deck's voice and shown in the transcript. It never presses
self-destruct or eject unless asked in that message.
