// Panels: each ship system laid out like a cockpit panel, not a grid of squares.
// Widgets are real controls (faders, toggle switches, the gear lever, guarded covers,
// a slide-to-eject track, a rotary mode selector, a shield pad, hold-to-engage bars),
// sized by importance. Layout is nested flex: a "flip" box is a column in a tall pane
// and a row in a wide one, so one design fits a column on a tablet or a phone.
// Every control in a group always appears: anything a design doesn't place lands in a
// small grid at the end.
(() => {
  "use strict";
  const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const icon = (n) => window.deckIcon(n);
  const CAT = (g) => window.DeckCat.of(g);

  // Panel sizes: weight shares a column's height; min is the smallest height (px) the
  // design still works at on a tablet.
  const SIZE = {
    power: [10, 470], landing: [6, 250], flight: [7, 300], quantum: [3, 130], modes: [6, 270], shields: [6, 270],
    targeting: [4, 210], combat: [6, 260], emergency: [4, 200], crew: [6, 260], systems: [2.6, 110], mining: [7, 300],
    salvage: [5, 240], "on-foot": [3, 140], emotes: [8, 300], interface: [2.6, 100],
  };
  function size(g) {
    const s = SIZE[g.id];
    if (s) return { weight: s[0], min: s[1] };
    const n = g.controls.length;
    return { weight: Math.max(2, n * 0.7), min: 60 + 70 * Math.ceil(n / 4) };
  }

  // ---------- context for one panel ----------
  function ctx(g, opts) {
    const by = {};
    for (const c of g.controls) by[c.cid || c.id] = c;
    const used = new Set();
    const take = (id) => { const c = by[id]; if (c) used.add(c); return c; };
    return { g, by, used, take, cat: CAT(g.id), locked: (c) => opts.guest && c && c.locked, lit: opts.lit || {}, opts };
  }

  // ---------- layout helpers ----------
  const box = (cls, kids, f) => { kids = kids.filter(Boolean); return kids.length ? `<div class="${cls}"${f ? ` style="flex:${f}"` : ""}>${kids.join("")}</div>` : ""; };
  const col = (kids, f) => box("bx bcol", kids, f);
  const row = (kids, f) => box("bx brow", kids, f);
  const flip = (kids, f) => box("bx flip", kids, f);
  const flex = (html, f) => (html ? html.replace(/^<(\w+)/, `<$1 style="flex:${f}"`) : "");

  // ---------- widgets ----------
  function attrs(x, c, extra = "") {
    const on = c.state && x.lit[c.state.group];
    return `data-id="${esc(c.id)}" data-cat="${x.catOf ? x.catOf(c) : x.cat}" data-mode="${c.mode === "hold" ? "hold" : "tap"}"${c.state ? ` data-group="${esc(c.state.group)}"` : ""}${x.locked(c) ? ' data-locked="1"' : ""} aria-label="${esc(c.label)}"${on ? ' data-on="1"' : ""} ${extra}`;
  }
  const iconOf = (x, c, fallback) => window.DeckCat.icon(c.cid ? { ...c, id: c.cid } : c, x.g.id) || fallback;
  const short = (l) => String(l).replace(/^(toggle|request|engage|master:?)\s+/i, "").replace(/^prev /i, "");

  // A key: icon first, label under it (or beside it in a wide key).
  function key(x, id, o = {}) {
    const c = typeof id === "string" ? x.take(id) : id;
    if (!c) return "";
    x.used.add(c);
    const cls = ["k", o.cls || "", c.state ? "state" : "", c.mode === "hold" ? "holdk" : "", x.locked(c) ? "locked" : "", c.state && x.lit[c.state.group] && !x.locked(c) ? "on" : ""].filter(Boolean).join(" ");
    const badge = window.DeckCat.badge(c.label);
    return `<button type="button" class="${cls}" ${attrs(x, c, o.fx ? `data-fx="${o.fx}"` : "")}${o.f ? ` style="flex:${o.f}"` : ""}>
      ${c.state ? '<i class="led"></i>' : ""}${badge && !o.noBadge ? `<i class="bd">${badge}</i>` : ""}
      <span class="kin"><span class="ic">${icon(o.icon || iconOf(x, c))}</span><span class="l">${esc(o.label || short(c.label))}</span></span>
      ${c.mode === "hold" ? '<i class="ring"></i>' : ""}<i class="arm"></i></button>`;
  }
  // A big key for the panel's main action.
  const hero = (x, id, o = {}) => key(x, id, { ...o, cls: `hero ${o.cls || ""}` });

  // A toggle tile: the key with a lit bar that shows its state.
  const toggle = (x, id, o = {}) => key(x, id, { ...o, cls: `tog ${o.cls || ""}` });

  // Two keys shown as one segmented switch (SCM | NAV).
  function seg(x, ids, names, f) {
    const cs = ids.map((id) => x.take(id)).filter(Boolean);
    if (!cs.length) return "";
    return `<div class="seg2" style="flex:${f || 1}">${cs.map((c, i) => key(x, c, { cls: "segk", label: names[i], noBadge: true })).join("")}</div>`;
  }

  // The gear lever: a wheel-shaped knob in a slot, three gear lights.
  function lever(x, id, f) {
    const c = x.take(id);
    if (!c) return "";
    const on = c.state && x.lit[c.state.group];
    return `<div class="lever${on ? " on" : ""}" ${attrs(x, c)} style="flex:${f || 1}" role="button" tabindex="0">
      <span class="slot"><span class="knob">${icon("gear")}</span></span>
      <span class="greens"><i></i><i></i><i></i></span><span class="l">${esc(short(c.label))}</span></div>`;
  }

  // A fader: drag past each detent for one press of + or −; MAX and MIN sit at the ends.
  // It springs back to center because the game doesn't report the level.
  function fader(x, o) {
    const up = o.up && x.take(o.up), down = o.down && x.take(o.down);
    if (!up && !down) return "";
    const max = o.max && x.take(o.max), min = o.min && x.take(o.min);
    const hold = (up || down).mode === "hold";
    const cat = o.cat || x.cat;
    const end = (c, cls, ic) => c ? `<button type="button" class="k cap ${cls}" ${attrs(x, c).replace(`data-cat="${x.cat}"`, `data-cat="${cat}"`)}><span class="kin"><span class="ic">${icon(ic)}</span><span class="l">${cls.toUpperCase()}</span></span></button>` : "";
    return `<div class="fader ${o.h ? "horiz" : ""}${hold ? " throttle" : ""}" data-cat="${cat}" data-up="${up ? esc(up.id) : ""}" data-down="${down ? esc(down.id) : ""}"${up && x.locked(up) ? ' data-locked="1"' : ""} style="flex:${o.f || 1}">
      ${end(max, "max", o.h ? "chev-up" : "chev-up")}
      <span class="track" aria-label="${esc(o.label)}" role="slider" tabindex="0"><i class="ticks"></i><i class="fill"></i><span class="thumb"></span></span>
      ${end(min, "min", "chev-down")}
      <span class="fl"><span class="ic">${icon(o.icon || "sliders")}</span><span class="l">${esc(o.label)}</span></span></div>`;
  }

  // A guarded control: lift the striped cover (tap or swipe up), then press.
  function guard(x, id, f) {
    const c = x.take(id);
    if (!c) return "";
    return `<div class="guard" style="flex:${f || 1}">${key(x, c, { cls: "danger" })}<button type="button" class="cover" aria-label="Lift cover"><i></i></button></div>`;
  }

  // Slide to eject: drag the handle all the way along the track.
  function slide(x, id, f) {
    const c = x.take(id);
    if (!c) return "";
    return `<div class="slide" ${attrs(x, c)} style="flex:${f || 1}"><span class="rail"><span class="handle">${icon("exit-seat")}</span><span class="l">${esc(c.label)}</span></span></div>`;
  }

  // Rotary selector for operator modes; the center key cycles SCM/NAV.
  function knob(x, ids, center, f) {
    const cs = ids.map((id) => x.take(id)).filter(Boolean);
    if (!cs.length) return "";
    const n = cs.length, span = 270, start = -135;
    const pos = cs.map((c, i) => {
      const a = start + (n === 1 ? 0 : (span * i) / (n - 1));
      return key(x, c, { cls: "pos", label: short(c.label).replace(/ mode$/i, "") }).replace('class="', `style="--a:${a}deg" class="`);
    }).join("");
    const mid = center ? key(x, center, { cls: "hub" }) : "";
    return `<div class="knob" style="flex:${f || 1}"><div class="dial"><span class="pointer"></span>${pos}${mid}</div></div>`;
  }

  // Shield pad: the ship from above with a face on each side; top and bottom beside it.
  function shieldpad(x, f) {
    const k = (id, cls, side) => { const c = x.take(id); return c ? key(x, c, { cls: `${side ? "tbk" : "face"} ${cls}`, label: cls.toUpperCase(), noBadge: true, fx: "shield" }) : ""; };
    const front = k("shields-front", "front"), back = k("shields-back", "back"), left = k("shields-left", "left"), right = k("shields-right", "right");
    const top = k("shields-top", "top", 1), bot = k("shields-bottom", "bottom", 1);
    const reset = x.take("shields-reset");
    return `<div class="spad" style="flex:${f || 1}"><div class="ring">${front}${left}${right}${back}
      <span class="ship">${icon("ship")}</span>${reset ? key(x, reset, { cls: "hub", label: "Reset" }) : ""}</div>
      <div class="tb">${top}${bot}</div></div>`;
  }

  // A grid that sizes itself to fill its box (cols set after layout).
  const grid = (x, cs, f, cls = "") => {
    const ks = cs.filter(Boolean).map((c) => key(x, c)).filter(Boolean);
    return ks.length ? `<div class="agrid ${cls}" style="flex:${f || 1}" data-n="${ks.length}">${ks.join("")}</div>` : "";
  };

  // ---------- designs ----------
  const T = {
    power(x) {
      const sys = row([toggle(x, "power-on-off", { icon: "power", label: "PWR" }), toggle(x, "wep", { icon: "power-weapons", label: "WEP" }),
        toggle(x, "thr", { icon: "power-engines", label: "THR" }), toggle(x, "shld", { icon: "power-shields", label: "SHLD" })], 1);
      const fr = hero(x, "toggle-flight-ready", { label: "Flight Ready", fx: "power" });
      const faders = row([
        fader(x, { up: "weapons-up", down: "weapons-down", max: "max-weapons", min: "min-weapons", label: "Weapons", icon: "power-weapons", cat: "combat" }),
        fader(x, { up: "thrusters-up", down: "thrusters-down", max: "max-thrusters", min: "min-thrusters", label: "Thrusters", icon: "power-engines", cat: "flight" }),
        fader(x, { up: "shields-up", down: "shields-down", max: "max-shields", min: "min-shields", label: "Shields", icon: "power-shields", cat: "def" }),
      ], 3.4);
      return flip([col([sys, flex(fr, 0.9)], 1.1), col([faders, key(x, "reset-all", { cls: "wide" })].map((h, i) => (i ? flex(h, 0.45) : h)), 2.3)]);
    },
    landing(x) {
      const mode = x.opts.landingMode || "landing";
      const sw = `<div class="seg2 local" style="flex:.42"><button type="button" class="k segk${mode === "landing" ? " on" : ""}" data-local="landing-mode" data-v="landing"><span class="kin"><span class="l">Landing</span></span></button><button type="button" class="k segk${mode === "docking" ? " on" : ""}" data-local="landing-mode" data-v="docking"><span class="kin"><span class="l">Docking</span></span></button></div>`;
      const vtol = toggle(x, "vtol", { fx: "streak" });
      const land = row([lever(x, "landing-gear", 0.8), col([vtol, key(x, "engage-auto-land", { cls: "wide", fx: "dial" }),
        row([key(x, "request-landing"), toggle(x, "landing-speed-limiter")])], 1.9)], 3);
      const dock = flip([hero(x, "request-docking"), col([key(x, "docking-camera"), vtol], 1)], 3);
      return col([sw, `<div class="bx mode" data-show="${mode}" style="flex:3">${mode === "landing" ? land : dock}</div>`,
        `<div hidden>${mode === "landing" ? hidden(x, ["request-docking", "docking-camera"]) : hidden(x, ["landing-gear", "engage-auto-land", "request-landing", "landing-speed-limiter"])}</div>`]);
    },
    flight(x) {
      return col([
        seg(x, ["master-scm", "master-nav"], ["SCM", "NAV"], 1),
        row([toggle(x, "speed-limiter"), fader(x, { up: "speed-range-up", down: "speed-range-down", label: "Speed range", icon: "speed", h: true, f: 2.2 }), key(x, "reset-limit", { f: 0.8 })], 1.1),
        row([key(x, "match-speed"), toggle(x, "toggle-esp", { label: "ESP" }), toggle(x, "g-safety")], 1),
        key(x, "look-behind", { cls: "wide", f: 0.7 }),
      ]);
    },
    quantum(x) { return col([hero(x, "engage-quantum", { cls: "spool", fx: "quantum", label: "Engage Quantum" })]); },
    modes(x) {
      return col([knob(x, ["flight-mode", "guns-mode", "missiles", "scanning", "quantum-mode", "mining-mode", "salvage-mode"], x.take("scm-nav"))]);
    },
    shields(x) { return col([shieldpad(x)]); },
    targeting(x) {
      return col([row([hero(x, "prev-hostile", { fx: "lock", f: 1.6 }), key(x, "prev-attacker", { fx: "lock" })], 1.4),
        row([key(x, "prev-friendly", { fx: "lock" }), key(x, "prev-any-target", { fx: "lock" })], 1),
        row([key(x, "prev-in-view", { fx: "lock" }), key(x, "prev-subtarget", { fx: "lock" }), toggle(x, "lead-lag-pips")], 0.9)]);
    },
    combat(x) {
      return flip([col([row([hero(x, "decoy", { fx: "flares" }), hero(x, "noise", { fx: "noise" })], 1.4), key(x, "panic-decoys", { cls: "wide", fx: "flares", f: 0.8 })], 1),
        col([hero(x, "launch-missile", { fx: "missile", f: 1.4 }), row([key(x, "fewer-missiles"), toggle(x, "lead-lag-pips")], 0.8)], 1)]);
    },
    emergency(x) {
      return col([`<div class="bx brow hazard" style="flex:1.3">${slide(x, "slide-to-eject", 1.7)}${guard(x, "self-d", 1)}</div>`,
        row([key(x, "emergency-decoy", { fx: "flares" }), key(x, "emergency-noise", { fx: "noise" }), key(x, "emergency-panic-decoys", { fx: "flares" })], 1)]);
    },
    crew(x) {
      return col([row([key(x, "remote-turret-1", { label: "Turret 1" }), key(x, "remote-turret-2", { label: "Turret 2" }), key(x, "remote-turret-3", { label: "Turret 3" })], 1),
        flip([hero(x, "toggle-doors", { fx: "doors" }), col([row([key(x, "open-doors", { fx: "doors" }), key(x, "close-doors", { fx: "doors" })]), row([key(x, "lock-doors"), key(x, "unlock-doors")])], 1.2)], 1.8)]);
    },
    systems(x) { return flip([toggle(x, "lights", { cls: "hero", fx: "lights" }), key(x, "exit-seat")]); },
    mining(x) {
      return col([toggle(x, "mining-mode", { cls: "wide", f: 0.55 }),
        flip([hero(x, "fire-laser", { fx: "laser", f: 1.3 }), col([key(x, "fracture-extract"),
          fader(x, { up: "laser-power-up", down: "laser-power-down", label: "Laser power", icon: "laser", h: true })], 1.3)], 2),
        row([key(x, "module-1", { label: "Module 1" }), key(x, "module-2", { label: "Module 2" }), key(x, "module-3", { label: "Module 3" })], 0.8)]);
    },
    salvage(x) {
      return col([fader(x, { up: "modifier-right", down: "modifier-left", label: "Modifier", icon: "swap", h: true, f: 1 }),
        flip([fader(x, { up: "beam-spacing-up", down: "beam-spacing-down", label: "Beam spacing", icon: "beam", h: true }),
          col([key(x, "salvage-open-doors", { fx: "doors" }), key(x, "salvage-close-doors", { fx: "doors" })])], 2)]);
    },
    emotes(x) { return col([grid(x, x.g.controls, 1, "small")]); },
  };
  // Controls a mode hides still need to be "used" so they don't land in the extras.
  function hidden(x, ids) { ids.forEach((id) => x.take(id)); return ""; }

  // ---------- build ----------
  function render(g, opts = {}) {
    const x = ctx(g, opts);
    const t = T[g.id];
    let body = t ? t(x) : "";
    const rest = g.controls.filter((c) => !x.used.has(c));
    if (rest.length) body = t ? col([body ? `<div class="bx bcol" style="flex:4">${body}</div>` : "", grid(x, rest, rest.length > 4 ? 2 : 1)]) : grid(x, rest, 1);
    return body;
  }

  // After layout: auto grids pick a column count that keeps keys near square and big.
  function fitGrids(root) {
    for (const el of root.querySelectorAll(".agrid")) {
      const n = Number(el.dataset.n) || el.children.length, W = el.clientWidth, H = el.clientHeight, gap = 6;
      let best = 1, score = 0;
      for (let c = 1; c <= n; c++) {
        const r = Math.ceil(n / c), w = (W - gap * (c - 1)) / c, h = (H - gap * (r - 1)) / r;
        const s = Math.min(w, h * 1.5) * (r * c - n > c ? 0.85 : 1);
        if (s > score) { score = s; best = c; }
      }
      el.style.gridTemplateColumns = `repeat(${best}, minmax(0, 1fr))`;
      el.style.gridTemplateRows = `repeat(${Math.ceil(n / best)}, minmax(0, 1fr))`;
    }
  }

  window.DeckPanels = { render, size, fitGrids };
})();
