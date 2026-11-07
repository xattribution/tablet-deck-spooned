# Design

How SpoolUp looks, and the rules that keep the deck, the layout editor, the
website and the Windows app looking like one product. For running and building
the code, see [DEVELOPMENT.md](DEVELOPMENT.md).

- [Themes are different, not recolored](#themes-are-different-not-recolored)
- [Theme tokens](#theme-tokens)
- [Maker drawings](#maker-drawings)
- [Adding a theme](#adding-a-theme)
- [Button colors](#button-colors)
- [HUD style](#hud-style)
- [The other surfaces](#the-other-surfaces)
- [Motion, night mode and layering](#motion-night-mode-and-layering)
- [Accessibility](#accessibility)

## Themes are different, not recolored

The owner's rule, and the first test for any theme: **if it's different, be
different.** A manufacturer theme that is the same theme with another stripe of
color or another background shade is not a theme. Each one needs its own
character, the way the "Tablet Deck 2" design did it:

- **Solid, committed colors**, not one shared dark panel tinted per theme.
- **Its own shapes:** buttons, frames, corners, dividers and headers that only
  that manufacturer would draw (Drake's industrial plates, Origin's light
  luxury, Aegis's military hard edges, Banu's and Esperia's alien forms...).
- **Its own widgets:** how a dial, a gauge, a switch, a lit state or a state
  header looks, not only what color it is.
- **Its own type and motion**, where the manufacturer calls for it.

The tokens below carry the colors, type and material; `public/makers.css`
carries each maker's drawing (see [Maker drawings](#maker-drawings)). When a
theme needs something neither can say yet, add it (and its drawing in the deck,
and tell the Stream Deck key art, see [Stream Deck keys](#stream-deck-keys))
rather than settling for a recolor.

## Theme tokens

Every theme is a block of CSS custom properties in `public/themes.css`, keyed by
`[data-theme="<id>"]`. The deck's styles only read these variables, so a new
theme never needs changes anywhere else in the CSS. A theme sets **all** of the
tokens below, except the two marked optional.

| token | controls | example (RSI) |
|---|---|---|
| `--bg` | page background color | `#05090f` |
| `--bg-art` | backdrop drawn over `--bg`: glows, stripes, `var(--stars)` | radial glow + stars |
| `--panel` | button and panel fill | `#0b1420` |
| `--panel-hi` | raised or pressed fill | `#122236` |
| `--line` | borders and dividers | `#1c2c40` |
| `--line-hi` | highlighted borders | `#35598a` |
| `--text` | text on buttons and panels | `#e8f2fc` |
| `--dim` | secondary text on buttons: states ("OFF", "RETRACTED") | `#8ea4bc` |
| `--ink`, `--ink-dim` | text straight on the page and the HUD's plates: section titles, captions, the status line, the intro | `#e8f2fc`, `#8197b0` |
| `--face-hi`, `--face-lo` | a button face's top and bottom color (HUD buttons, Stream Deck keys) | `#1e2d42`, `#111b29` |
| `--material` | the maker's surface, one word that the Stream Deck key art draws (`glass`, `plate`, `carbon`, `armor`, `gloss`, `enamel`, `pearl`, `lacquer`, `shell`, `hazard`, `clay`, `silk`) | `glass` |
| `--plate`, `--plate-line` | the plate each half of the HUD sits on, and its border | translucent navy |
| `--accent` | the theme's signature color; buttons without a `color` | `#4fb3ff` |
| `--green`, `--amber`, `--red`, `--blue` | the four button colors (see [Button colors](#button-colors)) | `#46d79a` … |
| `--font-display` | headings, big labels | `"Exo 2", system-ui, sans-serif` |
| `--font-body` | button labels and everything else | same |
| `--label-case` | `text-transform` of labels | `uppercase` |
| `--label-track` | letter spacing of labels | `.06em` |
| `--label-weight` | font weight of labels | `600` |
| `--key-radius` | grid-layout button and Stream Deck key corner radius | `6px` |
| `--key-clip` | `clip-path` for cut corners on those, or `none` | `none` |
| `--btn-radius`, `--btn-clip` | the same for the HUD's (smaller) buttons | `5px`, `none` |
| `--key-edge` | width of the colored bar on a button's left edge (`0` for none) | `3px` |
| `--key-bg` | grid-layout button fill (usually a gradient) | `linear-gradient(...)` |
| `--key-top`, `--key-top-h` | highlight strip along a grid button's top, and its height | gradient, `2px` |
| `--key-shadow` | button shadow; optional | `0 8px 22px …` |
| `--icon-stroke` | stroke width of the line icons | `2.2` |
| `--icon-glow` | filter on icons: a `drop-shadow` glow, or `none` | `drop-shadow(...)` |
| `--tab-radius` | tab shape in the tabbed grid layout | `8px 8px 0 0` |
| `--label-shadow`, `--state-shadow` | shadow behind labels and state text; optional | dark blur by default |

`:root` also defines `--stars`, `--grid` and `--scratches`, shared backdrop
pieces that themes can use inside `--bg-art`, `--plate` and `--key-bg`.

**Light pages** (Origin, Crusader, Aopoa, Argo, Consolidated Outland) set
`color-scheme: light`, and their `--ink` is dark while `--text` stays whatever
reads on their buttons (Origin's are black, Crusader's white). A theme with
**light buttons** (Crusader) must also set `--label-shadow` and
`--state-shadow`: the default dark blur makes text look smudged on them;
Crusader uses `0 1px 0 rgba(255,255,255,.8)` and `none`.

## Maker drawings

`public/makers.css` has a section per maker that draws what tokens can't: the
plates' edges and marks (Drake's hazard strip and rivets, Anvil's screws,
CO's sunset stripe), the bezel over the whole HUD (`--bezel`, `--bezel-art`),
the section-title mark that replaces `//`, and each widget: the dial (Origin's
cream ball, Anvil's knurled knob, MISC's gauge), the framed buttons' shape, the
mode switch, the eject slider's knob, the danger zone, the pager.

Buttons are drawn from variables set on the button itself, which `.hud-btn` in
`hud.css` reads, so a maker's look keeps the button's color (`--c`) in every
state:

| variable | what it is |
|---|---|
| `--face` | the face's layers (gradient, texture) |
| `--mark` | layers drawn over it: rivets, tabs, sheen, a slot bar |
| `--edge`, `--bevel` | border color, and the box-shadow that gives it depth |
| `--lit`, `--lit-ring` | layers and shadow while lit: Drake's hazard band, Anvil's lamp, Origin's gold bar |
| `--press`, `--press-ring`, `--press-move` | the same while pressed, and how it moves (Drake's slab sinks, Banu's squashes) |
| `--deny` | the flash of a press the PC skipped (Drake's red hazard, Aegis's red scanlines) |
| `--veil` (on `#canvas`) | what covers the plates while the PC is offline |

The cockpit controls (2.0) are drawn the same way, with a section per maker:
the gear lever's slot, knob and lamps (`.hud-lever`, `.lv-*`), the level
sliders' and fader's track and knob (`.lvl-track`, `.lvl-knob`, `.hud-fader`),
and the mode dial's face, ticks, needle and hub (`.hud-rotary`, `.rt-*`). A
new maker draws all four, so each looks like that maker's own hardware
(Drake's riveted hazard lever, Origin's cream fader knob, Anvil's knurled dial).
Whatever a finger grabs stays at least 44 points, and the high-contrast block at
the end resets them like the buttons.

The cockpit clusters and the mode dial (2.1, `public/clusters.js`,
`public/dial.js`, `public/clusters.css`) are SVG drawn from a few tokens each
maker sets on `#canvas`, plus a `/* 2.1 clusters */` block per maker in
`makers.css` for their lines, fills and lit looks:

| token | what it picks | e.g. |
|---|---|---|
| `--dial-shape` | the mode dial's face and where its chips sit: `circle`, `hex`, `chevron`, `octagon`, `oval`, `rect`, `arc`, `bloom`, `crescent`, `triangle`, `sunrise`, `diamond` | RSI's instrument ring, Drake's hex nut, Aegis's V, Esperia's star |
| `--dial-chip-w` | a chip's width over its height: 1 is round, more is a pill or a module | Origin 1.45, MISC 1.25 |
| `--arc-style` | shield arcs, rings and reticles as smooth curves (`curve`) or straight facets (`facet`) | Anvil and Argo facet, Banu and Aopoa curve |
| `--arc-gap` | degrees between an arc's segments | 2 (Esperia) to 9 (Aegis) |
| `--cl-line`, `--cl-seg`, `--cl-cap` | line width, segment thickness and line ends | Origin's hairline, Anvil's heavy butt-capped plates |

Each cluster keeps one drawing everywhere and lets the maker change only these
and its colors, so a cluster reads the same in every theme while each maker
still draws its own (no two makers share a dial shape). The drawings:

- **Shields:** the ship from above in the middle, five segments on each face's
  arc (lit up to the steps sent), a side view for top and bottom, and an outer
  ring that is the power switch. No words.
- **Targeting:** a reticle (corner brackets, crosshair, a ring for
  sub-targets, the center pip for next) between the deck's own buttons: the
  four cycles, each with its own glyph, and Previous and Next Hostile as
  chevrons at its sides.
- **Lights and doors:** Lights, Lock and Unlock as buttons, and the doors
  drawn as a frame on a sill with two leaves that slide apart while open; the
  maker sets the frame and sill (Drake's hazard sill, Origin's gold hairline,
  Argo's yellow load frame).
- **Turrets:** plain buttons with the turret icon and T1, T2, T3.
- **Countermeasures:** two backing plates, defense (Decoy and Noise either
  side of Panic under a hazard-striped flip cover) over weapons (Launch
  Missile, then Fewer Missiles and the pips, each with its own glyph); each
  maker draws the plates its own way.
- **Quantum:** a spool ring that fills while Engage is held, with the drive's
  rails out to the sides on a wide box. Flight Ready sits with the power
  switches, a start-up control, not here.

With **Icons only** (Deck settings, or a layout's `labels`) a button shows its
icon alone: give buttons that would look alike a glyph or number of their own
(the targeting cycles, Fewer Missiles, turret seats, mining modules), and keep
words on a button with no icon.
- **Mining** and **salvage heads:** the head's art in the middle (the laser's
  power gauge and Fracture/Extract switch; the salvage beams that widen and
  narrow) with their buttons around it.
- **Switch banks:** rocker switches lit by their state.

Like the buttons, the art shows the steps SpoolUp sent, and its timers
animate the picture. The emote board's tiles are
the deck's own `s-tile` buttons, so each maker draws them; its tabs, sheet and
Arrange look use theme tokens only.

Keep it to gradients and borders: no filters or animations per frame, so the
deck stays smooth on an iPad. Marks go where the state text and key labels
aren't (a slot bar sits on the very bottom edge, under "RETRACTED"). High
contrast resets every maker at the end of the file; a new drawing with its own
colors needs a line there.

## Adding a theme

1. **Copy a block** in `public/themes.css`, give it a new id, and set every
   token in the table. Start from a theme with a similar feel: a dark one for a
   dark theme, Crusader or Origin for a light one. Then give it its own section
   in `public/makers.css`. A `--material` no existing theme uses also needs a
   drawing in the Stream Deck key art (see [Stream Deck keys](#stream-deck-keys)).
2. **Fonts:** if the theme uses a new Google font, add it to the Google Fonts
   link in both `public/index.html` and `public/editor.html`, and end the font
   stack with a system fallback (`system-ui, sans-serif`). The deck also has to
   look right with the fallback, since it works offline too.
3. **Register it** in `THEMES` at the top of `public/app.js`: `id`, `name`
   (short, for the picker), `full` (the manufacturer's full name), `tag` (a
   short tagline) and `emblem`.
4. **The emblem** is an original badge design on a 64×64 SVG grid that suggests
   the maker's character. Never copy or trace a real company logo. Strokes use
   `currentColor`; `${F}` marks solid accent fills and `T(...)` wraps parts drawn
   in the theme's text color. Keep a one-line comment describing the idea, as
   the existing emblems do.
5. **Check it:**
   - the theme picker card;
   - the deck on a tablet and a phone (the showcase in `docs/DEVELOPMENT.md` has
     both frames);
   - pressed buttons and every button color;
   - night mode, and the intro screen (it draws the emblem);
   - the text contrast rules under [Accessibility](#accessibility).
6. **Update the counts:** "12 manufacturer themes" appears in the README,
   `CHANGELOG.md`, the website (`site/index.html`) and the theme gallery image
   `docs/themes.jpg`.

## Button colors

Layouts give a button one of four colors, or none for the theme's accent. Use
them by meaning, so a color tells you what a button does in every theme:

| color | meaning | examples |
|---|---|---|
| `green` | power, systems on, flight | Flight mode, Master: SCM, shield power |
| `amber` | work modes and caution | Mining, Salvage, Fracture/Extract, G-Safety |
| `red` | weapons and danger | Guns, Missiles, Fire Laser, Exit Seat, Self-destruct |
| `blue` | navigation, scanning and info | Quantum, Scanning, mining modules |
| none | everything else | most buttons |

Use colors sparingly. A panel where every button is colored has no colors.

## HUD style

The deck should feel like a ship's display, not a web page:

- **Labels** are short and in Title Case in the layout JSON ("Landing Gear",
  "Engage Quantum"); themes decide upper or lower case through `--label-case`.
  Keep them to two or three words that fit a button on a phone.
- **State text** under a label is one short word or two in capitals: "OFF",
  "RETRACTED", "HOLD TO ENGAGE".
- **Section headers** start with `//` and use underscores between words:
  `// POWER_MANAGEMENT`, `// SHIP_SYSTEMS`.
- **Lines are thin** (1px), corners are bracketed (the intro screen, the
  banner, the website), and glow comes from a `drop-shadow` of the element's own
  color, not from a separate color.
- **Lit buttons** (a `state` that's on) use the "on" look from `fx.css`: the
  border and glow in the button's color, and a one-word status (ON, OPEN,
  DEPLOYED, LOCKED). In button grids the status sits top right, elsewhere under
  the label. Buttons that only switch something off never stay lit.
- **Key labels** (Deck settings, `data-keys` on `<html>`, styles in `hud.css`) put
  a button's key in a free corner in small capitals, in `--dim`: top right, under
  a top-right status line in tablet grids, bottom right in phone grids, top
  center on the framed hero and quantum buttons. A new button style needs a
  corner that's free in every theme, including the cut corners of `--key-clip`.
- **Icons** are line icons on a 48×48 grid in `public/icons.js`, drawn with
  `currentColor`, using the theme's `--icon-stroke`. Add new ones in the same
  style, not as images.

## The other surfaces

The layout editor, the showcase and the website don't switch themes. They use
the RSI look as the house style:

| role | color | used for |
|---|---|---|
| background | `#05090f` | page background |
| panel | `#0b1726` | cards, panels |
| panel, raised | `#12253b` | hover, selected |
| line | `#1f3a58` | borders |
| accent | `#4fb3ff` | links, buttons, focus, highlights |
| text | `#e8eef5` | main text |
| dim | `#8aa0b8` | secondary text |

Fonts: **Oxanium** for display text, **Exo 2** for everything else, with
`system-ui, sans-serif` as the fallback.

The website keeps these as tokens in `site/style.css` (`--bg`, `--panel`,
`--accent`, `--display`, `--body` …); reuse them there instead of new hex
values.

The Windows app window (`native/launcher.cs`, the colors in `Hud`) shares the
accent but has its own solid graphite palette, picked for reading at a glance
next to a game:

| role | color | used for |
|---|---|---|
| back | `#0e1116` | window background, fields inside panels |
| panel | `#161b22` | panels |
| panel, raised | `#1f2630` | buttons, hover |
| line | `#2d3643` | borders |
| ink | `#f2f5f8` | main text, headings |
| dim | `#aab4c0` | secondary text (7:1 on panel) |
| accent | `#4fb3ff` | primary buttons, links, switches, focus |
| good / warn / bad | `#3ddc84` / `#ffb020` / `#ff5a5f` | status only |

Text is Ink or Dim on a solid fill, never a status color and never over a
gradient. Status colors mark a dot, a pill or a banner fill, nothing else; the
Public-network banner is solid warn with dark text (`#1a1300`) and one dark
button, **Open network settings**, which opens Windows' page for that Wi-Fi or
Ethernet network. Spacing is 16 px inside and around panels and 12 px between
them; buttons are at least 32 px tall, one primary (solid accent, dark text)
per panel; keyboard focus is a solid 2 px accent ring.

Windows doesn't ship Oxanium or Exo 2, so the window's labels and headings use
**Bahnschrift** (SemiBold and Bold, never the light weights; Windows 10 and
later), falling back to Segoe UI; body text is Segoe UI at 13 px or more, and
the address and pairing code are Consolas Bold. The main window is drawn by
hand (`HudView`) onto a solid layer, so text gets ClearType. A thin band of
stars drifts in the title strip only, never behind text, and only while the
window is in front and Windows' "Show animations" is on. Its readouts show only
real data, never decoration made to look like data.

The app's other windows (Settings, Voice commands, Install keybind profile) use
the same panels, buttons and switches (`HudDialog`), without the stars. Fields
you type into or pick from stay real Windows controls, in the window's colors
inside a drawn frame that gets the accent focus ring while it has focus.

## Stream Deck keys

SpoolUp for Stream Deck, a separate paid product, draws its keys in its own
private repository (`xattribution/SpoolUp-StreamDeck`) as the deck's buttons
redrawn as 144×144 pictures. It reads every theme's tokens from
`public/themes.css` and the icons from `public/icons.js`, so a key looks like
the same button on the tablet: `--face-hi`, `--face-lo`, `--material` and
`--key-radius` matter there as well as on the deck. Keep every token in every
theme, and when a theme gets a new `--material` or drawing, say so in that
repository so its keys follow. See [STREAMDECK-API.md](STREAMDECK-API.md).

## Motion, night mode and layering

- **Keep motion short.** UI transitions (paging, sheets, toasts) take about
  200 ms. Longer effects are only for moments the player triggers: press
  animations, the quantum charge, the intro.
- **Every animation respects "reduce motion".** `fx.css`, `hud.css` and
  `intro.css` each turn their animations off under
  `@media (prefers-reduced-motion: reduce)`. A new animation needs the same.
  The app window follows Windows' "Show animations" setting instead.
- **The app window never animates behind the game.** Its stars move only while
  the window is in front; minimized, in the tray or behind Star Citizen, it
  draws nothing.
- **Sound** (Button sounds, on unless turned off) is made from tones, never from
  files. Each sound in `BANK` in `public/sound.js` is a list of tones; the
  tablet plays it with Web Audio, or sends it to the PC, where
  `native/sound.cs` synthesizes the same tones on the output the player picked.
  Keep the two renderers in step: a tone field means the same in both. Sounds
  are short and quiet, follow the same moments as the press animations, and use
  the button colors' meaning: a press's pitch drops from blue to green, the
  accent, amber and red. `fx.js` and `canvas.js` reach it through
  `window.DeckSound?.`, since the layout editor doesn't load it.
- **Night mode** is a black overlay (`#night`) over the deck. Anything that must
  stay readable at night goes above it.
- **Layering** (`z-index`), from bottom to top: deck content and its menus (up
  to 5), the night overlay (9), the theme picker (10), toasts (20), the intro
  screen (50). Put a new overlay in this order rather than picking a large
  number.

## Accessibility

- **Contrast:** `--text` on `--face-hi`, `--face-lo` and `--panel` should
  reach at least 4.5:1, and `--dim` on them at least 3:1; `--ink` on `--bg` at
  least 4.5:1 and `--ink-dim` at least 3:1, in every theme. All 12 themes do
  today (text 6.2:1 or more, dim 4.0:1, ink 8.6:1, ink-dim 4.5:1; MISC's
  lighter face and Origin's cream page are the lowest). Check a new theme with
  any contrast checker.
- **Color is never the only signal:** a button's label says what it does,
  and its state text says whether it's on. Color adds to that.
- **Touch targets:** the smallest buttons are the phone's top bar, 44×36
  points. Don't make anything tappable smaller than that, and give new buttons
  at least 44 points in both directions where there's room.
- **Messages** (toasts, the theme picker, "Star Citizen not focused") must stay
  readable with night mode on.
- **Large text, high contrast and button lights off** (switches in Deck
  settings, `data-large`, `data-contrast` and `data-no-lights` on `<html>`,
  styles at the end of `deck.css`) must keep working: check new button styles with both on, at tablet and phone size.
  High contrast overrides the theme tokens, so a new style that only uses the
  tokens follows it by itself; one with its own colors or glows needs an
  override there.
- Buttons without a text label, like the icon-only top bar, need an
  `aria-label`.
