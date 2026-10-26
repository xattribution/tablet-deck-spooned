// Tablet Deck for Stream Deck. Talks to the Tablet Deck app on this PC through its
// local API (port and key from %APPDATA%\TabletDeck\config.json), draws each key with
// the control's icon in its category colour, and lights toggles from the app's live
// event stream. No dependencies: Node's own modules and a small WebSocket client for
// the Stream Deck connection.
"use strict";
const net = require("node:net");
const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

// ---------- Stream Deck connection ----------

const args = {};
for (let i = 2; i < process.argv.length; i += 2) args[process.argv[i].replace(/^-/, "")] = process.argv[i + 1];

// A minimal RFC 6455 client: text frames, masked as clients must, ping answered.
function wsConnect(port, onOpen, onText) {
  const key = crypto.randomBytes(16).toString("base64");
  const sock = net.connect(Number(port), "127.0.0.1", () => {
    sock.write(`GET / HTTP/1.1\r\nHost: 127.0.0.1:${port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: ${key}\r\nSec-WebSocket-Version: 13\r\n\r\n`);
  });
  let buf = Buffer.alloc(0), open = false;
  const frame = (op, payload) => {
    const mask = crypto.randomBytes(4);
    const len = payload.length;
    const head = len < 126 ? Buffer.from([0x80 | op, 0x80 | len])
      : len < 65536 ? Buffer.from([0x80 | op, 0x80 | 126, len >> 8, len & 255])
        : Buffer.concat([Buffer.from([0x80 | op, 0x80 | 127]), (() => { const b = Buffer.alloc(8); b.writeBigUInt64BE(BigInt(len)); return b; })()]);
    const body = Buffer.from(payload);
    for (let i = 0; i < body.length; i++) body[i] ^= mask[i % 4];
    sock.write(Buffer.concat([head, mask, body]));
  };
  sock.on("data", (chunk) => {
    buf = Buffer.concat([buf, chunk]);
    if (!open) {
      const end = buf.indexOf("\r\n\r\n");
      if (end < 0) return;
      buf = buf.subarray(end + 4);
      open = true;
      onOpen();
    }
    while (buf.length >= 2) {
      const op = buf[0] & 15;
      let len = buf[1] & 127, off = 2;
      if (len === 126) { if (buf.length < 4) return; len = buf.readUInt16BE(2); off = 4; }
      else if (len === 127) { if (buf.length < 10) return; len = Number(buf.readBigUInt64BE(2)); off = 10; }
      if (buf.length < off + len) return;
      const data = buf.subarray(off, off + len);
      buf = buf.subarray(off + len);
      if (op === 1) onText(data.toString("utf8"));
      else if (op === 9) frame(10, data);
      else if (op === 8) process.exit(0);
    }
  });
  sock.on("close", () => process.exit(0));
  return { send: (obj) => frame(1, JSON.stringify(obj)) };
}

const sd = wsConnect(args.port, () => sd.send({ event: args.registerEvent, uuid: args.pluginUUID }), (text) => {
  let m; try { m = JSON.parse(text); } catch { return; }
  onEvent(m).catch((e) => log(String(e && e.message || e)));
});
const log = (msg) => sd.send({ event: "logMessage", payload: { message: `[tabletdeck] ${msg}` } });

// ---------- Tablet Deck API ----------

function deckConfig() {
  const dir = process.env.TABLETDECK_DATA || path.join(process.env.APPDATA || "", "TabletDeck");
  try { return JSON.parse(fs.readFileSync(path.join(dir, "config.json"), "utf8")); } catch { return {}; }
}
async function api(route, body) {
  const c = deckConfig();
  const r = await fetch(`http://127.0.0.1:${c.port || 47615}/api/v1/${route}`, {
    method: body ? "POST" : "GET",
    headers: { "X-Deck-Key": c.apiKey || "", "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!r.ok) throw new Error(`${route}: ${r.status}`);
  return r.json();
}

// The deck's own icons and categories, loaded from the app so keys match the tablets.
let ICONS = null, CAT = null, catalog = null, lit = {}, afk = false;
async function loadIcons() {
  const c = deckConfig();
  const base = `http://127.0.0.1:${c.port || 47615}`;
  const sandbox = { window: {} };
  vm.createContext(sandbox);
  for (const f of ["icons.js", "icons2.js"]) vm.runInContext(await (await fetch(`${base}/${f}`)).text(), sandbox, { timeout: 1000 });
  ICONS = sandbox.window.DECK_ICONS;
  CAT = sandbox.window.DeckCat;
}
async function loadCatalog() {
  catalog = await api("catalog");
  if (!ICONS) await loadIcons();
}
const COLORS = { flight: "#4C9BE8", eng: "#E8A93A", combat: "#E5534B", def: "#3CC4D6", nav: "#A783FF", ind: "#7CC44E", crew: "#E479B9", ui: "#A7B2C0", emerg: "#FF4A3D" };

const xml = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
function keySVG({ icon, label, color, on, sub }) {
  const bg = on ? color : "#111720", fg = on ? "#0A0E13" : color, text = on ? "#0A0E13" : "#DCE3EC";
  const body = (ICONS && (ICONS[icon] || ICONS.default)) || "";
  const words = String(label).toUpperCase();
  const size = words.length > 14 ? 14 : words.length > 11 ? 16 : 20;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="144" height="144" viewBox="0 0 144 144">
    <rect width="144" height="144" fill="${bg}"/>
    <g transform="translate(40 14) scale(1.333)" fill="none" stroke="${fg}" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round" color="${fg}" style="color:${fg}">${body.replace(/currentColor/g, fg).replace(/var\(--text\)/g, fg)}</g>
    <text x="72" y="${sub ? 112 : 122}" text-anchor="middle" font-family="Barlow Condensed, Bahnschrift, Arial Narrow, sans-serif" font-weight="700" font-size="${size}" fill="${text}">${xml(words.slice(0, 16))}</text>
    ${sub ? `<text x="72" y="134" text-anchor="middle" font-family="Consolas, monospace" font-size="13" fill="${on ? "#0A0E13" : "#8592A3"}">${xml(sub)}</text>` : ""}
  </svg>`;
}
const dataURI = (svg) => "data:image/svg+xml;charset=utf8," + encodeURIComponent(svg);

// ---------- keys ----------

const keys = new Map(); // context -> { action, settings }

function controlOf(settings) {
  return catalog && catalog.controls.find((c) => c.id === settings.control);
}

function draw(ctx) {
  const k = keys.get(ctx);
  if (!k) return;
  let img;
  if (k.action.endsWith(".press")) {
    const c = controlOf(k.settings);
    if (!c) img = keySVG({ icon: "default", label: "Pick a control", color: COLORS.ui });
    else {
      const cat = CAT ? CAT.of(c.group) : "ui";
      const icon = CAT ? CAT.icon(c, c.group) : "default";
      const on = c.stateGroup ? !!lit[c.stateGroup] : false;
      img = keySVG({ icon, label: k.settings.label || c.label, color: COLORS[cat] || COLORS.ui, on, sub: c.mode === "hold" ? "HOLD" : "" });
    }
  } else if (k.action.endsWith(".view")) {
    img = keySVG({ icon: "grid", label: k.settings.view || "View", color: COLORS.ui, sub: k.settings.device ? k.settings.device.toUpperCase() : "ALL DECKS" });
  } else {
    img = keySVG({ icon: "walk", label: "AFK", color: "#E3A33B", on: afk, sub: afk ? "AWAY" : "" });
  }
  sd.send({ event: "setImage", context: ctx, payload: { image: dataURI(img), target: 0 } });
}
const drawAll = () => { for (const ctx of keys.keys()) draw(ctx); };

const held = new Map(); // context -> control id held down

async function onEvent(m) {
  const ctx = m.context;
  switch (m.event) {
    case "willAppear":
      keys.set(ctx, { action: m.action, settings: (m.payload && m.payload.settings) || {} });
      if (!catalog) await loadCatalog().catch((e) => log(`Tablet Deck isn't running? ${e.message}`));
      draw(ctx);
      break;
    case "willDisappear":
      keys.delete(ctx);
      break;
    case "didReceiveSettings":
      if (keys.has(ctx)) keys.get(ctx).settings = m.payload.settings || {};
      draw(ctx);
      break;
    case "keyDown": {
      const k = keys.get(ctx);
      if (!k) return;
      try {
        if (k.action.endsWith(".press")) {
          const c = controlOf(k.settings);
          if (!c) return sd.send({ event: "showAlert", context: ctx });
          const mode = c.mode === "hold" ? "down" : "tap";
          if (mode === "down") held.set(ctx, c.id);
          const r = await api("press", { id: c.id, mode, who: "Stream Deck" });
          if (!r.ok) sd.send({ event: "showAlert", context: ctx });
        } else if (k.action.endsWith(".view")) {
          await api("show-view", { view: k.settings.view || "flight", device: k.settings.device || "" });
          sd.send({ event: "showOk", context: ctx });
        } else {
          await api("afk", { on: !afk });
        }
      } catch (e) {
        sd.send({ event: "showAlert", context: ctx });
        catalog = null; // reload once the app is back
      }
      break;
    }
    case "keyUp": {
      const id = held.get(ctx);
      if (id) { held.delete(ctx); await api("press", { id, mode: "up", who: "Stream Deck" }).catch(() => {}); }
      break;
    }
    case "propertyInspectorDidAppear":
    case "sendToPlugin":
      if (!catalog) await loadCatalog().catch(() => {});
      sd.send({ event: "sendToPropertyInspector", context: ctx, payload: { catalog: catalog ? { controls: catalog.controls, groups: catalog.groups, views: catalog.views } : null } });
      break;
  }
}

// ---------- live lights from the app ----------

async function followEvents() {
  for (;;) {
    try {
      const c = deckConfig();
      const r = await fetch(`http://127.0.0.1:${c.port || 47615}/api/v1/events`, { headers: { "X-Deck-Key": c.apiKey || "" } });
      if (!r.ok || !r.body) throw new Error(String(r.status));
      if (!catalog) await loadCatalog();
      const st = await api("states").catch(() => ({}));
      lit = st || {};
      afk = !!(await api("status").catch(() => ({}))).afk;
      drawAll();
      const dec = new TextDecoder();
      let pending = "";
      for await (const chunk of r.body) {
        pending += dec.decode(chunk, { stream: true });
        let i;
        while ((i = pending.indexOf("\n\n")) >= 0) {
          const block = pending.slice(0, i);
          pending = pending.slice(i + 2);
          const line = block.split("\n").find((l) => l.startsWith("data: "));
          if (!line) continue;
          let ev; try { ev = JSON.parse(line.slice(6)); } catch { continue; }
          const d = ev.data || {};
          if (ev.t === "state") { lit[d.group] = d.on; drawAll(); }
          else if (ev.t === "states") { lit = d.states || {}; drawAll(); }
          else if (ev.t === "afk") { afk = !!d.on; drawAll(); }
          else if (ev.t === "keybinds") { catalog = null; await loadCatalog().catch(() => {}); drawAll(); }
        }
      }
    } catch { /* the app is closed: try again shortly */ }
    await new Promise((res) => setTimeout(res, 3000));
  }
}
followEvents();
