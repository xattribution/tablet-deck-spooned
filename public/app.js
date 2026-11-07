(() => {
  // Emblems are original badge designs on a 64x64 grid that suggest each maker's
  // character; they are not the manufacturers' real logos.
  // `F` = solid accent fill, `T` = group drawn in the theme's text color.
  const F = `fill="currentColor" stroke="none"`;
  const T = (body) => `<g style="color:var(--text)">${body}</g>`;
  const THEMES = [
    { id: "rsi", name: "RSI", full: "Roberts Space Industries", tag: "Classic interstellar lines",
      // Orbital ascent: a ship rising through a planetary ring.
      emblem: `<circle cx="32" cy="32" r="27"/><circle cx="32" cy="32" r="20" opacity=".3"/>
        <path d="M32 8 40 34 32 28 24 34Z" ${F}/>
        <path d="M5 41c12-9 36-17 54-15" stroke-width="3.5"/>
        ${T(`<path d="M21 47h22"/><path d="M25 53h14" opacity=".6"/>`)}` },
    { id: "drake", name: "Drake", full: "Drake Interplanetary", tag: "Built rough, flown hard",
      // Talon: three claw slashes across a heavy hex nut.
      emblem: `<path d="M32 4 56 18v28L32 60 8 46V18Z" stroke-width="3.5"/>
        <path d="M32 12 49 22v20L32 52 15 42V22Z" opacity=".3"/>
        <path d="M19 15c9 8 12 18 10 33l-5-3c1-11-1-20-5-30Z" ${F}/>
        <path d="M29 13c9 9 12 19 10 35l-5-3c1-12-1-22-5-32Z" ${F}/>
        <path d="M40 17c6 7 9 16 7 27l-4-3c0-9-1-16-3-24Z" ${F} opacity=".75"/>` },
    { id: "aegis", name: "Aegis", full: "Aegis Dynamics", tag: "Military precision",
      // Winged spear: a central blade with swept delta wings.
      emblem: `<path d="M27 24 4 40h23Z" ${F} opacity=".55"/><path d="M37 24l23 16H37Z" ${F} opacity=".55"/>
        <path d="M27 31 12 44h15M37 31l15 13H37"/>
        <path d="M32 3 38 30 32 54 26 30Z" ${F}/>
        ${T(`<path d="M21 50h22M27 59h10"/>`)}` },
    { id: "anvil", name: "Anvil", full: "Anvil Aerospace", tag: "Frontline hardware",
      // Bulwark: a heavy shield with a command band and rank chevrons.
      emblem: `<path d="M32 4 56 12v18c0 14-10 24-24 30C18 54 8 44 8 30V12Z" stroke-width="3.5"/>
        <path d="M32 10 50 16v14c0 11-8 19-18 24-10-5-18-13-18-24V16Z" opacity=".3"/>
        <path d="M15 17h34v6H15Z" ${F}/>
        ${T(`<path d="M20 30l12 8 12-8M20 40l12 8 12-8" stroke-width="3.5"/>`)}` },
    { id: "origin", name: "Origin", full: "Origin Jumpworks", tag: "Luxury in the void",
      // Halo: a fine ring, a crescent and a single axis line.
      emblem: `<circle cx="32" cy="32" r="26" stroke-width="1.6"/>
        <path d="M40 10a22 22 0 1 0 0 44a30 30 0 0 1 0-44Z" ${F}/>
        <circle cx="45" cy="21" r="2.2" ${F}/>
        ${T(`<path d="M32 1v9M32 54v9" stroke-width="1.6"/>`)}` },
    { id: "misc", name: "MISC", full: "Musashi Industrial & Starflight", tag: "Industrial workhorses",
      // Cargo hex: a horizontal hexagon holding stacked cargo modules.
      emblem: `<path d="M18 8h28l14 24-14 24H18L4 32Z" stroke-width="3"/>
        <rect x="19" y="19" width="26" height="6" rx="1" ${F}/>
        <rect x="14" y="29" width="36" height="6" rx="1" ${F}/>
        <rect x="19" y="39" width="26" height="6" rx="1" ${F} opacity=".6"/>
        ${T(`<path d="M8 32h3M53 32h3"/>`)}` },
    { id: "crusader", name: "Crusader", full: "Crusader Industries", tag: "Open skies, big haulers",
      // Skysail: two wind-filled sails above a horizon.
      emblem: `<circle cx="32" cy="32" r="27"/>
        <path d="M19 46C23 28 35 16 51 12c-6 10-9 22-9 34Z" ${F}/>
        <path d="M13 46c3-10 8-17 15-22-2 7-3 14-3 22Z" ${F} opacity=".45"/>
        ${T(`<path d="M8 46h48"/><path d="M15 52h34" opacity=".4"/>`)}` },
    { id: "banu", name: "Banu", full: "Banu Souli", tag: "Alien craft, merchant spirit",
      // Bloom sigil: four petals and four buds around a seed, in a dotted ring.
      emblem: `<circle cx="32" cy="32" r="28" stroke-dasharray="1 5"/>
        ${[0, 90, 180, 270].map((r) => `<path d="M32 5c7 8 7 15 0 21-7-6-7-13 0-21Z" ${F} transform="rotate(${r} 32 32)"/>`).join("")}
        ${[45, 135, 225, 315].map((r) => `<path d="M32 12c4 5 4 9 0 13-4-4-4-8 0-13Z" ${F} opacity=".5" transform="rotate(${r} 32 32)"/>`).join("")}
        ${T(`<circle cx="32" cy="32" r="4.5"/>`)}` },
    { id: "aopoa", name: "Aopoa", full: "Aopoa", tag: "Xi'an elegance",
      // Twin crescents: two mirrored, flowing blades.
      emblem: `<path d="M12 52C9 31 22 13 45 8 32 18 27 31 29 52Z" ${F}/>
        <path d="M52 12c3 21-10 38-33 44 13-10 18-24 16-44Z" ${F} opacity=".5"/>
        ${T(`<circle cx="32" cy="32" r="2.6" fill="currentColor" stroke="none"/>`)}` },
    { id: "argo", name: "Argo", full: "Argo Astronautics", tag: "Utility first",
      // Liftframe: a load-bearing triangle with a crane hook and rivets.
      emblem: `<path d="M32 5 59 55H5Z" stroke-width="3.5"/><path d="M32 19 47 47H17Z" opacity=".3"/>
        <circle cx="14" cy="50" r="2.4" ${F}/><circle cx="50" cy="50" r="2.4" ${F}/>
        ${T(`<path d="M32 15v15a5 5 0 1 1-5 5" stroke-width="3.5"/>`)}` },
    { id: "cnou", name: "CO", full: "Consolidated Outland", tag: "Frontier newcomer",
      // Frontier sunrise: a bold open ring around a rising sun.
      emblem: `<path d="M48 14a24 24 0 1 0 0 36" stroke-width="6"/>
        <path d="M33 41a9 9 0 0 1 18 0Z" ${F}/>
        ${T(`<path d="M29 41h30"/><path d="M42 26v-4M32 30l-3-3M52 30l3-3" opacity=".7"/>`)}` },
    { id: "esperia", name: "Esperia", full: "Esperia", tag: "Reborn Tevarin designs",
      // Tevarin crescent: a sweeping blade under a four-point star.
      emblem: `<path d="M8 32A24 24 0 0 1 32 8" opacity=".4"/>
        <path d="M10 52C14 27 30 10 55 8 39 16 29 30 27 54Z" ${F}/>
        ${T(`<path d="M45 30l2.5 8 8 2.5-8 2.5-2.5 8-2.5-8-8-2.5 8-2.5Z" fill="currentColor" stroke="none"/>`)}` },
  ];
  const themeById = Object.fromEntries(THEMES.map((t) => [t.id, t]));
  const svg = (body) => `<svg viewBox="0 0 48 48" aria-hidden="true" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round">${body}</svg>`;
  const emblemSvg = (body) => `<svg viewBox="0 0 64 64" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">${body}</svg>`;

  const store = {
    get(k) { try { return localStorage.getItem(k); } catch { return null; } },
    set(k, v) { try { localStorage.setItem(k, v); } catch {} },
  };

  const $ = (id) => document.getElementById(id);
  const grid = $("grid");
  const tabs = $("tabs");
  let token = (new URLSearchParams(location.search).get("t") || store.get("token") || "").trim().toLowerCase();
  // Phones (short side under 500pt, e.g. iPhone 15 Pro at 393pt) get the phone layout;
  // ?device=tablet or ?device=phone in the address overrides the guess.
  const device = new URLSearchParams(location.search).get("device")
    || (Math.min(screen.width, screen.height) < 500 ? "phone" : "tablet");
  document.documentElement.dataset.device = device;
  // Demo mode (?demo, or the website's "Try it" page): no PC at all. Layouts come from the
  // layouts/ folder next to the page, and presses only play their animations.
  const DEMO = new URLSearchParams(location.search).has("demo") || window.DECK_DEMO === true;
  // Grid layouts fit every screen, so phones and tablets get the same files; only the
  // 2.0 HUD kept as it was has a phone version of its own.
  const DEMO_LAYOUTS = [
    ["layouts/hud.json", "SpoolUp HUD"],
    ["layouts/dashboards.json", "Dashboards"],
    ...[["combat", "Combat"], ["mining-salvage", "Mining & Salvage"], ["hauling-exploration", "Hauling & Exploration"],
      ["prospector", "Prospector"], ["vulture", "Vulture"]]
      .map(([id, name]) => [`layouts/${id}.json`, name]),
    [device === "phone" ? "layouts/hud-classic-phone.json" : "layouts/hud-classic.json", "SpoolUp HUD 2.0"],
    ["layouts/star-citizen.json", "Star Citizen (tabbed grid)"],
  ];
  // ?viewport=small|large forces a grid layout's size class (canvas.js), e.g. to try the
  // phone arrangement on a tablet.
  const viewport = new URLSearchParams(location.search).get("viewport") || "";
  // ?safe=top,right,bottom,left (CSS px) stands in for the screen's safe-area insets. Pages
  // in an iframe always see zero insets, so the showcase frame passes the phone's here.
  const safe = (new URLSearchParams(location.search).get("safe") || "").split(",").map(Number);
  if (safe.length === 4 && safe.every((n) => n >= 0)) {
    ["top", "right", "bottom", "left"].forEach((side, i) => document.documentElement.style.setProperty(`--safe-${side}`, `${safe[i]}px`));
  }
  // Shown text in the deck's language (i18n.js); layouts, keys and voice phrases stay English.
  const tr = window.DeckI18n.t;
  let layout = null;
  let ws = null;
  let pageIndex = Number(store.get("page") || 0);

  // ---------- theme ----------

  function applyTheme(id) {
    const theme = themeById[id] || THEMES[0];
    const changed = document.documentElement.dataset.theme !== theme.id;
    document.documentElement.dataset.theme = theme.id;
    document.documentElement.dataset.maker = theme.full; // shown on the intro screen
    for (const m of document.querySelectorAll("#brandMark, .js-emblem")) m.innerHTML = emblemSvg(theme.emblem);
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.content = getComputedStyle(document.documentElement).getPropertyValue("--bg").trim();
    store.set("theme", theme.id);
    for (const card of document.querySelectorAll(".card")) card.classList.toggle("current", card.dataset.theme === theme.id);
    // Controls drawn from the maker's shape (the mode dial) follow the new theme.
    if (changed && deckCanvas) deckCanvas.render();
  }

  function buildPicker() {
    const cards = $("cards");
    cards.innerHTML = "";
    for (const t of THEMES) {
      const card = document.createElement("button");
      card.type = "button";
      card.className = "card";
      card.dataset.theme = t.id;
      card.innerHTML = `
        <div class="top"><span class="emblem">${emblemSvg(t.emblem)}</span>
          <div><div class="name"></div><div class="tag"></div></div></div>
        <div class="key sample" data-color="">
          <span class="glyph">${window.deckIcon("quantum-mode")}</span><span class="label">${tr("Quantum Mode")}</span>
        </div>
        <div class="swatches"><i style="background:var(--accent)"></i><i style="background:var(--green)"></i><i style="background:var(--amber)"></i><i style="background:var(--red)"></i><i style="background:var(--blue)"></i></div>`;
      card.querySelector(".name").textContent = t.name;
      card.querySelector(".tag").textContent = `${t.full} · ${tr(t.tag)}`;
      card.onclick = () => { applyTheme(t.id); closePicker(); };
      cards.appendChild(card);
    }
  }
  // Deck settings: the copy of Star Citizen the PC uses for keybinds and battle alerts
  // (hello and "game" from server.js). The folder is changed in the PC app's Settings.
  let gameInUse = null;
  const fillGame = () => {
    const note = $("gameNote");
    note.hidden = !gameInUse;
    if (gameInUse) note.textContent = tr(gameInUse.picked ? "Star Citizen {0}, from the folder picked in the PC app's Settings." : "Star Citizen {0}, found automatically.", `${gameInUse.channel}: ${gameInUse.dir}`);
  };
  const openPicker = () => { $("picker").classList.add("open"); applyTheme(document.documentElement.dataset.theme); fillLayoutPick(); fillLinks(); fillSounds(); fillSwipes(); fillAwake(); fillGame(); };

  // ---------- sounds ----------

  // Button sounds (sound.js), kept per device: on or off, where they play (here, or on one
  // of the PC's outputs, e.g. a headset, to hear them over the game) and how loud. The PC
  // lists its outputs; this device only picks one by name, and the PC plays only on those.
  const S = window.DeckSound;
  let lastPC = null; // the PC's last answer, to redraw the list in another language
  function fillSounds() {
    $("soundBtn").setAttribute("aria-checked", String(S.on));
    $("soundOut").disabled = $("soundVol").disabled = !S.on;
    $("soundVol").value = S.volume;
    $("soundVolText").textContent = `${S.volume}%`;
    showOutputs(lastPC);
    if (DEMO) { $("soundNote").textContent = tr("With the app, sounds can also play on your PC, e.g. in your headset."); return; }
    // Asking starts the PC's sound player, which isn't needed while sounds are off.
    if (!S.on) return;
    fetch(`/sound-devices.json?t=${encodeURIComponent(token)}`)
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then(showOutputs)
      .catch(() => {});
  }
  // `pc` is what the PC said (null while asking, or in demo mode).
  function showOutputs(pc) {
    if (pc) lastPC = pc;
    const pick = $("soundOut");
    pick.innerHTML = "";
    const add = (value, text) => { const o = document.createElement("option"); o.value = value; o.textContent = text; pick.appendChild(o); };
    add("", tr("This device"));
    const works = pc && pc.state !== "unavailable";
    if (works) {
      const preferred = pc.devices[pc.preferred];
      add("pc:", preferred ? tr("PC: default output ({0})", preferred) : tr("PC: default output"));
      for (const name of pc.devices) add(`pc:${name}`, tr("PC: {0}", name));
    }
    const current = S.out === "pc" ? `pc:${S.device}` : "";
    // Keep the choice showing while the list loads, or when that output is unplugged.
    if (![...pick.options].some((o) => o.value === current)) {
      add(current, S.device ? tr(works ? "PC: {0} (not connected)" : "PC: {0}", S.device) : tr("PC: default output"));
    }
    pick.value = current;
    if (DEMO) return;
    $("soundNote").textContent = pc && !works
      ? tr("The PC can't play sounds{0}, so they play on this device.", pc.reason ? ` (${pc.reason})` : "")
      : tr("On the PC, they play on the output you pick, e.g. your headset, so you hear them over the game.");
  }
  $("soundBtn").onclick = () => { S.set(!S.on); fillSounds(); };
  $("soundOut").onchange = (e) => {
    const v = e.target.value;
    if (v.startsWith("pc:")) S.route("pc", v.slice(3)); else S.route("device");
  };
  $("soundVol").oninput = (e) => { $("soundVolText").textContent = `${e.target.value}%`; };
  $("soundVol").onchange = (e) => S.setVolume(Number(e.target.value));
  // Sounds for this device reach the PC over the deck's connection.
  if (!DEMO) S.connect((msg) => { if (!connected()) return false; send(msg); return true; });

  // "Open on PC": websites set up on the PC (Settings → Links), opened in its browser. The
  // tablet only gets their names and asks by position, so it can't open anything else.
  function fillLinks() {
    const names = DEMO ? Promise.resolve(["UEX", "Erkul", "SPViewer", "SCMDB"])
      : fetch(`/links.json?t=${encodeURIComponent(token)}`).then((r) => (r.ok ? r.json() : Promise.reject())).then((d) => d.links);
    names.then((list) => {
      const box = $("pcLinkBtns");
      box.innerHTML = "";
      list.forEach((name, index) => {
        const b = document.createElement("button");
        b.type = "button";
        b.textContent = name;
        b.onclick = () => {
          if (DEMO) { toast(tr("With the app, this opens {0} on your PC", name), 2600); return; }
          buzz();
          send({ t: "open", index });
        };
        box.appendChild(b);
      });
      $("pcLinks").hidden = !list.length;
    }).catch(() => { $("pcLinks").hidden = true; });
  }
  const closePicker = () => $("picker").classList.remove("open");
  // The same sliders icon as the HUD layouts' settings button, so it reads as the way to
  // other layouts too, not just a theme switcher.
  $("settingsIco").innerHTML = window.deckIcon("sliders");
  $("themeBtn").onclick = openPicker;
  $("closePicker").onclick = closePicker;

  // ---------- layout choice ----------

  // Each device can pick its own layout (e.g. Combat on one tablet, Mining on another);
  // "" means the one chosen on the PC. Kept per device type, since phones have their own.
  const layoutKey = `layout-${device}`;
  let chosenLayout = store.get(layoutKey) || "";
  function fillLayoutPick() {
    if (DEMO) {
      const pick = $("layoutPick");
      pick.innerHTML = "";
      for (const [file, name] of DEMO_LAYOUTS) { const o = document.createElement("option"); o.value = file; o.textContent = tr(name); pick.appendChild(o); }
      pick.value = DEMO_LAYOUTS.some(([f]) => f === chosenLayout) ? chosenLayout : DEMO_LAYOUTS[0][0];
      return;
    }
    fetch(`/layouts.json?t=${encodeURIComponent(token)}&device=${device}`)
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((data) => {
        const pick = $("layoutPick");
        pick.innerHTML = "";
        const pcName = (data.layouts.find((l) => l.file === data.pcChoice) || {}).name || data.pcName || "default";
        const add = (value, text) => { const o = document.createElement("option"); o.value = value; o.textContent = text; pick.appendChild(o); };
        add("", tr("PC's choice: {0}", tr(pcName)));
        for (const l of data.layouts) add(l.file, l.custom ? tr("{0} (yours)", l.name) : tr(l.name));
        pick.value = data.layouts.some((l) => l.file === chosenLayout) ? chosenLayout : "";
      })
      .catch(() => {});
  }
  $("layoutPick").onchange = (e) => {
    closePicker();
    useLayout(e.target.value);
  };

  // ---------- language ----------

  // Kept per device, like the layout. Switching redraws everything in place, without the
  // reload that would replay the intro.
  const langPick = $("langPick");
  for (const [code, name] of window.DeckI18n.languages) {
    const o = document.createElement("option");
    o.value = code;
    o.textContent = name;
    langPick.appendChild(o);
  }
  langPick.value = window.DeckI18n.lang;
  langPick.onchange = () => {
    window.DeckI18n.set(langPick.value);
    window.DeckI18n.apply();
    buildPicker();
    applyTheme(document.documentElement.dataset.theme);
    fillLayoutPick();
    refreshStatus();
    fillSounds();
    fillAwake();
    if (document.body.classList.contains("pairing")) showPairing(false);
    else if (layout) showLayout(layout);
  };

  // ---------- swipes and gestures ----------

  // Swiping sideways with two fingers over one half of the HUD flips that half to its next
  // or previous panel, like its arrows (on tabbed layouts, to the next or previous tab;
  // in a grid layout's one-panel view, the panel). Two fingers swiping up or down do what
  // this device picked in Deck settings: one tap of the mobiGlas button, the Quick actions
  // sheet, or a jump to a panel. On grid layouts one finger can also swipe sideways
  // anywhere (Deck settings, off by default), except on sliders, dials and other controls
  // that are dragged ([data-drag]).
  //
  // A swipe never presses anything. While swipes are on, a hold button's press waits
  // SETTLE_MS and a tap button taps when the finger lifts: a second finger landing
  // meanwhile, or the finger sliding off (SLIDE_PX), makes it no press at all; a plain tap,
  // quick or slow, is still one press. A finger already holding something (hold to talk, a slider) isn't part of a
  // swipe, so another finger can still press beside it. Two-finger swipes are on by
  // default on grid layouts and off on 2.0 layouts, so their presses stay instant.
  const SETTLE_MS = 120;  // how long a press waits for a second finger
  const SLOP_PX = 14;     // sideways travel that makes one finger a swipe
  const PAIR_MS = 250;    // two fingers landing this close together can be a swipe
  const SLIDE_PX = 24;    // a finger that slides this far off where it landed isn't a tap
  const gridLayout = () => Boolean(layout && window.DeckGrid && window.DeckGrid.isGrid(layout));
  const sizeNow = () => (deckCanvas && deckCanvas.sizeClass ? deckCanvas.sizeClass() : device === "phone" ? "small" : "large");
  // Grid layouts keep their settings per size class, 2.0 layouts per device type as before.
  const twoKey = () => (gridLayout() ? `swipe-two-${sizeNow()}` : `swipe-${device}`);
  const oneKey = () => `swipe-any-${sizeNow()}`;
  const twoOn = () => (gridLayout() ? store.get(twoKey()) !== "off" : store.get(twoKey()) === "on");
  const oneOn = () => gridLayout() && store.get(oneKey()) === "on";
  const swipesOn = () => twoOn() || oneOn();
  const fingers = new Map();  // pointerId -> { x0, y0, x, y, t0, target, drag, pressed }
  const settling = new Map(); // pointerId -> cancels its press that is still waiting
  let gesture = null;         // { kind: "two" | "one", ids, sx, sy, x, y, t0, dead }
  let swiping = false;        // a swipe owns the fingers now: press nothing on lift
  const isSwiping = () => swiping;
  // Only touches on the deck itself count, not its sheets, menus or the Edit layout veil.
  const onDeck = (t) => t instanceof Element && Boolean(t.closest("#canvas, #grid"))
    && !t.closest(".hud-menu, .plates-veil, .plates-seam, .plates-bar, #pair");
  const centroid = (ids) => {
    let x = 0, y = 0, n = 0;
    for (const id of ids) { const f = fingers.get(id); if (f) { x += f.x; y += f.y; n++; } }
    return n ? { x: x / n, y: y / n } : null;
  };
  // A finger whose press went (or that holds the mic) is busy, not part of a swipe.
  const holding = (pointerId) => { const f = fingers.get(pointerId); if (f) f.pressed = true; };

  function startGesture(kind, ids, at) {
    gesture = { kind, ids, sx: at.x, sy: at.y, x: at.x, y: at.y, t0: performance.now(), dead: false };
    swiping = true;
    $("canvas").classList.add("swiping");
    for (const id of ids) {
      const cancel = settling.get(id);
      if (cancel) cancel(); // its press hadn't gone yet: now it never will
    }
    // A control the finger had started dragging (a slider, a widget) lets go, as if the
    // touch had been canceled. After this event's own handlers have run, so a control the
    // second finger just touched lets go too.
    setTimeout(() => {
      for (const id of ids) {
        const f = fingers.get(id);
        if (!f || !f.target || f.target.closest(".hud-btn")) continue;
        try { f.target.dispatchEvent(new PointerEvent("pointercancel", { pointerId: id, pointerType: "touch", bubbles: true })); } catch { /* old browser */ }
      }
    }, 0);
  }

  document.addEventListener("pointerdown", (e) => {
    if (!e.isTrusted || e.pointerType === "mouse") return;
    // The first finger of a new touch: forget any finger whose lift was missed, so a stale
    // one can't turn every later tap into a "second finger" and swallow it.
    if (e.isPrimary) { fingers.clear(); if (!gesture) swiping = false; }
    if (!onDeck(e.target) || !swipesOn()) return;
    const now = performance.now();
    const f = { x0: e.clientX, y0: e.clientY, x: e.clientX, y: e.clientY, t0: now, target: e.target,
      drag: Boolean(e.target.closest("[data-drag]")), pressed: false, extra: Boolean(gesture) };
    fingers.set(e.pointerId, f);
    if (gesture || !twoOn() || !(deckCanvas || (layout && layout.pages))) return;
    // Two fingers that landed together and aren't busy are a two-finger swipe.
    const free = [...fingers].filter(([, g]) => !g.extra && !g.pressed && now - g.t0 <= PAIR_MS);
    if (free.length >= 2) {
      const ids = free.slice(-2).map(([id]) => id);
      startGesture("two", ids, centroid(ids));
    }
  }, true);

  document.addEventListener("pointermove", (e) => {
    if (!e.isTrusted) return;
    const f = fingers.get(e.pointerId);
    if (!f) return;
    f.x = e.clientX; f.y = e.clientY;
    if (gesture) {
      // Two fingers move the swipe while both are down; once one lifts it stays put.
      if (gesture.ids.includes(e.pointerId) && gesture.ids.every((id) => fingers.has(id))) Object.assign(gesture, centroid(gesture.ids));
      return;
    }
    if (!oneOn() || fingers.size !== 1 || f.drag || f.pressed || !deckCanvas) return;
    const dx = f.x - f.x0, dy = f.y - f.y0;
    if (Math.abs(dx) > SLOP_PX && Math.abs(dx) > Math.abs(dy)) {
      startGesture("one", [e.pointerId], { x: f.x0, y: f.y0 });
      Object.assign(gesture, { x: f.x, y: f.y, t0: f.t0 });
    }
  }, true);

  const lift = (e) => {
    if (!e.isTrusted || !fingers.has(e.pointerId)) return;
    fingers.delete(e.pointerId);
    if (!gesture || !gesture.ids.includes(e.pointerId)) return;
    // The browser took the touch over (it scrolled a panel): that's no swipe.
    if (e.type === "pointercancel") gesture.dead = true;
    if (gesture.ids.some((id) => fingers.has(id))) return;
    const g = gesture;
    gesture = null;
    // Still a swipe for this lift's own handlers (a widget's tap on lift checks it).
    setTimeout(() => { if (!gesture) { swiping = false; $("canvas").classList.remove("swiping"); } }, 0);
    if (!g.dead) finishGesture(g);
  };
  document.addEventListener("pointerup", lift, true);
  document.addEventListener("pointercancel", lift, true);

  // Far enough sideways, and more sideways than up or down, it pages (left: next, as on a
  // phone); a quick flick needs less. Two fingers up or down run their gesture.
  function finishGesture(g) {
    const dx = g.x - g.sx, dy = g.y - g.sy;
    if (g.kind === "two" && !hintSeen() && Math.hypot(dx, dy) >= 60) hideSwipeHint(true);
    const far = sizeNow() === "small" ? 60 : 90;
    const flick = Math.abs(dx) >= 30 && Math.abs(dx) / Math.max(1, performance.now() - g.t0) > 0.5;
    if ((Math.abs(dx) >= far || flick) && Math.abs(dy) < Math.abs(dx) * 0.6) {
      const step = dx < 0 ? 1 : -1;
      if (deckCanvas) deckCanvas.pageAt(g.sx, g.sy, step);
      else if (layout && layout.pages) {
        pageIndex = (pageIndex + step + layout.pages.length) % layout.pages.length;
        store.set("page", pageIndex);
        renderTabs();
        renderPage();
      }
      return;
    }
    if (g.kind === "two" && deckCanvas && Math.abs(dy) >= far && Math.abs(dx) < Math.abs(dy) * 0.6) runGesture(dy < 0 ? "up" : "down");
  }

  // When a grid layout opens on a touch screen, a card above the pager says what two
  // fingers do (they're easy to miss), until "Got it" or the first two-finger swipe.
  const hintSeen = () => store.get("swipe-hint") === "seen";
  function hideSwipeHint(seen) {
    clearTimeout(hideSwipeHint.timer);
    $("swipeHint").hidden = true;
    if (seen) store.set("swipe-hint", "seen");
  }
  function maybeSwipeHint() {
    clearTimeout(hideSwipeHint.timer);
    const touch = navigator.maxTouchPoints > 0 || matchMedia("(pointer: coarse)").matches;
    if (hintSeen() || !touch || !gridLayout() || !twoOn() || !deckCanvas) { $("swipeHint").hidden = true; return; }
    const name = (what) => (what === "mobiglas" ? tr("Open mobiGlas") : what === "quick" ? tr("Quick actions")
      : what === "primary" ? tr("Primary panel") : what.startsWith("panel:") ? tr("Go to {0}", tr(((deckCanvas.panelList ? deckCanvas.panelList() : []).find((p) => `panel:${p.id}` === what) || {}).title || what.slice(6))) : "");
    const ud = [["up", "Up: {0}"], ["down", "Down: {0}"]].map(([d, f]) => (name(gestureOf(d)) ? tr(f, name(gestureOf(d))) : "")).filter(Boolean).join(" · ");
    const el = $("swipeHint");
    el.querySelector("b").textContent = tr("Swipe with two fingers");
    el.querySelector(".sh-side").textContent = tr("Sideways: next or previous panel");
    el.querySelector(".sh-ud").textContent = ud;
    el.querySelector(".sh-ud").hidden = !ud;
    el.querySelector(".sh-ok").textContent = tr("Got it");
    // It steps aside by itself after a while (and comes back next time) so it never sits
    // over a control for long.
    hideSwipeHint.timer = setTimeout(() => { el.hidden = false; hideSwipeHint.timer = setTimeout(() => hideSwipeHint(false), 15000); }, 900);
  }
  $("swipeHint").querySelector(".sh-ok").onclick = () => hideSwipeHint(true);

  // What a two-finger swipe up or down does, per device. Each sends at most one tap.
  const GESTURE_DEFAULTS = { up: "mobiglas", down: "quick" };
  const gestureOf = (dir) => store.get(`gesture-${dir}`) || GESTURE_DEFAULTS[dir];
  function buttonInLayout(id) {
    let found = null;
    (function walk(n) {
      if (found || !n || typeof n !== "object") return;
      if (Array.isArray(n)) { n.forEach(walk); return; }
      if (n.id === id && typeof n.keys === "string") { found = n; return; }
      for (const v of Object.values(n)) if (v && typeof v === "object") walk(v);
    })(layout);
    return found;
  }
  function runGesture(dir) {
    const what = gestureOf(dir);
    if (what === "mobiglas") {
      const btn = buttonInLayout("top-mobiglas");
      if (!btn) { toast("This layout has no mobiGlas button"); return; }
      if (!tapStep(btn)) return;
      for (const el of document.querySelectorAll('[data-btn="top-mobiglas"]')) {
        el.classList.add("pressed");
        setTimeout(() => el.classList.remove("pressed"), 180);
      }
    } else if (what === "quick") {
      if (deckCanvas.quickActions && deckCanvas.quickActions()) window.DeckSound.play("page");
    } else if (what === "primary") {
      deckCanvas.showPanel(deckCanvas.primaryPanel());
    } else if (what.startsWith("panel:")) {
      if (!deckCanvas.showPanel(what.slice(6))) toast("That panel isn't in this layout");
    }
  }

  // Deck settings: the swipe switches and gestures follow the layout shown.
  function fillSwipes() {
    const grid = gridLayout();
    $("swipeBtn").setAttribute("aria-checked", String(twoOn()));
    $("swipe1Btn").hidden = !grid;
    $("swipe1Btn").setAttribute("aria-checked", String(oneOn()));
    $("densityRow").hidden = !grid;
    if (grid) {
      const v = store.get(`grid-density-${sizeNow()}`);
      $("densityPick").value = ["compact", "default", "roomy"].includes(v) ? v : "default";
    }
    const canGesture = Boolean(deckCanvas) && twoOn();
    $("gestures").hidden = !deckCanvas;
    const panels = deckCanvas && deckCanvas.panelList ? deckCanvas.panelList() : [];
    for (const dir of ["up", "down"]) {
      const pick = $(dir === "up" ? "gestureUp" : "gestureDown");
      pick.innerHTML = "";
      const add = (value, label) => { const o = document.createElement("option"); o.value = value; o.textContent = label; pick.appendChild(o); };
      add("none", tr("Nothing"));
      add("mobiglas", tr("Open mobiGlas"));
      add("quick", tr("Quick actions"));
      add("primary", tr("Primary panel"));
      for (const p of panels) add(`panel:${p.id}`, tr("Go to {0}", tr(p.title)));
      const cur = gestureOf(dir);
      if (![...pick.options].some((o) => o.value === cur)) add(cur, tr("Go to {0}", cur.replace(/^panel:/, "")));
      pick.value = cur;
      pick.disabled = !canGesture;
    }
  }
  $("swipeBtn").onclick = () => { store.set(twoKey(), twoOn() ? "off" : "on"); fillSwipes(); };
  $("swipe1Btn").onclick = () => { store.set(oneKey(), oneOn() ? "off" : "on"); fillSwipes(); };
  $("gestureUp").onchange = (e) => store.set("gesture-up", e.target.value);
  $("gestureDown").onchange = (e) => store.set("gesture-down", e.target.value);
  $("densityPick").onchange = (e) => {
    store.set(`grid-density-${sizeNow()}`, e.target.value);
    if (deckCanvas) deckCanvas.render();
  };

  // Button labels: icons and labels, or icons only, kept per device. A layout can set a
  // panel, cluster or button its own way ("labels" in the layout editor), which wins; a
  // button without an icon always keeps its label. canvas.js reads "labels".
  $("labelsPick").value = store.get("labels") === "icons" ? "icons" : "show";
  $("labelsPick").onchange = (e) => {
    store.set("labels", e.target.value === "icons" ? "icons" : "");
    if (deckCanvas) deckCanvas.render();
  };

  // Keep screen on (awake.js): the switch, how the screen is kept on right now, and what
  // to set on this kind of device when a browser can't keep it on by itself.
  const AWAKE_STATE = {
    idle: "Starts with your first tap on the deck.",
    lock: "Kept on with the browser's wake lock.",
    video: "Kept on with a silent, hidden video.",
    none: "This browser can't keep the screen on. Try the tip below.",
    off: "Off: the screen dims and locks as the device is set.",
  };
  const platform = (() => {
    const ua = navigator.userAgent || "";
    if (/iPad|iPhone|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1)) return "ios";
    if (/Android/.test(ua)) return "android";
    return "";
  })();
  const AWAKE_TIPS = {
    ios: ["iPad and iPhone: Settings > Display & Brightness > Auto-Lock > Never, or Guided Access (Settings > Accessibility) to keep the deck on screen."],
    android: ["Android: set a longer Screen timeout in Settings > Display, or turn on Stay awake (while charging) in Developer options."],
  };
  let awakeHooked = false;
  function fillAwake() {
    const A = window.DeckAwake;
    $("awakeBtn").hidden = !A;
    document.querySelector(".sheet-awake").hidden = !A; // inside the apps, which keep the screen on
    if (!A) return;
    if (!awakeHooked) { awakeHooked = true; A.onChange(fillAwake); }
    $("awakeBtn").setAttribute("aria-checked", String(A.on()));
    $("awakeNote").textContent = tr(AWAKE_STATE[A.status()] || AWAKE_STATE.idle);
    $("awakeNote").dataset.state = A.status();
    const tips = AWAKE_TIPS[platform] || [...AWAKE_TIPS.ios, ...AWAKE_TIPS.android];
    $("awakeTip").textContent = tips.map((t) => tr(t)).join(" ");
  }
  $("awakeBtn").onclick = () => { const A = window.DeckAwake; if (A) { A.setOn(!A.on()); fillAwake(); } };

  // Follow Stream Deck scenes: on unless turned off on this device.
  const followScenes = () => store.get("follow-scenes") !== "off";
  $("followBtn").setAttribute("aria-checked", String(followScenes()));
  $("followBtn").onclick = () => {
    const on = !followScenes();
    store.set("follow-scenes", on ? "" : "off");
    $("followBtn").setAttribute("aria-checked", String(on));
  };

  // The other way: a HUD half the player pages (its arrows, a two-finger swipe) tells
  // Stream Decks with the SpoolUp plugin, which go to the scene with that panel. Only
  // the panel it settles on, once paging stops for PAGED_MS, so flipping through several
  // sends one, and nothing if it ends where it started. A Stream Deck's scene, a layout
  // loading and connecting again send nothing, or the two would page each other back.
  const PAGED_MS = 400;
  const paging = {}; // side -> { from, timer } while a half is being paged
  function panelPaged(side, panel, from) {
    if (side !== "left" && side !== "right") return; // only the two halves of a HUD
    const p = paging[side] || (paging[side] = { from });
    clearTimeout(p.timer);
    p.timer = setTimeout(() => {
      delete paging[side];
      if (panel !== p.from) send({ t: "panel", side, panel });
    }, PAGED_MS);
  }
  function stopPaging() {
    for (const side of Object.keys(paging)) { clearTimeout(paging[side].timer); delete paging[side]; }
  }


  // Night mode dims the whole deck for playing in a dark room; each device remembers it.
  function applyNight(on) {
    document.documentElement.toggleAttribute("data-night", on);
    $("nightBtn").setAttribute("aria-checked", String(on));
    store.set("night", on ? "1" : "");
  }
  $("nightBtn").onclick = () => applyNight(!document.documentElement.hasAttribute("data-night"));
  $("resetStatesBtn").onclick = resetStates;
  // Edit layout: unlocks the seam between the HUD's halves until Lock (canvas.js).
  $("editPlatesBtn").onclick = () => {
    if (!deckCanvas || !deckCanvas.editPlates(true)) { toast("This layout has no halves to resize"); return; }
    closePicker();
    toast("Drag the seam, then Lock", 2600);
  };
  applyNight(store.get("night") === "1");

  // Large text, high contrast and key labels (each button's key in a corner, for learning
  // or checking binds); each device remembers them.
  for (const [btn, attr] of [["largeBtn", "data-large"], ["contrastBtn", "data-contrast"], ["keysBtn", "data-keys"]]) {
    const apply = (on) => {
      document.documentElement.toggleAttribute(attr, on);
      $(btn).setAttribute("aria-checked", String(on));
      store.set(attr, on ? "1" : "");
    };
    $(btn).onclick = () => apply(!document.documentElement.hasAttribute(attr));
    apply(store.get(attr) === "1");
  }

  // Button lights: on unless turned off on this device. Kept as "data-no-lights" so the
  // default (no attribute) is lit.
  function applyLights(on, refresh) {
    document.documentElement.toggleAttribute("data-no-lights", !on);
    $("lightsBtn").setAttribute("aria-checked", String(on));
    store.set("lights", on ? "" : "off");
    if (refresh) showStates(); // not at start: the buttons and their states come later
  }
  $("lightsBtn").onclick = () => applyLights(!lightsOn(), true);
  applyLights(store.get("lights") !== "off", false);

  // ---------- connection ----------

  // Status shows in the header (grid layouts) and in canvas chrome, so update every copy.
  // While connected it adds the round trip to the PC ("Connected · 8 ms"), and the dot
  // turns amber when that's slow enough to feel.
  const SLOW_MS = 120;
  let online = { on: false, text: "Connecting…" };
  let rtt = null; // smoothed round trip in ms, from the heartbeats; null until measured
  function setOnline(on, text) {
    if (on !== online.on && !DEMO) window.DeckSound.play(on ? "online" : "offline");
    online = { on, text };
    if (!on) rtt = null;
    if (!on && window.DeckBattle) window.DeckBattle.offline(); // battle alerts may be out of date
    refreshStatus();
  }
  function refreshStatus() {
    const shown = online.on && rtt !== null;
    const text = tr(online.text) + (shown ? ` · ${Math.max(1, Math.round(rtt))} ms` : "");
    document.body.classList.toggle("offline", !online.on);
    for (const d of document.querySelectorAll("#dot, .js-dot")) {
      d.classList.toggle("on", online.on);
      d.classList.toggle("slow", shown && rtt > SLOW_MS);
    }
    for (const t of document.querySelectorAll("#statusText, .js-status")) t.textContent = text;
  }

  const connected = () => Boolean(ws && ws.readyState === WebSocket.OPEN);
  function send(obj) {
    if (connected()) ws.send(JSON.stringify(obj));
  }

  // Short notice at the bottom of the screen, e.g. when the PC skipped a press.
  let toastTimer = null;
  function toast(text, ms = 1800) {
    const el = $("toast");
    el.textContent = tr(text);
    // On canvas layouts, sit just above the pager bars so a message never covers the page names.
    const pager = document.querySelector("#canvas .hud-pager");
    el.style.bottom = pager ? `${Math.round(innerHeight - pager.getBoundingClientRect().top + 10)}px` : "";
    el.classList.add("show");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.classList.remove("show"), ms);
  }

  // Heartbeats go both ways: the PC drops a tablet it hasn't heard from for 5 s (which
  // releases its keys) and answers each one. A link can die without closing (the PC
  // asleep, Wi-Fi gone), and would then still look connected while presses go nowhere, so
  // with no answer for DEAD_MS the deck gives up on it and connects again. Only once this
  // PC has answered at all, so a PC too old to answer never looks dead.
  const DEAD_MS = 5000;
  let lastHeard = 0, answers = false;

  function connect() {
    const sock = new WebSocket(`ws://${location.host}/ws?t=${encodeURIComponent(token)}&device=${device}&layout=${encodeURIComponent(chosenLayout)}`);
    ws = sock;
    answers = false;
    let opened = false;
    // A connection attempt that hangs (the PC gone from the network) counts as failed.
    const giveUp = setTimeout(() => { if (sock.readyState === WebSocket.CONNECTING) sock.close(); }, DEAD_MS);
    sock.onopen = () => {
      S.announce(); // before the "online" sound, which may play on the PC
      opened = true;
      clearTimeout(giveUp);
      lastHeard = performance.now();
      setOnline(true, "Connected");
      send({ t: "hb", n: performance.now() }); // measure the round trip straight away
    };
    sock.onclose = () => {
      clearTimeout(giveUp);
      if (ws !== sock) return;
      ws = null;
      setOnline(false, "Reconnecting…");
      if (opened) setTimeout(connect, 1000); else retryOrPair();
    };
    sock.onerror = () => sock.close();
    sock.onmessage = (e) => {
      let msg;
      try { msg = JSON.parse(e.data); } catch { return; }
      lastHeard = performance.now();
      if (msg.t === "hb") {
        answers = true;
        const sample = performance.now() - msg.n;
        if (sample >= 0 && sample < 10000) rtt = rtt === null ? sample : rtt * 0.7 + sample * 0.3;
        refreshStatus();
      }
      else if (msg.t === "blocked") { toast("Star Citizen not focused"); buzz([30, 60, 30]); window.DeckSound.play("deny"); denied(msg.id); }
      // The player cleared this bind in Star Citizen, so the PC pressed nothing.
      else if (msg.t === "unbound") { toast("Not bound in Star Citizen", 2600); buzz([30, 60, 30]); window.DeckSound.play("deny"); denied(msg.id); }
      // The PC let go of a hold: Star Citizen lost focus, or it was held for 2 minutes.
      else if (msg.t === "released") toast(msg.reason === "focus" ? "Keys released: Star Citizen lost focus" : "Keys released after 2 minutes", 2600);
      else if (msg.t === "hello") { applyVoiceStatus(msg.voiceStatus || { state: msg.voice ? "ready" : "off" }); states = msg.states || {}; showStates(); S.status(msg.soundStatus); gameInUse = msg.game || null; fillGame(); }
      else if (msg.t === "game") { gameInUse = msg.game || null; fillGame(); }
      else if (msg.t === "sound-status") S.status(msg);
      // A scene picked on a Stream Deck (its SpoolUp plugin): flip to its panels. It's
      // newer than a page still waiting to go to the Stream Deck, which would undo it.
      else if (msg.t === "scene") { stopPaging(); if (followScenes() && deckCanvas && deckCanvas.showPanels(msg)) window.DeckSound.play("page"); }
      else if (msg.t === "state") { states[msg.group] = msg.on; showStates(msg.group); }
      else if (msg.t === "states") { states = msg.states || {}; showStates(); }
      else if (msg.t === "voice-status") applyVoiceStatus(msg);
      else if (msg.t === "heard") { toast(tr("Heard: {0}", msg.text || tr(msg.label))); window.DeckSound.play("heard"); }
      else if (msg.t === "unheard") { toast("Didn't catch that"); window.DeckSound.play("unheard"); }
      else if (msg.t === "voice-off") toast(msg.reason);
      // Push-to-talk held on the PC: the mic buttons light up as if held here.
      else if (msg.t === "listening") {
        for (const el of document.querySelectorAll(".js-talk")) el.classList.toggle("listening", msg.on);
        // The PC chimes on its own for push-to-talk, so a tablet playing its sounds on the PC
        // doesn't add another.
        if (msg.on && !(msg.chime && S.out === "pc")) window.DeckSound.play("listen");
      }
      else if (msg.t === "reload") load().then(reconnect); // the layout was changed in the editor
      else if (msg.t === "opened") toast(tr(msg.dryRun ? "{0}: test mode, not opened" : "Opened {0} on the PC", msg.name));
      // Battle alerts from Star Citizen's Game.log (battle.js); they press nothing.
      else if (msg.t === "battle") { if (window.DeckBattle) window.DeckBattle.update(msg); }
    };
  }

  // A connection that never opened may mean the pairing code changed on the PC, or that
  // this device is locked out after wrong codes. Each try would count as another wrong
  // code, so ask once over HTTP, which says which, and show the pairing screen then.
  function retryOrPair() {
    fetch(`/links.json?t=${encodeURIComponent(token)}`, { cache: "no-store" })
      .then((r) => {
        if (r.status === 403) showPairing("changed");
        else if (r.status === 429) showPairing("locked");
        else setTimeout(connect, 1000);
      })
      .catch(() => setTimeout(connect, 1000));
  }

  setInterval(() => {
    if (!connected()) return;
    if (answers && performance.now() - lastHeard > DEAD_MS) {
      const dead = ws;
      dead.onclose = dead.onmessage = dead.onerror = null;
      dead.close();
      ws = null;
      setOnline(false, "Reconnecting…");
      connect();
      return;
    }
    send({ t: "hb", n: performance.now() });
  }, 1500);

  // ---------- buttons ----------

  function renderTabs() {
    tabs.innerHTML = "";
    layout.pages.forEach((p, i) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = tr(p.name);
      b.className = i === pageIndex ? "active" : "";
      b.onclick = () => { pageIndex = i; store.set("page", i); renderTabs(); renderPage(); };
      tabs.appendChild(b);
    });
  }

  const modeLabel = { tap: "", hold: "Hold", timed: "Long press" };
  const keyLabel = (keys) => keys.split("+").map((k) => k.replace(/^np_/, "num ").replace("period", ".")).join(" + ");

  function renderPage() {
    grid.innerHTML = "";
    for (const btn of layout.pages[pageIndex].buttons) {
      const el = document.createElement("button");
      el.type = "button";
      el.className = "key";
      el.dataset.color = btn.color || "";
      el.innerHTML = `<span class="glyph">${window.deckIcon(btn.icon || btn.id)}</span>
        <span class="label"></span>
        <span class="foot"><span class="mode"></span><span class="keycap"></span></span>`;
      el.querySelector(".label").textContent = tr(btn.label);
      el.querySelector(".keycap").textContent = keyLabel(btn.keys);
      el.querySelector(".mode").textContent = tr(modeLabel[btn.mode] || "");
      bindButton(el, btn);
      grid.appendChild(el);
    }
  }

  // Inside the SpoolUp iPhone/iPad and Android apps, window.SpoolUpApp plays the device's own
  // haptics (iOS Safari has no navigator.vibrate). The browser deck never has it.
  const appHaptic = (ms) => (Array.isArray(ms) ? (ms[0] >= 30 ? "error" : "press") : ms >= 30 ? "press" : "tap");
  const buzz = (ms = 12) => {
    const app = window.SpoolUpApp;
    if (app && typeof app.haptic === "function") { try { app.haptic(appHaptic(ms)); } catch {} return; }
    if (navigator.vibrate) navigator.vibrate(ms);
  };

  // ---------- lit buttons ----------

  // On/off state per group (doors open, gear down), kept by the PC so every tablet and
  // voice command agree. Buttons with a "state" light up while their group is on.
  let states = {};
  // With Button lights off (Deck settings), everything shows as off and stays that way.
  const lightsOn = () => !document.documentElement.hasAttribute("data-no-lights");
  // Controls drawn from the state itself (the gear lever's position) get it as it is, and
  // whether lights are on, as a third argument; buttons only see it while lights are on.
  function showStates(group) {
    for (const el of document.querySelectorAll("[data-state-group]")) {
      if (group !== undefined && el.dataset.stateGroup !== group) continue;
      const on = Boolean(states[el.dataset.stateGroup]);
      if (el._rawState) el._showState(on, group !== undefined, lightsOn());
      else el._showState(lightsOn() && on, group !== undefined && lightsOn());
    }
  }
  // Demo mode has no PC, so presses change the state here, the way server.js does.
  function demoState(btn) {
    const s = btn.state;
    states[s.group] = typeof s.set === "boolean" ? s.set : !states[s.group];
    showStates(s.group);
  }
  function resetStates() {
    if (DEMO) { states = {}; showStates(); } else send({ t: "reset-states" });
    toast("Button lights reset");
  }

  // The PC skipped a press because Star Citizen wasn't in front: besides the toast, every
  // copy of that button on screen flashes red and stops any charge or arming it started.
  function denied(id) {
    for (const el of document.querySelectorAll("[data-btn]")) if (el.dataset.btn === id) el._deny();
    for (const el of document.querySelectorAll("[data-btns]")) if (el.dataset.btns.split(" ").includes(id)) el._deny(id);
  }

  // One tap of a button from a control that isn't a button (the gear lever, a slider's step,
  // the mode dial): the same message a tapped button sends, so the PC's focus check, press
  // scheduling and release all apply. Returns whether it went; offline it says so instead.
  // Controls call it once per step the player makes; nothing here repeats.
  function tapStep(btn) {
    if (!DEMO && !connected()) { toast("Not connected to the PC"); buzz([30, 60, 30]); window.DeckSound.play("deny"); return false; }
    buzz();
    window.DeckSound.play("press", btn.color);
    send({ t: "tap", id: btn.id });
    deckCanvas?.pressed?.(btn.id); // controls following it update (a cluster's Reset)
    if (DEMO && btn.state && typeof btn.state.group === "string") demoState(btn);
    return true;
  }
  // A state group's lit state as buttons show it: off with Button lights off.
  const stateOf = (group) => lightsOn() && Boolean(states[group]);
  // A control showing a state group (the gear lever): app.js calls show(on, animate, lit)
  // whenever the PC's state for the group changes. `ids` are the buttons it taps, so a press
  // the PC skipped reaches deny(id).
  function bindControl(el, { group, show, ids, deny }) {
    if (ids && ids.length) { el.dataset.btns = ids.join(" "); el._deny = deny || (() => {}); }
    if (group) {
      el.dataset.stateGroup = group;
      el._rawState = true;
      el._showState = show;
      show(Boolean(states[group]), false, lightsOn());
    }
  }

  // While a finger holds a button or the mic, its lift is heard on the document too: a
  // redraw (paging with another finger, a Stream Deck scene, turning the screen) can take
  // the element away meanwhile, and the pointerup then goes to whatever is there now, which
  // would leave a hold button's keys down on the PC.
  const hearLift = (end, on) => { for (const type of ["pointerup", "pointercancel"]) document[on ? "addEventListener" : "removeEventListener"](type, end, true); };

  function bindButton(el, btn) {
    const fx = window.DeckFX.attach(el, btn);
    el.dataset.btn = btn.id;
    el._deny = fx.deny;
    // Cleared in the player's Star Citizen keybinds: dimmed, with a badge, until it's bound.
    if (btn.unbound) {
      el.classList.add("unbound");
      const badge = document.createElement("span");
      badge.className = "bind-badge";
      badge.textContent = tr("BIND");
      el.appendChild(badge);
    }
    if (btn.state && typeof btn.state.group === "string") {
      el.dataset.stateGroup = btn.state.group;
      el._showState = fx.show;
      fx.show(lightsOn() && Boolean(states[btn.state.group]), false);
    }
    let pointer = null;
    let waiting = null; // a press held back in case the touch turns into a swipe
    const press = (e) => {
      waiting = null;
      holding(e.pointerId);
      el.classList.add("pressed");
      // Offline, nothing reaches the PC: say so, rather than play the effect as if it had.
      if (!DEMO && !connected()) { toast("Not connected to the PC"); buzz([30, 60, 30]); window.DeckSound.play("deny"); return; }
      buzz();
      window.DeckSound.play("press", btn.color);
      send({ t: btn.mode === "hold" ? "down" : "tap", id: btn.id });
      fx.down(e);
      deckCanvas?.pressed?.(btn.id); // Max Weapons puts the Weapons slider at the top
      if (DEMO && el.dataset.stateGroup) demoState(btn);
    };
    el.addEventListener("pointerdown", (e) => {
      if (pointer !== null || swiping) return; // fingers that are part of a swipe press nothing
      pointer = e.pointerId;
      el.setPointerCapture(e.pointerId);
      hearLift(end, true);
      // A mouse can't swipe, and with swipes off nothing needs to wait.
      if (!swipesOn() || e.pointerType === "mouse" || !fingers.has(e.pointerId)) { press(e); return; }
      const id = e.pointerId;
      const cancel = () => { clearTimeout(waiting.timer); waiting = null; pointer = null; settling.delete(id); hearLift(end, false); el.classList.remove("pressed"); };
      // A hold button has to go down while the finger is down. A tap button lights up after
      // the wait but taps on lift, so a second finger landing late, or the finger sliding
      // off on a swipe, still cancels it: a swipe never presses what it started on.
      const onLift = btn.mode !== "hold";
      waiting = { cancel, x0: e.clientX, y0: e.clientY, timer: setTimeout(() => {
        if (onLift) { if (!swiping) el.classList.add("pressed"); return; }
        settling.delete(id);
        press(e);
      }, SETTLE_MS) };
      settling.set(id, cancel);
    });
    el.addEventListener("pointermove", (e) => {
      if (!waiting || e.pointerId !== pointer) return;
      if (Math.hypot(e.clientX - waiting.x0, e.clientY - waiting.y0) > SLIDE_PX) waiting.cancel();
    });
    const end = (e) => {
      if (e.pointerId !== pointer) return;
      hearLift(end, false);
      if (waiting) {
        // Lifted before the wait was over: still a press, just a quick one. A touch the
        // browser canceled (it scrolled), or one a swipe took, presses nothing.
        waiting.cancel();
        if (e.type === "pointercancel" || swiping) return;
        pointer = e.pointerId;
        press(e);
      }
      pointer = null;
      el.classList.remove("pressed");
      if (btn.mode === "hold") send({ t: "up", id: btn.id });
      fx.up();
    };
    el.addEventListener("pointerup", end);
    el.addEventListener("pointercancel", end);
    el.addEventListener("contextmenu", (e) => e.preventDefault());
  }

  // Hold-to-talk: the PC listens for a voice command while this is held down.
  function bindTalk(el) {
    let pointer = null;
    el.addEventListener("pointerdown", (e) => {
      if (pointer !== null) return;
      pointer = e.pointerId;
      el.setPointerCapture(e.pointerId);
      holding(e.pointerId);
      hearLift(end, true);
      el.classList.add("listening");
      buzz();
      window.DeckSound.play("listen");
      if (DEMO) toast("Voice commands work with SpoolUp on your PC", 2600);
      send({ t: "voice", on: true });
    });
    const end = (e) => {
      if (e.pointerId !== pointer) return;
      hearLift(end, false);
      pointer = null;
      el.classList.remove("listening");
      send({ t: "voice", on: false });
    };
    el.addEventListener("pointerup", end);
    el.addEventListener("pointercancel", end);
    el.addEventListener("contextmenu", (e) => e.preventDefault());
  }
  bindTalk($("talk"));

  // ---------- voice commands list ----------

  // The mic and "?" only show when voice can work: on, and starting or ready on the PC.
  let voiceStatus = { state: "starting" };
  function applyVoiceStatus(status) {
    voiceStatus = status;
    document.body.classList.toggle("no-voice", status.state === "off" || status.state === "unavailable");
    if ($("voiceHelp").classList.contains("open")) loadVoiceList();
  }

  // What can be said and which button it presses, from the PC (the same list the PC listens for).
  let voiceData = null;
  function openVoiceHelp() {
    $("voiceHelp").classList.add("open");
    $("voiceSearch").value = "";
    voiceNote("Loading…");
    loadVoiceList();
  }
  function loadVoiceList() {
    if (DEMO) { voiceData = { on: true, state: "ready", commands: phrasesInLayout() }; renderVoiceList(); return; }
    fetch(`/voice.json?t=${encodeURIComponent(token)}`)
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((data) => { voiceData = data; renderVoiceList(); })
      .catch(() => voiceNote("Couldn't load the list from the PC."));
  }
  const closeVoiceHelp = () => $("voiceHelp").classList.remove("open");

  // Demo mode has no PC to ask, so the list comes from the layout, the way server.js
  // builds it: each button's label as said aloud ("Shields +" is "shields up") plus its
  // "voice" phrases, leaving out buttons with "voice": false.
  const spoken = (label) => String(label).toLowerCase()
    .replace(/\s*\+$/, " up").replace(/\s*[−-]$/, " down").replace(/&/g, " and ")
    .replace(/[^a-z0-9' ]+/g, " ").replace(/\s+/g, " ").trim();
  function phrasesInLayout() {
    const seen = new Map();
    (function walk(n) {
      if (Array.isArray(n)) { n.forEach(walk); return; }
      if (!n || typeof n !== "object") return;
      if (typeof n.id === "string" && typeof n.keys === "string" && n.voice !== false) {
        for (const p of [n.label, ...(Array.isArray(n.voice) ? n.voice : [])].map(spoken)) if (p && !seen.has(p)) seen.set(p, n.label);
      }
      for (const v of Object.values(n)) if (v && typeof v === "object") walk(v);
    })(layout);
    return [...seen].sort((a, b) => a[1].localeCompare(b[1]) || a[0].localeCompare(b[0]));
  }

  function voiceNote(text) {
    const list = $("voiceList");
    list.innerHTML = `<p class="vh-note"></p>`;
    list.querySelector("p").textContent = tr(text);
  }

  // One card per button, with its phrases; the search matches phrases and button names.
  function renderVoiceList() {
    if (!voiceData) return;
    // The live status from the PC wins over the one the list was loaded with.
    const state = DEMO ? "ready" : voiceStatus.state;
    const reason = voiceStatus.reason || voiceData.reason;
    if (!voiceData.on || state === "off") { voiceNote("Voice commands are turned off on the PC. Turn them on in the SpoolUp window's Settings."); return; }
    if (state === "unavailable") {
      voiceNote(tr("Voice commands can't run on this PC{0}. The SpoolUp window shows what to do.", reason ? `: ${reason}` : ""));
      return;
    }
    const q = $("voiceSearch").value.trim().toLowerCase();
    const byLabel = new Map();
    for (const [phrase, label] of voiceData.commands) {
      if (q && !phrase.includes(q) && !label.toLowerCase().includes(q) && !tr(label).toLowerCase().includes(q)) continue;
      if (!byLabel.has(label)) byLabel.set(label, []);
      byLabel.get(label).push(phrase);
    }
    if (!byLabel.size) { voiceNote(q ? "Nothing matches that." : "The PC is still starting voice commands. Try again in a moment."); return; }
    const list = $("voiceList");
    list.innerHTML = "";
    for (const [label, phrases] of byLabel) {
      const card = document.createElement("div");
      card.className = "vh-card";
      card.innerHTML = `<div class="vh-label"></div><div class="vh-phrases"></div>`;
      card.querySelector(".vh-label").textContent = tr(label);
      for (const phrase of phrases) {
        const chip = document.createElement("span");
        chip.textContent = phrase;
        card.querySelector(".vh-phrases").appendChild(chip);
      }
      list.appendChild(card);
    }
  }
  $("voiceSearch").oninput = renderVoiceList;
  $("voiceHelpBtn").onclick = openVoiceHelp;
  $("closeVoiceHelp").onclick = closeVoiceHelp;

  // ---------- pairing + load ----------

  // Shown until the device has a working pairing code. The deck's own controls stay hidden
  // (body.pairing), since none of them work before pairing.
  // `wrong` is false, true (the typed code was wrong), "locked" or "changed" (the code
  // stopped working while the deck was open, so the deck makes way for the form).
  function showPairing(wrong) {
    setOnline(false, "Not paired");
    leaveCanvas();
    closePicker();
    closeVoiceHelp();
    document.body.classList.add("pairing");
    tabs.innerHTML = "";
    grid.innerHTML = `
      <form id="pair" novalidate>
        <div class="pair-emblem"></div>
        <h2><b>//</b> ${tr("Pair this device")}</h2>
        <p class="pair-help">${tr("Open {0} on your gaming PC and type the pairing code its window shows.", "<strong>SpoolUp</strong>")}</p>
        <label for="code">${tr("Pairing code")}</label>
        <input id="code" autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" maxlength="32"
          placeholder="${tr("e.g. 1a2b3c4d")}" aria-describedby="pairError">
        <p id="pairError" class="pair-error" role="alert" hidden></p>
        <button type="submit">${tr("Connect")}</button>
        <p class="pair-hint">${tr("Quicker: scan the QR code in the SpoolUp window with this device's camera.")}</p>
      </form>`;
    grid.querySelector(".pair-emblem").innerHTML = emblemSvg((themeById[document.documentElement.dataset.theme] || THEMES[0]).emblem);
    const input = $("code"), error = $("pairError"), button = grid.querySelector("#pair button");
    const fail = (text) => {
      error.textContent = tr(text);
      error.hidden = false;
      input.classList.add("bad");
      input.setAttribute("aria-invalid", "true");
      buzz([30, 60, 30]);
    };
    input.value = wrong === "changed" ? "" : token; // a typo is easier to fix in place
    input.oninput = () => { input.classList.remove("bad"); input.removeAttribute("aria-invalid"); error.hidden = true; };
    if (wrong === "locked") fail("Too many wrong codes. Wait a minute, then try again.");
    else if (wrong === "changed") fail("The pairing code on the PC has changed. Type the new one from the SpoolUp window.");
    else if (wrong) fail("That code didn't match. Check the code in the SpoolUp window on your PC and try again.");
    input.focus();
    $("pair").onsubmit = (e) => {
      e.preventDefault();
      token = input.value.trim().toLowerCase();
      if (!token) { fail("Type the pairing code first."); return; }
      button.disabled = true;
      button.textContent = tr("Connecting…");
      load();
    };
  }

  // Draws a layout: canvas layouts on the stage, tabbed grid layouts in the page. Also used
  // to switch layouts in place, without reloading (which would replay the intro and leave
  // full screen).
  let deckCanvas = null;
  function showLayout(l) {
    layout = l;
    stopPaging(); // its panels are the old layout's
    $("title").textContent = tr(layout.name);
    document.title = tr(layout.name);
    if (layout.canvas || gridLayout()) {
      document.body.classList.add("canvas-mode");
      deckCanvas = window.renderCanvas(layout, {
        send, bindButton, bindTalk, buzz, store, refreshStatus, openPicker, openVoiceHelp, toggleFullscreen,
        tapStep, bindControl, toast, canPress: () => DEMO || connected(), stateOf,
        swiping: isSwiping, swipeMode: () => ({ two: twoOn(), one: oneOn() }), viewport,
        paged: panelPaged,
        deckIcon: window.deckIcon,
        emblemHtml: () => emblemSvg((themeById[document.documentElement.dataset.theme] || THEMES[0]).emblem),
      });
    } else {
      leaveCanvas();
      if (pageIndex >= layout.pages.length) pageIndex = 0;
      renderTabs();
      renderPage();
    }
    fillSwipes();
    maybeSwipeHint();
  }
  // Takes a canvas layout off the screen, for a grid layout or the pairing form.
  function leaveCanvas() {
    stopPaging();
    if (deckCanvas) { deckCanvas.dispose(); deckCanvas = null; }
    document.body.classList.remove("canvas-mode");
    $("stage").hidden = true;
    $("canvas").innerHTML = "";
  }

  // Fetches the chosen layout (from the PC, or the layouts/ folder in demo mode) and shows it.
  function load() {
    const demoFile = (DEMO_LAYOUTS.find(([f]) => f === chosenLayout) || DEMO_LAYOUTS[0])[0];
    return fetch(DEMO ? demoFile : `/layout.json?t=${encodeURIComponent(token)}&device=${device}&layout=${encodeURIComponent(chosenLayout)}`)
      .then((r) => {
        if (r.status === 403) return Promise.reject("pair");
        if (r.status === 429) return Promise.reject("locked");
        return r.ok ? r.json() : Promise.reject(tr("Layout error ({0})", r.status));
      })
      .then((l) => {
        const first = !layout;
        if (!DEMO) store.set("token", token);
        document.body.classList.remove("pairing");
        showLayout(l);
        if (DEMO) {
          setOnline(true, "Demo");
          if (first) setTimeout(() => toast("Demo: taps play their animations; with the app, they press keys in Star Citizen", 4500), 400);
        } else if (!ws) connect();
      })
      .catch((err) => {
        if (err === "pair") { showPairing(Boolean(token)); return; }
        if (err === "locked") { showPairing("locked"); return; }
        grid.innerHTML = `<div id="msg"></div>`;
        $("msg").textContent = String(err);
        setOnline(false, "Error");
      });
  }

  // A new layout means new buttons on the PC's side too, so connect again with it.
  function reconnect() {
    if (DEMO || !ws) return;
    ws.onclose = null;
    ws.close();
    ws = null;
    connect();
  }

  // Shows another layout (by file), remembered for this device.
  function useLayout(file, name) {
    chosenLayout = file;
    store.set(layoutKey, file);
    return load().then(() => { reconnect(); if (name) toast(name, 1600); });
  }

  function toggleFullscreen() {
    const d = document.documentElement;
    if (document.fullscreenElement) document.exitFullscreen();
    else (d.requestFullscreen || d.webkitRequestFullscreen)?.call(d);
  }
  $("fs").onclick = toggleFullscreen;
  // Inside the SpoolUp apps, Deck settings gets the way back to the app's list of PCs.
  if (window.SpoolUpApp && typeof window.SpoolUpApp.switchPC === "function") {
    const back = document.createElement("button");
    back.id = "switchPcBtn";
    back.type = "button";
    back.className = "sheet-action";
    back.innerHTML = `<span class="sw-text"><b>Switch PC</b><span>Leaves the deck for the app's list of PCs.</span></span>`;
    back.onclick = () => window.SpoolUpApp.switchPC();
    $("langPick").after(back);
  }

  window.DeckI18n.apply();
  buildPicker();
  applyTheme(store.get("theme") || "rsi");
  if (DEMO || token) load(); else showPairing(false);
})();
