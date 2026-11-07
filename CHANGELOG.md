# Changelog

What changed in each version of SpoolUp, newest first. Each version's
section becomes the notes of its
[GitHub release](https://github.com/xattribution/SpoolUp/releases), so add one
before tagging a release.

## 2.2.0 (2026-11-07)

- **No more stuck keys.** When Star Citizen loses focus (you alt-tab, a
  window pops up), SpoolUp lets go of every key a hold button is holding and
  ends a long press early. It also lets go of any hold after 2 minutes. The
  deck tells you: "Keys released: Star Citizen lost focus" or "Keys released
  after 2 minutes".
- **Binds you cleared stay cleared.** A bind you removed in Star Citizen's
  keybindings no longer presses the game's default key (which may do
  something else now). Its button shows dimmed with a **BIND** badge, and
  pressing it says "Not bound in Star Citizen".
- **Your keybinds are found anywhere.** SpoolUp looks on every drive (C to Z)
  and in every channel (LIVE, PTU, EPTU, TECH-PREVIEW, HOTFIX). While the game
  runs it reads that copy's binds, wherever it's installed, and connected
  decks pick them up; otherwise LIVE's. Binds the game wrote with the modifier last
  (`f6+lalt`) are now pressed modifier first, so the game sees LAlt+F6.
- **Install keybind profile keeps your binds.** For a copy of the game where
  you've set binds of your own, the app window now builds **SpoolUp Keyboard
  (your binds)**: every bind you have (keyboard, mouse, joystick, gamepad,
  device settings and binds you cleared), plus keys only for the deck's
  actions you haven't bound. Where your binds already use one of the deck's
  usual keys, it picks a free one that doesn't clash, and the window tells you
  to reload the deck. A copy without binds of its own still gets the plain
  SpoolUp Keyboard profile.
- **Star Citizen folder in Settings.** The app window's Settings shows the copy
  of the game in use (for example LIVE and its folder). **Change...** picks
  another folder and **Reset to automatic** goes back to finding it by itself.
  Keybinds, Install keybind profile and battle alerts all use that folder, and
  Deck settings on the tablet shows it under Battle alerts.
- **Battle alerts.** SpoolUp reads Star Citizen's own Game.log (the text log
  next to the game) and tells your decks when your ship is under
  fire and by whom, when it's disabled or destroyed, when you die and when
  you get a kill. A thin strip at the top of the deck shows it quietly, and
  the new **Last hits** readout on the Combat panel lists who hit you last,
  your ship and its state, and your kills this session.
  Turn it off on each device with **Battle alerts** in Deck settings, or on
  the PC with `"gameLog": false` in `config.json`.
- **25 more buttons, all on Star Citizen 4.10's default keys**, so they work
  without the SpoolUp profile:
  - **Combat:** More Missiles and Reset Missiles beside Fewer Missiles in the
    countermeasures cluster, and Decoy Burst − / + under it.
  - **Targeting:** a Cycle forward row (Next Attacker, Friendly, Any, In view
    and Subtarget), Pin Target 1–3 and Unlock Target.
  - **Scanning:** Radar Ping under the operator-mode dial. It's held like Tab
    in the game, whose ping is "Hold & Release".
  - **Salvage:** Focus Left, All or Right head, and Fire Left / Right Head
    (also on the Vulture panel).
  - **Mining:** Jettison Cargo, in a red danger frame (also on the
    Prospector panel).
  - **Crew/Doors:** Light Amplification (lit while on) and Toggle Port Locks.
  - **QT/Systems:** Emergency Exit under Eject and Self-D.
  - **Power/Landing:** Power Throttle − / +.
  - On tablets, the Combat dashboard adds Radar Ping, the pins and the decoy
    burst, Industry adds the power throttle and Jettison Cargo, and Hauling
    adds Light Amp and Port Locks. Every new button has its own icon, voice
    phrases and Spanish, French and German labels.
- **SpoolUp for Android.** A new app that finds the SpoolUp PCs on your
  network by itself, opens the deck full screen, keeps the screen on and
  gives a haptic tick on every press. It also takes the PC's address or its
  QR code. It's the same deck, with the same pairing code and focus check.
  Releases attach `SpoolUp-<version>.apk` once the signing key is set up
  (see [docs/ANDROID.md](docs/ANDROID.md)).
- **The apps find your PC.** SpoolUp answers the apps on your network with
  the PC's name, the deck's port and SpoolUp's version, skipping VPN and
  virtual adapters. Settings → Network → **Let
  the SpoolUp app find this PC** in the app window turns it off.
- **An iPhone and iPad app is on the way** (see [docs/IOS.md](docs/IOS.md)):
  it finds your PC by itself, pairs with the QR code, keeps the screen on and
  includes an offline demo. The deck is ready for it already: inside either
  app it plays the device's own haptics, Deck settings gets **Switch PC**
  (back to the app's list of PCs), and the Keep screen on switch hides
  because the app keeps the screen on.
- For developers: the keybind profile check now compares every action with
  Star Citizen 4.10's own list of actions and default keys
  (`data/sc-actions-4.10.json`), so a misspelled action or one in the wrong
  map stops the build, and `node server.js --dry-run --game-log <file>`
  follows a sample log for trying battle alerts without the game.

## 2.1.0 (2026-11-07)

- **One HUD for every screen.** The SpoolUp HUD is now laid out on a grid that
  follows your screen, so phones and tablets use the same layout and
  everything stays lined up when you turn the screen, switch between landing
  and docking, or add buttons. A phone opens on one panel at a time, a tablet
  on two halves. Deck settings → Button size picks Compact, Default or Roomy
  for each screen size.
- **One panel or two halves.** The new view button next to the page arrows
  shows one panel across the whole screen or two halves side by side; a phone
  held upright always shows one. Wide **dashboards** (Pilot, Combat,
  Industry) put the controls for one kind of flying on one screen, and the
  new **Dashboards** layout has those plus Hauling. On a phone each dashboard
  keeps its three or four main clusters, so it fits without scrolling, and a
  panel of clusters alone fills a phone held upright.
- **Swipes and taps are told apart.** Swipe sideways with two fingers to flip
  panels, as before. A press now waits a moment, and a second finger landing
  cancels it, so a swipe that starts on a button flips the panel. Holding a
  button (hold to talk) and dragging a slider still work. A tap now goes when
  your finger lifts, so a finger that slides off a button, or a second finger
  that lands late, cancels the press. A card on the first start shows
  what two fingers do. If you'd rather use
  one finger, turn on **One-finger swipe** in Deck settings (off by default;
  sliders, the gear lever and dials keep their own drag). The page arrows still work.
- **Swipe up and down.** Two fingers swiping up or down do what you pick in
  Deck settings, on each device: open mobiGlas (one tap of its button), show
  **Quick actions** (the top bar's buttons, the ones that matter in a hurry
  and your starred emotes), jump to your primary panel or to any panel.
- **Cockpit clusters.** Shields, targeting, lights and doors, turrets,
  countermeasures, the quantum spool, the mining head, the salvage head and
  the switch banks are each one instrument you can place on any panel or
  dashboard. Every gesture is one tap of a button you already have.
  - **Shields** have no words: the ship from above with an arc on each face
    and an outer ring for power. Slide a finger toward a face to shift
    strength there (one tap per step, at most 5 per slide), tap a face for one
    step, tap the ship to reset and tap the ring for Shields On/Off.
  - **Targeting** is a reticle between its buttons: Attacker, Friendly, Any
    and In view, with Previous and Next Hostile at its sides, each with room
    for its icon and word. Tap the reticle's center (or swipe right across
    it) for the next hostile, swipe left for the previous one, and tap the
    inner ring for sub-targets.
  - **Countermeasures** is one tight weapons and defense block: Decoy and
    Noise either side of Panic Decoys on top, Launch Missile below, then
    Fewer Missiles and the lead / lag pips, each with its own icon. Panic
    Decoys stays under a flip cover: the first tap only lifts it, and it drops
    again after 4 seconds.
  - **Lights and doors**: Lights, Lock and Unlock, and the doors drawn as two
    leaves that slide apart while they're open. Tap the doors to toggle them,
    or tap Open or Close. Lights now sits with the doors, on the Stream Deck
    too.
  - **Turrets** are plain T1, T2 and T3 buttons with the turret icon.
  - **QT/Systems** puts Flight Ready, a normal-size button now, beside the
    PWR, WEP, THR and SHLD switches, the things you press once at start-up,
    and gives the rest of the panel to the quantum spool. The Pilot, Industry
    and Hauling dashboards start with the same group, and every panel and
    dashboard keeps together what's used together.
  - The arcs, gauges and lamps show the taps SpoolUp sent.
- **A new mode dial.** The operator-mode dial has round mode chips on a clean,
  symmetric face that keeps its shape in any space, and each maker draws it
  its own way: RSI's instrument ring, Drake's hex nut, Aegis's V, Anvil's
  octagon, Origin's slim oval, MISC's cargo rail, Crusader's open ring,
  Banu's flower, Aopoa's crescent, Argo's load-frame triangle, Consolidated
  Outland's sunrise and Esperia's star. The classic 2.0 layouts use it too.
- **Emote favorites.** The Emotes panel is one board with tabs: ★ Favorites,
  All and groups you make. Hold an emote to star it; the
  board opens on Favorites once you have some. **Arrange** lets you reorder
  Favorites or a group, drag an emote onto a tab to add it there and take one
  out with ×; + makes a group. Each tablet and phone keeps its own.
- **The layout editor snaps to a grid.** Drag a button or cluster to reorder
  it and the rest moves out of the way, or hold Shift (or pick **Pin**) to pin
  it to a cell; drag its corner to resize it by whole cells. Preview a small
  or large screen, two halves or one panel, and each button size.
- **Add from a list, build dashboards.** The editor's Add side lists new
  buttons, groups and headers and every cluster and button already in the
  layout, by category. Something added again is a copy: the same buttons, so
  a press on either is the same press. The Panels side adds, moves and
  deletes panels, sets the **primary panel** a swipe can jump to, and builds a
  Pilot, Gunner, Miner, Salvager or Hauler dashboard from a preset.
- **Your 2.0 layouts are kept.** The 2.0 HUD stays as "SpoolUp HUD 2.0", your
  own 2.0 layouts draw and edit exactly as before, and **Convert to grid** in
  the editor makes a grid copy of one under a new name.
- **A clearer app window.** The SpoolUp window and its Settings and keybind
  profile windows have a solid graphite look: near-white text on solid
  panels, larger type, one blue accent and color only where it means
  something (a status dot, the status pill, a banner). The stars only drift in
  the thin title strip.
- **Public network, fixed in one click.** When Windows treats your network as
  Public (which blocks tablets), an amber banner across the top of the window
  says so, with an **Open network settings** button that opens the page for
  that network, where you pick Private. Settings has a Network section with
  the same button.
- **The Stream Deck pack is sold separately.** SpoolUp for Stream Deck (the
  plugin, icon pack and profiles) is now on the
  [Elgato Marketplace](https://marketplace.elgato.com), and the free Stream
  Deck downloads stop with 2.1. Stream Deck downloads from earlier releases
  keep working with SpoolUp 2.1.
- **Icons only.** Deck settings → **Button labels** picks Icons and labels, or
  Icons only, on each device. In the layout editor any panel, cluster or
  button can show or hide its labels regardless (Inherit / Show / Hide); the
  most specific one wins. Buttons without an icon keep their label, buttons
  that share an icon keep a number or their words, and a hidden label is still
  the button's name for screen readers and its voice phrase. A button can also
  have a **short label** for its face.
- **Keep screen on.** Deck settings has a **Keep screen on** switch (on by
  default), with a line saying how it's kept on (the browser's wake lock, or a
  silent video) and, where neither works, what to set on an iPad, iPhone or
  Android device. The deck asks again when the system drops the wake lock,
  plays the video again when the system pauses it, and starts at the first
  touch.
- "Centre sliders" is now "Center sliders", and SpoolUp uses American
  spelling throughout.

## 2.0.0 (2026-11-06)

- **A landing-gear lever.** The Landing Gear button is now a lever: pull the
  knob down to deploy, push it up to retract. Each pull or push taps the gear
  key once. Its three lamps blink red while the gear is in transit and turn
  green when it's down; like the button lights, they follow what SpoolUp
  pressed, not the game.
- **Power sliders.** The power triangle's eight chevron buttons are now three
  sliders, Weapons, Thrusters and Shields. Each step you drag taps that
  system's up or down key once, at most five per drag, spaced out. Tapping the
  track beside the knob steps once. Max and Min stay above and below. The
  sliders count the taps SpoolUp sent (SpoolUp can't read the game), so
  Center sliders puts them back in the middle without pressing anything, and
  Max, Min and Reset All move them too.
- **A speed-range fader.** Speed Range + and − are now one fader on the
  Flight Assist panel: each step left or right taps Speed Range − or + once,
  the same way as the power sliders.
- **An operator-mode dial.** The Oprt Mode menu on QT/Systems is now a dial
  with a needle: turn it to a mode and let go, or tap the mode, and it taps
  that mode's key once. SCM/NAV sits in the middle. On phones the dial has
  its own panel, Master Modes, next to QT/Systems.
- **Resize the halves.** Deck settings → Edit layout unlocks the seam between
  the HUD's two halves: drag it to make one wider, then Lock. Reset puts it
  back. Each tablet and phone keeps its own, and nothing is pressed while
  you arrange it.
- Every new control is drawn in each maker's own style (Drake's riveted
  hazard lever, Origin's cream fader knob, Anvil's knurled dial...), works
  with touch, mouse and keyboard, and follows Large text, High contrast,
  night mode and Reduce motion. The layout editor places and resizes them,
  and lists their buttons to edit.
- Each step, pull or turn is still one press of an existing button through
  the PC's usual checks; voice commands and Stream Deck keys are unchanged.

## 1.9.0 (2026-11-06)

- **Silent installs leave your keybinds alone.** Installing SpoolUp silently
  (winget, the Microsoft Store) no longer copies the keybind profile into the
  game unless you chose it in an earlier install.
- **Every manufacturer theme is its own design**, not the same deck in other
  colors. Each maker has its own surfaces, plates, button shapes, section
  titles, dials, switches, eject slider and framed buttons, and its own lit,
  pressed, blocked and offline looks: RSI's navy glass and corner brackets,
  Drake's riveted, scratched metal and hazard stripes, Aegis's carbon weave and
  red tactical marks, Anvil's olive armor with screws and indicator lamps,
  Origin's cream and black gloss with a cream dial, MISC's teal enamel and
  orange tabs, Crusader's sky and pearl-white capsules, Banu's lacquer on
  carved wood, Aopoa's jade and flowing forms, Argo's safety-yellow work
  panels, Consolidated Outland's desert sunset and Esperia's plum night and
  coral blades. Origin, Crusader, Aopoa, Argo and Consolidated Outland are now
  light decks. Large text, high contrast and night mode work in all of them.
- **The Stream Deck pack matches.** Keys, icon packs, profiles and the plugin's
  touch strips are drawn in each maker's material, with its own lit look
  (Drake's hazard band, Anvil's lamp, Origin's gold bar...), on that theme's
  page color.
- What each button presses hasn't changed.

## 1.8.0 (2026-11-06)

- **Tablet Deck is now SpoolUp**, the name everywhere you see it. The keybind
  profile to load in-game is now **SpoolUp Keyboard**: install it again from the
  app window (Settings → Install keybind profile...) and load it in-game. The
  Stream Deck plugin is a new one (`com.xattribution.spoolup`), so if you
  installed 1.7.0's Tablet Deck plugin and icon packs, remove them and install
  the new ones. Your settings carry over, and updates now come from this
  repository.
- **Signed releases.** The installer, the app and its helpers are code-signed
  with Azure Artifact Signing, so Properties → Digital Signatures shows the
  publisher, and Windows and antivirus tools have a name to trust.

## 1.7.0 (2026-11-06)

- **A Stream Deck pack.** Profiles for every Stream Deck from the Mini to the
  new + XL, in all 12 manufacturer themes, with a page per scene (Flight, Combat,
  Mining, Salvage, Hauling & Exploration, On Foot, Comms), folders for
  emergencies, shields, power, emotes, websites and themes, and dials and touch
  strips on the + and + XL. The Comms page has Star Citizen's push to talk,
  hail and invites, Discord, OBS and media keys. An icon pack per theme has
  every button (222, the animated ones also as GIFs) for pages of your own. The **Tablet Deck plugin** presses
  through Tablet Deck, with the focus check, lit buttons, animations and live
  theme changes. Scenes can follow both ways: picking a scene on the Stream
  Deck flips tablets that follow it (**Tablets follow scenes**, off until
  turned on in the plugin's settings), and flipping a panel on a tablet takes
  the Stream Deck to that panel's scene (**Follow the tablet**, on unless
  turned off).
  The profiles also work on their own, without Tablet Deck. Each release has
  the pack as downloads; see "Stream Deck" in the README.
- **Presses no longer mix.** Two quick taps of one button are two presses (the
  game saw one longer press), and buttons tapped together take turns when their
  keys could combine: Weapons + (F5) pressed while Shields − held LAlt reached
  the game as Weapons −. While you hold Proximity PTT, Engage Quantum or Laser
  Power, which hold a modifier key, other buttons are skipped and flash red
  rather than press with it.
- **Keys let go sooner when a tablet vanishes mid-press.** A tab that's closed
  without saying goodbye releases its held buttons straight away, not 5 seconds
  later.
- **A held button lets go even if the HUD redraws under it.** Holding Engage
  Quantum, push to talk or another hold button while the HUD redrew (the other
  half paged with a second finger, a Stream Deck scene, the screen turned) left
  its keys down on the PC until you pressed it again.
- **A keybind Tablet Deck can't press no longer stops it from starting.** If
  your actionmaps.xml binds an action to a key it has no code for, that button
  keeps its own key and the console says which one.
- **Taps stay taps:** the tap length (Settings, `tapMs`) now goes up to 200 ms,
  since Star Citizen counts a press held past 0.25 s as a hold.
- **A Comms panel** on the right half: push to talk and proximity push to talk
  (held for as long as you hold the button), the voice channel, hail target,
  accept and decline invites, chat, contacts, the inner thought wheel and wipe
  visor.
- **Decoupled and Throttle Mode** replace Match Speed and Reset Limit in Flight
  Assist, which Star Citizen 4.10 no longer has. Decoupled lights while it's on.
- **Next Hostile** joins Prev Hostile in Targeting, so the Stream Deck's Targets
  dial cycles hostiles both ways.
- **Buttons follow Star Citizen 4.10's keys.** Decoy is now H and Noise J,
  Flight Ready RAlt+R and Request Docking RAlt+N; SCM/NAV and the docking camera
  are held long enough to count; Engage Quantum presses RAlt+J (holding B
  switched SCM/NAV instead); Launch Missile launches (it toggled the missile
  camera); the Speed Limiter and Speed Range buttons work again; and the salvage
  modifiers, Salvage and Guns modes, Fewer Missiles, Panic Decoys, Prev Attacker
  and Toggle Doors moved off keys the game also uses for Flight Ready, night
  vision, the disintegrate beam, the voice channel and more. That also fixes the
  Speed Limit, Modifier and Beam dials on the Stream Deck + and + XL. Install the
  keybind profile again (Settings → Install keybind profile...) and load
  **Tablet Deck Keyboard** again in-game. Layouts you made in the layout editor
  keep some of their old keys; see "Buttons to double-check" in the README.
- **Follow Stream Deck scenes** (a switch under Display in Deck settings, on
  unless turned off): picking a scene on a Stream Deck with the new Tablet Deck
  plugin flips the tablet's HUD halves to that scene's panels.
- **Key labels** (a switch under Display in Deck settings): each HUD button
  shows the key it presses in a corner, taken from your keybinds, so in-game
  rebinds show up too.
- **Button sounds**: soft HUD sounds, made from a few tones rather than sound
  files. Presses chirp at a pitch that follows the button's color, the quantum
  drive spools up and engages, self-destruct arming beeps faster as it fills,
  panels flip, voice commands answer, and a press that didn't go through buzzes.
  They're on by default; a new **Sounds** section in Deck settings turns them
  off and sets the volume, per device.
- **Hear the deck in your headset:** under Sounds → Play on, a tablet can play
  its sounds on one of the PC's outputs instead of on itself, e.g. your
  headset, so you hear them over the game. The PC makes the same sounds there
  with a small new helper (`native/sound.exe`); if that output is unplugged,
  they play on Windows' default output, and while the tablet isn't connected,
  on the tablet.
- **The deck shows how quick the link to the PC is**, next to Connected
  ("Connected · 8 ms"); the dot turns amber when it's slow enough to feel.
- **A dead connection is noticed within seconds.** If the PC goes to sleep or the
  Wi-Fi drops without the connection closing, the deck used to keep saying
  Connected while taps went nowhere; now it says Reconnecting… and connects again
  by itself.
- **Changing the pairing code now disconnects devices paired with the old one**,
  and they show the pairing screen with a note that the code changed. Before,
  they kept working until they reconnected, then retried the old code forever.
- **HUD layouts show a lost connection**: their buttons dim, as on the tabbed
  grid layout, and a tap says "Not connected to the PC" instead of playing its
  animation as if it had worked.
- **A press skipped because Star Citizen wasn't in front now shows on the
  button too**: it flashes red, and a quantum charge, self-destruct arming or
  lock-on it started stops instead of ending as ENGAGED, ARMED or LOCKED.

## 1.6.0 (2026-10-26)

- **Español, Français and Deutsch:** the deck and the app window can show
  their text in Spanish, French or German. Each tablet and phone picks its
  language under Layout in Deck settings, starting from the device's own
  language; the window follows Windows, or the new Language choice in
  Settings. Button labels, panels and status words are translated; keys, voice
  phrases (still English) and your own layouts' labels stay as they are. The
  translations were made with AI, so corrections from native speakers are
  welcome on GitHub.

## 1.5.0 (2026-10-25)

- **Button lights can be turned off** under Display in Deck settings. The deck
  can't read the game, so lit buttons only follow your taps; with this off,
  buttons just press their keys, without ON/OFF.
- **The window tells you when Windows treats your network as Public**, the most
  common reason a tablet can't connect, with a link to Windows' network
  settings. The warning goes away by itself once it's Private.
- **Feedback & ideas** links in the window's System panel, its tray menu and at
  the bottom of Deck settings, to GitHub's issue forms.
- The app window no longer shows a dotted focus ring around Settings when it
  opens; it only appears when you move around with Tab.

## 1.4.0 (2026-10-25)

- **Share layouts:** the layout editor can **Export** a layout as a file and
  **Import** one from a friend. An import opens unsaved, so you can check its
  buttons and keys before saving it with the usual checks.
- **Ship layouts for the Prospector and the Vulture:** each starts with a panel
  of that ship's controls, made from the deck's own buttons, on tablets and
  phones.
- **Large text** and **High contrast** switches under Display in Deck settings,
  for reading the deck at a glance. High contrast works over any theme.

## 1.3.1 (2026-10-24)

- **Sharper text in the app window:** text is drawn with ClearType on solid
  panels, each letter on a whole pixel, and the smallest labels are a size
  larger. The stars now pass behind the panels instead of showing through them.

## 1.3.0 (2026-10-24)

- **The new app window is back, and now starts:** HUD panels with corner
  brackets over drifting stars that follow the mouse (click an empty spot to
  send a pulse), telemetry of tablets, where keys go and the last minute's
  presses, a dark title bar, and the same look for Settings (now in panels with
  on/off switches), Voice commands and Install keybind profile. The stars only
  move while the window is in front.
- **If Tablet Deck ever runs into a problem, it says so:** a message with the
  reason, and the details saved in `crash.log` in the Settings folder, instead
  of not starting.

## 1.2.1 (2026-10-24)

- **Back to the 1.1.0 app window.** 1.2.0's new look kept Tablet Deck from
  starting on some PCs, so this release brings back the previous window while
  that's fixed. Everything else from 1.2.0 stays.

## 1.2.0 (2026-10-24)

- **A new look for the app window:** HUD panels with corner brackets over a
  field of drifting stars that follows the mouse; click an empty spot to send a
  pulse through them. The stars only move while the window is in front, and
  stand still when Windows' "Show animations" is off.
- **Telemetry** in the app window: tablets connected, where keys go, and the
  presses of the last minute, including those held back because Star Citizen
  wasn't in front.
- The app window has a dark title bar on Windows 10 and 11.
- **Settings, Voice commands and Install keybind profile** have the same look.
  Settings is now sorted into panels (Pairing, Keys, Voice, Push-to-talk,
  Links) with on/off switches.

## 1.1.0 (2026-10-24)

- **The QR code picks the right network:** VPN and virtual adapters (WSL,
  Hyper-V, VirtualBox, NordLynx, Tailscale and others) are skipped, so the QR
  code uses your Wi-Fi or Ethernet. The new `host` setting chooses the address
  yourself, and running from source lists each address with its adapter.
- **Wrong pairing codes are limited:** after 5 in a minute, that device is
  ignored for a minute, and the tablet says to wait.
- Fixed a held combo button (like Look Behind) that could leave a key stuck
  down when released very quickly, and a quick hold that could let go of a key
  another button was still holding.
- The tabbed grid layout's settings button now has the sliders icon and says
  **Settings**, like the HUD's, instead of the theme's name, so it's clear how
  to get to the other layouts from there.
- **A new pairing screen:** a clear card with a big code field, a red error
  when the code doesn't match, and a hint to scan the QR code instead. The
  deck's buttons stay hidden until the device is paired. It also no longer
  looks grayed out.
- **Lit buttons:** buttons that switch something on stay lit until it's
  switched off. Open Doors stays lit until Close Doors, Landing Gear shows
  Deployed, and the power toggles, lights, VTOL, flight ready, speed limiter,
  ESP, G-safety and door locks light up while on. The state is kept on the PC,
  so it survives paging and reloads, matches on every tablet and follows voice
  commands. **Reset button lights** in Deck settings turns them all off.
- **Deck settings:** the sliders sheet is now organized in sections (Layout,
  Display, Open on PC, Theme), with real on/off switches for night mode and
  two-finger swipe.
- The mic and **?** buttons only show when voice commands can actually run on
  the PC, and the voice list says why when they can't.
- Messages on the tablet appear above the page names instead of covering them.
- "Hold to engage" no longer runs into the quantum charge marks, and status
  lines like "Retracted" have room under their label.
- The installer now includes `THIRD-PARTY-NOTICES.txt` with the license of the
  Node.js runtime inside `TabletDeckServer.exe`.

## 1.0.0 (2026-10-22)

The first public release.

- **A HUD-style control deck for any tablet or phone.** It runs in the browser,
  so iPad, Android and iPhone all work, with no app or account. Scan the QR
  code in the PC window to pair. Each button presses the real key on your PC,
  and by default only while Star Citizen is the window in front.
- **12 manufacturer themes**, press animations and a night mode.
- **Layouts:** the Tablet Deck HUD, plus the **Combat**, **Mining & Salvage**
  and **Hauling & Exploration** starter layouts, for tablets and phones. Each
  device picks its own under **Layout** in the sliders sheet. You can turn on
  a two-finger swipe there to flip panels.
- **Layout editor** (**Edit layout** in the window): drag and resize buttons,
  and change their label, keys, press mode, color, icon, animation and voice
  phrases, with undo.
- **Voice commands:** hold the mic on the tablet, or set a push-to-talk key or
  mouse button on the PC. Over 300 phrases like "gear up", "guns mode", "full
  power to shields", "previous hostile" and "emote wave". The **?** next to the
  mic lists them all, and the window has a **Voice strictness** slider.
- **Follows your keybinds:** buttons read your Star Citizen keybinds, so
  rebinding in-game moves them too. The optional Tablet Deck Keyboard profile
  covers actions with no default key, and **Install keybind profile...** puts
  it in your game folders.
- **Open on PC:** buttons in the sliders sheet open UEX, Erkul, SPViewer and
  SCMDB in the PC's browser. Change the list in **Settings**.
- **A Windows app:** one installer, a window with the QR code, connected
  tablets and a log of presses, a tray icon, optional **Start with Windows**,
  and **Settings** for the pairing code, port, tap length, voice and more.
- **Updates:** Tablet Deck tells you when a new version is out and installs it
  when you click **Update now**. Turn the check off in the window if you like.
- Lets itself through Windows Firewall on private networks only. Settings live
  in `%APPDATA%\TabletDeck`, and uninstalling asks before deleting them.
