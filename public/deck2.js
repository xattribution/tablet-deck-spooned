// Tablet Deck 2: the deck. Every control comes from the PC's catalog; a view is a
// list of groups (ship systems) the PC sends, so a view can be changed from here,
// from the PC, by voice or by an agent over MCP. Plain JS, no build step.
(() => {
  "use strict";

  // ---------- basics ----------

  const $ = (s, el = document) => el.querySelector(s);
  const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const store = {
    get(k) { try { return localStorage.getItem(k); } catch { return null; } },
    set(k, v) { try { if (v === null || v === undefined || v === "") localStorage.removeItem(k); else localStorage.setItem(k, v); } catch {} },
  };
  const buzz = (ms = 10) => { if (navigator.vibrate) navigator.vibrate(ms); };
  const keyText = (k) => String(k || "").split("+").map((p) => p.replace(/^np_/, "num ").replace(/^l(alt|ctrl|shift)$/, "$1").replace(/^r(alt|ctrl|shift)$/, "r$1")).join(" ").toUpperCase();
  const ICON = {
    search: '<svg class="i" viewBox="0 0 16 16" aria-hidden="true"><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5 14 14"/></svg>',
    mic: '<svg class="i" viewBox="0 0 18 18" aria-hidden="true"><rect x="6" y="2" width="6" height="9" rx="3"/><path d="M3.5 8.5a5.5 5.5 0 0 0 11 0M9 14v2.5"/></svg>',
    menu: '<svg class="i" viewBox="0 0 18 18" aria-hidden="true"><path d="M3 5h12M3 9h12M3 13h12"/></svg>',
    back: '<svg class="i" viewBox="0 0 16 16" aria-hidden="true"><path d="M10 3 5 8l5 5"/></svg>',
    share: '<svg class="i" viewBox="0 0 18 18" aria-hidden="true"><circle cx="4.5" cy="9" r="2"/><circle cx="13.5" cy="4.5" r="2"/><circle cx="13.5" cy="13.5" r="2"/><path d="m6.3 8 5.4-2.6M6.3 10l5.4 2.6"/></svg>',
  };
  const THEMES = [
    ["rsi", "RSI", "Barlow Condensed"], ["drake", "Drake", "Saira Stencil One"], ["aegis", "Aegis", "Rajdhani"], ["anvil", "Anvil", "Saira Condensed"],
    ["origin", "Origin", "Marcellus"], ["misc", "MISC", "Barlow Condensed"], ["crusader", "Crusader", "Exo 2"], ["banu", "Banu", "Cinzel"],
    ["aopoa", "Aopoa", "Oxanium"], ["argo", "Argo", "Chakra Petch"], ["cnou", "CO", "Saira Condensed"], ["esperia", "Esperia", "Oxanium"],
  ];
  const BLOCKED = {
    "not-running": "Star Citizen isn't running", "not-focused": "Star Citizen isn't in front", "focus-failed": "Couldn't bring the game to front",
    "denied": "Not shared with you", "not-afk": "Only while they're AFK", "rate-limited": "Slow down", "expired": "This link ran out",
  };
  const QUICK = ["BRB", "On it", "Cover me", "Take the helm", "Gear down", "Spool QT", "Need you back"];

  const app = $("#app"), sheet = $("#sheet");
  const GUEST = (location.pathname.match(/^\/g\/([a-f0-9]{20,64})/) || [])[1] || "";
  const urlCode = (new URLSearchParams(location.search).get("t") || "").trim().toLowerCase();
  // The pairing code is swapped for this device's own token on first connect
  // ("paired"); the address keeps the code (a home screen app reopens it), so the token
  // is used while the address still has the code it came from.
  let token = GUEST ? "" : (urlCode && urlCode !== store.get("pairCode") ? urlCode : store.get("token") || urlCode || "");
  const auth = () => (GUEST ? `g=${GUEST}` : `t=${encodeURIComponent(token)}`);
  const device = new URLSearchParams(location.search).get("device") || (Math.min(screen.width, screen.height) < 500 ? "phone" : "tablet");

  const S = {
    hello: null, views: [], view: null, groups: [], lit: {}, game: null, afk: false, crew: [], requests: [], grants: [],
    voice: { state: "off" }, latency: 0, last: "", agent: "", online: false, guest: null, owner: "", listening: false,
  };

  // ---------- theme and display ----------

  function applyTheme(id) {
    if (!THEMES.some((t) => t[0] === id)) id = "rsi";
    document.documentElement.dataset.theme = id;
    store.set("theme", id);
    const meta = $('meta[name="theme-color"]');
    if (meta) meta.content = getComputedStyle(document.documentElement).getPropertyValue("--ground").trim();
  }
  function applyDisplay() {
    for (const k of ["night", "large", "contrast"]) document.documentElement.toggleAttribute(`data-${k}`, store.get(k) === "1");
  }
  applyTheme(store.get("theme") || "rsi");
  applyDisplay();

  let toastTimer = null;
  function toast(text, ms = 2000) {
    const el = $("#toast");
    el.textContent = text;
    el.classList.add("show");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.classList.remove("show"), ms);
  }

  // ---------- connection ----------

  let ws = null, backoff = 500, lastPong = 0;
  const send = (obj) => { if (ws && ws.readyState === 1) ws.send(JSON.stringify(obj)); };

  function connect() {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    ws = new WebSocket(`${proto}://${location.host}/${GUEST ? "gws" : "ws"}?${auth()}&device=${device}`);
    ws.onopen = () => { backoff = 500; lastPong = Date.now(); S.online = true; ping(); paintStatus(); };
    ws.onmessage = (e) => { let m; try { m = JSON.parse(e.data); } catch { return; } handle(m); };
    ws.onclose = (e) => {
      S.online = false;
      paintStatus();
      document.querySelectorAll(".k.pressed").forEach((k) => k.classList.remove("pressed"));
      if (ws && ws.dead) return;
      setTimeout(connect, backoff);
      backoff = Math.min(backoff * 1.7, 8000);
    };
  }
  function ping() { send({ t: "ping", n: performance.now() }); }
  setInterval(() => {
    ping();
    if (ws && ws.readyState === 1 && Date.now() - lastPong > 6000) ws.close();
  }, 1500);
  function stop() { if (ws) { ws.dead = true; ws.close(); ws = null; } }

  function handle(m) {
    switch (m.t) {
      case "pong": lastPong = Date.now(); S.latency = Math.round(performance.now() - m.n); paintStatus(); break;
      case "hello":
        S.hello = m; S.lit = m.states || {}; S.game = m.game; S.afk = !!m.afk; S.owner = m.name || "";
        if (m.guest) { S.guest = m.guest; drawGuest(); break; }
        S.views = m.views || []; S.crew = m.crew || []; S.requests = m.requests || []; S.grants = m.grants || [];
        S.voice = m.voiceStatus || S.voice;
        send({ t: "view", id: store.get(`view-${device}`) || "flight" });
        drawOwner();
        break;
      case "paired": token = m.token; store.set("token", token); store.set("pairCode", urlCode); break;
      case "view":
        S.view = m.view; S.groups = m.groups || [];
        if (!m.view.id.startsWith("agent-")) store.set(`view-${device}`, m.view.id);
        drawOwner();
        break;
      case "views": S.views = m.views || []; paintTabs(); break;
      case "state": S.lit[m.group] = m.on; paintLights(); break;
      case "states": S.lit = m.states || {}; paintLights(); break;
      case "game": S.game = m; paintStatus(); break;
      case "afk": S.afk = m.on; if (S.guest) drawGuest(); else { paintBar(); if (m.on) toast("Away: your crew has the controls"); } break;
      case "crew": S.crew = m.crew || []; paintRail(); paintFoot(); break;
      case "grants": S.grants = m.grants || []; if (sheet.dataset.open === "share") paintLiveLinks(); break;
      case "request": S.requests.push(m.request); paintReq(); buzz([20, 40, 20]); break;
      case "request-gone": S.requests = S.requests.filter((r) => r.id !== m.id); paintReq(); break;
      case "answered": toast(`${m.result === "done" ? "Done" : "Declined"}: ${m.label || m.text || ""}`, 2600); break;
      case "pressed": S.last = `${(m.label || "").toUpperCase()} · ${keyText(m.keys)}${m.kind !== "deck" && m.who ? " · " + m.who.toUpperCase() : ""}`; paintFoot(); break;
      case "agent": S.agent = m.tool; paintFoot(); break;
      case "blocked": {
        toast(BLOCKED[m.reason] || "Not pressed");
        buzz([30, 60, 30]);
        document.querySelectorAll(`.k[data-id="${CSS.escape(m.id)}"]`).forEach((k) => { k.classList.remove("blocked"); void k.offsetWidth; k.classList.add("blocked"); });
        break;
      }
      case "released":
        document.querySelectorAll(`.k[data-id="${CSS.escape(m.id)}"]`).forEach((k) => k.classList.remove("pressed"));
        toast(m.reason === "focus-lost" ? "Released: the game lost focus" : "Released");
        break;
      case "highlight": highlight(m.ids || []); break;
      case "theme": applyTheme(m.id); break;
      case "voice-status": S.voice = m; paintFoot(); break;
      case "listening": S.listening = m.on; paintBar(); break;
      case "heard": toast(`Heard: ${m.text || m.label}`); break;
      case "unheard": toast("Didn't catch that"); break;
      case "voice-off": toast(m.reason); break;
      case "toast": toast(m.text, 2600); break;
      case "opened": toast(`Opened ${m.name} on the PC`); break;
      case "shared": showShared(m); break;
      case "reload": send({ t: "view", id: S.view ? S.view.id : "flight" }); break;
      case "unpaired": store.set("token", ""); store.set("pairCode", ""); token = ""; stop(); drawPair("This device was unpaired on the PC."); break;
      case "revoked": stop(); drawGone(m.reason === "expired" ? "This share link ran out." : "This share link was turned off."); break;
      case "crew-request": break;
    }
  }

  // ---------- owner deck ----------

  function viewTabs() {
    const own = S.views.filter((v) => !v.preset);
    const list = S.views.filter((v) => v.preset).concat(own);
    if (S.view && !list.some((v) => v.id === S.view.id)) list.push(S.view);
    return list;
  }

  function drawOwner() {
    app.innerHTML = `
      <header class="bar" id="bar"></header>
      <div id="req"></div>
      <div class="body">
        <main class="deck" id="deck" aria-label="Controls"></main>
        <aside class="rail" id="rail" aria-label="Crew"></aside>
      </div>
      <nav class="ptabs" id="ptabs" aria-label="Views"></nav>
      <footer class="foot" id="foot"></footer>`;
    paintBar(); paintReq(); paintDeck(); paintRail(); paintFoot();
  }

  function paintBar() {
    const bar = $("#bar");
    if (!bar) return;
    const theme = THEMES.find((t) => t[0] === document.documentElement.dataset.theme) || THEMES[0];
    bar.innerHTML = `
      <div class="brand"><i></i><b>${esc(theme[1])}</b></div>
      <nav class="tabs" aria-label="Views">${viewTabs().map((v) => `<button type="button" class="tab" data-view="${esc(v.id)}" aria-current="${S.view && S.view.id === v.id}">${esc(v.name)}</button>`).join("")}</nav>
      <button type="button" class="tbtn dim" data-act="find" aria-label="Find a control">${ICON.search}<span class="vh">Find</span></button>
      <div class="spacer"></div>
      <div class="readout" id="link"></div>
      <div class="readout" id="game"></div>
      <button type="button" class="tbtn" data-act="afk" aria-pressed="${S.afk}">AFK</button>
      <button type="button" class="tbtn ${S.listening ? "listening" : ""}" data-act="talk" aria-label="Hold to talk"${voiceUsable() ? "" : " hidden"}>${ICON.mic}</button>
      <button type="button" class="tbtn" data-act="share" aria-label="Share controls">${ICON.share}</button>
      <button type="button" class="tbtn" data-act="settings" aria-label="Settings">${ICON.menu}</button>`;
    paintStatus();
    const pt = $("#ptabs");
    if (pt) pt.innerHTML = viewTabs().map((v) => `<button type="button" class="tab" data-view="${esc(v.id)}" aria-current="${S.view && S.view.id === v.id}">${esc(v.name)}</button>`).join("");
  }
  const voiceUsable = () => S.voice && (S.voice.state === "ready" || S.voice.state === "starting");
  function paintTabs() { paintBar(); }

  function paintStatus() {
    const link = $("#link"), game = $("#game");
    if (link) link.innerHTML = `<span>LINK</span><span>${S.online ? (S.latency ? `${S.latency} ms` : "UP") : "DOWN"}</span>`;
    if (game) {
      const g = S.game || {};
      const [cls, text] = !g.running ? ["bad", "NOT RUNNING"] : g.focused ? ["good", "IN FRONT"] : ["caution", "BEHIND"];
      game.innerHTML = `<span>GAME</span><span><i class="dot ${cls}"></i>${text}</span>`;
    }
  }

  function keyHTML(c, { locked = false } = {}) {
    const on = c.state && S.lit[c.state.group];
    const hold = c.mode === "hold";
    let tag = "", tcls = "t";
    if (c.danger) { tag = "HOLD 1S"; }
    else if (hold) { tag = "HOLD"; tcls += " hold"; }
    else if (c.state) tag = on ? "ON" : "OFF";
    if (locked) tag = "ASK";
    const cls = ["k", on && !locked ? "on" : "", c.danger && !locked ? "danger" : "", locked ? "locked" : ""].filter(Boolean).join(" ");
    return `<button type="button" class="${cls}" data-id="${esc(c.id)}" data-mode="${hold ? "hold" : "tap"}"${c.danger ? ' data-danger="1"' : ""}${c.state ? ` data-group="${esc(c.state.group)}"` : ""}${locked ? ' data-locked="1"' : ""}>
      <span class="l">${esc(c.label)}</span><span class="f"><span class="b">${esc(keyText(c.keys))}</span><span class="${tcls}">${tag}</span></span><span class="p"></span></button>`;
  }

  function groupHTML(g, opts) {
    const n = g.controls.length;
    const cols = n <= 2 ? n : n === 4 ? 2 : 3;
    const wide = n > 12 ? " wide" : "";
    return `<section class="grp${wide}" aria-label="${esc(g.name)}">
      <div class="gh"><span class="code">${esc(g.code)}</span><span class="name">${esc(g.name)}</span><span class="rule"></span></div>
      <div class="keys" style="--cols:${wide ? 6 : cols}">${g.controls.map((c) => keyHTML(c, { locked: opts && opts.locked && c.locked })).join("")}</div></section>`;
  }

  function paintDeck() {
    const deck = $("#deck");
    if (!deck) return;
    deck.innerHTML = S.groups.length ? S.groups.map((g) => groupHTML(g)).join("") : `<p class="empty">Loading controls…</p>`;
  }

  function paintLights() {
    document.querySelectorAll(".k[data-group]").forEach((k) => {
      if (k.dataset.locked) return;
      const on = !!S.lit[k.dataset.group];
      k.classList.toggle("on", on);
      const t = $(".t", k);
      if (t && !k.dataset.danger && k.dataset.mode !== "hold") t.textContent = on ? "ON" : "OFF";
    });
  }

  function paintReq() {
    const box = $("#req");
    if (!box) return;
    const r = S.requests[S.requests.length - 1];
    box.innerHTML = r ? `<div class="req" role="alert">
        <span class="mono">REQUEST${S.requests.length > 1 ? ` · ${S.requests.length}` : ""}</span>
        <b>${esc(r.from)}${r.label ? ` · ${esc(r.label)}` : ""}</b>${r.text ? `<q>${esc(r.text)}</q>` : ""}
        <span class="acts"><button type="button" class="do" data-req="${esc(r.id)}" data-do="1">${r.label ? "Do it" : "On it"}</button><button type="button" class="no" data-req="${esc(r.id)}">${r.label ? "No" : "OK"}</button></span>
      </div>` : "";
  }

  function paintRail() {
    const rail = $("#rail");
    if (!rail) return;
    rail.innerHTML = `<div class="gh"><span class="name">Crew</span><span class="rule"></span><span class="code">${S.crew.length}</span></div>
      ${S.crew.map((c) => {
        const role = c.afk ? "AFK" : c.guest ? (c.peer ? "LINKED TO YOU" : "GUEST") : c.online ? "LINKED" : "OFFLINE";
        const dot = !c.online ? "" : c.afk ? "caution" : "good";
        const chips = (c.buttons || []).slice(0, 8).map((b) => `<button type="button" data-crew="${esc(c.id)}" data-press="${esc(b.id)}"${!c.online || (c.afkOnly && !c.afk) ? " disabled" : ""}>${esc(b.label)}</button>`).join("");
        return `<div class="mate"><div class="row"><i class="dot ${dot}"></i><b>${esc(c.name)}</b><span class="role">${role}</span>
          <button type="button" class="sbtn ask" data-ask="${esc(c.id)}" data-name="${esc(c.name)}">Ask</button></div>${chips ? `<div class="chips">${chips}</div>` : ""}</div>`;
      }).join("")}
      <button type="button" class="add" data-act="share">Share controls</button>`;
  }

  function paintFoot() {
    const foot = $("#foot");
    if (!foot) return;
    const crew = S.crew.filter((c) => c.online).slice(0, 3).map((c) => `<span><i class="dot ${c.afk ? "caution" : "good"}"></i>${esc(c.name.toUpperCase())}${c.afk ? " · AFK" : ""}</span>`).join("");
    const v = S.voice || {};
    const voice = v.state === "ready" ? "READY" : v.state === "starting" ? "STARTING" : v.state === "unavailable" ? "UNAVAILABLE" : "OFF";
    foot.innerHTML = `<div><span class="lbl">LAST</span><span>${esc(S.last || "—")}</span></div>
      ${crew ? `<div><span class="lbl">CREW</span>${crew}</div>` : ""}
      ${S.agent ? `<div><span class="lbl">AGENT</span><span>${esc(S.agent.toUpperCase())}</span></div>` : ""}
      <div><span class="lbl">VOICE</span><span>${voice}</span></div>`;
  }

  function highlight(ids) {
    const visible = ids.filter((id) => document.querySelector(`.k[data-id="${CSS.escape(id)}"]`));
    if (!visible.length) { toast("Not on this view"); return; }
    for (const id of visible) {
      const k = document.querySelector(`.k[data-id="${CSS.escape(id)}"]`);
      k.classList.remove("hl"); void k.offsetWidth; k.classList.add("hl");
      if (id === visible[0]) k.scrollIntoView({ block: "nearest", behavior: "smooth" });
      setTimeout(() => k.classList.remove("hl"), 3200);
    }
  }

  // ---------- pressing ----------

  const holds = new Map(); // pointerId -> { el, id, mode, timer, start }
  app.addEventListener("pointerdown", (e) => {
    const k = e.target.closest(".k");
    if (!k || k.disabled) return;
    e.preventDefault();
    if (k.dataset.locked) { // a guest asking for something not shared
      buzz();
      send({ t: "request", id: k.dataset.id });
      toast(`Asked for ${$(".l", k).textContent}`);
      return;
    }
    try { k.setPointerCapture(e.pointerId); } catch {}
    buzz();
    const h = { el: k, id: k.dataset.id, mode: k.dataset.mode, start: performance.now() };
    holds.set(e.pointerId, h);
    if (k.dataset.danger) { // guarded: hold for a second, the bar fills, then it fires
      k.classList.add("arming");
      const p = $(".p", k);
      p.style.transition = "width 1s linear";
      requestAnimationFrame(() => { p.style.width = "100%"; });
      h.timer = setTimeout(() => { send({ t: "tap", id: h.id }); buzz([40, 30, 40]); h.fired = true; }, 1000);
      return;
    }
    k.classList.add("pressed");
    send({ t: h.mode === "hold" ? "down" : "tap", id: h.id });
  });
  const lift = (e) => {
    const h = holds.get(e.pointerId);
    if (!h) return;
    holds.delete(e.pointerId);
    const k = h.el;
    if (k.dataset.danger) {
      clearTimeout(h.timer);
      k.classList.remove("arming");
      const p = $(".p", k);
      p.style.transition = "none"; p.style.width = "0";
      if (!h.fired) toast("Hold for a second");
      return;
    }
    if (h.mode === "hold") send({ t: "up", id: h.id });
    // A short visible press even for a quick tap.
    const left = Math.max(0, 110 - (performance.now() - h.start));
    setTimeout(() => k.classList.remove("pressed"), left);
  };
  app.addEventListener("pointerup", lift);
  app.addEventListener("pointercancel", lift);
  app.addEventListener("contextmenu", (e) => { if (e.target.closest(".k, .tbtn")) e.preventDefault(); });

  // Hold to talk.
  app.addEventListener("pointerdown", (e) => {
    const b = e.target.closest('[data-act="talk"]');
    if (!b) return;
    e.preventDefault();
    try { b.setPointerCapture(e.pointerId); } catch {}
    b.classList.add("listening");
    buzz();
    send({ t: "voice", on: true });
    const end = () => { b.classList.remove("listening"); send({ t: "voice", on: false }); b.removeEventListener("pointerup", end); b.removeEventListener("pointercancel", end); };
    b.addEventListener("pointerup", end);
    b.addEventListener("pointercancel", end);
  });

  app.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t) return;
    if (t.dataset.view) { send({ t: "view", id: t.dataset.view }); return; }
    if (t.dataset.req) {
      send({ t: "answer", id: t.dataset.req, do: !!t.dataset.do });
      S.requests = S.requests.filter((r) => r.id !== t.dataset.req);
      paintReq();
      return;
    }
    if (t.dataset.press && t.dataset.crew) { buzz(); send({ t: "crew-press", peer: t.dataset.crew, id: t.dataset.press }); return; }
    if (t.dataset.ask) { openAsk(t.dataset.ask, t.dataset.name); return; }
    switch (t.dataset.act) {
      case "afk": S.afk = !S.afk; send({ t: "afk", on: S.afk }); paintBar(); buzz(); break;
      case "find": openFind(); break;
      case "share": openShare(); break;
      case "settings": openSettings(); break;
      case "gask": openAsk(null, S.owner); break;
    }
  });

  // ---------- sheets ----------

  function openSheet(name, html) {
    sheet.dataset.open = name;
    sheet.innerHTML = `<div class="sheet" role="dialog" aria-modal="true">${html}</div>`;
  }
  function closeSheet() { sheet.dataset.open = ""; sheet.innerHTML = ""; }
  const sheetBar = (title, extra = "") => `<header class="bar"><button type="button" class="tbtn back" data-close>${ICON.back}<span>Deck</span></button><h1>${esc(title)}</h1>${extra}</header>`;
  sheet.addEventListener("click", (e) => { if (e.target.closest("[data-close]")) closeSheet(); });

  // Find: every control, searchable; press it, or build a view from results.
  let findQ = "", findGroup = "", catalog = null, draft = null;
  function openFind() {
    openSheet("find", `
      <header class="bar"><button type="button" class="tbtn back" data-close>${ICON.back}<span>Deck</span></button>
        <div class="search">${ICON.search}<label class="vh" for="fq">Find a control</label><input id="fq" placeholder="Find a control, group or view" autocomplete="off" value="${esc(findQ)}"><span class="note" id="fcount"></span></div></header>
      <div class="sheet-body"><div class="cols">
        <section aria-label="Controls"><div class="filters" id="ff"></div><div id="fr"></div></section>
        <section class="pad" aria-label="Views" id="fv"></section>
      </div></div>`);
    const input = $("#fq");
    input.oninput = () => { findQ = input.value; paintFind(); };
    fetch(`/api/v1/catalog?${auth()}`).then((r) => r.json()).then((d) => { catalog = d; paintFind(); paintViews(); });
  }
  function paintFind() {
    if (!catalog || sheet.dataset.open !== "find") return;
    const groups = catalog.groups;
    $("#ff").innerHTML = [["", "All"], ...groups.map((g) => [g.id, g.name])]
      .map(([id, name]) => `<button type="button" data-fg="${id}" aria-pressed="${findGroup === id}">${esc(name)}</button>`).join("");
    const words = findQ.toLowerCase().split(/\s+/).filter(Boolean);
    const rows = catalog.controls.filter((c) => (!findGroup || c.group === findGroup) &&
      words.every((w) => `${c.id} ${c.label} ${c.group} ${(c.tags || []).join(" ")}`.toLowerCase().includes(w)));
    $("#fcount").textContent = `${rows.length} OF ${catalog.controls.length}`;
    const code = Object.fromEntries(groups.map((g) => [g.id, g.code]));
    $("#fr").innerHTML = rows.slice(0, 120).map((c) => {
      const inDraft = draft && draft.controls.includes(c.id);
      return `<div class="res"><span class="code">${esc(code[c.group] || "")}</span>
        <span class="nm"><b>${esc(c.label)}</b><span>${esc((c.tags || []).join(" · "))}</span></span>
        <span class="bind">${esc(keyText(c.keys))}</span>
        <span class="acts"><button type="button" class="sbtn" data-fpress="${esc(c.id)}">Press</button>
        <button type="button" class="sbtn" data-fadd="${esc(c.id)}" aria-pressed="${!!inDraft}">${inDraft ? "Added" : "Add"}</button></span></div>`;
    }).join("");
  }
  function paintViews() {
    if (!catalog || sheet.dataset.open !== "find") return;
    const box = $("#fv");
    const views = viewTabs();
    const gname = Object.fromEntries(catalog.groups.map((g) => [g.id, g]));
    let html = `<div class="sec"><div class="gh"><span class="name">Views</span><span class="rule"></span></div>
      <div class="grid-btns">${views.map((v) => `<button type="button" class="opt" data-fview="${esc(v.id)}" aria-pressed="${S.view && S.view.id === v.id}">
        <span class="mono">${v.preset ? "PRESET" : "MINE"} · ${(v.groups || []).length + (v.sections || []).length}</span><span>${esc(v.name)}</span></button>`).join("")}
        <button type="button" class="opt" data-fnew="1"><span class="mono">BUILD</span><span>+ New view</span></button></div></div>`;
    if (draft) {
      html += `<div class="sec"><div class="gh"><span class="code">NEW</span><span class="name">Build a view</span><span class="rule"></span></div>
        <label class="vh" for="vname">View name</label><input class="field" id="vname" placeholder="Name, e.g. Bounty run" value="${esc(draft.name)}">
        <div class="grid-btns">${catalog.groups.map((g) => `<button type="button" class="opt" data-dgroup="${g.id}" aria-pressed="${draft.groups.includes(g.id)}"><span class="mono">${g.code}</span><span>${esc(g.name)}</span></button>`).join("")}</div>
        ${draft.controls.length ? `<div class="vgrp"><span class="code">SEL</span><b>${draft.controls.length} single controls</b><span></span><button type="button" class="x" data-dclear aria-label="Clear">×</button></div>` : ""}
        <div class="row2"><button type="button" class="big" data-dsave>Save view</button><button type="button" class="big ghost" data-dcancel>Cancel</button></div></div>`;
    } else if (S.view && !S.view.preset && !S.view.id.startsWith("agent-")) {
      html += `<div class="sec"><button type="button" class="sbtn danger" data-fdel="${esc(S.view.id)}">Delete “${esc(S.view.name)}”</button></div>`;
    }
    if (S.view) {
      html += `<div class="sec"><div class="gh"><span class="code">NOW</span><span class="name">${esc(S.view.name)}</span><span class="rule"></span></div>
        ${S.groups.map((g) => `<div class="vgrp"><span class="code">${esc(g.code)}</span><b>${esc(g.name)}</b><span class="mono">${g.controls.length}</span><span></span></div>`).join("")}</div>`;
    }
    box.innerHTML = html;
    void gname;
  }
  sheet.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheet.dataset.open !== "find") return;
    if (t.dataset.fg !== undefined) { findGroup = t.dataset.fg; paintFind(); }
    else if (t.dataset.fpress) { buzz(); send({ t: "tap", id: t.dataset.fpress }); }
    else if (t.dataset.fadd) {
      draft = draft || { name: "", groups: [], controls: [] };
      const i = draft.controls.indexOf(t.dataset.fadd);
      if (i >= 0) draft.controls.splice(i, 1); else draft.controls.push(t.dataset.fadd);
      paintFind(); paintViews();
    }
    else if (t.dataset.fview) { send({ t: "view", id: t.dataset.fview }); closeSheet(); }
    else if (t.dataset.fnew) { draft = { name: "", groups: [], controls: [] }; paintViews(); paintFind(); }
    else if (t.dataset.dgroup) {
      draft.name = $("#vname").value;
      const i = draft.groups.indexOf(t.dataset.dgroup);
      if (i >= 0) draft.groups.splice(i, 1); else draft.groups.push(t.dataset.dgroup);
      paintViews();
    }
    else if (t.dataset.dclear !== undefined) { draft.controls = []; paintViews(); paintFind(); }
    else if (t.dataset.dcancel !== undefined) { draft = null; paintViews(); paintFind(); }
    else if (t.dataset.dsave !== undefined) {
      const name = $("#vname").value.trim();
      if (!name) { toast("Name the view"); $("#vname").focus(); return; }
      if (!draft.groups.length && !draft.controls.length) { toast("Pick groups or add controls"); return; }
      const v = { name, groups: draft.groups, sections: draft.controls.length ? [{ name, controls: draft.controls }] : [] };
      send({ t: "save-view", text: JSON.stringify(v) });
      draft = null;
      setTimeout(() => { const id = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""); send({ t: "view", id }); closeSheet(); }, 250);
    }
    else if (t.dataset.fdel) { send({ t: "delete-view", id: t.dataset.fdel }); send({ t: "view", id: "flight" }); closeSheet(); }
  });

  // Share: who, which groups, how long, then a link and its QR code.
  let share = null;
  function openShare() {
    share = { who: "", groups: [], minutes: 60, afk: false, taps: false };
    const groups = (S.hello && S.hello.groups) || [];
    openSheet("share", `${sheetBar("Share controls")}
      <div class="sheet-body"><div class="cols">
        <div class="pad">
          <div class="sec"><div class="gh"><span class="code">01</span><span class="name">Who</span><span class="rule"></span></div>
            <label class="vh" for="swho">Who it's for</label><input class="field" id="swho" placeholder="Name, e.g. Mike" maxlength="32" autocomplete="off"></div>
          <div class="sec"><div class="gh"><span class="code">02</span><span class="name">What</span><span class="rule"></span></div>
            <div class="grid-btns" id="sgroups">${[{ id: "*", code: "ALL", name: "Everything" }, ...groups].map((g) => `<button type="button" class="opt" data-sg="${g.id}" aria-pressed="false"><span class="mono">${esc(g.code)}</span><span>${esc(g.name)}</span></button>`).join("")}</div></div>
          <div class="sec"><div class="gh"><span class="code">03</span><span class="name">How long</span><span class="rule"></span></div>
            <div class="seg" id="stime">${[[15, "15 min"], [60, "1 hour"], [240, "4 hours"], [0, "Until off"]].map(([m, l]) => `<button type="button" data-sm="${m}" aria-pressed="${m === 60}">${l}</button>`).join("")}</div>
            <div class="row2"><button type="button" class="sw caution" role="switch" aria-checked="false" data-safk>Only while AFK<span class="mono">OFF</span></button>
              <button type="button" class="sw" role="switch" aria-checked="false" data-staps>Taps only<span class="mono">OFF</span></button></div></div>
          <button type="button" class="big" data-screate>Create link</button>
        </div>
        <div class="pad"><div id="sout" class="sec"></div>
          <div class="sec"><div class="gh"><span class="name">Live links</span><span class="rule"></span></div><div id="slinks"></div></div></div>
      </div></div>`);
    paintLiveLinks();
  }
  function paintLiveLinks() {
    const box = $("#slinks");
    if (!box) return;
    box.innerHTML = S.grants.length ? S.grants.map((g) => {
      const left = g.expires && !String(g.expires).startsWith("0001") ? `${Math.max(0, Math.round((new Date(g.expires) - Date.now()) / 60000))} MIN LEFT` : "UNTIL OFF";
      const what = g.all ? "EVERYTHING" : (g.buttons || []).length ? `${g.buttons.length} CONTROLS` : `${(g.panels || []).length} PANELS`;
      return `<div class="list-row"><b>${esc(g.label)}</b><button type="button" class="sbtn danger" style="grid-row: span 2" data-revoke="${esc(g.id)}">Revoke</button>
        <span class="mono">${what} · ${g.afkOnly ? "WHILE AFK · " : ""}${left} · ${g.connected} CONNECTED</span></div>`;
    }).join("") : `<p class="note">No live links.</p>`;
  }
  function showShared(m) {
    const link = m.links.internet || m.links.lan;
    const out = $("#sout");
    if (!out) return;
    out.innerHTML = `<div style="display:flex;gap:16px;align-items:flex-start;flex-wrap:wrap"><canvas class="qr" id="sqr"></canvas>
      <div class="sec" style="flex:1;min-width:180px"><b style="font-family:var(--label);font-size:22px;text-transform:uppercase;letter-spacing:.06em">${esc(m.label)}</b>
      <span class="note"><i class="dot ${m.links.internet ? "good" : "caution"}"></i>${m.links.internet ? "Works anywhere" : "Home network only"}</span></div></div>
      <code class="link">${esc(link)}</code>
      <div class="row2"><button type="button" class="big" data-scopy="${esc(link)}">Copy link</button>${navigator.share ? `<button type="button" class="big ghost" data-ssend="${esc(link)}">Send…</button>` : ""}</div>`;
    drawQR($("#sqr"), link);
  }
  function drawQR(canvas, text) {
    if (!window.DeckQR) return;
    const rows = window.DeckQR.matrix(text), n = rows.length, scale = Math.max(3, Math.floor(148 / n));
    canvas.width = canvas.height = n * scale;
    const g = canvas.getContext("2d");
    g.fillStyle = "#fff"; g.fillRect(0, 0, canvas.width, canvas.height); g.fillStyle = "#000";
    rows.forEach((row, y) => row.forEach((d, x) => { if (d) g.fillRect(x * scale, y * scale, scale, scale); }));
  }
  sheet.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheet.dataset.open !== "share") return;
    if (t.dataset.sg) {
      if (t.dataset.sg === "*") share.groups = share.groups.includes("*") ? [] : ["*"];
      else { share.groups = share.groups.filter((g) => g !== "*"); const i = share.groups.indexOf(t.dataset.sg); if (i >= 0) share.groups.splice(i, 1); else share.groups.push(t.dataset.sg); }
      sheet.querySelectorAll("[data-sg]").forEach((b) => b.setAttribute("aria-pressed", String(share.groups.includes(b.dataset.sg))));
    } else if (t.dataset.sm !== undefined) {
      share.minutes = Number(t.dataset.sm);
      sheet.querySelectorAll("[data-sm]").forEach((b) => b.setAttribute("aria-pressed", String(b === t)));
    } else if (t.dataset.safk !== undefined || t.dataset.staps !== undefined) {
      const key = t.dataset.safk !== undefined ? "afk" : "taps";
      share[key] = !share[key];
      t.setAttribute("aria-checked", String(share[key]));
      $(".mono", t).textContent = share[key] ? "ON" : "OFF";
    } else if (t.dataset.screate !== undefined) {
      const who = $("#swho").value.trim();
      if (!who) { toast("Who is it for?"); $("#swho").focus(); return; }
      if (!share.groups.length) { toast("Pick what to share"); return; }
      const all = share.groups.includes("*");
      send({ t: "share", label: who, minutes: share.minutes, all, groups: all ? [] : share.groups, afkOnly: share.afk, tapsOnly: share.taps });
    } else if (t.dataset.scopy) {
      (navigator.clipboard ? navigator.clipboard.writeText(t.dataset.scopy) : Promise.reject()).then(() => toast("Copied"), () => toast(t.dataset.scopy, 6000));
    } else if (t.dataset.ssend) {
      navigator.share({ title: "Tablet Deck", text: "Fly with me", url: t.dataset.ssend }).catch(() => {});
    } else if (t.dataset.revoke) {
      fetch(`/api/v1/grants/${encodeURIComponent(t.dataset.revoke)}?${auth()}`, { method: "DELETE" }).catch(() => {});
    }
  });

  // Ask: a quick message to a crewmate (or, for a guest, to the owner).
  let askTo = null;
  function openAsk(to, name) {
    askTo = to;
    openSheet("ask", `${sheetBar(`Ask ${name || "your crew"}`)}
      <div class="sheet-body"><div class="pad" style="max-width:760px">
        <div class="grid-btns">${QUICK.map((q) => `<button type="button" class="opt" data-quick="${esc(q)}"><span class="mono">QUICK</span><span>${esc(q)}</span></button>`).join("")}</div>
        <form id="askf" style="display:flex;gap:6px"><label class="vh" for="askt">Message</label><input class="field" id="askt" maxlength="140" placeholder="Message" autocomplete="off" style="flex:1"><button class="big" style="padding:0 22px">Send</button></form>
      </div></div>`);
    $("#askf").onsubmit = (e) => { e.preventDefault(); sendAsk($("#askt").value); };
  }
  function sendAsk(text) {
    text = String(text || "").trim();
    if (!text) return;
    if (askTo) send({ t: "crew-request", peer: askTo, text }); else send({ t: "request", text });
    buzz(); closeSheet();
  }
  sheet.addEventListener("click", (e) => { const t = e.target.closest("[data-quick]"); if (t) sendAsk(t.dataset.quick); });

  // Settings: theme, display, and the deck's own actions.
  function openSettings() {
    const cur = document.documentElement.dataset.theme;
    const sw = (k, label) => `<button type="button" class="sw" role="switch" aria-checked="${store.get(k) === "1"}" data-disp="${k}">${label}<span class="mono">${store.get(k) === "1" ? "ON" : "OFF"}</span></button>`;
    openSheet("settings", `${sheetBar("Settings")}
      <div class="sheet-body"><div class="pad">
        <div class="sec"><div class="gh"><span class="name">Theme</span><span class="rule"></span></div>
          <div class="themes">${THEMES.map(([id, name, font]) => themeTile(id, name, font, id === cur)).join("")}</div></div>
        <div class="sec"><div class="gh"><span class="name">Display</span><span class="rule"></span></div>
          <div class="grid-btns">${sw("night", "Night")}${sw("large", "Large text")}${sw("contrast", "High contrast")}</div></div>
        <div class="sec"><div class="gh"><span class="name">Deck</span><span class="rule"></span></div>
          <div class="grid-btns">
            <button type="button" class="sw" data-do="release">Release all keys</button>
            <button type="button" class="sw" data-do="lights">Reset button lights</button>
            <button type="button" class="sw" data-do="fullscreen">Full screen</button>
            <a class="sw" href="/classic.html?${auth()}" style="text-decoration:none">Classic HUD</a>
            <button type="button" class="sw" data-do="unpair" style="color:var(--danger-text);border-color:var(--danger)">Forget this PC</button>
          </div></div>
        <p class="note">Tablet Deck · <a href="https://github.com/xattribution/tablet-deck-spooned" target="_blank" rel="noopener" style="color:var(--accent-text)">GitHub</a></p>
      </div></div>`);
  }
  function themeTile(id, name, font, sel) {
    // Read the theme's tokens by applying it for a moment.
    const root = document.documentElement, prev = root.dataset.theme;
    root.dataset.theme = id;
    const cs = getComputedStyle(root), v = (n) => cs.getPropertyValue(n).trim();
    const t = { ground: v("--ground"), key: v("--key"), line: v("--line"), text: v("--text"), dim: v("--dim"), accent: v("--accent"), on: v("--on-accent") };
    root.dataset.theme = prev;
    return `<button type="button" class="theme" data-theme-pick="${id}" aria-pressed="${sel}" style="background:${t.ground};color:${t.text};border:${sel ? `2px solid ${t.text}` : `1px solid ${t.line}`}">
      <span class="tb" style="border-color:${t.line};background:${sel ? t.accent : "transparent"};color:${sel ? t.on : t.text}"><b style="font-family:'${font}',sans-serif">${esc(name)}</b><span class="mono">${sel ? "IN USE" : ""}</span></span>
      <span class="tk"><span style="background:${t.accent};color:${t.on}">Gear<span class="mono">DOWN</span></span><span style="background:${t.key};border:1px solid ${t.line}">VTOL<span class="mono" style="color:${t.dim}">K</span></span></span></button>`;
  }
  sheet.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheet.dataset.open !== "settings") return;
    if (t.dataset.themePick) { applyTheme(t.dataset.themePick); openSettings(); paintBar(); }
    else if (t.dataset.disp) { store.set(t.dataset.disp, store.get(t.dataset.disp) === "1" ? "" : "1"); applyDisplay(); openSettings(); }
    else if (t.dataset.do === "release") { send({ t: "release-all" }); }
    else if (t.dataset.do === "lights") { send({ t: "reset-states" }); toast("Lights reset"); }
    else if (t.dataset.do === "fullscreen") { const d = document.documentElement; if (document.fullscreenElement) document.exitFullscreen(); else (d.requestFullscreen || d.webkitRequestFullscreen)?.call(d); }
    else if (t.dataset.do === "unpair") { store.set("token", ""); store.set("pairCode", ""); token = ""; stop(); closeSheet(); drawPair(); }
  });

  // ---------- pairing ----------

  function drawPair(error) {
    closeSheet();
    app.innerHTML = `<header class="bar"><div class="brand"><i></i><b>Tablet Deck</b></div></header>
      <div class="sheet-body"><form class="pair" id="pairf" novalidate>
        <span class="note">${esc(location.host)}</span>
        <h1>Pair this device</h1>
        <label for="code">Code on your PC</label>
        <input id="code" autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" maxlength="32" value="${esc(token)}" class="${error ? "bad" : ""}">
        ${error ? `<p class="err" role="alert">${esc(error)}</p>` : ""}
        <button class="big">Connect</button>
      </form></div>`;
    $("#pairf").onsubmit = (e) => {
      e.preventDefault();
      token = $("#code").value.trim().toLowerCase();
      if (!token) return;
      start();
    };
  }

  // A share link that ran out or was turned off.
  function drawGone(text) {
    app.innerHTML = `<header class="bar"><div class="brand"><i></i><b>Tablet Deck</b></div></header>
      <div class="sheet-body"><div class="pair"><h1>Link closed</h1><p class="note">${esc(text)}</p></div></div>`;
  }

  // ---------- guest on a share link ----------

  function drawGuest() {
    const g = S.guest;
    const waiting = g.afkOnly && !S.afk;
    const groups = (g.groups || []);
    const shared = groups.filter((gr) => gr.controls.some((c) => !c.locked)).map((gr) => gr.name).join(", ");
    app.innerHTML = `
      <div class="guestbar ${waiting ? "waiting" : ""}"><b>${esc(S.owner || "Their")}'s deck</b><span class="clock" id="gclock"></span>
        <span class="mono">${esc(shared || "nothing yet")}${waiting ? ` · works while ${esc(S.owner)} is AFK` : S.afk ? ` · ${esc(S.owner)} is AFK` : ""}</span></div>
      <main class="deck" id="deck" aria-label="Controls">${groups.map((gr) => groupHTML(gr, { locked: true })).join("")}</main>
      <div class="askbar"><div class="quick">${QUICK.map((q) => `<button type="button" data-gquick="${esc(q)}">${esc(q)}</button>`).join("")}</div>
        <form id="gaskf"><label class="vh" for="gask">Message</label><input class="field" id="gask" placeholder="Message ${esc(S.owner)}" maxlength="140" autocomplete="off"><button class="big" style="padding:0 22px">Send</button></form></div>`;
    $("#gaskf").onsubmit = (e) => { e.preventDefault(); const v = $("#gask").value.trim(); if (v) { send({ t: "request", text: v }); $("#gask").value = ""; } };
    app.querySelectorAll("[data-gquick]").forEach((b) => { b.onclick = () => { send({ t: "request", text: b.dataset.gquick }); buzz(); }; });
    paintLights();
    tickClock();
  }
  function tickClock() {
    const el = $("#gclock");
    if (!el || !S.guest) return;
    const exp = S.guest.expires;
    if (!exp || String(exp).startsWith("0001")) { el.textContent = ""; return; }
    const s = Math.max(0, Math.round((new Date(exp) - Date.now()) / 1000));
    el.textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
  }
  setInterval(tickClock, 1000);

  // ---------- start ----------

  function start() {
    // Check the code (or token) before opening the socket, so a wrong one shows the
    // pairing screen with a reason instead of reconnecting forever.
    fetch(GUEST ? `/layout.json?${auth()}` : `/api/v1/views?${auth()}`, { cache: "no-store" }).then((r) => {
      if (r.ok) { if (!GUEST && token !== store.get("token")) { store.set("token", token); store.set("pairCode", ""); } connect(); return; }
      if (GUEST) { drawGone(r.status === 410 ? "This share link ran out." : "This share link isn't valid any more."); return; }
      drawPair(r.status === 429 ? "Too many wrong codes. Wait a minute." : token ? "That code didn't match the one on your PC." : "");
    }).catch(() => { app.innerHTML = `<p class="empty">Can't reach the PC. Retrying…</p>`; setTimeout(start, 2000); });
  }
  if (GUEST || token) start(); else drawPair();
})();
