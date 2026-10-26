// Panels: each group drawn from its panel definition (docs/PANELS.md), sent by the PC
// with the group. Widgets are real controls (faders, toggle switches, the gear lever,
// guarded covers, a slide-to-eject track, a rotary selector, a shield pad, hold to
// engage keys, and the comms widgets), sized by importance. Layout is nested flex: a
// "flip" box is a column in a tall pane and a row in a wide one, so one design fits a
// column on a tablet or a phone. Every control in a group always appears: anything the
// layout doesn't place lands in a grid at the end (or where the layout says "rest").
(() => {
  "use strict";
  const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const icon = (n) => window.deckIcon(n);
  const CAT = (g) => window.DeckCat.of(g);

  // Panel sizes: weight shares a column's height; min is the smallest height (px) the
  // design still works at on a tablet.
  function size(g) {
    const n = g.controls.length, s = g.size || {};
    return { weight: s.weight || Math.max(2, n * 0.7), min: s.min || Math.min(620, 60 + 70 * Math.ceil(n / 4)) };
  }

  // ---------- context for one panel ----------
  function ctx(g, opts) {
    const by = {};
    for (const c of g.controls) by[c.cid || c.id] = c;
    const used = new Set();
    const take = (id) => { const c = by[g.id + "." + id] || by[id]; if (c) used.add(c); return c; };
    return { g, by, used, take, cat: g.category || CAT(g.id), locked: (c) => opts.guest && c && c.locked, lit: opts.lit || {}, opts };
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
    const cls = ["k", o.cls || "", c.state ? "state" : "", c.mode === "hold" ? "holdk" : "", x.locked(c) ? "locked" : "", c.unbound ? "unbound" : "", c.state && x.lit[c.state.group] && !x.locked(c) ? "on" : ""].filter(Boolean).join(" ");
    const badge = window.DeckCat.badge(c.label);
    return `<button type="button" class="${cls}" ${attrs(x, c, o.fx ? `data-fx="${o.fx}"` : "")}${o.f ? ` style="flex:${o.f}"` : ""}>
      ${c.state ? '<i class="led"></i>' : ""}${c.unbound ? '<i class="bd ub">BIND</i>' : badge && !o.noBadge ? `<i class="bd">${badge}</i>` : ""}
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
  function shieldpad(x, ids, f) {
    const k = (id, cls, side) => { const c = id && x.take(id); return c ? key(x, c, { cls: `${side ? "tbk" : "face"} ${cls}`, label: cls.toUpperCase(), noBadge: true, fx: "shield" }) : ""; };
    const front = k(ids.front, "front"), back = k(ids.back, "back"), left = k(ids.left, "left"), right = k(ids.right, "right");
    const top = k(ids.top, "top", 1), bot = k(ids.bottom, "bottom", 1);
    const reset = ids.reset && x.take(ids.reset);
    return `<div class="spad" style="flex:${f || 1}"><div class="ring">${front}${left}${right}${back}
      <span class="ship">${icon("ship")}</span>${reset ? key(x, reset, { cls: "hub", label: "Reset" }) : ""}</div>
      <div class="tb">${top}${bot}</div></div>`;
  }

  // A grid that sizes itself to fill its box (cols set after layout).
  const grid = (x, cs, f, cls = "") => {
    const ks = cs.filter(Boolean).map((c) => key(x, c)).filter(Boolean);
    return ks.length ? `<div class="agrid ${cls}" style="flex:${f || 1}" data-n="${ks.length}">${ks.join("")}</div>` : "";
  };

  // ---------- comms ----------
  const mic = '<svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><rect x="17" y="6" width="14" height="24" rx="7"/><path d="M10 22a14 14 0 0 0 28 0M24 36v7M17 43h14"/></svg>';
  const SEG = (name, opts, cur, attr) => `<div class="seg2 local" style="flex:.42">${opts.map(([v, l]) => `<button type="button" class="k segk${v === cur ? " on" : ""}" data-${attr}="${v}"><span class="kin"><span class="l">${esc(l)}</span></span></button>`).join("")}</div>`;
  // The voice key: tap and talk until you pause, hold to talk, or leave it live.
  function talk(x, n) {
    const c = x.opts.comms || {};
    const mode = c.talkMode || "tap";
    const label = mode === "live" ? (c.live ? "Live" : "Off air") : mode === "hold" ? "Hold to talk" : "Tap to talk";
    return `<div class="bx bcol talk" style="flex:${n.flex || 1}">
      <button type="button" class="k hero talkk${c.listening || (mode === "live" && c.live) ? " on" : ""}" data-talk="${mode}" data-cat="${x.cat}" aria-label="${esc(label)}"><span class="kin"><span class="ic">${mic}</span><span class="l">${esc(label)}</span></span><i class="vu"></i></button>
      ${SEG("talk", [["tap", "Tap"], ["hold", "Hold"], ["live", "Live"]], mode, "talkmode")}</div>`;
  }
  // Where voice goes: commands only, commands then the agent, or always the agent.
  function route(x, n) {
    const c = x.opts.comms || {}, ag = c.agent || {};
    const name = ag.name || "Agent";
    return `<div class="bx brow route" style="flex:${n.flex || 1}">${SEG("route", [["command", "Commands"], ["auto", "Auto"], ["agent", name]], ag.route || "command", "route").replace('style="flex:.42"', "")}${ag.ready ? "" : '<span class="nolink" title="No agent set up on the PC">i</span>'}</div>`;
  }
  // The last lines said to and by the agent; tap to type to it.
  function transcript(x, n) {
    const lines = ((x.opts.comms || {}).chat || []).slice(-3);
    return `<button type="button" class="transcript" data-sheet="agent" style="flex:${n.flex || 1}" aria-label="Talk to the agent">${lines.length ? lines.map((l) => `<span class="ln ${esc(l.who)}"><b>${l.who === "you" ? "›" : l.who === "agent" ? "‹" : "·"}</b>${esc(l.text)}</span>`).join("") : '<span class="ln system"><b>·</b>—</span>'}${(x.opts.comms || {}).busy ? '<i class="dots"><i></i><i></i><i></i></i>' : ""}</button>`;
  }
  // Push to talk, radio style: pick Discord, the game or both; hold to transmit.
  function radio(x, n) {
    const o = n.radio || {}, c = x.opts.comms || {};
    const d = o.discord && x.take(o.discord), g = o.game && x.take(o.game);
    if (!d && !g) return "";
    const ch = c.radioCh || (d ? "discord" : "game");
    const ids = { discord: d ? [d.id] : [], game: g ? [g.id] : [], both: [d, g].filter(Boolean).map((z) => z.id) }[ch] || [];
    return `<div class="bx bcol radio" style="flex:${n.flex || 1}">
      <button type="button" class="k hero radiok" data-radio="${esc(ids.join(","))}" data-cat="${x.cat}" aria-label="Push to talk"><span class="kin"><span class="ic">${icon("radio")}</span><span class="l">${ch === "both" ? "Discord + game" : ch === "game" ? "Game PTT" : "Discord PTT"}</span></span><i class="tx">TX</i></button>
      ${SEG("radio", [d && ["discord", "Discord"], g && ["game", "Game"], d && g && ["both", "Both"]].filter(Boolean), ch, "radioch")}</div>`;
  }

  // ---------- panels from their definition ----------
  const ids = (n) => { const out = []; const walk = (v) => { if (typeof v === "string") out.push(v); else if (Array.isArray(v)) v.forEach(walk); else if (v && typeof v === "object") Object.values(v).forEach(walk); }; walk(n); return out; };
  const kopts = (n) => ({ label: n.label, icon: n.icon, fx: n.fx, f: n.flex, cls: [n.size === "hero" ? "hero" : n.size === "wide" ? "wide" : "", n.spool ? "spool" : ""].filter(Boolean).join(" ") });
  function node(x, n) {
    if (!n || typeof n !== "object") return "";
    if (n.row || n.col || n.flip) {
      const kind = n.row ? "brow" : n.col ? "bcol" : "flip";
      const kids = (n.row || n.col || n.flip).map((k) => node(x, k)).filter(Boolean);
      return kids.length ? `<div class="bx ${kind}${n.style === "hazard" ? " hazard" : ""}"${n.flex ? ` style="flex:${n.flex}"` : ""}>${kids.join("")}</div>` : "";
    }
    if (n.key) return key(x, n.key, kopts(n));
    if (n.toggle) return toggle(x, n.toggle, kopts(n));
    if (n.lever) return lever(x, n.lever, n.flex);
    if (n.fader) return fader(x, { ...n.fader, h: n.fader.horizontal, f: n.flex });
    if (n.pair) return seg(x, n.pair, n.names || [], n.flex);
    if (n.dial) return knob(x, n.dial, n.center && x.take(n.center), n.flex);
    if (n.shieldpad) return shieldpad(x, n.shieldpad, n.flex);
    if (n.guard) return guard(x, n.guard, n.flex);
    if (n.slide) return slide(x, n.slide, n.flex);
    if (n.grid) return n.grid === "rest" ? `<!--rest:${n.flex || 1}:${n.small ? "small" : ""}-->` : grid(x, n.grid.map((id) => x.take(id)), n.flex, n.small ? "small" : "");
    if (n.modes) {
      const opts = n.options || [];
      const cur = (x.opts.local && x.opts.local(n.modes)) || (opts[0] && opts[0].value);
      const pick = opts.find((o) => o.value === cur) || opts[0];
      opts.forEach((o) => { if (o !== pick) ids(o.layout).forEach((id) => x.take(id)); });
      const sw = `<div class="seg2 local" style="flex:.42">${opts.map((o) => `<button type="button" class="k segk${o === pick ? " on" : ""}" data-local="${esc(n.modes)}" data-v="${esc(o.value)}"><span class="kin"><span class="l">${esc(o.name)}</span></span></button>`).join("")}</div>`;
      return `<div class="bx bcol"${n.flex ? ` style="flex:${n.flex}"` : ""}>${sw}<div class="bx bcol" style="flex:3">${node(x, pick.layout)}</div></div>`;
    }
    if (n.talk) return talk(x, n);
    if (n.route) return route(x, n);
    if (n.transcript) return transcript(x, n);
    if (n.radio) return radio(x, n);
    return "";
  }

  // ---------- build ----------
  function render(g, opts = {}) {
    const x = ctx(g, opts);
    let body = g.layout ? node(x, g.layout) : "";
    const rest = () => g.controls.filter((c) => !x.used.has(c));
    const m = body.match(/<!--rest:([\d.]+):(\w*)-->/);
    if (m) {
      const r = rest();
      body = body.replace(m[0], r.length ? grid(x, r, Number(m[1]), m[2]) : "");
    } else {
      const r = rest();
      if (r.length) body = body ? col([`<div class="bx bcol" style="flex:4">${body}</div>`, grid(x, r, r.length > 4 ? 2 : 1)]) : grid(x, r, 1);
    }
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
