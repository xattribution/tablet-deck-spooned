// Tablet Deck 3: the console. The screen is one cockpit panel. Panes of keys fill it
// exactly and never scroll: the deck pages sideways, systems rise from below (swipe up)
// and comms drop from above (swipe down). Every control comes from the PC's catalog,
// and a view can be changed from here, the PC, voice or an agent over MCP.
(() => {
  "use strict";

  // ---------- basics ----------

  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => Array.from(el.querySelectorAll(s));
  const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const store = {
    get(k) { try { return localStorage.getItem(k); } catch { return null; } },
    set(k, v) { try { if (v === null || v === undefined || v === "") localStorage.removeItem(k); else localStorage.setItem(k, v); } catch {} },
  };
  const buzz = (ms = 10) => { if (navigator.vibrate) navigator.vibrate(ms); };
  const reduced = () => matchMedia("(prefers-reduced-motion: reduce)").matches;
  const keyText = (k) => String(k || "").split("+").map((p) => p.replace(/^np_/, "num ").replace(/^l(alt|ctrl|shift)$/, "$1").replace(/^r(alt|ctrl|shift)$/, "r$1")).join(" ").toUpperCase();
  const icon = (name) => window.deckIcon(name);
  const CAT = (group) => window.DeckCat.of(group);
  const CATS = ["flight", "nav", "eng", "def", "combat", "ind", "crew", "ui", "emerg"];
  const CAT_CODE = { flight: "FLT", nav: "NAV", eng: "ENG", def: "DEF", combat: "CBT", ind: "IND", crew: "CRW", ui: "UI", emerg: "EMG" };
  const THEMES = ["rsi", "hud", "mobiglas", "drake", "origin", "crusader", "aegis", "anvil", "misc", "argo", "cnou", "banu", "aopoa", "esperia"];
  const BLOCKED = {
    "not-running": ["GAME NOT RUNNING", "game"], "not-focused": ["GAME NOT IN FRONT", "game"], "focus-failed": ["GAME WON'T COME FORWARD", "game"],
    "denied": ["NOT SHARED", ""], "not-afk": ["ONLY WHILE AFK", "afk"], "rate-limited": ["SLOW DOWN", ""], "expired": ["LINK RAN OUT", ""],
  };
  const QUICK = ["BRB", "On it", "Cover me", "Take the helm", "Gear down", "Spool QT", "Need you back"];
  const UI = {
    up: '<svg class="i" viewBox="0 0 24 24"><path d="m6 15 6-6 6 6"/></svg>',
    down: '<svg class="i" viewBox="0 0 24 24"><path d="m6 9 6 6 6-6"/></svg>',
    close: '<svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6 6 18"/></svg>',
    find: '<svg class="i" viewBox="0 0 24 24"><circle cx="10.5" cy="10.5" r="6"/><path d="m15 15 5 5"/></svg>',
    share: '<svg class="i" viewBox="0 0 24 24"><circle cx="6" cy="12" r="2.5"/><circle cx="18" cy="6" r="2.5"/><circle cx="18" cy="18" r="2.5"/><path d="m8.3 10.8 7.4-3.6M8.3 13.2l7.4 3.6"/></svg>',
    theme: '<svg class="i" viewBox="0 0 24 24"><path d="M4 20h16M6 16l6-12 6 12"/><path d="M8.5 11h7"/></svg>',
    release: '<svg class="i" viewBox="0 0 24 24"><path d="M12 3v9"/><path d="M6.3 7.5a8 8 0 1 0 11.4 0"/></svg>',
    full: '<svg class="i" viewBox="0 0 24 24"><path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/></svg>',
    afk: '<svg class="i" viewBox="0 0 24 24"><circle cx="12" cy="5" r="2"/><path d="m9 21 2-6-3-3 2-4 4 2 3 1M13 15l3 6"/></svg>',
    mic: '<svg class="i" viewBox="0 0 24 24"><rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/></svg>',
    plus: '<svg class="i" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></svg>',
    send: '<svg class="i" viewBox="0 0 24 24"><path d="M4 12 20 4l-4 16-4-7Z"/></svg>',
    lights: '<svg class="i" viewBox="0 0 24 24"><circle cx="12" cy="11" r="5"/><path d="M10 19h4M12 2v2M4 11H2M22 11h-2"/></svg>',
    unlink: '<svg class="i" viewBox="0 0 24 24"><path d="M9 7H7a5 5 0 0 0 0 10h2M15 7h2a5 5 0 0 1 0 10h-2M4 4l16 16"/></svg>',
    lock: '<svg class="i" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="10" rx="1"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>',
    reset: '<svg class="i" viewBox="0 0 24 24"><path d="M4 12a8 8 0 1 0 2.3-5.6M4 4v4h4"/></svg>',
  };

  const root = document.documentElement;
  const con = $("#console"), layer = $("#layer");
  const GUEST = (location.pathname.match(/^\/g\/([a-f0-9]{20,64})/) || [])[1] || "";
  const urlCode = (new URLSearchParams(location.search).get("t") || "").trim().toLowerCase();
  let token = GUEST ? "" : (urlCode && urlCode !== store.get("pairCode") ? urlCode : store.get("token") || urlCode || "");
  const auth = () => (GUEST ? `g=${GUEST}` : `t=${encodeURIComponent(token)}`);
  const device = new URLSearchParams(location.search).get("device") || (Math.min(screen.width, screen.height) < 500 ? "phone" : "tablet");

  const S = {
    hello: null, views: [], view: null, groups: [], lit: {}, game: null, afk: false, crew: [], requests: [], grants: [],
    voice: { state: "off" }, latency: 0, agent: "", online: false, guest: null, owner: "", listening: false,
    pages: [], page: 0, mode: "", editing: false, // "", "deck", "guest", "pair", "gone", "wait"
  };

  // ---------- theme ----------

  function applyTheme(id) {
    if (!THEMES.includes(id)) id = "rsi";
    root.dataset.theme = id;
    store.set("theme", id);
    const meta = $('meta[name="theme-color"]');
    if (meta) meta.content = getComputedStyle(root).getPropertyValue("--surface").trim();
    if (window.DeckArt) window.DeckArt.apply(id);
    paintMaker();
  }
  const emblem = (id) => {
    const e = (window.DECK_EMBLEMS || {})[id || root.dataset.theme];
    return e ? `<svg viewBox="0 0 64 64" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="--text:currentColor">${e.emblem}</svg>` : "";
  };
  const makerName = (id) => ((window.DECK_EMBLEMS || {})[id || root.dataset.theme] || {}).name || "Tablet Deck";

  let toastTimer = null;
  function toast(text, ms = 1800) {
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
    ws.onopen = () => { backoff = 500; lastPong = Date.now(); S.online = true; ping(); paintLamps(); };
    ws.onmessage = (e) => { let m; try { m = JSON.parse(e.data); } catch { return; } handle(m); };
    ws.onclose = () => {
      S.online = false;
      paintLamps();
      $$(".k.pressed").forEach((k) => k.classList.remove("pressed"));
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
      case "pong": lastPong = Date.now(); S.latency = Math.round(performance.now() - m.n); paintLamps(); break;
      case "hello":
        S.hello = m; S.lit = m.states || {}; S.game = m.game; S.afk = !!m.afk; S.owner = m.name || "";
        if (m.guest) { S.guest = m.guest; S.groups = m.guest.groups || []; drawConsole("guest"); break; }
        S.views = m.views || []; S.crew = m.crew || []; S.requests = m.requests || []; S.grants = m.grants || [];
        S.voice = m.voiceStatus || S.voice;
        send({ t: "view", id: store.get(`view-${device}`) || "flight" });
        break;
      case "paired": token = m.token; store.set("token", token); store.set("pairCode", urlCode); break;
      case "view": {
        const changed = !S.view || S.view.id !== m.view.id;
        S.view = m.view; S.groups = m.groups || [];
        if (!m.view.id.startsWith("agent-")) store.set(`view-${device}`, m.view.id);
        if (changed) S.page = 0;
        drawConsole("deck", { announce: changed });
        if (sheetName() === "systems") openSystems();
        break;
      }
      case "views": S.views = m.views || []; if (sheetName() === "systems") openSystems(); break;
      case "state": S.lit[m.group] = m.on; paintLights(); break;
      case "states": S.lit = m.states || {}; paintLights(); break;
      case "game": S.game = m; paintLamps(); break;
      case "afk":
        S.afk = m.on;
        if (S.guest) drawConsole("guest"); else paintLamps();
        if (sheetName() === "comms") openComms();
        break;
      case "crew": S.crew = m.crew || []; paintLamps(); if (sheetName() === "comms") openComms(); break;
      case "grants": S.grants = m.grants || []; if (sheetName() === "share") paintShare(); break;
      case "request": S.requests.push(m.request); paintIncoming(); paintLamps(); buzz([20, 40, 20]); break;
      case "request-gone": S.requests = S.requests.filter((r) => r.id !== m.id); paintIncoming(); paintLamps(); break;
      case "answered": ticker(m.result === "done" ? "DONE" : "DECLINED", m.label || m.text || ""); break;
      case "pressed":
        if (m.kind !== "deck") { ticker(m.who || m.kind, m.label || keyText(m.keys)); flash(m.id); }
        break;
      case "agent": S.agent = m.tool; ticker("AGENT", String(m.tool || "").replace(/_/g, " ")); break;
      case "blocked": {
        const [text, lamp] = BLOCKED[m.reason] || ["NOT PRESSED", ""];
        toast(text);
        buzz([30, 60, 30]);
        if (lamp) alarm(lamp);
        $$(`.k[data-id="${CSS.escape(m.id)}"]`).forEach((k) => { k.classList.remove("blocked"); void k.offsetWidth; k.classList.add("blocked"); });
        break;
      }
      case "released":
        $$(`.k[data-id="${CSS.escape(m.id)}"]`).forEach((k) => k.classList.remove("pressed"));
        if (m.reason === "focus-lost") { toast("RELEASED · GAME LOST FOCUS"); alarm("game"); }
        break;
      case "highlight": highlight(m.ids || []); break;
      case "theme": applyTheme(m.id); drawConsole(S.mode); break;
      case "voice-status": S.voice = m; paintLamps(); break;
      case "listening": S.listening = m.on; paintLamps(); break;
      case "heard": ticker("HEARD", m.text || m.label); break;
      case "unheard": ticker("VOICE", "didn't catch that"); break;
      case "voice-off": toast(m.reason); break;
      case "toast": toast(m.text, 2600); break;
      case "opened": ticker("PC", `opened ${m.name}`); break;
      case "shared": shareResult = m; paintShare(); break;
      case "reload": send({ t: "view", id: S.view ? S.view.id : "flight" }); break;
      case "unpaired": store.set("token", ""); store.set("pairCode", ""); token = ""; stop(); drawPair("This device was unpaired on the PC."); break;
      case "revoked": stop(); drawGone(m.reason === "expired" ? "This share link ran out." : "This share link was turned off."); break;
    }
  }

  // ---------- layout: panels into columns, columns into pages ----------

  // Panels stack in columns (three on a tablet, two on a small one, one on a phone) and
  // fill the screen exactly; a seam is all that separates them. What doesn't fit goes to
  // the next page. Once the user rearranges a view, that arrangement is kept per device.
  const ncols = (W) => (W >= 1000 ? 3 : W >= 620 ? 2 : 1);
  const layoutKey = () => `layout-${device}-${S.mode === "guest" ? "guest" : S.view ? S.view.id : ""}`;
  let saved = null;
  const loadSaved = () => { try { saved = JSON.parse(store.get(layoutKey()) || "null"); } catch { saved = null; } };
  const saveLayout = () => store.set(layoutKey(), saved ? JSON.stringify(saved) : "");

  // Pages hold as many panels as fit in C columns; within a page, panels split into
  // columns so the columns carry about the same weight (no column left with one key).
  function autoPages(groups, W, H) {
    const C = ncols(W), scale = W < 620 ? 0.82 : 1;
    const it = groups.map((g) => { const s = window.DeckPanels.size(g); return { id: g.id, w: s.weight, min: Math.min(H, s.min * scale) + 2 }; });
    const fits = (a, b, cols) => { let n = 1, used = 0; for (let i = a; i < b; i++) { if (used && used + it[i].min > H) { n++; used = 0; } used += it[i].min; } return n <= cols; };
    const pages = [];
    for (let a = 0; a < it.length;) {
      let b = a + 1;
      while (b < it.length && fits(a, b + 1, C)) b++;
      const k = Math.min(C, b - a);
      // Best split of it[a..b) into k columns: smallest heaviest column, every column fits.
      const memo = new Map();
      const best = (i, cols) => {
        const key = `${i}:${cols}`;
        if (memo.has(key)) return memo.get(key);
        let res = null;
        if (cols === 1) {
          let h = 0, w = 0;
          for (let j = i; j < b; j++) { h += it[j].min; w += it[j].w; }
          res = h <= H || b - i === 1 ? { cost: w, cuts: [] } : null;
        } else {
          let h = 0, w = 0;
          for (let j = i; j < b - cols + 1; j++) {
            h += it[j].min; w += it[j].w;
            if (h > H && j > i) break;
            const rest = best(j + 1, cols - 1);
            if (rest) { const cost = Math.max(w, rest.cost); if (!res || cost < res.cost) res = { cost, cuts: [j + 1, ...rest.cuts] }; }
          }
        }
        memo.set(key, res);
        return res;
      };
      const plan = best(a, k) || { cuts: [] };
      const edges = [a, ...plan.cuts, b];
      pages.push(edges.slice(0, -1).map((s0, i) => it.slice(s0, edges[i + 1]).map((x) => x.id)));
      a = b;
    }
    return pages;
  }
  // A saved arrangement against the view's groups now: drop what's gone, add what's new.
  function reconcile(pages, groups) {
    const ids = new Set(groups.map((g) => g.id)), seen = new Set();
    const out = pages.map((p) => p.map((c) => c.filter((id) => ids.has(id) && !seen.has(id) && seen.add(id))).filter((c) => c.length)).filter((p) => p.length);
    const extra = groups.filter((g) => !seen.has(g.id)).map((g) => g.id);
    if (extra.length) { if (!out.length) out.push([[]]); out[out.length - 1][out[out.length - 1].length - 1].push(...extra); }
    return out;
  }

  const labelMode = () => (store.get("labels") === "off" ? "off" : "full");
  const showBinds = () => store.get("binds") === "on";
  const panelOpts = () => ({ guest: S.mode === "guest", lit: S.lit, landingMode: store.get("landing-mode") || "landing" });
  function paneHTML(g) {
    const wt = (saved && saved.wt && saved.wt[g.id]) || window.DeckPanels.size(g).weight;
    return `<section class="pane" data-panel="${esc(g.id)}" data-cat="${CAT(g.id)}" style="flex:${wt} 1 0" aria-label="${esc(g.name)}">
      <header class="plate"><i class="lamp"></i><span class="nm">${esc(g.name)}</span><span class="no">${esc(g.code || "")}</span></header>
      <div class="pbody">${window.DeckPanels.render(g, panelOpts())}</div></section>`;
  }

  // The frame around the panels: maker plate, page notches, annunciator lamps and the
  // layout lock. Grips on the top and bottom edges open comms and systems.
  function drawConsole(mode, { announce = false } = {}) {
    S.mode = mode;
    loadSaved();
    con.innerHTML = `
      <button type="button" class="grip top" data-sheet="comms" aria-label="Comms"><i></i></button>
      <div class="viewport" id="vp"><div class="pages" id="pages"></div></div>
      <button type="button" class="edge l" data-go="-1" aria-label="Previous page"></button>
      <button type="button" class="edge r" data-go="1" aria-label="Next page"></button>
      <footer class="frame">
        <div class="maker" id="maker"></div>
        <div class="notches" id="notches" role="tablist" aria-label="Pages"></div>
        <div class="lamps" id="lamps"></div>
        <div class="lock" id="lock" role="slider" aria-label="Unlock layout: slide up, then right" aria-valuemin="0" aria-valuemax="1" aria-valuenow="0" tabindex="0"><i class="groove"></i><i class="lk">${UI.lock}</i></div>
      </footer>
      <button type="button" class="grip bottom" data-sheet="systems" aria-label="Systems"><i></i></button>`;
    con.classList.toggle("editing", !!S.editing);
    layoutPages();
    paintMaker(announce);
    paintLamps();
    paintIncoming();
    if (announce) boot($$(".pane"));
    orient();
  }

  let lastSize = "";
  function layoutPages() {
    const vp = $("#vp"), box = $("#pages");
    if (!vp || !box) return;
    const W = vp.clientWidth, H = vp.clientHeight;
    lastSize = `${W}x${H}`;
    if (S.mode !== "deck" && S.mode !== "guest") return;
    if (!S.groups.length) {
      box.innerHTML = `<div class="page"><div class="pcol"><section class="pane" data-cat="ui"><header class="plate"><i class="lamp blink"></i><span class="nm">Linking</span></header><div class="pbody"></div></section></div></div>`;
      S.pages = [];
      paintNotches();
      return;
    }
    const by = Object.fromEntries(S.groups.map((g) => [g.id, g]));
    const ids = saved && saved.pages ? reconcile(saved.pages, S.groups) : autoPages(S.groups, W, H);
    S.pages = ids.map((p) => p.map((c) => c.map((id) => by[id])));
    S.page = Math.min(S.page, S.pages.length - 1);
    const cw = (pi, ci) => (saved && saved.cw && saved.cw[`${pi}.${ci}`]) || 1;
    box.innerHTML = S.pages.map((p, pi) => `<div class="page" data-p="${pi}">${p.map((c, ci) =>
      `${ci ? `<i class="seam v" data-p="${pi}" data-c="${ci}"></i>` : ""}<div class="pcol" style="flex:${cw(pi, ci)} 1 0">${c.map((g, ri) =>
        `${ri ? `<i class="seam h" data-p="${pi}" data-c="${ci}" data-r="${ri}"></i>` : ""}${paneHTML(g)}`).join("")}</div>`).join("")}</div>`).join("");
    window.DeckPanels.fitGrids(box);
    goPage(S.page, true);
  }

  // Each notch carries the colors of the systems on its page, so the strip is a map.
  const pageCats = (p) => [...new Set(p.flat().map((g) => CAT(g.id)))];
  function paintNotches() {
    const el = $("#notches");
    if (!el) return;
    el.innerHTML = S.pages.length > 1 ? S.pages.map((p, i) => `<button type="button" class="notch" role="tab" data-page="${i}" aria-selected="${i === S.page}" aria-label="Page ${i + 1}">
      ${pageCats(p).map((c) => `<i style="background:var(--c-${c})"></i>`).join("")}</button>`).join("") : "";
    for (const [sel, d] of [[".edge.l", -1], [".edge.r", 1]]) {
      const e = $(sel), p = S.pages[S.page + d];
      if (!e) continue;
      e.hidden = !p;
      if (p) e.innerHTML = pageCats(p).map((c) => `<i style="background:var(--c-${c})"></i>`).join("");
    }
  }

  function goPage(i, instant) {
    const box = $("#pages");
    if (!box) return;
    const n = Math.max(1, S.pages.length);
    i = Math.max(0, Math.min(n - 1, i));
    const moved = i !== S.page;
    S.page = i;
    box.classList.toggle("dragging", !!instant);
    box.style.transform = `translateX(${-i * 100}%)`;
    if (instant) { void box.offsetWidth; box.classList.remove("dragging"); }
    paintNotches();
    if (moved && !instant) { buzz(6); const pg = $$(".page", box)[i]; if (pg) boot($$(".pane", pg)); }
  }

  // Panels power on in a short stagger when they arrive, so the eye catches the change.
  function boot(panes) {
    if (reduced()) return;
    panes.forEach((p, j) => { p.classList.remove("boot"); p.style.setProperty("--d", `${j * 45}ms`); void p.offsetWidth; p.classList.add("boot"); });
  }

  function paintMaker(announce) {
    const el = $("#maker");
    if (!el) return;
    const name = S.mode === "guest" ? `${S.owner || "Their"} deck` : S.view ? S.view.name : makerName();
    el.innerHTML = `<span class="em">${emblem()}</span><b>${esc(makerName())}</b><span class="vw${announce ? " new" : ""}">${esc(name)}</span><span class="tick" id="tick"></span>`;
  }

  // A line in the frame for things that happen without this deck: a crewmate's press,
  // the agent, voice. It fades on its own.
  let tickTimer = null;
  function ticker(who, what) {
    const el = $("#tick");
    if (!el) return;
    el.innerHTML = `<b>${esc(String(who || "").toUpperCase())}</b> ${esc(String(what || "").toUpperCase())}`;
    el.classList.remove("show"); void el.offsetWidth; el.classList.add("show");
    clearTimeout(tickTimer);
    tickTimer = setTimeout(() => el.classList.remove("show"), 3200);
  }

  function lampHTML(id, label, cls, extra = "") {
    return `<button type="button" class="lampb ${cls}" data-lamp="${id}" ${extra}><i></i><span>${label}</span></button>`;
  }
  function paintLamps() {
    const el = $("#lamps");
    if (!el) return;
    if (S.editing) {
      el.innerHTML = `<button type="button" class="lampb edit" data-edit="reset">${UI.reset}<span>Reset</span></button><button type="button" class="lampb edit on" data-edit="done">${UI.lock}<span>Lock</span></button>`;
      return;
    }
    const g = S.game || {};
    const link = !S.online ? "bad" : S.latency > 120 ? "caution" : "good";
    const game = !g.running ? "bad" : g.focused ? "good" : "caution";
    let html = lampHTML("link", S.online && S.latency ? `${S.latency}MS` : "LINK", link);
    if (S.mode === "guest") {
      html += lampHTML("afk", "AFK", S.afk ? "caution" : "") + `<span class="lampb clock"><span id="gclock"></span></span>`;
    } else {
      html += lampHTML("game", "GAME", game);
      const online = S.crew.filter((c) => c.online).length;
      html += lampHTML("comms", S.requests.length ? `COMMS ${S.requests.length}` : online ? `CREW ${online}` : "COMMS", S.requests.length ? "caution blink" : online ? "good" : "");
      html += lampHTML("afk", "AFK", S.afk ? "caution" : "");
      const v = S.voice || {};
      if (v.state === "ready" || v.state === "starting") html += lampHTML("talk", "TALK", S.listening ? "good blink" : v.state === "ready" ? "" : "caution", 'aria-label="Hold to talk"');
    }
    el.innerHTML = html;
    tickClock();
  }
  function alarm(id) {
    const l = $(`[data-lamp="${id}"]`);
    if (!l) return;
    l.classList.remove("alarm"); void l.offsetWidth; l.classList.add("alarm");
  }

  // State lights: keys light, the gear lever moves and its lights go through red.
  function paintLights() {
    $$("[data-group]").forEach((k) => {
      if (k.dataset.locked) return;
      const on = !!S.lit[k.dataset.group];
      if (k.classList.contains("on") === on) return;
      k.classList.toggle("on", on);
      if (k.classList.contains("lever")) { k.classList.remove("transit"); void k.offsetWidth; k.classList.add("transit"); }
      if (window.DeckFX3) window.DeckFX3.state(k, on);
    });
  }

  // A request drops in from the top edge, where comms live.
  function paintIncoming() {
    let el = $("#incoming");
    const r = S.requests[S.requests.length - 1];
    if (!r || S.mode !== "deck" || sheetName() === "comms") { if (el) el.remove(); return; }
    if (!el) { el = document.createElement("div"); el.id = "incoming"; el.className = "incoming"; el.setAttribute("role", "alert"); document.body.appendChild(el); }
    el.innerHTML = `<span class="who">${esc(r.from.toUpperCase())}${S.requests.length > 1 ? ` · ${S.requests.length}` : ""}</span>
      <div>${r.label ? `<b>${esc(r.label)}</b>` : ""}${r.text ? `<q>${esc(r.text)}</q>` : ""}</div>
      <span class="acts"><button type="button" class="do" data-req="${esc(r.id)}" data-do="1">${r.label ? "Do it" : "On it"}</button><button type="button" class="no" data-req="${esc(r.id)}">${r.label ? "No" : "OK"}</button></span>`;
  }

  const els = (id) => $$(`[data-id="${CSS.escape(id || "")}"], [data-up="${CSS.escape(id || "")}"], [data-down="${CSS.escape(id || "")}"]`);
  function flash(id) {
    els(id).forEach((k) => { k.classList.remove("hl"); void k.offsetWidth; k.classList.add("hl"); setTimeout(() => k.classList.remove("hl"), 1600); });
  }
  // An agent or the PC points something out: go to its page, then flash it.
  function highlight(ids) {
    const pages = $$("#pages > .page");
    const p = pages.findIndex((pg) => ids.some((id) => pg.querySelector(`[data-id="${CSS.escape(id)}"], [data-up="${CSS.escape(id)}"], [data-down="${CSS.escape(id)}"]`)));
    if (p < 0) { toast("NOT ON THIS VIEW"); return; }
    closeSheet();
    goPage(p);
    setTimeout(() => ids.forEach((id) => els(id).forEach((k) => { k.classList.remove("hl"); void k.offsetWidth; k.classList.add("hl"); setTimeout(() => k.classList.remove("hl"), 3200); })), 380);
  }

  // The first time the deck opens with more than one page, it shows the way once: the
  // pages lean towards the next one and the grips lift. No words.
  function orient() {
    if (store.get("oriented") || reduced()) return;
    setTimeout(() => {
      if (S.pages.length > 1) $("#pages")?.classList.add("peek");
      $(".grip.bottom")?.classList.add("peek");
      $(".grip.top")?.classList.add("peek");
      store.set("oriented", "1");
    }, 900);
  }

  let resizeTimer = null;
  const relayout = () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      root.style.setProperty("--vvh", `${(window.visualViewport ? visualViewport.height : innerHeight)}px`);
      const vp = $("#vp");
      if (vp && `${vp.clientWidth}x${vp.clientHeight}` !== lastSize && (S.mode === "deck" || S.mode === "guest")) layoutPages();
      if (layer.firstElementChild && layer.firstElementChild.reflow) layer.firstElementChild.reflow();
      if (window.DeckFX3) window.DeckFX3.resize();
      if (window.DeckArt) window.DeckArt.apply(root.dataset.theme);
    }, 120);
  };
  addEventListener("resize", relayout);
  if (window.visualViewport) visualViewport.addEventListener("resize", relayout);
  root.style.setProperty("--vvh", `${innerHeight}px`);

  // ---------- pressing ----------

  // Effects by control: flares burst, missiles streak away, shields ripple, and so on.
  const FX_BY_ID = [
    [/decoy|panic/, "flares"], [/noise/, "noise"], [/missile/, "missile"], [/scan/, "scan"], [/fire-laser|fracture/, "laser"],
    [/^lights$/, "lights"], [/door/, "doors"], [/^shields-/, "shield"], [/^prev-/, "lock"], [/flight-ready|power-on/, "power"],
    [/self-d|eject/, "alarm"], [/quantum/, "quantum"],
  ];
  const fxOf = (el) => el.dataset.fx || (FX_BY_ID.find(([re]) => re.test(el.dataset.id || "")) || [])[1] || "press";
  const fx = (name, el, e) => { if (window.DeckFX3) window.DeckFX3.play(name, el, e); };

  const holds = new Map();
  function press(el, e, id = el.dataset.id, mode = el.dataset.mode) {
    buzz();
    if (el.dataset.locked) { send({ t: "request", id }); ticker("ASKED", el.getAttribute("aria-label")); return null; }
    send({ t: mode === "hold" ? "down" : "tap", id });
    fx(fxOf(el), el, e);
    if (el.classList.contains("pos")) { const d = el.closest(".dial"); if (d) d.style.setProperty("--pa", el.style.getPropertyValue("--a")); }
    return mode === "hold" ? id : null;
  }

  con.addEventListener("pointerdown", (e) => {
    if (S.editing) return;
    const k = e.target.closest(".k[data-id]");
    if (!k || k.disabled || k.closest(".fader .track")) return;
    if (k.closest(".guard") && !k.closest(".guard").classList.contains("open")) return;
    e.preventDefault();
    try { k.setPointerCapture(e.pointerId); } catch {}
    k.classList.add("pressed");
    const h = { el: k, start: performance.now(), held: press(k, e) };
    if (h.held) { k.classList.add("charging"); if (k.classList.contains("spool")) h.spool = setTimeout(() => fx("warp", k, e), 1100); }
    holds.set(e.pointerId, h);
  });
  const lift = (e) => {
    const h = holds.get(e.pointerId);
    if (!h) return;
    holds.delete(e.pointerId);
    const k = h.el;
    clearTimeout(h.spool);
    k.classList.remove("charging");
    if (h.held) send({ t: "up", id: h.held });
    const g = k.closest(".guard");
    if (g) setTimeout(() => g.classList.remove("open"), 600);
    const left = Math.max(0, 110 - (performance.now() - h.start));
    setTimeout(() => k.classList.remove("pressed"), left);
  };
  con.addEventListener("pointerup", lift);
  con.addEventListener("pointercancel", lift);
  addEventListener("contextmenu", (e) => { if (e.target.closest(".k, .lampb, .grip, .fader, .lever, .slide, .lock")) e.preventDefault(); });

  // Guarded covers lift with a tap or a flick up, and close again on their own.
  con.addEventListener("click", (e) => {
    const c = e.target.closest(".cover");
    if (!c || S.editing) return;
    const g = c.closest(".guard");
    g.classList.add("open");
    buzz(14);
    clearTimeout(g._t);
    g._t = setTimeout(() => g.classList.remove("open"), 5000);
  });
  // Local switches (landing / docking mode) change the panel, not the game.
  con.addEventListener("click", (e) => {
    const b = e.target.closest("[data-local]");
    if (!b) return;
    store.set(b.dataset.local, b.dataset.v);
    buzz(6);
    const pane = b.closest(".pane");
    const g = S.groups.find((x) => x.id === pane.dataset.panel);
    if (g) { pane.querySelector(".pbody").innerHTML = window.DeckPanels.render(g, panelOpts()); window.DeckPanels.fitGrids(pane); }
  });

  // Drag controls: faders, the gear lever, slide to eject.
  function drag(sel, h) {
    con.addEventListener("pointerdown", (e) => {
      const el = e.target.closest(sel);
      if (!el || S.editing || e.target.closest(".cap")) return;
      e.preventDefault();
      e.stopPropagation();
      try { el.setPointerCapture(e.pointerId); } catch {}
      const st = h.down(el, e) || {};
      const move = (ev) => { if (ev.pointerId === e.pointerId) h.move(el, ev, st); };
      const up = (ev) => {
        if (ev.pointerId !== e.pointerId) return;
        el.removeEventListener("pointermove", move); el.removeEventListener("pointerup", up); el.removeEventListener("pointercancel", up);
        h.up(el, ev, st);
      };
      el.addEventListener("pointermove", move); el.addEventListener("pointerup", up); el.addEventListener("pointercancel", up);
    }, true);
  }

  // A fader: each detent dragged past is one press; a throttle (hold controls) holds the
  // key while it's off center. A tap above or below center is one press.
  drag(".fader .track", {
    down(t, e) {
      const f = t.closest(".fader"), r = t.getBoundingClientRect(), horiz = f.classList.contains("horiz");
      const len = horiz ? r.width : r.height;
      if (f.dataset.locked) { send({ t: "request", id: f.dataset.up }); ticker("ASKED", t.getAttribute("aria-label")); return { dead: true }; }
      f.classList.add("active");
      return { f, horiz, len, half: len / 2 - 14, step: Math.max(16, len / 12), x0: e.clientX, y0: e.clientY, n: 0, moved: false,
        at: horiz ? (e.clientX - r.left) / r.width : (e.clientY - r.top) / r.height, throttle: f.classList.contains("throttle"), held: "" };
    },
    move(t, e, s) {
      if (s.dead) return;
      let d = s.horiz ? e.clientX - s.x0 : s.y0 - e.clientY;
      if (Math.abs(d) > 6) s.moved = true;
      d = Math.max(-s.half, Math.min(s.half, d));
      s.f.style.setProperty("--off", `${d}px`);
      s.f.style.setProperty("--p", (d / s.half).toFixed(3));
      if (s.throttle) {
        const want = d > s.step ? s.f.dataset.up : d < -s.step ? s.f.dataset.down : "";
        if (want !== s.held) {
          if (s.held) send({ t: "up", id: s.held });
          if (want) { send({ t: "down", id: want }); buzz(8); }
          s.held = want;
        }
        return;
      }
      const n = Math.trunc(d / s.step);
      while (s.n !== n) {
        const upward = n > s.n;
        if (upward ? s.n >= 0 : s.n <= 0) { const id = upward ? s.f.dataset.up : s.f.dataset.down; if (id) { send({ t: "tap", id }); buzz(5); } }
        s.n += upward ? 1 : -1;
        tick(s.f);
      }
    },
    up(t, e, s) {
      if (s.dead) return;
      if (s.held) send({ t: "up", id: s.held });
      if (!s.moved) {
        const upward = s.horiz ? s.at > 0.5 : s.at < 0.5;
        const id = upward ? s.f.dataset.up : s.f.dataset.down;
        if (id) { send({ t: "tap", id }); buzz(); tick(s.f); }
      }
      s.f.classList.remove("active");
      s.f.style.setProperty("--off", "0px");
      s.f.style.setProperty("--p", 0);
    },
  });
  function tick(f) { f.classList.remove("tick"); void f.offsetWidth; f.classList.add("tick"); }

  // The gear lever: tap to cycle, or push it down to lower and up to raise.
  drag(".lever", {
    down(el, e) { return { y0: e.clientY, x0: e.clientX }; },
    move(el, e, s) { el.style.setProperty("--pull", `${Math.max(-30, Math.min(30, e.clientY - s.y0))}px`); },
    up(el, e, s) {
      el.style.setProperty("--pull", "0px");
      const dy = e.clientY - s.y0, on = el.classList.contains("on");
      if (Math.abs(dy) < 12 || (dy > 0 && !on) || (dy < 0 && on)) {
        press(el, e);
        el.classList.remove("transit"); void el.offsetWidth; el.classList.add("transit");
      } else buzz(4);
    },
  });

  // Slide to eject: the handle has to travel the whole rail.
  drag(".slide .handle", {
    down(h, e) {
      const s = h.closest(".slide"), rail = h.parentElement.getBoundingClientRect();
      return { s, x0: e.clientX, max: rail.width - h.offsetWidth - 8 };
    },
    move(h, e, st) { const d = Math.max(0, Math.min(st.max, e.clientX - st.x0)); st.d = d; st.s.style.setProperty("--slid", `${d}px`); st.s.style.setProperty("--sp", (d / st.max).toFixed(3)); },
    up(h, e, st) {
      if ((st.d || 0) >= st.max * 0.94) { press(st.s, e); st.s.classList.add("fired"); setTimeout(() => st.s.classList.remove("fired"), 1200); }
      st.s.style.setProperty("--slid", "0px"); st.s.style.setProperty("--sp", 0);
    },
  });

  // Hold the TALK lamp to talk.
  con.addEventListener("pointerdown", (e) => {
    const b = e.target.closest('[data-lamp="talk"]');
    if (!b) return;
    e.preventDefault();
    e.stopPropagation();
    try { b.setPointerCapture(e.pointerId); } catch {}
    buzz();
    S.listening = true; b.classList.add("good", "blink");
    send({ t: "voice", on: true });
    const end = () => { S.listening = false; b.classList.remove("good", "blink"); send({ t: "voice", on: false }); b.removeEventListener("pointerup", end); b.removeEventListener("pointercancel", end); };
    b.addEventListener("pointerup", end);
    b.addEventListener("pointercancel", end);
  }, true);

  // ---------- the layout lock ----------

  // Panels stay where they are until the lock is slid up and then right: a move that
  // doesn't happen by accident. Unlocked, drag a panel's plate onto another to swap
  // them, and drag the seams to resize.
  drag("#lock", {
    down(el, e) { return { x0: e.clientX, y0: e.clientY, phase: 0 }; },
    move(el, e, s) {
      const dx = e.clientX - s.x0, dy = s.y0 - e.clientY;
      if (s.phase === 0) { const u = Math.max(0, Math.min(22, dy)); el.style.setProperty("--ly", `${-u}px`); if (u >= 20) { s.phase = 1; buzz(6); } }
      if (s.phase === 1) { const o = Math.max(0, Math.min(30, dx)); el.style.setProperty("--lx", `${o}px`); if (o >= 28) s.phase = 2; }
    },
    up(el, e, s) {
      el.style.setProperty("--lx", "0px"); el.style.setProperty("--ly", "0px");
      if (s.phase === 2) setEditing(true);
    },
  });
  function setEditing(on) {
    S.editing = on;
    if (on && !saved) saved = { pages: S.pages.map((p) => p.map((c) => c.map((g) => g.id))), cw: {}, wt: {} };
    if (on) buzz([10, 30, 10]);
    con.classList.toggle("editing", on);
    paintLamps();
  }
  con.addEventListener("click", (e) => {
    const b = e.target.closest("[data-edit]");
    if (!b) return;
    if (b.dataset.edit === "reset") { saved = null; saveLayout(); S.editing = false; drawConsole(S.mode); setEditing(true); return; }
    setEditing(false);
    saveLayout();
  });
  // Seams: drag to share space between neighbors.
  drag(".editing .seam", {
    down(el, e) {
      const v = el.classList.contains("v"), a = el.previousElementSibling, b = el.nextElementSibling;
      const ra = a.getBoundingClientRect(), rb = b.getBoundingClientRect();
      return { v, a, b, x0: e.clientX, y0: e.clientY, sa: v ? ra.width : ra.height, sb: v ? rb.width : rb.height, fa: parseFloat(a.style.flexGrow) || 1, fb: parseFloat(b.style.flexGrow) || 1 };
    },
    move(el, e, s) {
      const d = s.v ? e.clientX - s.x0 : e.clientY - s.y0, tot = s.sa + s.sb, sum = s.fa + s.fb;
      const na = Math.max(90, Math.min(tot - 90, s.sa + d));
      s.ga = (sum * na) / tot; s.gb = sum - s.ga;
      s.a.style.flexGrow = s.ga; s.b.style.flexGrow = s.gb;
    },
    up(el, e, s) {
      if (s.ga === undefined) return;
      if (s.v) { saved.cw = saved.cw || {}; saved.cw[`${el.dataset.p}.${el.dataset.c - 1}`] = s.ga; saved.cw[`${el.dataset.p}.${el.dataset.c}`] = s.gb; }
      else { saved.wt = saved.wt || {}; saved.wt[s.a.dataset.panel] = s.ga; saved.wt[s.b.dataset.panel] = s.gb; }
      saveLayout();
    },
  });
  // Panels: drag a plate onto another panel to swap them (on any page).
  drag(".editing .plate", {
    down(el, e) {
      const pane = el.closest(".pane");
      pane.classList.add("lifted");
      return { pane, x0: e.clientX, y0: e.clientY };
    },
    move(el, e, s) {
      s.pane.style.translate = `${e.clientX - s.x0}px ${e.clientY - s.y0}px`;
      const over = document.elementsFromPoint(e.clientX, e.clientY).find((n) => n.classList && n.classList.contains("pane") && n !== s.pane);
      $$(".pane.target").forEach((p) => p !== over && p.classList.remove("target"));
      if (over) over.classList.add("target");
      s.over = over;
    },
    up(el, e, s) {
      s.pane.classList.remove("lifted");
      s.pane.style.translate = "";
      $$(".pane.target").forEach((p) => p.classList.remove("target"));
      if (!s.over) return;
      const a = s.pane.dataset.panel, b = s.over.dataset.panel;
      saved.pages = saved.pages.map((p) => p.map((c) => c.map((id) => (id === a ? b : id === b ? a : id))));
      saveLayout();
      layoutPages();
      buzz(10);
    },
  });

  // ---------- gestures ----------

  // Swipes start anywhere that isn't a control: plates, the frame and the edges. Left
  // and right turn pages (the pages follow the finger), up raises systems, down lowers
  // comms. On a sheet, swipes page its lists or close it.
  const NOSWIPE = ".k:not(.pick), input, textarea, [data-lamp=talk], .fader, .lever, .slide, .cover, .lock, .seam";
  let swallowClick = false;
  function gesture(surface, { onMove, onEnd }) {
    let g = null;
    surface.addEventListener("pointerdown", (e) => {
      if (e.target.closest(NOSWIPE) || (S.editing && e.target.closest(".plate"))) return;
      g = { id: e.pointerId, x: e.clientX, y: e.clientY, t: performance.now(), el: e.target, axis: "" };
    });
    surface.addEventListener("pointermove", (e) => {
      if (!g || e.pointerId !== g.id) return;
      const dx = e.clientX - g.x, dy = e.clientY - g.y;
      if (!g.axis && Math.hypot(dx, dy) > 12) {
        g.axis = Math.abs(dx) > Math.abs(dy) ? "x" : "y";
        try { surface.setPointerCapture(e.pointerId); } catch {}
      }
      if (g.axis && onMove) onMove(g, dx, dy);
    });
    const end = (e) => {
      if (!g || e.pointerId !== g.id) return;
      const dx = e.clientX - g.x, dy = e.clientY - g.y, dt = performance.now() - g.t;
      const cur = g; g = null;
      if (!cur.axis) { onEnd(cur, 0, 0, dt, true); return; }
      swallowClick = true; setTimeout(() => { swallowClick = false; }, 0);
      onEnd(cur, dx, dy, dt, e.type === "pointercancel");
    };
    surface.addEventListener("pointerup", end);
    surface.addEventListener("pointercancel", end);
  }
  addEventListener("click", (e) => { if (swallowClick) { e.stopPropagation(); e.preventDefault(); swallowClick = false; } }, true);

  gesture(con, {
    onMove(g, dx) {
      if (g.axis !== "x" || S.pages.length < 2) return;
      const box = $("#pages"), W = $("#vp").clientWidth;
      const edge = (S.page === 0 && dx > 0) || (S.page === S.pages.length - 1 && dx < 0);
      box.classList.add("dragging");
      box.style.transform = `translateX(${-S.page * W + (edge ? dx / 3 : dx)}px)`;
    },
    onEnd(g, dx, dy, dt, tap) {
      const box = $("#pages");
      if (box) box.classList.remove("dragging");
      if (tap) return;
      const fast = dt < 260;
      if (g.axis === "x") {
        const W = $("#vp") ? $("#vp").clientWidth : 1;
        if (Math.abs(dx) > W * 0.18 || (fast && Math.abs(dx) > 40)) goPage(S.page + (dx < 0 ? 1 : -1)); else goPage(S.page);
      } else if (S.editing) return;
      else if (dy < -50 || (fast && dy < -30)) openSystems();
      else if (dy > 50 || (fast && dy > 30)) openComms();
    },
  });

  // Trackpads and mice: a horizontal wheel turns pages.
  let wheelLock = 0;
  con.addEventListener("wheel", (e) => {
    if (Math.abs(e.deltaX) < 30 && Math.abs(e.deltaY) < 30) return;
    if (performance.now() < wheelLock) return;
    wheelLock = performance.now() + 450;
    const d = Math.abs(e.deltaX) > Math.abs(e.deltaY) ? e.deltaX : e.deltaY;
    goPage(S.page + (d > 0 ? 1 : -1));
  }, { passive: true });

  addEventListener("keydown", (e) => {
    if (e.target.closest && e.target.closest("input, textarea")) { if (e.key === "Escape") closeSheet(); return; }
    if (e.key === "Escape") { if (S.editing) { setEditing(false); saveLayout(); } closeSheet(); }
    else if (sheetName()) return;
    else if (e.key === "ArrowRight") goPage(S.page + 1);
    else if (e.key === "ArrowLeft") goPage(S.page - 1);
    else if (e.key === "ArrowUp") openSystems();
    else if (e.key === "ArrowDown") openComms();
    else if (e.key === "/" && S.mode === "deck") { e.preventDefault(); openFind(); }
  });

  con.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t) return;
    if (t.dataset.go) goPage(S.page + Number(t.dataset.go));
    else if (t.dataset.page) goPage(Number(t.dataset.page));
    else if (t.dataset.sheet === "systems") openSystems();
    else if (t.dataset.sheet === "comms") openComms();
    else if (t.dataset.lamp === "afk" && S.mode === "deck") { S.afk = !S.afk; send({ t: "afk", on: S.afk }); paintLamps(); buzz(); }
    else if (t.dataset.lamp && t.dataset.lamp !== "talk") openComms();
  });
  document.addEventListener("click", (e) => {
    const t = e.target.closest("[data-req]");
    if (!t) return;
    send({ t: "answer", id: t.dataset.req, do: !!t.dataset.do });
    S.requests = S.requests.filter((r) => r.id !== t.dataset.req);
    paintIncoming(); paintLamps();
    if (sheetName() === "comms") openComms();
  });

  // ---------- sheets ----------

  const sheetName = () => (layer.firstElementChild ? layer.firstElementChild.dataset.name : "");
  function openSheet(name, from, title, body, reflow) {
    const same = sheetName() === name;
    layer.innerHTML = `<div class="sheet ${from === "top" ? "down" : ""}${same ? " still" : ""}" data-name="${name}" data-from="${from}" role="dialog" aria-modal="true" aria-label="${esc(title)}">
      ${from === "top" ? "" : `<button type="button" class="grip top" data-close aria-label="Close"><i></i></button>`}
      <header class="sheet-head"><h1>${esc(title)}</h1><button type="button" class="x" data-close aria-label="Close">${UI.close}</button></header>
      <div class="sheet-body">${body}</div>
      ${from === "top" ? `<button type="button" class="grip bottom" data-close aria-label="Close"><i></i></button>` : ""}</div>`;
    const sh = layer.firstElementChild;
    sh.reflow = reflow || null;
    if (reflow) reflow();
    paintIncoming();
    return sh;
  }
  function closeSheet() {
    const sh = layer.firstElementChild;
    if (!sh) return;
    layer.innerHTML = "";
    paintIncoming();
  }
  layer.addEventListener("click", (e) => { if (e.target.closest("[data-close]")) closeSheet(); });
  gesture(layer, {
    onEnd(g, dx, dy, dt, tap) {
      if (tap) return;
      const sh = layer.firstElementChild;
      if (!sh) return;
      if (g.axis === "x") {
        const pg = g.el.closest && (g.el.closest(".pgr") || g.el.closest(".steps"));
        if (pg && pg.turn) pg.turn(dx < 0 ? 1 : -1);
        return;
      }
      // A sheet goes back the way it came.
      if (sh.dataset.from === "bottom" && dy > 50) closeSheet();
      else if (sh.dataset.from === "top" && dy < -50) closeSheet();
    },
  });

  // A list that pages instead of scrolling: it measures how many cells fit in its box
  // and shows that many, with notches to turn.
  function pager(box, items, render, { cellW, cellH, gap = 10, rows: fixedRows, fixed = false }) {
    box.classList.add("pgr");
    let page = Number(box.dataset.pg || 0);
    const draw = () => {
      const W = box.clientWidth, H = box.clientHeight - 18;
      const cols = Math.max(1, Math.floor((W + gap) / (cellW + gap)));
      const rows = fixedRows || Math.max(1, Math.floor((H + gap) / (cellH + gap)));
      const per = cols * rows, n = Math.max(1, Math.ceil(items.length / per));
      page = Math.min(page, n - 1);
      box.dataset.pg = page;
      box.innerHTML = `<div class="pg-grid" style="grid-template-columns:repeat(${cols},${fixed ? `${cellW}px` : "minmax(0,1fr)"});${fixed ? "justify-content:center;" : ""}grid-auto-rows:${cellH}px;gap:${gap}px">${items.slice(page * per, (page + 1) * per).map(render).join("")}</div>
        ${n > 1 ? `<div class="pg-dots">${Array.from({ length: n }, (_, i) => `<button type="button" data-pg="${i}" aria-label="Page ${i + 1}" aria-selected="${i === page}"></button>`).join("")}</div>` : ""}`;
    };
    box.turn = (d) => { page = Math.max(0, page + d); box.dataset.pg = page; draw(); };
    box.onclick = (e) => { const b = e.target.closest("[data-pg]"); if (b) { page = Number(b.dataset.pg); box.dataset.pg = page; draw(); } };
    draw();
    return draw;
  }

  // Systems: the deck's map, views, and what the deck can do. Rises from below.
  function openSystems() {
    if (S.mode === "pair" || S.mode === "gone") return;
    const guest = S.mode === "guest";
    const views = viewList();
    const acts = guest ? [] : [
      ["find", "Find", UI.find], ["share", "Share", UI.share], ["theme", "Panel", UI.theme],
      ["release", "Release", UI.release], ["full", "Full", UI.full],
    ];
    openSheet("systems", "bottom", guest ? "Map" : "Systems", `
      ${guest ? "" : `<div class="views" id="views"></div>`}
      <div class="map" id="map"></div>
      ${acts.length ? `<div class="acts">${acts.map(([id, l, ic]) => `<button type="button" class="act" data-act="${id}">${ic}<span>${l}</span></button>`).join("")}</div>` : ""}`,
    () => {
      if (!guest) pager($("#views"), views, (v) => {
        const cats = [...new Set((v.groups || []).map(CAT))];
        return `<button type="button" class="tile view" data-view="${esc(v.id)}" aria-current="${!!(S.view && S.view.id === v.id)}">
          <span class="tn">${esc(v.name)}</span><span class="sw">${cats.map((c) => `<i style="background:var(--c-${c})"></i>`).join("")}</span></button>`;
      }, { cellW: 150, cellH: 74, rows: 1 });
      paintMap();
    });
  }
  function viewList() {
    const list = S.views.filter((v) => v.preset).concat(S.views.filter((v) => !v.preset));
    if (S.view && !list.some((v) => v.id === S.view.id)) list.push(S.view);
    return list.concat([{ id: "+new", name: "+ New", groups: [] }]);
  }
  // The map is the console in miniature: every page, every pane, in its color.
  function paintMap() {
    const box = $("#map");
    if (!box) return;
    const vp = $("#vp");
    const W = vp ? vp.clientWidth : 1, H = vp ? vp.clientHeight : 1;
    const n = Math.max(1, S.pages.length);
    const avail = box.clientWidth - 16 * (n - 1);
    const scale = Math.min(avail / n / W, (box.clientHeight - 8) / H, 0.32);
    const pg = $$("#pages > .page");
    box.innerHTML = S.pages.map((page, i) => `<button type="button" class="mini" data-mpage="${i}" aria-current="${i === S.page}" style="width:${W * scale}px;height:${H * scale}px">
      ${page.map((c, ci) => `<span class="mcol" style="flex:${pg[i] ? pg[i].querySelectorAll(".pcol")[ci]?.style.flexGrow || 1 : 1} 1 0">${c.map((g) => {
        const el = pg[i] && pg[i].querySelector(`.pane[data-panel="${CSS.escape(g.id)}"]`);
        return `<span class="mp" data-cat="${CAT(g.id)}" style="flex:${el ? el.style.flexGrow || 1 : 1} 1 0"><b>${esc(g.code || "")}</b></span>`;
      }).join("")}</span>`).join("")}</button>`).join("");
  }
  layer.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheetName() !== "systems") return;
    if (t.dataset.mpage) { closeSheet(); goPage(Number(t.dataset.mpage)); }
    else if (t.dataset.view === "+new") openFind(true);
    else if (t.dataset.view) { send({ t: "view", id: t.dataset.view }); closeSheet(); }
    else if (t.dataset.act === "find") openFind();
    else if (t.dataset.act === "share") openShare();
    else if (t.dataset.act === "theme") openPanel();
    else if (t.dataset.act === "release") { send({ t: "release-all" }); ticker("DECK", "all keys released"); closeSheet(); }
    else if (t.dataset.act === "full") { const d = root; if (document.fullscreenElement) document.exitFullscreen(); else (d.requestFullscreen || d.webkitRequestFullscreen)?.call(d); }
  });

  // Comms: crew, requests, and the state of the link. Drops from above.
  function openComms() {
    if (S.mode === "pair" || S.mode === "gone") return;
    if (S.mode === "guest") { openAsk(null, S.owner); return; }
    const g = S.game || {};
    const v = S.voice || {};
    openSheet("comms", "top", "Comms", `
      <div class="spread">
        ${S.requests.length ? `<section class="glass col"><span class="lbl">Requests</span><div class="fill" id="reqs"></div></section>` : ""}
        <section class="glass col"><span class="lbl">Crew</span>${S.crew.length ? `<div class="fill" id="crew"></div>` : `<button type="button" class="invite" data-cact="share">${UI.share}<span>Share controls</span></button>`}</section>
        <section class="col status">
          <div class="readouts">
            ${readout("LINK", S.online ? (S.latency ? `${S.latency} MS` : "UP") : "DOWN", S.online ? "good" : "bad")}
            ${readout("GAME", !g.running ? "NOT RUNNING" : g.focused ? "IN FRONT" : "BEHIND", !g.running ? "bad" : g.focused ? "good" : "caution")}
            ${readout("VOICE", v.state === "ready" ? "READY" : v.state === "starting" ? "STARTING" : "OFF", v.state === "ready" ? "good" : "")}
            ${S.agent ? readout("AGENT", String(S.agent).replace(/_/g, " ").toUpperCase(), "") : ""}
          </div>
          <button type="button" class="btn big${S.afk ? " on" : ""}" data-cact="afk" aria-pressed="${S.afk}">${UI.afk}<span>${S.afk ? "Away" : "AFK"}</span></button>
          <button type="button" class="btn big" data-cact="release">${UI.release}<span>Release all</span></button>
        </section>
      </div>`,
    () => {
      const reqs = S.requests.slice().reverse();
      if (reqs.length) pager($("#reqs"), reqs, (r) => `<div class="req"><span class="lbl">${esc(r.from)}</span><b>${esc(r.label || r.text || "")}</b>
        <span class="row"><button type="button" class="btn primary" data-req="${esc(r.id)}" data-do="1">${r.label ? "Do it" : "On it"}</button><button type="button" class="btn" data-req="${esc(r.id)}">${r.label ? "No" : "OK"}</button></span></div>`,
      { cellW: 220, cellH: 118 });
      if (S.crew.length) pager($("#crew"), S.crew, (c) => {
        const dot = !c.online ? "" : c.afk ? "caution" : "good";
        const role = c.afk ? "AFK" : c.guest ? (c.peer ? "LINKED" : "GUEST") : c.online ? "LINKED" : "OFFLINE";
        const chips = (c.buttons || []).slice(0, 4).map((b) => `<button type="button" class="chip" data-crew="${esc(c.id)}" data-press="${esc(b.id)}"${!c.online || (c.afkOnly && !c.afk) ? " disabled" : ""}>${esc(b.label)}</button>`).join("");
        return `<div class="mate"><span class="row"><i class="dot ${dot}"></i><b>${esc(c.name)}</b><span class="lbl">${role}</span>
          <button type="button" class="btn sm" data-ask="${esc(c.id)}" data-name="${esc(c.name)}">${UI.send}</button></span>${chips ? `<span class="chips">${chips}</span>` : ""}</div>`;
      }, { cellW: 240, cellH: 112 });
    });
  }
  const readout = (l, v, cls) => `<div class="ro ${cls}"><span class="lbl">${l}</span><b><i></i>${esc(v)}</b></div>`;
  layer.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheetName() !== "comms") return;
    if (t.dataset.press && t.dataset.crew) { buzz(); send({ t: "crew-press", peer: t.dataset.crew, id: t.dataset.press }); }
    else if (t.dataset.ask) openAsk(t.dataset.ask, t.dataset.name);
    else if (t.dataset.cact === "afk") { S.afk = !S.afk; send({ t: "afk", on: S.afk }); buzz(); openComms(); paintLamps(); }
    else if (t.dataset.cact === "share") openShare();
    else if (t.dataset.cact === "release") { send({ t: "release-all" }); ticker("DECK", "all keys released"); closeSheet(); }
  });

  // Ask: quick lines first, typing second.
  let askTo = null;
  function openAsk(to, name) {
    askTo = to;
    openSheet("ask", "top", name || "Crew", `
      <div class="quick">${QUICK.map((q) => `<button type="button" class="btn big" data-quick="${esc(q)}">${esc(q)}</button>`).join("")}</div>
      <form id="askf" class="row"><input class="field" id="askt" maxlength="140" placeholder="Message" autocomplete="off" aria-label="Message"><button class="btn primary big" aria-label="Send">${UI.send}</button></form>`);
    $("#askf").onsubmit = (e) => { e.preventDefault(); sendAsk($("#askt").value); };
  }
  function sendAsk(text) {
    text = String(text || "").trim();
    if (!text) return;
    if (askTo) send({ t: "crew-request", peer: askTo, text }); else send({ t: "request", text });
    buzz();
    ticker("SENT", text);
    closeSheet();
  }
  layer.addEventListener("click", (e) => { const t = e.target.closest("[data-quick]"); if (t) sendAsk(t.dataset.quick); });

  // Find: every control as a key. Tap presses it; in build mode a tap picks it for a
  // new view instead.
  let catalog = null, findQ = "", findCat = "", findGroup = "", draft = null;
  function openFind(build) {
    if (build) draft = draft || { name: "", groups: [], controls: [] };
    const sh = openSheet("find", "bottom", draft ? "New view" : "Find", `
      <div class="row search"><span class="sic">${UI.find}</span><input class="field" id="fq" placeholder="Gear, quantum, shields…" autocomplete="off" value="${esc(findQ)}" aria-label="Find a control">
        <span class="count mono" id="fcount"></span></div>
      <div class="cats" id="fcats"></div>
      <div class="fill" id="fres"></div>
      ${draft ? `<div class="row build"><input class="field" id="vname" placeholder="View name" maxlength="32" value="${esc(draft.name)}" aria-label="View name">
        <span class="count mono" id="dcount"></span><button type="button" class="btn" data-dcancel>${UI.close}</button><button type="button" class="btn primary big" data-dsave>Save</button></div>` : ""}`,
    () => paintFind());
    void sh;
    const input = $("#fq");
    input.oninput = () => { findQ = input.value; paintFind(); };
    if (!draft) setTimeout(() => input.focus({ preventScroll: true }), 300);
    if (!catalog) fetch(`/api/v1/catalog?${auth()}`).then((r) => r.json()).then((d) => { catalog = d; paintFind(); }).catch(() => {});
  }
  function paintFind() {
    if (!catalog || sheetName() !== "find") return;
    const groups = catalog.groups;
    // Build mode filters by system (and can take a whole one); find filters by color.
    $("#fcats").innerHTML = draft
      ? groups.map((g) => `<button type="button" class="cat${findGroup === g.id ? " sel" : ""}${draft.groups.includes(g.id) ? " whole" : ""}" data-cat="${CAT(g.id)}" data-fgroup="${g.id}"><i></i>${esc(g.code)}</button>`).join("")
      : [`<button type="button" class="cat${findCat ? "" : " sel"}" data-fcat=""><i style="background:var(--frame-text)"></i>ALL</button>`]
        .concat(CATS.map((c) => `<button type="button" class="cat${findCat === c ? " sel" : ""}" data-cat="${c}" data-fcat="${c}"><i></i>${CAT_CODE[c]}</button>`)).join("");
    const words = findQ.toLowerCase().split(/\s+/).filter(Boolean);
    let rows = catalog.controls.filter((c) => (!findCat || CAT(c.group) === findCat) && (!findGroup || c.group === findGroup) &&
      words.every((w) => `${c.id} ${c.label} ${c.group} ${(c.tags || []).join(" ")}`.toLowerCase().includes(w)));
    $("#fcount").textContent = `${rows.length}/${catalog.controls.length}`;
    if (draft && findGroup) rows = [{ id: "+group", group: findGroup, label: "Whole system" }].concat(rows);
    pager($("#fres"), rows, (c) => {
      const picked = draft && (c.id === "+group" ? draft.groups.includes(findGroup) : draft.controls.includes(c.id));
      const ic = c.id === "+group" ? "grid" : window.DeckCat.icon(c, c.group);
      return `<button type="button" class="k pick${picked ? " on" : ""}" data-cat="${CAT(c.group)}" data-fid="${esc(c.id)}" aria-label="${esc(c.label)}">
        <span class="ic">${icon(ic)}</span><span class="l">${esc(c.label)}</span>${showBinds() ? `<i class="kb">${esc(keyText(c.keys))}</i>` : ""}</button>`;
    }, { cellW: 108, cellH: 108, gap: 12, fixed: true });
    if (draft) $("#dcount").textContent = `${draft.groups.length + draft.controls.length}`;
  }
  layer.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheetName() !== "find") return;
    if (t.dataset.fcat !== undefined) { findCat = t.dataset.fcat; paintFind(); }
    else if (t.dataset.fgroup) { findGroup = findGroup === t.dataset.fgroup ? "" : t.dataset.fgroup; $("#fres").dataset.pg = 0; paintFind(); }
    else if (t.dataset.fid) {
      const id = t.dataset.fid;
      if (!draft) { buzz(); send({ t: "tap", id }); t.classList.add("pressed"); setTimeout(() => t.classList.remove("pressed"), 120); return; }
      const list = id === "+group" ? draft.groups : draft.controls, key = id === "+group" ? findGroup : id;
      const i = list.indexOf(key);
      if (i >= 0) list.splice(i, 1); else list.push(key);
      draft.name = $("#vname").value;
      paintFind();
    } else if (t.dataset.dcancel !== undefined) { draft = null; findGroup = ""; closeSheet(); }
    else if (t.dataset.dsave !== undefined) {
      const name = $("#vname").value.trim();
      if (!name) { $("#vname").focus(); $("#vname").classList.add("want"); return; }
      if (!draft.groups.length && !draft.controls.length) { toast("PICK SOMETHING"); return; }
      send({ t: "save-view", text: JSON.stringify({ name, groups: draft.groups, sections: draft.controls.length ? [{ name, controls: draft.controls }] : [] }) });
      draft = null; findGroup = "";
      setTimeout(() => { send({ t: "view", id: name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") }); closeSheet(); }, 250);
    }
  });

  // Share: who, what, how long; then a link and its QR code.
  let share = null, shareResult = null;
  function openShare() {
    share = { who: "", groups: [], minutes: 60, afk: false, taps: false };
    shareResult = null;
    openSheet("share", "bottom", "Share", `<div id="sharebody" class="fillcol"></div>`, () => paintShare());
  }
  function paintShare() {
    const box = $("#sharebody");
    if (!box) return;
    if (shareResult) {
      const m = shareResult, link = m.links.internet || m.links.lan;
      box.innerHTML = `<div class="spread">
        <section class="glass col qrcol"><canvas class="qr" id="sqr"></canvas><b class="who">${esc(m.label)}</b>
          <span class="ro ${m.links.internet ? "good" : "caution"}"><b><i></i>${m.links.internet ? "ANYWHERE" : "HOME NETWORK"}</b></span></section>
        <section class="col"><code class="link">${esc(link)}</code>
          <div class="row"><button type="button" class="btn primary big" data-scopy="${esc(link)}">Copy</button>${navigator.share ? `<button type="button" class="btn big" data-ssend="${esc(link)}">${UI.send}</button>` : ""}
          <button type="button" class="btn big" data-snew>${UI.plus}</button></div>
          <span class="lbl">Live</span><div class="fill" id="slinks"></div></section></div>`;
      drawQR($("#sqr"), link);
      paintLinks();
      return;
    }
    const groups = (S.hello && S.hello.groups) || [];
    const names = [...new Set(S.crew.map((c) => c.name))].slice(0, 4);
    box.innerHTML = `<div class="spread steps">
      <section class="glass col"><span class="lbl">1 · Who</span><input class="field" id="swho" placeholder="Name" maxlength="32" autocomplete="off" value="${esc(share.who)}" aria-label="Who it's for">
        ${names.length ? `<div class="row">${names.map((n) => `<button type="button" class="chip" data-sname="${esc(n)}">${esc(n)}</button>`).join("")}</div>` : ""}
        <span class="lbl">Live</span><div class="fill" id="slinks"></div></section>
      <section class="glass col"><span class="lbl">2 · What</span><div class="fill" id="sgroups"></div></section>
      <section class="glass col"><span class="lbl">3 · For</span>
        <div class="seg col">${[[15, "15 min"], [60, "1 hour"], [240, "4 hours"], [0, "Until off"]].map(([m, l]) => `<button type="button" class="btn" data-sm="${m}" aria-pressed="${share.minutes === m}">${l}</button>`).join("")}</div>
        <button type="button" class="btn" data-sopt="afk" aria-pressed="${share.afk}">${UI.afk}<span>While AFK</span></button>
        <button type="button" class="btn" data-sopt="taps" aria-pressed="${share.taps}"><span>Taps only</span></button>
        <button type="button" class="btn primary big" data-screate>${UI.share}<span>Create</span></button></section></div>`;
    $("#swho").oninput = (e) => { share.who = e.target.value; };
    steps($(".steps"));
    pager($("#sgroups"), [{ id: "*", code: "ALL", name: "Everything" }, ...groups], (g) =>
      `<button type="button" class="opt" data-cat="${g.id === "*" ? "" : CAT(g.id)}" data-sg="${g.id}" aria-pressed="${share.groups.includes(g.id)}"><i></i><span>${esc(g.name)}</span></button>`,
    { cellW: 120, cellH: 48, gap: 8 });
    paintLinks();
  }
  // On a phone the share columns become steps: one at a time, swiped through.
  function steps(sp) {
    if (!sp) return;
    let i = 0;
    const cols = $$(":scope > .col", sp);
    const show = () => cols.forEach((c, j) => c.classList.toggle("cur", j === i));
    sp.turn = (d) => { i = Math.max(0, Math.min(cols.length - 1, i + d)); show(); };
    show();
  }
  function paintLinks() {
    const box = $("#slinks");
    if (!box) return;
    if (!S.grants.length) { box.innerHTML = `<p class="quiet">—</p>`; return; }
    pager(box, S.grants, (g) => {
      const left = g.expires && !String(g.expires).startsWith("0001") ? `${Math.max(0, Math.round((new Date(g.expires) - Date.now()) / 60000))}M` : "∞";
      return `<div class="live"><b>${esc(g.label)}</b><span class="mono">${left} · ${g.connected}</span><button type="button" class="btn sm danger" data-revoke="${esc(g.id)}" aria-label="Revoke">${UI.close}</button></div>`;
    }, { cellW: 200, cellH: 46, gap: 6 });
  }
  function drawQR(canvas, text) {
    if (!window.DeckQR || !canvas) return;
    const rows = window.DeckQR.matrix(text), n = rows.length, scale = Math.max(3, Math.floor(168 / n));
    canvas.width = canvas.height = n * scale;
    const g = canvas.getContext("2d");
    g.fillStyle = "#fff"; g.fillRect(0, 0, canvas.width, canvas.height); g.fillStyle = "#000";
    rows.forEach((row, y) => row.forEach((d, x) => { if (d) g.fillRect(x * scale, y * scale, scale, scale); }));
  }
  layer.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheetName() !== "share") return;
    if (t.dataset.sg) {
      if (t.dataset.sg === "*") share.groups = share.groups.includes("*") ? [] : ["*"];
      else { share.groups = share.groups.filter((g) => g !== "*"); const i = share.groups.indexOf(t.dataset.sg); if (i >= 0) share.groups.splice(i, 1); else share.groups.push(t.dataset.sg); }
      $$("[data-sg]", layer).forEach((b) => b.setAttribute("aria-pressed", String(share.groups.includes(b.dataset.sg))));
    } else if (t.dataset.sname) { share.who = t.dataset.sname; $("#swho").value = share.who; }
    else if (t.dataset.sm !== undefined) { share.minutes = Number(t.dataset.sm); $$("[data-sm]", layer).forEach((b) => b.setAttribute("aria-pressed", String(b === t))); }
    else if (t.dataset.sopt) { share[t.dataset.sopt] = !share[t.dataset.sopt]; t.setAttribute("aria-pressed", String(share[t.dataset.sopt])); }
    else if (t.dataset.screate !== undefined) {
      const who = $("#swho").value.trim();
      if (!who) { $("#swho").focus(); $("#swho").classList.add("want"); return; }
      if (!share.groups.length) { $("#sgroups").classList.remove("want"); void $("#sgroups").offsetWidth; $("#sgroups").classList.add("want"); return; }
      const all = share.groups.includes("*");
      send({ t: "share", label: who, minutes: share.minutes, all, groups: all ? [] : share.groups, afkOnly: share.afk, tapsOnly: share.taps });
    } else if (t.dataset.scopy) { (navigator.clipboard ? navigator.clipboard.writeText(t.dataset.scopy) : Promise.reject()).then(() => toast("COPIED"), () => toast(t.dataset.scopy, 6000)); }
    else if (t.dataset.ssend) { navigator.share({ title: "Tablet Deck", url: t.dataset.ssend }).catch(() => {}); }
    else if (t.dataset.snew !== undefined) { shareResult = null; share = { who: "", groups: [], minutes: 60, afk: false, taps: false }; paintShare(); }
    else if (t.dataset.revoke) { fetch(`/api/v1/grants/${encodeURIComponent(t.dataset.revoke)}?${auth()}`, { method: "DELETE" }).catch(() => {}); }
  });

  // Panel: the manufacturer's console (each tile is a live piece of that panel), and how
  // keys read.
  function openPanel() {
    const cur = root.dataset.theme;
    openSheet("panel", "bottom", "Panel", `
      <div class="fill" id="themes"></div>
      <div class="row opts">
        <div class="seg">${[["full", "Label"], ["off", "Icon"]].map(([v, l]) => `<button type="button" class="btn" data-labels="${v}" aria-pressed="${labelMode() === v}">${l}</button>`).join("")}</div>
        <span class="grow"></span>
        <button type="button" class="btn" data-do="lights" aria-label="Reset lights">${UI.lights}</button>
        <a class="btn" href="/classic.html?${auth()}">Classic</a>
        <button type="button" class="btn danger" data-do="unpair" aria-label="Forget this PC">${UI.unlink}</button>
      </div>`,
    () => pager($("#themes"), THEMES, (id) => `<button type="button" class="tile theme" data-theme="${id}" data-pick="${id}" aria-current="${id === cur}">
        <span class="mini-pane"><span class="plate"><i class="lamp" style="background:var(--accent)"></i><span class="nm">${esc(makerName(id))}</span></span>
          <span class="screen" style="--ks:52px;--kc:3;--kg:8px;--sp:8px">
            <span class="k on" data-cat="flight"><span class="ic">${icon("gear")}</span></span><span class="k" data-cat="def"><span class="ic">${icon("shield")}</span></span><span class="k" data-cat="combat"><span class="ic">${icon("target")}</span></span>
          </span></span><span class="em">${emblem(id)}</span></button>`,
    { cellW: 230, cellH: 170, gap: 14 }));
  }
  layer.addEventListener("click", (e) => {
    const t = e.target.closest("button");
    if (!t || sheetName() !== "panel") return;
    if (t.dataset.pick) { applyTheme(t.dataset.pick); drawConsole(S.mode); openPanel(); }
    else if (t.dataset.labels) { store.set("labels", t.dataset.labels === "full" ? "" : t.dataset.labels); root.dataset.labels = labelMode(); openPanel(); }
    else if (t.dataset.binds !== undefined) { store.set("binds", showBinds() ? "" : "on"); openPanel(); layoutPages(); }
    else if (t.dataset.do === "lights") { send({ t: "reset-states" }); ticker("DECK", "lights reset"); }
    else if (t.dataset.do === "unpair") { store.set("token", ""); store.set("pairCode", ""); token = ""; stop(); closeSheet(); drawPair(); }
  });

  // ---------- pairing and closed links ----------

  function plateScreen(title, inner) {
    S.mode = title === "Pair" ? "pair" : "gone";
    closeSheet();
    con.innerHTML = `<div class="viewport solo-wrap"><section class="pane solo" data-cat="ui"><header class="plate"><i class="lamp"></i><span class="nm">${esc(title)}</span><span class="no">${esc(location.host)}</span></header>
      <div class="glass">${inner}</div></section></div>
      <footer class="frame"><div class="maker" id="maker"></div><div></div><div></div></footer>`;
    paintMaker();
  }
  function drawPair(error) {
    plateScreen("Pair", `<form id="pairf" class="pairf" novalidate>
        <input class="field code${error ? " want" : ""}" id="code" autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" maxlength="32" value="${esc(token)}" placeholder="Code on your PC" aria-label="Code on your PC">
        ${error ? `<p class="err" role="alert">${esc(error)}</p>` : ""}
        <button class="btn primary big">Connect</button></form>`);
    $("#pairf").onsubmit = (e) => {
      e.preventDefault();
      token = $("#code").value.trim().toLowerCase();
      if (token) start();
    };
  }
  function drawGone(text) { plateScreen("Link closed", `<p class="err">${esc(text)}</p>`); }

  function tickClock() {
    const el = $("#gclock");
    if (!el || !S.guest) return;
    const exp = S.guest.expires;
    if (!exp || String(exp).startsWith("0001")) { el.textContent = "∞"; return; }
    const s = Math.max(0, Math.round((new Date(exp) - Date.now()) / 1000));
    el.textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
  }
  setInterval(tickClock, 1000);

  // ---------- start ----------

  applyTheme(store.get("theme") || "rsi");
  root.dataset.labels = labelMode();
  function start() {
    // Check the code (or token) first, so a wrong one shows pairing with a reason
    // instead of reconnecting forever.
    drawConsole(GUEST ? "guest" : "deck");
    fetch(GUEST ? `/layout.json?${auth()}` : `/api/v1/views?${auth()}`, { cache: "no-store" }).then((r) => {
      if (r.ok) { if (!GUEST && token !== store.get("token")) { store.set("token", token); store.set("pairCode", ""); } connect(); return; }
      if (GUEST) { drawGone(r.status === 410 ? "This share link ran out." : "This share link isn't valid any more."); return; }
      drawPair(r.status === 429 ? "Too many wrong codes. Wait a minute." : token ? "That code didn't match the one on your PC." : "");
    }).catch(() => setTimeout(start, 2000));
  }
  if (GUEST || token) start(); else drawPair();
})();
