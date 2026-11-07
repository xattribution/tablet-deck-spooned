// SpoolUp: serve a touch button page to a tablet on the LAN and turn button
// presses into keyboard scancodes on this PC. No dependencies beyond Node itself.
//
//   node server.js            normal mode
//   node server.js --dry-run  log key events instead of sending them, and stream them
//                             to the sandbox page (public/showcase.html)
//
// The installer ships this as TabletDeck.exe, a Node single executable (scripts/build.js).
"use strict";
const http = require("http");
const fs = require("fs");
const path = require("path");
const os = require("os");
const crypto = require("crypto");
const { spawn, spawnSync, execFile, execFileSync } = require("child_process");
const qr = require("./qr");
const gameLog = require("./gamelog");
const discovery = require("./discovery");

// node:sea is missing before Node 20; that's never the installed app.
const sea = (() => { try { return require("node:sea"); } catch { return null; } })();
const INSTALLED = Boolean(sea && sea.isSea());
// Running from source, everything lives in the repo folder. The installed app keeps
// public/, layouts/ and keybinds/ inside TabletDeckServer.exe, the native helpers next to
// it in Program Files (read-only), and config.json plus the player's own files in %APPDATA%.
const APP_DIR = INSTALLED ? path.dirname(process.execPath) : __dirname;
const DATA_DIR = INSTALLED ? path.join(process.env.APPDATA || os.homedir(), "TabletDeck") : __dirname;
const DRY_RUN = process.argv.includes("--dry-run");
// --ui: run under the app window (native/launcher.cs), which shows the console output as
// its log and reads "@ui {json}" status lines. It sends "quit" on stdin to stop the server.
const UI = process.argv.includes("--ui");
// --port <n> overrides config.json, e.g. to run a dry-run copy next to the real one.
const portArg = process.argv.indexOf("--port");
const PORT_OVERRIDE = portArg > -1 ? Number(process.argv[portArg + 1]) : null;
const CONFIG_PATH = path.join(DATA_DIR, "config.json");

// The installed app has its own console window, which closes as soon as the process
// ends. Like start.bat's "pause", keep it open after a fatal error so it can be read.
function fatal(message) {
  console.error(message);
  ui({ t: "fatal", text: message });
  if (INSTALLED && process.stdin.isTTY) spawnSync("cmd.exe", ["/c", "pause"], { stdio: "inherit" });
  process.exit(1);
}

// A status line for the app window; nothing without --ui.
function ui(event) {
  if (UI) console.log(`@ui ${JSON.stringify(event)}`);
}
if (INSTALLED) {
  process.title = "SpoolUp";
  process.on("uncaughtException", (err) => fatal(err.stack || String(err)));
}

// ---------- files ----------

// A bundled file (e.g. "public/index.html") as a Buffer, or null if there's no such file.
function readBundled(rel) {
  if (INSTALLED) {
    try { return Buffer.from(sea.getAsset(rel)); } catch { return null; }
  }
  const file = path.join(__dirname, rel);
  return fs.existsSync(file) && fs.statSync(file).isFile() ? fs.readFileSync(file) : null;
}

// A file named in config.json (layouts, actionmaps) as a Buffer, or null if it's missing.
// Absolute paths are used as they are. Relative ones come from the data folder first, so
// a player's own layouts/my.json works, then from the bundled files.
function readConfigFile(file) {
  const own = path.resolve(DATA_DIR, file);
  if (fs.existsSync(own) && fs.statSync(own).isFile()) return fs.readFileSync(own);
  return path.isAbsolute(file) ? null : readBundled(file.replace(/\\/g, "/"));
}

const VERSION = (() => { try { return JSON.parse(readBundled("package.json")).version; } catch { return ""; } })();

// ---------- config ----------

// ---------- Star Citizen installs ----------

// The game's release channels, each its own folder under StarCitizen\ with its own binds.
const SC_CHANNELS = ["LIVE", "PTU", "EPTU", "TECH-PREVIEW", "HOTFIX"];
// Full path of the running StarCitizen.exe, "" while the game is closed (refreshGameExe).
let gameExe = "";

const isDir = (p) => { try { return fs.statSync(p).isDirectory(); } catch { return false; } };
const isFile = (p) => { try { return fs.statSync(p).isFile(); } catch { return false; } };

// ...\StarCitizen\PTU\Bin64\StarCitizen.exe -> ...\StarCitizen\PTU
function channelDirFromExe(exe) {
  if (!exe) return "";
  const bin = path.dirname(exe);
  return path.basename(bin).toLowerCase() === "bin64" ? path.dirname(bin) : "";
}

// The usual StarCitizen folders on drives C to Z that exist. Looked up once: a mapped
// network drive that's offline can stall each look, and a new install while SpoolUp runs
// is found anyway through the running game or a picked folder.
let driveRoots = null;
function scDriveRoots() {
  if (driveRoots) return driveRoots;
  driveRoots = [];
  if (process.platform !== "win32") return driveRoots;
  for (let d = 67; d <= 90; d++) {
    const drive = `${String.fromCharCode(d)}:\\`;
    if (!isDir(drive)) continue;
    for (const sub of ["Program Files\\Roberts Space Industries\\StarCitizen", "Roberts Space Industries\\StarCitizen",
      "RSI\\StarCitizen", "Games\\Roberts Space Industries\\StarCitizen", "Games\\StarCitizen", "StarCitizen"]) {
      if (isDir(drive + sub)) driveRoots.push(drive + sub);
    }
  }
  return driveRoots;
}

// Every channel folder found. Automatically (config.gameFolders empty): the running game's
// first (it can be anywhere), then the usual folders on every drive. With a folder the
// player picked in the app window's Settings (a StarCitizen folder, the folder holding it,
// or one channel's folder), only the channels in it, the running one marked. Running
// first, then LIVE, then the order of SC_CHANNELS.
function findInstalls(picked = config.gameFolders) {
  const seen = new Map();
  const add = (dir, running) => {
    const key = path.resolve(dir).toLowerCase();
    if (seen.has(key)) { if (running) seen.get(key).running = true; return; }
    if (!isDir(dir)) return;
    const actionmaps = path.join(dir, "user", "client", "0", "Profiles", "default", "actionmaps.xml");
    seen.set(key, { channel: path.basename(path.resolve(dir)).toUpperCase(), dir: path.resolve(dir), actionmaps: isFile(actionmaps) ? actionmaps : "", running });
  };
  const addRoot = (root) => { for (const ch of SC_CHANNELS) add(path.join(root, ch), false); };
  const folders = [].concat(picked || []).filter((e) => typeof e === "string" && e.trim()).map((e) => e.trim());
  const running = channelDirFromExe(gameExe);
  if (folders.length) {
    for (const e of folders) {
      if (SC_CHANNELS.includes(path.basename(e).toUpperCase())) add(e, false);
      else { addRoot(e); addRoot(path.join(e, "StarCitizen")); }
    }
    const inPicked = running && seen.get(path.resolve(running).toLowerCase());
    if (inPicked) inPicked.running = true;
  } else {
    if (running) { add(running, true); addRoot(path.dirname(running)); }
    for (const root of scDriveRoots()) addRoot(root);
  }
  const rank = (i) => (i.running ? -1 : SC_CHANNELS.includes(i.channel) ? SC_CHANNELS.indexOf(i.channel) : SC_CHANNELS.length);
  return [...seen.values()].sort((a, b) => rank(a) - rank(b));
}

// The copy of the game the app window and Deck settings show: the running one, else the
// one whose keybinds are read, else the first found. null when none is found.
function gameInUse() {
  const all = findInstalls();
  const pick = all.find((i) => i.running) || all.find((i) => i.actionmaps) || all[0];
  const folder = [].concat(config.gameFolders || []).find((e) => typeof e === "string" && e.trim()) || "";
  return pick ? { channel: pick.channel, dir: pick.dir, running: pick.running, picked: folder !== "" } : null;
}

// The player's actionmaps.xml: the running game's channel when it has one, else LIVE's,
// else the first other channel that has one. null when there's none.
function findActionmaps() {
  return findInstalls().find((i) => i.actionmaps)?.actionmaps || null;
}

// "auto" in config.actionmaps stands for findActionmaps(), looked up each time the
// keybinds are read, so the deck follows the channel the player is running.
const AUTO_ACTIONMAPS = "auto";
const expandActionmaps = (files) => [].concat(files || []).flatMap((f) => (f === AUTO_ACTIONMAPS ? [findActionmaps()].filter(Boolean) : [f]));

function loadConfig() {
  let config = {};
  if (fs.existsSync(CONFIG_PATH)) config = JSON.parse(fs.readFileSync(CONFIG_PATH, "utf8"));
  let changed = false;
  if (!config.port) { config.port = 8765; changed = true; }
  if (!config.token) { config.token = crypto.randomBytes(4).toString("hex"); changed = true; }
  if (!config.layout) { config.layout = "layouts/hud.json"; changed = true; }
  if (!config.phoneLayout) { config.phoneLayout = "layouts/hud-phone.json"; changed = true; }
  // Later files win; missing files are skipped. The player's actionmaps.xml goes last:
  // loading a profile in-game copies it there, and later rebinds land there too. Before
  // 2.2 the server wrote the LIVE file it found on first start (or none) here; that list,
  // left as written, becomes "auto" so other channels and drives are found as well.
  const PROFILE = "keybinds/TabletDeck-StarCitizen.xml";
  const oldAuto = Array.isArray(config.actionmaps) && config.actionmaps[0] === PROFILE && (config.actionmaps.length === 1
    || (config.actionmaps.length === 2 && /StarCitizen[\\/]LIVE[\\/]user[\\/]client[\\/]0[\\/]Profiles[\\/]default[\\/]actionmaps\.xml$/i.test(String(config.actionmaps[1]))));
  if (config.actionmaps === undefined || oldAuto) { config.actionmaps = [PROFILE, AUTO_ACTIONMAPS]; changed = true; }
  // The Star Citizen folder the player picked; empty finds it automatically (findInstalls).
  if (config.gameFolders === undefined) { config.gameFolders = []; changed = true; }
  if (!config.tapMs) { config.tapMs = 60; changed = true; }
  // Star Citizen counts a press held past 0.25 s as a hold, so a tap stays well short of it.
  if (!(config.tapMs >= 20 && config.tapMs <= 200)) { config.tapMs = Math.min(200, Math.max(20, Math.round(+config.tapMs) || 60)); changed = true; }
  if (config.onlyWhenGameFocused === undefined) { config.onlyWhenGameFocused = true; changed = true; }
  if (config.voice === undefined) { config.voice = true; changed = true; }
  if (!config.voiceLanguage) { config.voiceLanguage = "en-US"; changed = true; }
  if (!config.voiceConfidence) { config.voiceConfidence = 0.6; changed = true; }
  // The installed app window checks GitHub for new versions unless this is false.
  if (INSTALLED && config.autoUpdate === undefined) { config.autoUpdate = true; changed = true; }
  // Websites a tablet may open on this PC ("Open on PC" in its sliders sheet).
  if (config.links === undefined) {
    config.links = [
      { name: "UEX", url: "https://uexcorp.space/" },
      { name: "Erkul", url: "https://www.erkul.games/" },
      { name: "SPViewer", url: "https://www.spviewer.eu/" },
      { name: "SCMDB", url: "https://scmdb.net/" },
    ];
    changed = true;
  }
  if (changed) {
    fs.mkdirSync(DATA_DIR, { recursive: true });
    fs.writeFileSync(CONFIG_PATH, JSON.stringify(config, null, 2) + "\n");
  }
  // Allow a numeric code such as "token": 1234 without quotes; the page sends the code as lowercase text.
  config.token = String(config.token).trim().toLowerCase();
  return config;
}
const config = loadConfig();
if (PORT_OVERRIDE) config.port = PORT_OVERRIDE;

// Settings the app window may change, each turning its text into a value, or undefined
// when it isn't valid. See applySetting() and --set below.
const onOff = (v) => (v === "on" ? true : v === "off" ? false : undefined);
const wholeNumber = (min, max) => (v) => (/^\d+$/.test(v) && +v >= min && +v <= max ? +v : undefined);
const SETTINGS = {
  voiceConfidence: (v) => (/^[\d.]+$/.test(v) && +v >= 0 && +v <= 1 ? Math.round(+v * 100) / 100 : undefined),
  autoUpdate: onOff,
  onlyWhenGameFocused: onOff,
  voice: onOff,
  shortEmotes: onOff,
  // The SpoolUp apps find this PC on the local network (discovery.js); on unless false.
  discovery: onOff,
  token: (v) => (/^[a-z0-9]{1,32}$/i.test(v) ? v.toLowerCase() : undefined),
  port: wholeNumber(1024, 65535),
  tapMs: wholeNumber(20, 200),
  voiceLanguage: (v) => (/^[a-z]{2}(-[A-Za-z]{2,4})?$/.test(v) ? v : undefined),
  // Push-to-talk from a PC key: its Windows virtual-key code (0 = none) and a name to show.
  pttKey: wholeNumber(0, 255),
  pttKeyName: (v) => (/^[\w +-]{1,30}$/.test(v) ? v : undefined),
  pttMode: (v) => (v === "hold" || v === "toggle" ? v : undefined),
  pttSound: onOff,
  // The address tablets should use (the QR code), when the automatic choice is wrong;
  // empty for automatic.
  host: (v) => (v === "" || /^(\d{1,3}\.){3}\d{1,3}$/.test(v) || /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/i.test(v) ? v : undefined),
  // JSON list of { name, url }: at most 12, https only.
  // The Star Citizen folder: a full path, or "auto" to find it automatically.
  gameFolders: (v) => (v === "auto" ? [] : v.length <= 400 && !/["<>|?*\x00-\x1f]/.test(v)
    && (path.win32.isAbsolute(v) || path.isAbsolute(v)) ? [v] : undefined),
  links: (v) => {
    let list;
    try { list = JSON.parse(v); } catch { return undefined; }
    if (!Array.isArray(list) || list.length > 12) return undefined;
    const ok = list.every((l) => l && typeof l.name === "string" && /^[^<>"]{1,24}$/.test(l.name.trim())
      && typeof l.url === "string" && /^https:\/\/[^\s"<>]{3,300}$/.test(l.url));
    return ok ? list.map((l) => ({ name: l.name.trim(), url: l.url })) : undefined;
  },
};

// --set name=value ...: save settings to config.json and exit without starting. The app
// window uses this while the server is stopped, e.g. to change a port that's taken.
const setArg = process.argv.indexOf("--set");
if (setArg > -1) {
  let ok = true;
  for (const pair of process.argv.slice(setArg + 1)) {
    const [name, ...rest] = pair.split("=");
    const value = SETTINGS[name] ? SETTINGS[name](rest.join("=")) : undefined;
    if (value === undefined) { console.error(`"${pair}" isn't a valid setting.`); ok = false; continue; }
    saveSetting(name, value);
  }
  process.exit(ok ? 0 : 1);
}

// --find-game: print the Star Citizen folder as JSON and exit: { folder (the picked one, or
// "" for automatic), game (gameInUse), installs (findInstalls) }. The app window's Settings
// uses it while the server is stopped, and "Install keybind profile..." for its list.
if (process.argv.includes("--find-game")) {
  console.log(JSON.stringify({ folder: [].concat(config.gameFolders || [])[0] || "", game: gameInUse(), installs: findInstalls() }));
  process.exit(0);
}

// ---------- merged keybind profile (--merge-profile) ----------

// --merge-profile <out.xml> [actionmaps.xml]: write "SpoolUp Keyboard (your binds)" and exit
// without starting. The app window's "Install keybind profile..." runs this for each copy of
// the game that has an actionmaps.xml; a copy without one gets the plain profile. Without a
// path, the actionmaps.xml findActionmaps() finds is used.
//
// Loading a profile in-game replaces every bind with the profile's, so the merged profile
// carries the player's whole profile over (keyboard, mouse, joystick and gamepad binds, device
// options, binds they cleared on purpose) and only adds keyboard binds: one for each action in
// SpoolUp's profile that the player has no keyboard bind for in that map. A new bind keeps
// SpoolUp's key when nothing active at the same time uses it (the player's binds and the
// game's defaults, by data/sc-contexts.json), else it takes the first free right-hand combo.
const MERGED_PROFILE_NAME = "SpoolUp Keyboard (your binds)";

// Free combos for the merged profile, in the order they're handed out: right-hand modifiers,
// which the game leaves almost unused, on ordinary keys. F4 is left out (Alt+F4 closes windows).
function mergeComboPool() {
  const mods = ["rctrl", "ralt", "rctrl+rshift", "ralt+rshift", "ralt+rctrl", "ralt+rctrl+rshift"];
  const keys = [..."abcdefghijklmnopqrstuvwxyz0123456789"];
  for (let i = 0; i <= 9; i++) keys.push(`np_${i}`);
  for (let i = 1; i <= 12; i++) if (i !== 4) keys.push(`f${i}`);
  keys.push("comma", "period", "slash", "semicolon", "apostrophe", "lbracket", "rbracket", "backslash", "minus", "equals",
    "insert", "home", "pgup", "delete", "end", "pgdn", "np_add", "np_subtract", "np_multiply", "np_divide", "np_period",
    "up", "down", "left", "right");
  return mods.flatMap((m) => keys.map((k) => `${m}+${k}`));
}

// Modifier order doesn't matter to the game: lalt+n == n+lalt.
const mergeCombo = (keys) => keys.trim().toLowerCase().split("+").sort().join("+");
// A key combination, not a mouse button, wheel or axis.
const mergeIsKeyboard = (keys) => Boolean(keys && keys.trim()) && !/(^|\+)(mouse|mwheel|maxis|hmd)/i.test(keys);

function mergeAttrs(tag) {
  const attrs = {};
  for (const m of tag.matchAll(/([\w:-]+)\s*=\s*"([^"]*)"/g)) attrs[m[1]] = m[2];
  return attrs;
}

// Every bind in an actionmaps.xml body or a profile: [{ map, action, input }], any device.
function mergeReadBinds(xml) {
  const out = [];
  let map = "";
  let action = "";
  for (const m of xml.matchAll(/<(\/?)(actionmap|action|rebind|addbind)\b([^>]*?)\/?>/g)) {
    const [whole, close, tag, rest] = m;
    const empty = whole.endsWith("/>");
    if (tag === "actionmap") map = close || empty ? "" : mergeAttrs(rest).name || "";
    else if (tag === "action") action = close || empty ? "" : mergeAttrs(rest).name || "";
    else if (!close && map && action && mergeAttrs(rest).input !== undefined) out.push({ map, action, input: mergeAttrs(rest).input });
  }
  return out;
}

// What goes inside the merged profile's ActionMaps element: the player's default profile from
// actionmaps.xml (or an exported profile's content), and the devices its options name. Null
// when the file isn't a Star Citizen profile.
function mergeProfileBody(current) {
  const src = current.replace(/^﻿/, "").replace(/^\s*<\?xml[^>]*\?>/, "");
  let body = null;
  const profiles = [...src.matchAll(/<ActionProfiles\b([^>]*?)(?:\/>|>([\s\S]*?)<\/ActionProfiles>)/g)];
  if (profiles.length) {
    const pick = profiles.find((p) => mergeAttrs(p[1]).profileName === "default") || profiles[0];
    body = pick[2] || "";
  } else {
    const open = /<ActionMaps\b[^>]*>/.exec(src);
    if (open) {
      body = src.slice(open.index + open[0].length);
      const end = body.lastIndexOf("</ActionMaps>");
      if (end >= 0) body = body.slice(0, end);
      body = body.replace(/<CustomisationUIHeader\b[\s\S]*?<\/CustomisationUIHeader>|<CustomisationUIHeader\b[^>]*\/>/g, "");
    }
  }
  if (body === null) return null;
  const devices = [["keyboard", "1"]];
  for (const m of body.matchAll(/<options\b([^>]*?)\/?>/g)) {
    const { type, instance, Product } = mergeAttrs(m[1]);
    if (!type || !instance || Product === undefined || !/^\w+$/.test(type) || !/^\d+$/.test(instance)) continue;
    if (!devices.some(([t, i]) => t === type && i === instance)) devices.push([type, instance]);
  }
  if (!/<modifiers\b/.test(body)) body = " <modifiers />\n" + body;
  return { body: body.replace(/\s+$/, ""), devices };
}

// Adds keyboard binds ([{ action, keys }]) to an action map in body: beside an action's other
// binds (a joystick button, say), as a new action, or in a new map.
function mergeAddToMap(body, map, rows) {
  const quote = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const rebind = (keys) => `\n    <rebind input="kb1_${keys}"/>`;
  // Other attributes after the name are allowed, so the map is never written twice.
  const mapRe = new RegExp(`<actionmap\\s+name="${quote(map)}"[^>]*?(\\/>|>([\\s\\S]*?)<\\/actionmap>)`);
  const found = mapRe.exec(body);
  if (!found) {
    const actions = rows.map((r) => `   <action name="${r.action}">${rebind(r.keys)}\n   </action>\n`).join("");
    return `${body}\n  <actionmap name="${map}">\n${actions}  </actionmap>`;
  }
  let inner = found[1] === "/>" ? "" : found[2] || "";
  // The map's and an action's own opening tags are kept as the player's file has them.
  const opening = (whole, close) => whole.slice(0, whole.length - close.length).replace(/\s+$/, "") + ">";
  for (const r of rows) {
    const empty = new RegExp(`<action\\s+name="${quote(r.action)}"[^>]*?\\/>`).exec(inner);
    const open = new RegExp(`<action\\s+name="${quote(r.action)}"[^>]*?(?<!\\/)>`).exec(inner);
    if (empty) inner = inner.slice(0, empty.index) + `${opening(empty[0], "/>")}${rebind(r.keys)}\n   </action>` + inner.slice(empty.index + empty[0].length);
    else if (open) inner = inner.slice(0, open.index + open[0].length) + rebind(r.keys) + inner.slice(open.index + open[0].length);
    else inner = inner.replace(/\s+$/, "") + `\n   <action name="${r.action}">${rebind(r.keys)}\n   </action>\n  `;
  }
  return body.slice(0, found.index) + opening(found[0], found[1]) + inner + "</actionmap>" + body.slice(found.index + found[0].length);
}

// Builds the merged profile. current: the player's actionmaps.xml; spoolup: SpoolUp's plain
// profile; game: data/sc-actions-4.10.json's actions; contexts: data/sc-contexts.json's map.
// Returns { xml, added: [{ action, keys, maps, moved }], kept } or null for a file that isn't
// a profile. `moved` says the bind isn't SpoolUp's usual key.
function mergeKeybindProfile(current, spoolup, game, contexts) {
  const profile = mergeProfileBody(current);
  if (!profile) return null;
  const contextOf = (map) => contexts[map] || "global";
  const mine = mergeReadBinds(profile.body);
  // The player's keyboard binds by map and action ("" when they cleared it on purpose).
  const theirs = new Map();
  for (const b of mine) {
    if (!/^kb\d*_/.test(b.input)) continue;
    theirs.set(`${b.map}/${b.action}`, b.input.slice(b.input.indexOf("_") + 1).trim());
  }
  // SpoolUp's binds, by action: its key and the maps it goes in.
  const needs = new Map();
  for (const b of mergeReadBinds(spoolup)) {
    const keys = b.input.startsWith("kb1_") ? b.input.slice(4).trim() : "";
    if (!keys) continue;
    const need = needs.get(b.action) || { action: b.action, keys, maps: [] };
    if (!need.maps.includes(b.map)) need.maps.push(b.map);
    needs.set(b.action, need);
  }
  const todo = [];
  for (const need of [...needs.values()].sort((a, b) => a.action.localeCompare(b.action))) {
    const maps = need.maps.filter((m) => !theirs.has(`${m}/${need.action}`));
    if (!maps.length) continue; // the player's own bind (or none, on purpose) stays
    const own = need.maps.map((m) => theirs.get(`${m}/${need.action}`)).find((k) => mergeIsKeyboard(k));
    todo.push({ ...need, maps, own });
  }
  const adding = new Set(todo.flatMap((t) => t.maps.map((m) => `${m}/${t.action}`)));

  // Combos in use, per context: the player's keyboard binds and the game's defaults they kept.
  // Each context maps a combo to the actions using it, so an action may share its own key.
  const used = new Map();
  const use = (map, action, keys) => {
    const c = contextOf(map);
    if (!used.has(c)) used.set(c, new Map());
    const combo = mergeCombo(keys);
    if (!used.get(c).has(combo)) used.get(c).set(combo, new Set());
    used.get(c).get(combo).add(action);
  };
  for (const [where, keys] of theirs) {
    const [map, action] = where.split("/");
    if (mergeIsKeyboard(keys)) use(map, action, keys);
  }
  for (const g of game) {
    const where = `${g.map}/${g.name}`;
    if (mergeIsKeyboard(g.kb) && !theirs.has(where) && !adding.has(where)) use(g.map, g.name, g.kb);
  }
  const taken = (keys, maps, action) => {
    const combo = mergeCombo(keys);
    const other = (set) => Boolean(set && set.has(combo) && [...set.get(combo)].some((a) => a !== action));
    return maps.some((m) => {
      const c = contextOf(m);
      return c === "global" ? [...used.values()].some(other) : other(used.get(c)) || other(used.get("global"));
    });
  };

  // First the player's own key for the action (from another map) or SpoolUp's, then free combos.
  const chosen = new Map();
  for (const t of todo) {
    const pick = [t.own, t.keys].find((k) => mergeIsKeyboard(k) && !taken(k, t.maps, t.action));
    if (pick) { chosen.set(t.action, pick); t.maps.forEach((m) => use(m, t.action, pick)); }
  }
  const pool = mergeComboPool();
  for (const t of todo) {
    if (chosen.has(t.action)) continue;
    const pick = pool.find((k) => !taken(k, t.maps, t.action));
    if (pick) { chosen.set(t.action, pick); t.maps.forEach((m) => use(m, t.action, pick)); }
  }

  const byMap = new Map();
  const added = [];
  for (const t of todo) {
    const keys = chosen.get(t.action);
    if (!keys) continue;
    added.push({ action: t.action, keys, maps: t.maps, moved: mergeCombo(keys) !== mergeCombo(t.keys) });
    for (const m of t.maps) {
      if (!byMap.has(m)) byMap.set(m, []);
      byMap.get(m).push({ action: t.action, keys });
    }
  }
  let body = profile.body;
  for (const [map, rows] of byMap) body = mergeAddToMap(body, map, rows);
  const lines = [
    `<ActionMaps version="1" optionsVersion="2" rebindVersion="2" profileName="${MERGED_PROFILE_NAME}">`,
    ` <CustomisationUIHeader label="${MERGED_PROFILE_NAME}" description="" image="">`,
    "  <devices>",
    ...profile.devices.map(([type, instance]) => `   <${type} instance="${instance}"/>`),
    "  </devices>",
    "  <categories />",
    " </CustomisationUIHeader>",
    body,
    "</ActionMaps>",
  ];
  return { xml: lines.join("\r\n") + "\r\n", added, kept: mine.length };
}

const mergeArg = process.argv.indexOf("--merge-profile");
if (mergeArg > -1) {
  const out = process.argv[mergeArg + 1];
  const source = process.argv[mergeArg + 2] || findActionmaps();
  if (!out || !/\.xml$/i.test(out) || /^actionmaps\.xml$/i.test(path.basename(out))) {
    console.error("Usage: --merge-profile <profile.xml> [actionmaps.xml]");
    process.exit(1);
  }
  if (!source || !fs.existsSync(source)) {
    console.error("No actionmaps.xml found, so there are no binds of yours to keep. Install the plain SpoolUp Keyboard profile instead.");
    process.exit(3);
  }
  if (path.resolve(source).toLowerCase() === path.resolve(out).toLowerCase()) {
    console.error("The merged profile can't replace the file it's made from.");
    process.exit(1);
  }
  const merged = mergeKeybindProfile(fs.readFileSync(source, "utf8"), readBundled("keybinds/TabletDeck-StarCitizen.xml").toString("utf8"),
    JSON.parse(readBundled("data/sc-actions-4.10.json")).actions, JSON.parse(readBundled("data/sc-contexts.json")).contexts);
  if (!merged) {
    console.error(`${source} isn't a Star Citizen keybind file.`);
    process.exit(1);
  }
  fs.mkdirSync(path.dirname(path.resolve(out)), { recursive: true });
  fs.writeFileSync(out, merged.xml);
  // First line for the app window: how many binds were added and how many of those aren't
  // SpoolUp's usual keys; then one line per added bind.
  console.log(`added ${merged.added.length} moved ${merged.added.filter((a) => a.moved).length}`);
  for (const a of merged.added) console.log(`${a.action} ${a.keys}${a.moved ? " (moved)" : ""}`);
  process.exit(0);
}

// ---------- scancodes (set 1). [code, extended] ----------

const SCANCODES = {
  escape: [0x01, 0], "1": [0x02, 0], "2": [0x03, 0], "3": [0x04, 0], "4": [0x05, 0], "5": [0x06, 0],
  "6": [0x07, 0], "7": [0x08, 0], "8": [0x09, 0], "9": [0x0a, 0], "0": [0x0b, 0], minus: [0x0c, 0],
  equals: [0x0d, 0], backspace: [0x0e, 0], tab: [0x0f, 0], q: [0x10, 0], w: [0x11, 0], e: [0x12, 0],
  r: [0x13, 0], t: [0x14, 0], y: [0x15, 0], u: [0x16, 0], i: [0x17, 0], o: [0x18, 0], p: [0x19, 0],
  lbracket: [0x1a, 0], rbracket: [0x1b, 0], enter: [0x1c, 0], lctrl: [0x1d, 0], a: [0x1e, 0],
  s: [0x1f, 0], d: [0x20, 0], f: [0x21, 0], g: [0x22, 0], h: [0x23, 0], j: [0x24, 0], k: [0x25, 0],
  l: [0x26, 0], semicolon: [0x27, 0], apostrophe: [0x28, 0], grave: [0x29, 0], lshift: [0x2a, 0],
  backslash: [0x2b, 0], z: [0x2c, 0], x: [0x2d, 0], c: [0x2e, 0], v: [0x2f, 0], b: [0x30, 0],
  n: [0x31, 0], m: [0x32, 0], comma: [0x33, 0], period: [0x34, 0], slash: [0x35, 0],
  rshift: [0x36, 0], np_multiply: [0x37, 0], lalt: [0x38, 0], space: [0x39, 0], capslock: [0x3a, 0],
  f1: [0x3b, 0], f2: [0x3c, 0], f3: [0x3d, 0], f4: [0x3e, 0], f5: [0x3f, 0], f6: [0x40, 0],
  f7: [0x41, 0], f8: [0x42, 0], f9: [0x43, 0], f10: [0x44, 0], numlock: [0x45, 0],
  scrolllock: [0x46, 0], np_7: [0x47, 0], np_8: [0x48, 0], np_9: [0x49, 0], np_subtract: [0x4a, 0],
  np_4: [0x4b, 0], np_5: [0x4c, 0], np_6: [0x4d, 0], np_add: [0x4e, 0], np_1: [0x4f, 0],
  np_2: [0x50, 0], np_3: [0x51, 0], np_0: [0x52, 0], np_period: [0x53, 0], f11: [0x57, 0],
  f12: [0x58, 0], f13: [0x64, 0], f14: [0x65, 0], f15: [0x66, 0], f16: [0x67, 0], f17: [0x68, 0],
  f18: [0x69, 0], f19: [0x6a, 0], f20: [0x6b, 0], f21: [0x6c, 0], f22: [0x6d, 0], f23: [0x6e, 0],
  f24: [0x76, 0],
  np_enter: [0x1c, 1], rctrl: [0x1d, 1], np_divide: [0x35, 1], print: [0x37, 1], ralt: [0x38, 1],
  home: [0x47, 1], up: [0x48, 1], pgup: [0x49, 1], left: [0x4b, 1], right: [0x4d, 1], end: [0x4f, 1],
  down: [0x50, 1], pgdn: [0x51, 1], insert: [0x52, 1], delete: [0x53, 1],
};
// Some Star Citizen profiles write numpad keys as num1 instead of np_1.
for (let d = 0; d <= 9; d++) SCANCODES[`num${d}`] = SCANCODES[`np_${d}`];

// A combo's keys with its modifiers first, in the order written: Star Citizen writes some
// binds modifier last ("f6+lalt"), and pressed in that order the game would see F6 alone.
const COMBO_MODIFIERS = ["lalt", "ralt", "lctrl", "rctrl", "lshift", "rshift"];
function comboKeys(combo) {
  const keys = String(combo).toLowerCase().split("+").map((k) => k.trim()).filter(Boolean);
  return [...keys.filter((k) => COMBO_MODIFIERS.includes(k)), ...keys.filter((k) => !COMBO_MODIFIERS.includes(k))];
}

// "lalt+n" or "n+lalt" -> [[0x38,0],[0x31,0]]; modifiers first, main key last.
function parseCombo(combo) {
  return comboKeys(combo).map((k) => {
    const sc = SCANCODES[k];
    if (!sc) throw new Error(`Unknown key "${k}" in "${combo}"`);
    return sc;
  });
}

// ---------- Star Citizen actionmaps overrides ----------

// Returns { actionName: "lalt+n" } for keyboard rebinds in Star Citizen actionmaps or
// Mappings profile files. `files` is one path or a list ("auto" is the player's
// actionmaps.xml); later files win, matching a profile loaded in-game on top of earlier
// customizations. A bind the player cleared in the game is written "kb1_ " and kept as "":
// that action has no key now, and its default key may do something else.
function readActionmapOverrides(files) {
  const overrides = {};
  for (const file of expandActionmaps(files)) {
    const xml = readConfigFile(file)?.toString("utf8");
    if (!xml) continue;
    // Not a self-closing <action/>, which would take the next action's binds as its own.
    const actionRe = /<action name="([^"]+)"[^>]*?(?<!\/)>([\s\S]*?)<\/action>/g;
    let m;
    while ((m = actionRe.exec(xml))) {
      const rebind = /<rebind input="kb1_([^"]*)"/.exec(m[2]);
      if (rebind) overrides[m[1]] = rebind[1].trim();
    }
  }
  return overrides;
}

// Only the binds that press something, for the layout editor's "Follow game keybind".
const pressableOverrides = (files) => Object.fromEntries(Object.entries(readActionmapOverrides(files)).filter(([, keys]) => keys));

// ---------- layout ----------

// Loads the layout, applies actionmaps overrides, and validates every key combo.
// A button is any object in the layout with an "id" and "keys", wherever it sits
// (grid pages, canvas panels, menus), so both layout styles share one code path.
// The page asks for device=phone on small screens; phones get config.phoneLayout
// when that file exists, everything else gets config.layout.
// A tablet may ask for another layout it picked (`chosen`, e.g. "layouts/combat.json");
// it gets that one when it exists, else the PC's choice for its device.
function loadLayout(device, chosen) {
  const text = (chosen && layoutFiles().includes(chosen) && readConfigFile(chosen))
    || (device === "phone" && config.phoneLayout && readConfigFile(config.phoneLayout)) || readConfigFile(config.layout);
  if (!text) throw new Error(`Layout file not found: ${config.layout}`);
  const layout = JSON.parse(text.toString("utf8"));
  return { layout, buttons: prepareLayout(layout) };
}

// Gives every button its keys (from the keybind files when it has an scAction) and
// scancodes, and returns them by id. Throws on an unknown key or a duplicate id, so the
// layout editor's saves get the same checks as loading. A keybind file's key that can't
// be pressed (one SpoolUp has no scancode for) only costs that button its rebind: it
// keeps the layout's keys, and the server says so once instead of refusing to start.
const unpressableRebinds = new Set();
function prepareLayout(layout) {
  const overrides = readActionmapOverrides(config.actionmaps);
  const buttons = new Map();
  (function collect(node) {
    if (Array.isArray(node)) { node.forEach(collect); return; }
    if (!node || typeof node !== "object") return;
    if (typeof node.id === "string" && typeof node.keys === "string") {
      const rebind = node.scAction && overrides[node.scAction];
      // Cleared in the game: the button keeps its keys for the checks below but presses
      // nothing (notBound), and the deck dims it.
      if (rebind === "") node.unbound = true;
      if (rebind) {
        try {
          parseCombo(rebind);
          node.keys = comboKeys(rebind).join("+");
        } catch (err) {
          const note = `${node.scAction}: ${rebind}`;
          if (!unpressableRebinds.has(note)) {
            unpressableRebinds.add(note);
            console.log(`Keybinds: ${err.message} for ${node.scAction}; "${node.label || node.id}" keeps ${node.keys}.`);
          }
        }
      }
      node.scancodes = parseCombo(node.keys);
      if (buttons.has(node.id)) throw new Error(`Duplicate button id "${node.id}"`);
      buttons.set(node.id, node);
    }
    for (const v of Object.values(node)) if (v && typeof v === "object") collect(v);
  })(layout);
  return buttons;
}

// ---------- key sender ----------

// Process name of the game (StarCitizen.exe). With onlyWhenGameFocused, presses are
// skipped unless this process owns the window in front.
const GAME_PROCESS = "StarCitizen";

// Returns the path to native/<name>.exe. The installer ships it prebuilt next to
// TabletDeck.exe. From source, it's built from <name>.cs with the C# compiler that ships
// with Windows, and rebuilt when the .cs is newer, so an update's changes reach it.
function buildNative(name, what, refs = []) {
  const exe = path.join(APP_DIR, "native", `${name}.exe`);
  if (INSTALLED) {
    if (!fs.existsSync(exe)) throw new Error(`${exe} is missing; reinstall SpoolUp`);
    return exe;
  }
  const src = path.join(APP_DIR, "native", `${name}.cs`);
  if (!fs.existsSync(exe) || fs.statSync(exe).mtimeMs < fs.statSync(src).mtimeMs) {
    const fw = path.join(process.env.WINDIR || "C:\\Windows", "Microsoft.NET", "Framework64", "v4.0.30319");
    console.log(`Compiling ${what}...`);
    // WPF holds System.Speech.dll, which csc doesn't search by default.
    try {
      execFileSync(path.join(fw, "csc.exe"), ["/nologo", "/optimize", "/target:exe", "/platform:x64", `/lib:${path.join(fw, "WPF")}`, ...refs.map((r) => `/r:${r}`), `/out:${exe}`, src], { stdio: "inherit" });
    } catch (err) {
      throw new Error(`native/${name}.cs didn't compile (${err.message})`);
    }
  }
  return exe;
}

// Starts native/keysender.exe. `proc.focusWaiters` and `proc.exeWaiters` hold pending
// "focus" and "exe" queries, each answered in order.
function spawnKeySender(onExit) {
  const proc = spawn(buildNative("keysender", "key sender"), [], { stdio: ["pipe", "pipe", "inherit"] });
  proc.focusWaiters = [];
  proc.exeWaiters = [];
  let pending = "";
  proc.stdout.on("data", (d) => {
    const lines = (pending + d.toString()).split(/\r?\n/);
    pending = lines.pop();
    for (const line of lines) {
      if (line.startsWith("err")) console.error("key sender:", line);
      else if (line.startsWith("focus ")) proc.focusWaiters.shift()?.(line === "focus 1");
      else if (line === "exe" || line.startsWith("exe ")) proc.exeWaiters.shift()?.(line.slice(4).trim());
    }
  });
  proc.on("exit", (code) => {
    for (const answer of proc.focusWaiters.splice(0)) answer(false);
    for (const answer of proc.exeWaiters.splice(0)) answer(null);
    onExit(code);
  });
  return proc;
}

// Resolves true if the game owns the window in front, asking the given key sender process.
function askFocus(proc) {
  return new Promise((resolve) => {
    let done = false;
    const answer = (focused) => { if (!done) { done = true; resolve(focused); } };
    proc.focusWaiters.push(answer);
    proc.stdin.write(`focus ${GAME_PROCESS}\n`);
    // An unanswered query counts as not focused; its late reply still shifts it off the queue.
    setTimeout(() => answer(false), 1000);
  });
}

// Resolves the full path of the running game's exe, "" when it isn't running, or null when
// the key sender didn't answer (one built before it knew "exe").
function askExe(proc) {
  return new Promise((resolve) => {
    let done = false;
    const answer = (exe) => { if (!done) { done = true; resolve(exe); } };
    proc.exeWaiters.push(answer);
    proc.stdin.write(`exe ${GAME_PROCESS}\n`);
    setTimeout(() => answer(null), 1000);
  });
}

// Keeps gameExe current, so findActionmaps reads the running channel's binds wherever the
// game is installed. When that changes the keybind file, tablets reload their layout.
let lastActionmaps;
async function refreshGameExe() {
  const proc = sender || focusProbe;
  if (!proc) return;
  const exe = await askExe(proc);
  // A key sender that never answers is one built before it knew "exe"; give up on it after
  // a few tries, but not on one slow answer (the PC is busy while the game starts).
  if (exe === null) { if (++exeMisses >= 3) clearInterval(exeWatch); return; }
  exeMisses = 0;
  if (exe === gameExe) return;
  gameExe = exe;
  gameChanged();
}

// After the running game or the picked folder changes: tablets reload when that changes
// the keybind file, decks hear which copy is in use, and Game.log is looked for again.
let lastGame = "";
function gameChanged() {
  const file = findActionmaps();
  if (file !== lastActionmaps && [].concat(config.actionmaps || []).includes(AUTO_ACTIONMAPS)) {
    console.log(file ? `Keybinds: reading ${file}` : "Keybinds: no actionmaps.xml found for Star Citizen.");
    reloadTablets();
  }
  lastActionmaps = file;
  const game = gameInUse();
  if (JSON.stringify(game) !== lastGame) {
    lastGame = JSON.stringify(game);
    for (const c of tablets) if (c.device !== "streamdeck") c.send({ t: "game", game });
  }
  battle.again();
}
let exeWatch = null;
let exeMisses = 0;

let sender = null;
function startSender() {
  if (DRY_RUN) { startFocusProbe(); return; }
  sender = spawnKeySender((code) => {
    console.error(`Key sender exited (${code}); restarting.`);
    sender = null;
    setTimeout(startSender, 500);
  });
}

// Resolves true when keys may be sent: the check is off, it's a dry run, or the game is in front.
function gameFocused() {
  if (!config.onlyWhenGameFocused || !sender) return Promise.resolve(true);
  return askFocus(sender);
}

// Dry runs press nothing, but a key sender that only ever gets "focus" queries lets the
// sandbox log say whether the focus check would have skipped a press. It's kept apart
// from `sender`, so sendKey can't reach it.
let focusProbe = null;
function startFocusProbe() {
  if (!config.onlyWhenGameFocused) return;
  try {
    focusProbe = spawnKeySender((code) => { console.error(`Focus probe exited (${code}).`); focusProbe = null; });
  } catch (err) {
    console.error(`Focus probe unavailable: ${err.message}`);
  }
}

// Dry run: true if the focus check would have skipped a press, null when that's unknown.
async function wouldBlock() {
  if (!config.onlyWhenGameFocused) return false;
  if (!focusProbe) return null;
  return !(await askFocus(focusProbe));
}

// ---------- stuck-key safety ----------

// A key that stays down after the player has moved on (alt-tabbed, put the tablet down
// with a finger on a hold button) would keep acting in the game or land in another app.
// So while any key is down the focus check runs every half second, and when Star Citizen
// loses focus every hold is released and every tap's wait ends early. A hold is also
// released after HOLD_LIMIT_MS; either way its tablet is told. Releases always go through.
const HOLD_LIMIT_MS = 120000;
const openHolds = new Set(); // holds with keys down, from every tablet: { drop(reason) }
const tapCuts = new Set(); // ends a pressing tap's wait
let focusWatch = null;
let focusChecking = false;
// Not in a dry run: it presses no keys, and the showcase runs in a browser in front of the
// game, so every hold would be cut after half a second. The 2-minute limit still applies.
const focusLost = async () => (DRY_RUN ? false : !(await gameFocused()));
function watchFocus() {
  if (focusWatch) return;
  focusWatch = setInterval(async () => {
    if (!keyHolds.size) { clearInterval(focusWatch); focusWatch = null; return; }
    if (focusChecking) return;
    focusChecking = true;
    try {
      if ((openHolds.size || tapCuts.size) && (await focusLost())) {
        console.log("Star Citizen lost focus: releasing held keys.");
        for (const hold of [...openHolds]) hold.drop("focus");
        for (const cut of [...tapCuts]) cut();
      }
    } finally { focusChecking = false; }
  }, 500);
}

function sendKey(direction, [code, ext]) {
  const cmd = `${direction} ${code.toString(16)} ${ext}`;
  if (DRY_RUN || !sender) console.log("  key", cmd);
  else sender.stdin.write(cmd + "\n");
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Reference-count physical keys so two buttons sharing a modifier don't release it early.
const keyHolds = new Map();
function keyDown(sc) {
  const k = sc.join(":");
  const n = keyHolds.get(k) || 0;
  keyHolds.set(k, n + 1);
  if (n === 0) { sendKey("d", sc); watchFocus(); }
}
function keyUp(sc) {
  const k = sc.join(":");
  const n = keyHolds.get(k) || 0;
  if (n <= 1) { keyHolds.delete(k); if (n === 1) sendKey("u", sc); }
  else keyHolds.set(k, n - 1);
}

async function pressCombo(scancodes) {
  for (const sc of scancodes) { keyDown(sc); await sleep(15); }
}
async function releaseCombo(scancodes) {
  for (const sc of [...scancodes].reverse()) { keyUp(sc); await sleep(15); }
}

// Tells a tablet a press was skipped because the game isn't the window in front.
function skipped(btn, client, why = "Star Citizen not focused") {
  console.log(`${btn.label}: skipped, ${why}`);
  ui({ t: "press", label: btn.label, blocked: true });
  client?.send({ t: "blocked", id: btn.id });
}

// Tells a tablet a press did nothing because the player cleared that bind in Star Citizen:
// its default key might do something else in the game now.
function notBound(btn, client, kind, extra = {}) {
  console.log(`${btn.label}: skipped, not bound in Star Citizen (${btn.scAction})`);
  ui({ t: "press", label: btn.label, blocked: true });
  report(btn, kind, { unbound: true, ...extra });
  client?.send({ t: "unbound", id: btn.id });
}

// ---------- sandbox event stream (dry run only) ----------

// The showcase page (public/showcase.html) reads these as its live "keys sent" log.
const eventStreams = new Set();
function emit(event) {
  if (!DRY_RUN) return;
  const data = `data: ${JSON.stringify({ time: Date.now(), ...event })}\n\n`;
  for (const res of eventStreams) res.write(data);
}
setInterval(() => { for (const res of eventStreams) res.write(": ping\n\n"); }, 20000).unref();

// Logs a press with the focus verdict. Queued so a hold's "up" can't overtake its "down".
let reports = Promise.resolve();
function report(btn, kind, extra = {}) {
  if (!DRY_RUN || !eventStreams.size) return;
  const time = Date.now(); // when it happened, not when the focus probe answered
  reports = reports.then(async () => {
    // Releases always go through, so only presses get a focus verdict.
    const blocked = kind === "up" ? false : await wouldBlock();
    emit({ t: "press", time, id: btn.id, label: btn.label, keys: btn.keys, kind, blocked, ...extra });
  });
}

// Buttons with a "state" switch something on or off in the game (doors open, gear down,
// lights on). The state is kept here, per group, so every tablet, voice commands and
// redrawn panels agree; buttons light up while their group is on. "set": true or false
// sets it (Open/Close Doors), otherwise a press flips it. The deck can't see the game, so
// this is only what SpoolUp pressed: it starts all off, and tablets can reset it.
const buttonStates = new Map(); // group -> true while on
function applyState(btn) {
  const s = btn.state;
  if (!s || typeof s.group !== "string") return;
  const on = typeof s.set === "boolean" ? s.set : !buttonStates.get(s.group);
  if (on) buttonStates.set(s.group, true); else buttonStates.delete(s.group);
  for (const c of tablets) c.send({ t: "state", group: s.group, on });
}

// Presses a button's keys for `ms`, then releases them, unless the game isn't in front.
// `kind` (tap, timed, voice) only labels the press in the sandbox log.
async function tapButton(btn, client, ms = btn.ms || config.tapMs, kind = btn.mode === "timed" ? "timed" : "tap") {
  if (btn.unbound) { notBound(btn, client, kind, { ms }); return; }
  if (!(await gameFocused())) { skipped(btn, client); return; }
  console.log(`${btn.label} (${btn.keys}) tap`);
  ui({ t: "press", label: btn.label });
  report(btn, kind, { ms });
  applyState(btn);
  await pressCombo(btn.scancodes);
  // Ends early if Star Citizen loses focus meanwhile (a long press is up to a few seconds).
  await new Promise((resolve) => {
    const cut = () => { clearTimeout(timer); tapCuts.delete(cut); resolve(); };
    const timer = setTimeout(cut, ms);
    tapCuts.add(cut);
  });
  await releaseCombo(btn.scancodes);
}

// The keyboard is shared, so presses that overlap can mix: Weapons + (F5) pressed while
// Shields − (LAlt+F7) holds LAlt reaches the game as LAlt+F5, Weapons −, and two taps of one
// key share a single key-down. So a tap waits for the earlier taps it could mix with: those
// sharing a key, or any with a modifier when it has one or they do. Plain taps of different
// keys still overlap, so a long Auto Land doesn't hold up the lights. A few may wait; more
// are dropped rather than pressed long after they were asked for, and so is a tap whose
// tablet disconnected while it waited.
const MODIFIER_KEYS = new Set(["lalt", "ralt", "lctrl", "rctrl", "lshift", "rshift"].map((k) => SCANCODES[k].join(":")));
const keyIds = (btn) => new Set(btn.scancodes.map((sc) => sc.join(":")));
const TAPS_WAITING = 6;
const tapsAhead = []; // { keys, mod, done } for taps waiting or pressing, oldest first
function queueTap(btn, client, ...rest) {
  const keys = keyIds(btn);
  const me = { keys, mod: [...keys].some((k) => MODIFIER_KEYS.has(k)) };
  const ahead = tapsAhead.filter((t) => t.mod || me.mod || [...t.keys].some((k) => keys.has(k)));
  if (ahead.length >= TAPS_WAITING) return Promise.resolve();
  me.done = Promise.all(ahead.map((t) => t.done))
    .then(() => {
      if (client?.gone) return undefined;
      if (heldModifierAgainst(btn)) { skipped(btn, client, "a held button is holding a modifier key"); return undefined; }
      return tapButton(btn, client, ...rest);
    })
    .catch((err) => console.error(`${btn.label}: ${err.message}`))
    .finally(() => tapsAhead.splice(tapsAhead.indexOf(me), 1));
  tapsAhead.push(me);
  return me.done;
}

// Modifier keys held down by hold buttons (Proximity PTT holds LAlt, Engage Quantum RAlt),
// counted. While one is down, a press without it would reach the game with it, as another
// action, so that press is skipped and its button flashes, like a press the focus check skips.
// Holding plain Push to Talk (Numpad +) holds no modifier and blocks nothing.
const heldModifiers = new Map();
function holdModifiers(keys, delta) {
  for (const k of keys) {
    if (!MODIFIER_KEYS.has(k)) continue;
    const n = (heldModifiers.get(k) || 0) + delta;
    if (n > 0) heldModifiers.set(k, n); else heldModifiers.delete(k);
  }
}
function heldModifierAgainst(btn) {
  const keys = keyIds(btn);
  for (const k of heldModifiers.keys()) if (!keys.has(k)) return true;
  return false;
}

// ---------- voice commands ----------

// Button label as a spoken phrase: "Shields +" -> "shields up", "Master: SCM" -> "master scm".
function spoken(label) {
  return String(label).toLowerCase()
    .replace(/\s*\+$/, " up").replace(/\s*[−-]$/, " down").replace(/&/g, " and ")
    .replace(/[^a-z0-9' ]+/g, " ").replace(/\s+/g, " ").trim();
}

// Phrase -> button id: each button's label plus its "voice" phrases. "voice": false leaves a
// button out (eject, self-destruct). A phrase that fits buttons with different keys is
// dropped rather than guessed; buttons with the same keys (e.g. Mining in two panels) share it.
// Emotes answer to "emote wave", so a stray "wait" or "point" doesn't set one off; with
// shortEmotes they answer to "wave" as well.
function voiceCommands(buttons) {
  const byPhrase = new Map(); // phrase -> button, or null when ambiguous
  for (const btn of buttons.values()) {
    if (btn.voice === false) continue;
    let phrases = [btn.label, ...(btn.voice || [])].map(spoken).filter(Boolean);
    if (/^emote_/.test(btn.scAction || "")) {
      const prefixed = phrases.map((p) => (p.startsWith("emote ") ? p : `emote ${p}`));
      phrases = config.shortEmotes ? [...prefixed, ...phrases] : prefixed;
    }
    for (const phrase of phrases) {
      const other = byPhrase.get(phrase);
      if (other === undefined) byPhrase.set(phrase, btn);
      else if (other && other.keys !== btn.keys) {
        byPhrase.set(phrase, null);
        console.warn(`Voice: "${phrase}" fits both ${other.label} and ${btn.label}, so it's left out.`);
      }
    }
  }
  return new Map([...byPhrase].filter(([, btn]) => btn).map(([phrase, btn]) => [phrase, btn.id]));
}

let voice = null;      // the voice listener, once its grammar is loaded
let voiceList = [];    // what can be said: [phrase, button label] pairs
let voiceFromKey = false; // the last listening came from the PC's push-to-talk key
let voiceStatus = null;   // the window's "voice ready" status, re-sent when push-to-talk changes
let voiceOwner = null; // the tablet holding the talk button; it gets the results
// What tablets are told about voice: "off", "starting", "ready", or "unavailable" with the
// reason. They hide the mic unless it can work, and the voice list explains why.
let tabletVoice = { state: config.voice ? "starting" : "off" };
function setTabletVoice(state, reason) {
  tabletVoice = reason ? { state, reason } : { state };
  for (const c of tablets) c.send({ t: "voice-status", ...tabletVoice });
}
function startVoice() {
  if (!config.voice) { ui({ t: "voice", state: "off" }); setTabletVoice("off"); return; }
  let exe;
  try { exe = buildNative("voice", "voice commands", ["System.Speech.dll"]); } catch (err) {
    console.error(`Voice commands unavailable: ${err.message}.`);
    ui({ t: "voice", state: "error", text: err.message });
    setTabletVoice("unavailable", err.message);
    return;
  }
  const { buttons } = loadLayout();
  const commands = voiceCommands(buttons);
  // [phrase, button label] pairs by label, for the window's list and the tablet's (/voice.json).
  voiceList = [...commands].map(([phrase, id]) => [phrase, buttons.get(id).label])
    .sort((a, b) => a[1].localeCompare(b[1]) || a[0].localeCompare(b[0]));
  ui({ t: "voice-commands", commands: voiceList });
  const proc = spawn(exe, [config.voiceLanguage], { stdio: ["pipe", "pipe", "inherit"] });
  let pending = "";
  proc.stdout.on("data", (d) => {
    const lines = (pending + d.toString()).split(/\r?\n/);
    pending = lines.pop();
    for (const line of lines) {
      if (line.startsWith("ready")) {
        voice = proc;
        sendPushToTalk();
        const key = config.pttKey ? ` or hold ${config.pttKeyName || "the push-to-talk key"} on the PC` : "";
        console.log(`Voice commands ready (${commands.size} phrases, ${line.slice(6)}). Hold the mic button on the tablet${key} and speak.`);
        voiceStatus = { t: "voice", state: "ready", phrases: commands.size, language: line.slice(6) };
        ui({ ...voiceStatus, ptt: config.pttKey ? config.pttKeyName || "on" : "" });
        setTabletVoice("ready");
      } else if (line === "ptt 1" || line === "ptt 0") {
        // Push-to-talk from a PC key: results go to every tablet, and their mic buttons light up.
        voiceFromKey = true;
        voiceOwner = null;
        // `chime`: the PC plays its own sound for this, so tablets playing theirs on the PC
        // don't add a second one.
        for (const c of tablets) c.send({ t: "listening", on: line === "ptt 1", chime: config.pttSound !== false });
        emit({ t: "voice", result: line === "ptt 1" ? "listen" : "stop", running: true });
      } else if (line.startsWith("err")) {
        console.error(`Voice commands unavailable: ${line.slice(4)}. See "Voice commands" at https://github.com/xattribution/SpoolUp#voice-commands`);
        ui({ t: "voice", state: "error", text: line.slice(4) });
        setTabletVoice("unavailable", line.slice(4));
      } else if (line.startsWith("heard ")) {
        const [, id, confidence, ...words] = line.split(" ");
        void onVoice(id, Number(confidence), words.join(" "));
      } else if (line === "none") {
        void onVoice(null, 0, "");
      }
    }
  });
  proc.on("exit", (code) => {
    const wasReady = voice === proc;
    if (wasReady) voice = null;
    console.error(`Voice listener exited (${code})${wasReady ? "; restarting" : ""}.`);
    ui({ t: "voice", state: wasReady ? "starting" : "error", text: `voice listener exited (${code})` });
    setTabletVoice(wasReady ? "starting" : "unavailable", `the voice listener exited (${code})`);
    if (wasReady) setTimeout(startVoice, 2000);
  });
  for (const [phrase, id] of commands) proc.stdin.write(`phrase ${id} ${phrase}\n`);
  proc.stdin.write("ready\n");
}

// Presses the button for a recognized phrase. The layout is re-read so in-game rebinds count.
// Tells the voice listener the push-to-talk key (Windows virtual-key code, 0 = none).
function sendPushToTalk() {
  if (!voice) return;
  voice.stdin.write(`ptt ${config.pttKey || 0} ${config.pttMode === "toggle" ? "toggle" : "hold"}\n`);
  voice.stdin.write(`chime ${config.pttSound === false ? "off" : "on"}\n`);
}

async function onVoice(id, confidence, text) {
  // After push-to-talk on the PC there's no tablet holding the mic, so all of them hear back.
  const client = voiceFromKey ? { send: (obj) => { for (const c of tablets) c.send(obj); } } : voiceOwner;
  const btn = id ? loadLayout().buttons.get(id) : null;
  if (!btn || confidence < config.voiceConfidence) {
    console.log(btn ? `Voice: "${text}" ignored (confidence ${confidence.toFixed(2)})` : "Voice: didn't catch that");
    emit(btn ? { t: "voice", result: "ignored", text, label: btn.label, confidence } : { t: "voice", result: "unheard" });
    ui(btn ? { t: "heard", result: "ignored", text, label: btn.label, confidence } : { t: "heard", result: "unheard" });
    client?.send({ t: "unheard" });
    return;
  }
  console.log(`Voice: "${text}" -> ${btn.label} (confidence ${confidence.toFixed(2)})`);
  emit({ t: "voice", result: "heard", text, label: btn.label, confidence });
  ui({ t: "heard", result: "accepted", text, label: btn.label, confidence });
  client?.send({ t: "heard", id: btn.id, label: btn.label, text });
  // Hold buttons (quantum engage, laser power) get a longer press, set per button.
  await queueTap(btn, client, btn.voiceMs || btn.ms || config.tapMs, "voice");
}

// ---------- button sounds on the PC ----------

// A tablet can have its button sounds (public/sound.js) play on this PC instead of on itself,
// on an output it picks (a headset, to hear them over the game). It sends each sound as a
// list of tones, which native/sound.exe synthesizes and plays. The tablet can only name one
// of the outputs below and send tones within what soundLine() allows (6 s at most, half
// level, 25 a second); the player limits the loudness again. Nothing reaches the game.
const SOUND_TYPES = new Set(["sine", "triangle", "square", "sawtooth"]);
const SOUND_RATE = 25, SOUND_PLAYING = 16; // per tablet: sounds per second, and at once
const SOUND_PLAYING_ALL = 32;      // across every tablet
let soundExe = null, soundBuildError = null; // native/sound.exe, built at start (see below)
let soundPlayer = null;             // native/sound.exe while it runs
let soundState = { state: "idle" }; // for tablets: idle (not started yet), ready or unavailable
let soundIds = 0;                   // the player's ids, across tablets (1 .. 2^31-1)
const soundsPlaying = new Map();    // the player's id -> when it ends, for every tablet
const soundsHeard = new Map();      // "name|output" -> when a tablet last played it there
let soundExits = [];                // when the player stopped lately, to stop restarting it
const deviceWaiters = [];           // answers to "devices", in order
function setSoundState(state, reason) {
  soundState = reason ? { state, reason } : { state };
  for (const c of tablets) c.send({ t: "sound-status", ...soundState });
}

// Built when the server starts (from source, compiling it once), never while tablets press
// keys: compiling blocks the server for a moment. Started on demand: when a tablet plays
// sounds here, or asks for the outputs.
function buildSoundPlayer() {
  try { soundExe = buildNative("sound", "PC sounds"); } catch (err) {
    soundBuildError = err.message;
    console.error(`PC sounds unavailable: ${err.message}.`);
    setSoundState("unavailable", err.message);
  }
}
function startSoundPlayer() {
  if (soundPlayer || soundState.state === "unavailable" || !soundExe) return soundPlayer;
  const proc = spawn(soundExe, [], { stdio: ["pipe", "pipe", "inherit"] });
  proc.stdin.on("error", () => {}); // it stopped; "exit" deals with that
  let pending = "", missing = new Set();
  proc.stdout.setEncoding("utf8"); // output names can be non-ASCII ("Kopfhörer")
  proc.stdout.on("data", (d) => {
    const lines = (pending + d).split(/\r?\n/);
    pending = lines.pop();
    for (const line of lines) {
      const [word, ...rest] = line.split("\t");
      if (word === "ready") setSoundState("ready");
      else if (word === "devices") deviceWaiters.shift()?.({ preferred: Number(rest[0]), devices: rest.slice(1).filter(Boolean) });
      else if (word === "missing" && !missing.has(rest[0]) && missing.size < 20) {
        missing.add(rest[0]);
        console.log(`PC sounds: ${JSON.stringify(rest[0])} isn't connected, so they play on the default output.`);
      } else if (line.startsWith("err")) console.error("PC sounds:", line.slice(4));
    }
  });
  const stopped = (why) => {
    if (soundPlayer !== proc) return;
    soundPlayer = null;
    for (const answer of deviceWaiters.splice(0)) answer(null);
    // Started again with the next sound, unless it keeps stopping.
    soundExits = [...soundExits.filter((t) => Date.now() - t < 60_000), Date.now()];
    if (soundExits.length >= 3) setSoundState("unavailable", `the sound player stopped (${why})`);
    else setSoundState("idle");
    console.error(`PC sounds stopped (${why}).`);
  };
  proc.on("exit", (code) => stopped(`exit ${code}`));
  proc.on("error", (err) => stopped(err.message));
  soundPlayer = proc;
  return proc;
}

// The PC's outputs, for a tablet's "Play on" list: { devices: [names], preferred: index of
// Windows' default, or -1 }, or null when the player can't say.
function soundDevices() {
  const proc = startSoundPlayer();
  if (!proc) return Promise.resolve(null);
  return new Promise((resolve) => {
    let done = false;
    const answer = (list) => { if (!done) { done = true; resolve(list); } };
    deviceWaiters.push(answer);
    proc.stdin.write("devices\n");
    setTimeout(() => answer(null), 3000); // a late answer still shifts it off the queue
  });
}

const inRange = (v, min, max) => typeof v === "number" && Number.isFinite(v) && v >= min && v <= max;
// A tablet's sound as the player's "play" line (without its id: `play\t<id>` goes in front)
// and how long it lasts (s), or null when any of it is off. See the tone fields in
// public/sound.js. Levels stop at 0.5 (the deck's loudest tone) and the tones' lengths add
// up to 8 s at most, so a tablet can't make the PC play anything much louder or longer than
// the deck's own sounds. Output names have no control characters (WinMM's never do).
function soundLine(msg) {
  const { tones, device, volume } = msg;
  if (!Array.isArray(tones) || tones.length < 1 || tones.length > 48) return null;
  if (typeof device !== "string" || device.length > 64 || /[\u0000-\u001f\u007f-\u009f]/.test(device) || !inRange(volume, 0, 1)) return null;
  let end = 0, total = 0;
  const parts = [];
  for (const k of tones) {
    if (!k || !SOUND_TYPES.has(k.type) || !inRange(k.f0, 20, 12000) || !inRange(k.f1, 20, 12000)
      || !inRange(k.len, 0.005, 4) || !inRange(k.level, 0, 0.5) || !inRange(k.at, 0, 4) || !inRange(k.attack, 0.001, 1)
      || (k.sustain !== 0 && k.sustain !== 1) || !inRange(k.release, 0, 1)
      || !((k.lp0 === 0 && k.lp1 === 0) || (inRange(k.lp0, 20, 20000) && inRange(k.lp1, 20, 20000)))) return null;
    end = Math.max(end, k.at + k.len + (k.sustain ? k.release : 0));
    total += k.len + (k.sustain ? k.release : 0) + 0.03;
    parts.push([k.type, k.f0, k.f1, k.len, k.level, k.at, k.attack, k.sustain, k.release, k.lp0, k.lp1].join(","));
  }
  if (end > 6 || total > 8) return null;
  return { rest: `${device}\t${volume}\t${parts.join(";")}`, end };
}

// ---------- open a website on the PC ----------

// A tablet asks by position in config.links, so it can only open the addresses set up on
// this PC, never one of its own. Opens in the default browser; dry runs only log it.
let lastOpen = 0;
function openLink(index, client) {
  const link = (config.links || [])[index];
  if (!link || !/^https:\/\/[^\s"<>]{3,300}$/.test(link.url)) return;
  if (Date.now() - lastOpen < 1500) return; // one at a time, even on a double tap
  lastOpen = Date.now();
  if (DRY_RUN) {
    console.log(`Open on PC: ${link.name} (${link.url}) - dry run, not opened`);
    client.send({ t: "opened", name: link.name, dryRun: true });
    return;
  }
  console.log(`Opening ${link.name} (${link.url}) on this PC.`);
  // rundll32 hands the address to the default browser without a command shell in between.
  spawn("rundll32.exe", ["url.dll,FileProtocolHandler", link.url], { detached: true, stdio: "ignore" }).unref();
  client.send({ t: "opened", name: link.name });
}

// ---------- minimal WebSocket server (RFC 6455, text frames) ----------

function wsAccept(req, socket) {
  const key = req.headers["sec-websocket-key"];
  const accept = crypto.createHash("sha1").update(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").digest("base64");
  socket.write(
    "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
    `Sec-WebSocket-Accept: ${accept}\r\n\r\n`
  );
  socket.setNoDelay(true);
}

function wsSend(socket, text) {
  const payload = Buffer.from(text);
  let header;
  if (payload.length < 126) header = Buffer.from([0x81, payload.length]);
  else if (payload.length < 65536) { header = Buffer.alloc(4); header[0] = 0x81; header[1] = 126; header.writeUInt16BE(payload.length, 2); }
  else { header = Buffer.alloc(10); header[0] = 0x81; header[1] = 127; header.writeBigUInt64BE(BigInt(payload.length), 2); }
  socket.write(Buffer.concat([header, payload]));
}

const MAX_FRAME = 64 * 1024;

// Calls onText for each complete text frame; handles ping and close.
function wsReader(socket, onText, onClose) {
  let buf = Buffer.alloc(0);
  socket.on("data", (chunk) => {
    buf = Buffer.concat([buf, chunk]);
    while (buf.length >= 2) {
      const opcode = buf[0] & 0x0f;
      const masked = (buf[1] & 0x80) !== 0;
      let len = buf[1] & 0x7f;
      let off = 2;
      if (len === 126) { if (buf.length < 4) return; len = buf.readUInt16BE(2); off = 4; }
      else if (len === 127) { if (buf.length < 10) return; len = Number(buf.readBigUInt64BE(2)); off = 10; }
      // The deck's messages are a few dozen bytes; don't buffer a frame claiming megabytes.
      if (len > MAX_FRAME) { socket.destroy(); return; }
      const maskOff = off;
      if (masked) off += 4;
      if (buf.length < off + len) return;
      const data = Buffer.from(buf.subarray(off, off + len));
      if (masked) for (let i = 0; i < data.length; i++) data[i] ^= buf[maskOff + (i % 4)];
      buf = buf.subarray(off + len);
      if (opcode === 0x1) onText(data.toString("utf8"));
      else if (opcode === 0x8) { socket.end(Buffer.from([0x88, 0])); return; }
      else if (opcode === 0x9 && data.length < 126) socket.write(Buffer.concat([Buffer.from([0x8a, data.length]), data]));
    }
  });
  // The HTTP server leaves sockets half-open, so a client that goes away without a close
  // frame (a killed tab, Stream Deck stopping its plugin) would otherwise keep its held
  // keys down until the watchdog notices, seconds later.
  socket.on("end", () => socket.destroy());
  socket.on("close", onClose);
  socket.on("error", () => socket.destroy());
}

// ---------- HTTP ----------

const MIME = { ".html": "text/html; charset=utf-8", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png", ".webmanifest": "application/manifest+json" };

// ---------- layout editor (public/editor.html) ----------

const fromThisPC = (req) => ["127.0.0.1", "::1", "::ffff:127.0.0.1"].includes(req.socket.remoteAddress);

// Wrong pairing codes: after PAIR_TRIES from one device within a minute, that device is
// refused for a minute, even with the right code, so nothing on the network can keep
// guessing. The PC itself is exempt (it can read the code from config.json anyway).
const PAIR_TRIES = 5, PAIR_WINDOW_MS = 60_000, PAIR_LOCK_MS = 60_000;
const pairFails = new Map(); // remote address -> { count, first, lockedUntil }
// "ok", "wrong" or "locked".
function checkPairing(req, code) {
  if (code === config.token && fromThisPC(req)) return "ok";
  const ip = req.socket.remoteAddress || "";
  const now = Date.now();
  const fails = pairFails.get(ip);
  if (fails && fails.lockedUntil > now) return "locked";
  if (code === config.token) { pairFails.delete(ip); return "ok"; }
  if (fromThisPC(req)) return "wrong";
  const rec = fails && now - fails.first < PAIR_WINDOW_MS ? fails : { count: 0, first: now, lockedUntil: 0 };
  rec.count++;
  if (rec.count >= PAIR_TRIES) {
    rec.lockedUntil = now + PAIR_LOCK_MS;
    console.log(`${PAIR_TRIES} wrong pairing codes from ${ip}; ignoring it for a minute.`);
  }
  pairFails.set(ip, rec);
  if (pairFails.size > 1000) for (const [k, v] of pairFails) if (v.lockedUntil < now && now - v.first > PAIR_WINDOW_MS) pairFails.delete(k);
  return "wrong";
}
// For the HTTP routes: answers 403 (wrong code) or 429 (too many) and returns true when refused.
function refusePairing(req, res, url) {
  const result = checkPairing(req, url.searchParams.get("t"));
  if (result === "ok") return false;
  res.writeHead(result === "locked" ? 429 : 403);
  res.end(result === "locked" ? "too many wrong pairing codes" : "bad token");
  return true;
}
function sendJson(res, status, obj) {
  res.writeHead(status, { "Content-Type": MIME[".json"], "Cache-Control": "no-store" });
  res.end(JSON.stringify(obj));
}

// Every layout file: the bundled ones plus the player's own in the settings folder.
function layoutFiles() {
  let bundled;
  if (INSTALLED) {
    try { bundled = JSON.parse(readBundled("bundled-files.json")); } catch { bundled = []; }
  } else {
    bundled = fs.readdirSync(path.join(__dirname, "layouts")).map((f) => `layouts/${f}`);
  }
  const files = new Set(bundled.filter((f) => /^layouts\/[^/]+\.json$/.test(f)));
  const own = path.join(DATA_DIR, "layouts");
  if (fs.existsSync(own)) for (const f of fs.readdirSync(own)) if (f.endsWith(".json")) files.add(`layouts/${f}`);
  return [...files].sort();
}

// Tablets that are connected, so a saved layout can make them reload.
const tablets = new Set();
const reloadTablets = () => { for (const client of tablets) client.send({ t: "reload" }); };

// The editor reads layouts and saves the player's own; it only answers this PC, so a
// device on the network can't change files even with the pairing code.
function handleEditor(req, res, url) {
  if (url.searchParams.get("t") !== config.token) return sendJson(res, 403, { error: "Wrong pairing code." });
  if (!fromThisPC(req)) return sendJson(res, 403, { error: "The layout editor only works on the PC running SpoolUp (open it at http://localhost)." });

  if (url.pathname === "/editor/layouts") {
    const layouts = [];
    for (const file of layoutFiles()) {
      try {
        const l = JSON.parse(readConfigFile(file));
        const grid = Boolean(l.grid && typeof l.grid === "object");
        layouts.push({ file, name: l.name || file, canvas: l.canvas || null, grid, hidden: l.hidden === true, custom: l.custom === true,
          active: file === config.layout ? "tablet" : file === config.phoneLayout ? "phone" : null });
      } catch { /* not a layout */ }
    }
    return sendJson(res, 200, {
      layouts,
      keys: Object.keys(SCANCODES).filter((k) => !/^num\d$/.test(k)),
      // scAction -> keys from the player's Star Citizen keybinds, for "Follow game keybind".
      overrides: pressableOverrides(config.actionmaps),
    });
  }

  if (url.pathname === "/editor/layout") {
    const file = url.searchParams.get("file") || "";
    if (!layoutFiles().includes(file)) return sendJson(res, 404, { error: "No such layout." });
    return sendJson(res, 200, JSON.parse(readConfigFile(file)));
  }

  if (url.pathname === "/editor/save" && req.method === "POST") {
    let body = "";
    req.setEncoding("utf8");
    req.on("data", (chunk) => { body += chunk; if (body.length > 4e6) req.destroy(); });
    req.on("end", () => {
      let data;
      try { data = JSON.parse(body); } catch { return sendJson(res, 400, { error: "The editor sent something unreadable." }); }
      const name = String(data.name || "").trim().toLowerCase();
      if (!/^[a-z0-9][a-z0-9-]{0,39}$/.test(name)) return sendJson(res, 400, { error: "Use letters, numbers and dashes for the name." });
      const file = `layouts/${name}.json`;
      // Only the player's own layouts (marked "custom") may be overwritten.
      const existing = readConfigFile(file);
      if (existing) {
        let custom = false;
        try { custom = JSON.parse(existing).custom === true; } catch { /* unreadable: treat as not ours */ }
        if (!custom) return sendJson(res, 400, { error: `"${name}" is a built-in layout. Pick another name.` });
      }
      const layout = data.layout;
      // A 2.0 canvas layout, or a grid layout (2.1), which fits every screen.
      const grid = Boolean(layout && typeof layout === "object" && layout.grid && typeof layout.grid === "object" && !Array.isArray(layout.grid));
      if (!layout || typeof layout !== "object" || !(grid || Array.isArray(layout.canvas))) return sendJson(res, 400, { error: "That isn't a layout." });
      if (grid && (!layout.panels || typeof layout.panels !== "object")) return sendJson(res, 400, { error: "That isn't a layout." });
      layout.custom = true;
      try { prepareLayout(JSON.parse(JSON.stringify(layout))); } catch (err) { return sendJson(res, 400, { error: err.message }); }
      const target = path.join(DATA_DIR, file);
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.writeFileSync(target, JSON.stringify(layout, null, 2) + "\n");
      console.log(`Saved layout ${file}.`);
      // "Use on tablets": a grid layout is for phones and tablets alike.
      const kinds = grid ? ["layout", "phoneLayout"] : [layout.canvas[1] < 500 ? "phoneLayout" : "layout"];
      if (data.use) {
        for (const kind of kinds) {
          if (config[kind] === file) continue;
          config[kind] = file;
          saveSetting(kind, file);
          console.log(`${kind === "layout" ? "Tablets" : "Phones"} now use ${file}.`);
        }
      }
      if (config.layout === file || config.phoneLayout === file) {
        reloadTablets();
        // Voice commands come from the tablet layout; restart the listener so they match.
        if (config.layout === file && voice) voice.kill();
      }
      sendJson(res, 200, { ok: true, file, active: kinds.every((kind) => config[kind] === file) });
    });
    return;
  }
  sendJson(res, 404, { error: "Not found." });
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, "http://x");
  // Server-sent events for the showcase page's key log; dry runs only.
  if (url.pathname === "/events" && DRY_RUN) {
    if (refusePairing(req, res, url)) return;
    res.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-store", Connection: "keep-alive" });
    res.write(`data: ${JSON.stringify({ t: "hello", time: Date.now(), focusCheck: config.onlyWhenGameFocused, focusProbe: Boolean(focusProbe), voice: Boolean(voice) })}\n\n`);
    eventStreams.add(res);
    req.on("close", () => eventStreams.delete(res));
    return;
  }
  // The voice commands list, for the tablet's "?" next to the mic button.
  // The names of the websites a tablet may open on this PC; the addresses stay here.
  if (url.pathname === "/links.json") {
    if (refusePairing(req, res, url)) return;
    return sendJson(res, 200, { links: (config.links || []).map((l) => l.name) });
  }
  // The PC's outputs a tablet can play its button sounds on; it picks one by name.
  if (url.pathname === "/sound-devices.json") {
    if (refusePairing(req, res, url)) return;
    soundDevices().then((list) => sendJson(res, 200, { ...soundState, devices: list ? list.devices : [], preferred: list ? list.preferred : -1 }));
    return;
  }
  if (url.pathname === "/voice.json") {
    if (refusePairing(req, res, url)) return;
    res.writeHead(200, { "Content-Type": MIME[".json"], "Cache-Control": "no-store" });
    res.end(JSON.stringify({ on: Boolean(config.voice), ready: Boolean(voice), ...tabletVoice, commands: voiceList }));
    return;
  }
  // Layouts a tablet can pick in its settings sheet: the 2.0 ones made for its screen (phone
  // layouts are under 500 px tall), plus grid layouts and tabbed layouts, which fit any
  // screen. Hidden ones (copies kept so old settings still find them) aren't listed.
  if (url.pathname === "/layouts.json") {
    if (refusePairing(req, res, url)) return;
    const phone = url.searchParams.get("device") === "phone";
    const layouts = [];
    for (const file of layoutFiles()) {
      try {
        const l = JSON.parse(readConfigFile(file));
        if (l.hidden === true) continue;
        if (l.canvas && (l.canvas[1] < 500) !== phone) continue;
        layouts.push({ file, name: l.name || file, custom: l.custom === true });
      } catch { /* not a layout */ }
    }
    const pcChoice = phone && config.phoneLayout && readConfigFile(config.phoneLayout) ? config.phoneLayout : config.layout;
    // The PC's choice by name too, since a hidden copy (hud-phone.json) isn't in the list.
    let pcName = null;
    try { pcName = JSON.parse(readConfigFile(pcChoice)).name || null; } catch { /* unreadable: the deck says "default" */ }
    return sendJson(res, 200, { layouts, pcChoice, pcName });
  }
  if (url.pathname === "/layout.json") {
    if (refusePairing(req, res, url)) return;
    try {
      const { layout } = loadLayout(url.searchParams.get("device"), url.searchParams.get("layout"));
      res.writeHead(200, { "Content-Type": MIME[".json"], "Cache-Control": "no-store" });
      res.end(JSON.stringify(layout, (k, v) => (k === "scancodes" ? undefined : v)));
    } catch (err) {
      res.writeHead(500); res.end(String(err.message));
    }
    return;
  }
  if (url.pathname.startsWith("/editor/")) { handleEditor(req, res, url); return; }
  const rel = path.posix.normalize(`public/${url.pathname === "/" ? "index.html" : url.pathname.slice(1)}`);
  // /layouts/ serves the built-in layouts as files, for the deck's demo mode (?demo).
  const body = rel.startsWith("public/layouts/") ? readBundled(rel.slice("public/".length))
    : rel.startsWith("public/") ? readBundled(rel) : null;
  if (!body) { res.writeHead(404); res.end("not found"); return; }
  res.writeHead(200, { "Content-Type": MIME[path.extname(rel)] || "application/octet-stream" });
  res.end(body);
});

let connectedTablets = 0;
server.on("upgrade", (req, socket) => {
  const url = new URL(req.url, "http://x");
  const pairing = url.pathname === "/ws" ? checkPairing(req, url.searchParams.get("t")) : "wrong";
  if (pairing === "locked") { socket.end("HTTP/1.1 429 Too Many Requests\r\n\r\n"); return; }
  if (pairing !== "ok") {
    socket.end("HTTP/1.1 403 Forbidden\r\n\r\n"); return;
  }
  let buttons;
  try { ({ buttons } = loadLayout(url.searchParams.get("device"), url.searchParams.get("layout"))); } catch (err) {
    console.error("Layout error:", err.message); socket.end("HTTP/1.1 500 Layout Error\r\n\r\n"); return;
  }
  wsAccept(req, socket);
  const who = socket.remoteAddress;
  const device = url.searchParams.get("device") || "tablet";
  // The Stream Deck plugin (docs/STREAMDECK-API.md) connects like a tablet and presses the same
  // buttons through the same checks; only its name in the log differs.
  const kind = device === "streamdeck" ? "Stream Deck" : "Tablet";
  console.log(`${kind} connected: ${who}`);
  emit({ t: "client", on: true, device });
  ui({ t: "clients", count: ++connectedTablets });
  const client = {
    device, // a tablet's paged panels go only to Stream Decks
    send: (obj) => { if (!socket.destroyed) wsSend(socket, JSON.stringify(obj)); },
    // Closing runs the same clean-up as a dropped connection, held keys included.
    close: () => socket.destroy(),
  };
  tablets.add(client);
  client.send({ t: "hello", voice: Boolean(config.voice), voiceStatus: tabletVoice, states: Object.fromEntries(buttonStates), soundStatus: soundState, tapMs: config.tapMs, game: gameInUse() });
  sendBattle(client);

  const held = new Map(); // button id -> { sc } currently pressed by this tablet
  // Its button sounds playing on the PC: its id -> { id: the player's id, ends }, and when
  // the last ones came, for the rate limit.
  const sounds = new Map();
  let soundTimes = [];
  let soundsHere = false;
  // A sound counts against the limits until it would have ended, even when it's stopped
  // early, so stopping sounds doesn't make room for more. Rejected ones count as well.
  const playSound = (msg) => {
    const now = Date.now();
    soundTimes = soundTimes.filter((t) => now - t < 1000);
    for (const [id, s] of sounds) if (s.ends < now) sounds.delete(id);
    for (const [id, ends] of soundsPlaying) if (ends < now) soundsPlaying.delete(id);
    if (soundTimes.length >= SOUND_RATE || sounds.size >= SOUND_PLAYING || soundsPlaying.size >= SOUND_PLAYING_ALL) return;
    soundTimes.push(now);
    if (!Number.isInteger(msg.id) || msg.id < 1 || msg.id > 2147483647 || sounds.has(msg.id)) return;
    const sound = soundLine(msg);
    if (!sound) return;
    const name = /^[a-z-]{1,24}$/.test(String(msg.name)) ? msg.name : "sound";
    // Voice results and the PC's push-to-talk reach every tablet, so two tablets playing on
    // the same output would play them twice: the second one within a moment is left out.
    const heard = `${name}|${msg.device}`, last = soundsHeard.get(heard);
    if (last && last.client !== client && now - last.time < 150) return;
    soundsHeard.set(heard, { client, time: now });
    if (soundsHeard.size > 200) soundsHeard.clear();
    emit({ t: "sound", name, device: msg.device });
    const player = startSoundPlayer();
    if (!player) return;
    soundIds = soundIds >= 2147483647 ? 1 : soundIds + 1;
    const ends = now + sound.end * 1000 + 200;
    player.stdin.write(`play\t${soundIds}\t${sound.rest}\n`);
    sounds.set(msg.id, { id: soundIds, ends });
    soundsPlaying.set(soundIds, ends);
  };
  const stopSound = (localId) => {
    const s = sounds.get(localId);
    if (!s || s.stopped) return;
    s.stopped = true;
    soundPlayer?.stdin.write(`stop\t${s.id}\n`);
  };
  let lastSeen = Date.now();
  // Releases a hold once its press has finished, and only if it pressed anything: a
  // combo's keys go down one by one, so a quick release could otherwise land in between
  // and leave the last key stuck, or let go of a key another button is holding.
  const releaseHold = async (hold) => {
    clearTimeout(hold.limit);
    openHolds.delete(hold);
    if (!hold.pressing) return;
    await hold.pressing;
    await releaseCombo(hold.sc);
    holdModifiers(hold.mods, -1);
  };
  const releaseAll = async () => {
    for (const [id, hold] of held) {
      held.delete(id);
      report(buttons.get(id) || { id, label: id }, "up", { released: true });
      await releaseHold(hold);
    }
  };
  // If the tablet sleeps or drops Wi-Fi mid-hold, don't leave keys stuck down.
  const watchdog = setInterval(() => {
    if (Date.now() - lastSeen > 5000) { void releaseAll(); socket.destroy(); }
  }, 1000);

  wsReader(socket, async (text) => {
    lastSeen = Date.now();
    let msg;
    try { msg = JSON.parse(text); } catch { return; }
    // Answered, so the tablet can show the round trip and notice a link that died without
    // closing (this PC asleep, Wi-Fi gone), which it would otherwise still show as connected.
    if (msg.t === "hb") { client.send({ t: "hb", n: Number(msg.n) || 0 }); return; }
    if (msg.t === "voice") {
      emit({ t: "voice", result: msg.on ? "listen" : "stop", running: Boolean(voice) });
      if (!voice) {
        client.send({ t: "voice-off", reason: config.voice ? "Voice commands aren't running on the PC" : "Voice commands are turned off" });
      } else if (msg.on) {
        voiceOwner = client;
        voiceFromKey = false;
        voice.stdin.write("listen\n");
      } else if (voiceOwner === client) {
        voice.stdin.write("stop\n");
      }
      return;
    }
    if (msg.t === "open") { openLink(msg.index, client); return; }
    // A Stream Deck scene key: tablets that follow scenes page their HUD halves to its
    // panels. Only a scene name and panel ids pass through; a tablet skips panels its
    // layout doesn't have.
    if (msg.t === "scene") {
      const name = (v) => (typeof v === "string" && /^[a-z0-9-]{1,40}$/.test(v) ? v : undefined);
      if (!name(msg.scene)) return;
      const scene = { t: "scene", scene: msg.scene, left: name(msg.left), right: name(msg.right) };
      for (const c of tablets) if (c !== client) c.send(scene);
      return;
    }
    // A tablet's player paged one HUD half: Stream Decks (the plugin) go to the scene with
    // that panel. Only a side and a panel id pass through, and never to tablets, which
    // would page each other.
    if (msg.t === "panel") {
      if ((msg.side !== "left" && msg.side !== "right") || typeof msg.panel !== "string" || !/^[a-z0-9-]{1,40}$/.test(msg.panel)) return;
      const panel = { t: "panel", side: msg.side, panel: msg.panel };
      for (const c of tablets) if (c !== client && c.device === "streamdeck") c.send(panel);
      return;
    }
    if (msg.t === "sound") { playSound(msg); return; }
    if (msg.t === "sound-stop") { stopSound(msg.id); return; }
    if (msg.t === "sound-out") {
      const here = msg.pc === true;
      if (here && !soundsHere) { console.log(`Button sounds from ${who} play on this PC.`); startSoundPlayer(); }
      soundsHere = here;
      return;
    }
    if (msg.t === "reset-states") {
      buttonStates.clear();
      for (const c of tablets) c.send({ t: "states", states: {} });
      return;
    }
    const btn = buttons.get(msg.id);
    if (!btn) return;
    if (msg.t === "down" && !held.has(btn.id)) {
      if (btn.unbound) { notBound(btn, client, "down"); return; }
      // Claim the hold before the focus check so a quick "up" during it cancels the press.
      const hold = { sc: btn.scancodes };
      held.set(btn.id, hold);
      if (!(await gameFocused())) {
        if (held.get(btn.id) === hold) held.delete(btn.id);
        skipped(btn, client);
        return;
      }
      if (held.get(btn.id) !== hold) return;
      if (heldModifierAgainst(btn)) {
        held.delete(btn.id);
        skipped(btn, client, "a held button is holding a modifier key");
        return;
      }
      console.log(`${btn.label} (${btn.keys}) down`);
      ui({ t: "press", label: btn.label });
      applyState(btn);
      report(btn, "down");
      hold.mods = keyIds(btn);
      holdModifiers(hold.mods, 1);
      hold.pressing = pressCombo(btn.scancodes);
      // Let go when Star Citizen loses focus (watchFocus) or after HOLD_LIMIT_MS, and tell
      // the tablet; its finger's own "up" later finds nothing held.
      hold.drop = (reason) => {
        if (held.get(btn.id) !== hold) return;
        held.delete(btn.id);
        console.log(`${btn.label} released: ${reason === "focus" ? "Star Citizen lost focus" : `held for ${HOLD_LIMIT_MS / 1000} s`}`);
        report(btn, "up", { released: reason });
        client.send({ t: "released", id: btn.id, reason });
        void releaseHold(hold);
      };
      openHolds.add(hold);
      hold.limit = setTimeout(() => hold.drop("limit"), HOLD_LIMIT_MS);
      await hold.pressing;
    } else if (msg.t === "up" && held.has(btn.id)) {
      // Releases always go through, even after the game loses focus, so no key stays stuck.
      const hold = held.get(btn.id);
      held.delete(btn.id);
      console.log(`${btn.label} up`);
      report(btn, "up");
      await releaseHold(hold);
    } else if (msg.t === "tap") {
      await queueTap(btn, client);
    }
  }, () => {
    clearInterval(watchdog);
    tablets.delete(client);
    client.gone = true;
    void releaseAll();
    for (const id of [...sounds.keys()]) stopSound(id);
    if (voiceOwner === client) { voice?.stdin.write("stop\n"); voiceOwner = null; }
    console.log(`${kind} disconnected: ${who}`);
    emit({ t: "client", on: false, device });
    ui({ t: "clients", count: --connectedTablets });
  });
});

// ---------- battle state from Game.log ----------

// gamelog.js follows the game's own log for hits on your ship, its damage, your
// death and your kills, and the paired decks show it. Stream Decks don't get it (it isn't
// in docs/STREAMDECK-API.md). "gameLog": false in config.json turns the reading off.
// --game-log <file> follows that file instead of the found install's, to try it with a
// sample log next to a dry run.
const gameLogArg = process.argv.indexOf("--game-log");
const battle = gameLog.create({
  files: () => (gameLogArg > -1 ? [{ path: path.resolve(String(process.argv[gameLogArg + 1] || "")), running: true }]
    : findInstalls().map((i) => ({ path: path.join(i.dir, "Game.log"), running: i.running }))),
  onChange: (event, state) => {
    if (DRY_RUN && event !== "sync") console.log(`Game.log: ${event}${event === "hit" ? ` from ${state.from} (${state.hits})` : ""}`);
    for (const c of tablets) if (c.device !== "streamdeck") c.send({ t: "battle", event, state });
  },
  log: (text) => console.log(text),
});
function sendBattle(client) {
  if (client.device !== "streamdeck" && config.gameLog !== false) client.send({ t: "battle", event: "sync", state: battle.state() });
}
if (config.gameLog !== false) battle.start();

// ---------- start ----------

try { loadLayout(); loadLayout("phone"); } catch (err) { fatal(`Layout error: ${err.message}`); }
startSender();
// The running game's folder: at once, then every 10 s.
lastActionmaps = findActionmaps();
lastGame = JSON.stringify(gameInUse());
if (sender || focusProbe) { exeWatch = setInterval(refreshGameExe, 10000); exeWatch.unref(); void refreshGameExe(); }
startVoice();
buildSoundPlayer();
server.on("error", (err) => {
  if (err.code !== "EADDRINUSE") throw err;
  fatal(`Port ${config.port} is already in use. Is SpoolUp already running? Otherwise change "port" in ${CONFIG_PATH}.`);
});
// Adapters a tablet can't be on: VPNs, virtual machines, WSL and Docker. Matched on the
// name Windows gives the adapter ("vEthernet (WSL)", "NordLynx", "Tailscale", ...).
const VIRTUAL_ADAPTER = new RegExp([
  "vEthernet", "WSL", "Hyper-V", "VirtualBox", "VMware", "VMnet", "Docker", "Loopback", "Bluetooth",
  "Tailscale", "ZeroTier", "Hamachi", "NordLynx", "Nord", "Proton", "WireGuard", "OpenVPN", "TAP-", "Wintun",
  "Mullvad", "ExpressVPN", "Surfshark", "PIA", "CyberGhost", "Windscribe", "Cisco", "AnyConnect",
  "Fortinet", "FortiClient", "GlobalProtect", "PANGP", "Juniper", "Pulse", "SonicWall", "Radmin", "VPN",
].join("|"), "i");

// This PC's network addresses, best guess first, and the one a tablet most likely shares
// (the QR code): the "host" setting if set, else a private address on a real adapter.
function addresses() {
  const all = [];
  for (const [name, infos] of Object.entries(os.networkInterfaces())) {
    for (const i of infos || []) {
      if (i.family === "IPv4" && !i.internal && !i.address.startsWith("169.254")) {
        all.push({ name, ip: i.address, virtual: VIRTUAL_ADAPTER.test(name) });
      }
    }
  }
  // Home networks are usually 192.168.x.x, then 10.x.x.x or 172.16-31.x.x.
  const rank = (a) => (a.virtual ? 10 : 0) + (a.ip.startsWith("192.168.") ? 0 : a.ip.startsWith("10.") ? 1
    : /^172\.(1[6-9]|2\d|3[01])\./.test(a.ip) ? 2 : 3);
  const sorted = all.sort((a, b) => rank(a) - rank(b));
  const lan = config.host || (sorted[0] && sorted[0].ip);
  return { all: sorted, ips: sorted.map((a) => a.ip), lan };
}

// Everything the app window shows and its settings dialog edits. Sent again after a
// setting changes, so a new pairing code gets a new QR code.
function sendReady() {
  const { ips, lan } = addresses();
  const link = lan ? `http://${lan}:${config.port}/?t=${config.token}` : null;
  ui({
    t: "ready", version: VERSION, port: config.port, token: config.token, link, settings: CONFIG_PATH,
    others: ips.filter((ip) => ip !== lan).map((ip) => `http://${ip}:${config.port}`),
    // The window draws the QR code itself; rows of "1" (dark) and "0".
    qr: link ? qr.matrix(link).map((row) => row.map((dark) => (dark ? "1" : "0")).join("")) : [],
    dryRun: DRY_RUN, focusCheck: Boolean(config.onlyWhenGameFocused), voiceConfidence: config.voiceConfidence,
    autoUpdate: config.autoUpdate !== false, tapMs: config.tapMs, voice: Boolean(config.voice),
    voiceLanguage: config.voiceLanguage, portFromCommandLine: Boolean(PORT_OVERRIDE), game: gameInUse(),
  });
  checkNetwork();
}

// Windows only lets tablets in on networks it treats as Private (the firewall rule covers
// private networks only), and it often calls a home network Public. Asks Windows which
// kind the tablets' network is, so the window and console can say how to fix it. Checks
// again every 20 s while it's Public, so the warning goes once the player changes it.
// Local only: PowerShell reads Windows' own setting.
let networkTimer = null, networkWarned = false;
function checkNetwork() {
  clearTimeout(networkTimer);
  if (process.platform !== "win32") return;
  const { all, lan } = addresses();
  const adapter = (all.find((a) => a.ip === lan) || {}).name;
  if (!adapter) return;
  const script = "Get-NetConnectionProfile | ForEach-Object { $_.InterfaceAlias + '|' + $_.NetworkCategory }";
  execFile("powershell.exe", ["-NoProfile", "-NonInteractive", "-Command", script], { windowsHide: true, timeout: 15000 }, (err, stdout) => {
    if (err) return; // can't tell: say nothing
    const profiles = new Map(String(stdout).split(/\r?\n/).map((l) => l.trim().split("|")).filter((p) => p.length === 2));
    const category = profiles.get(adapter);
    if (!category) return;
    ui({ t: "network", adapter, category });
    if (category === "Public") {
      if (!UI && !networkWarned) console.warn(`\nWindows treats "${adapter}" as a Public network, so tablets can't connect. In Windows: Settings > Network & Internet > ${adapter} > set it to Private.\n`);
      networkWarned = true;
      networkTimer = setTimeout(checkNetwork, 20000);
      networkTimer.unref();
    }
  });
}

// ---------- finding this PC from the apps (discovery.js) ----------

// The SpoolUp apps list PCs that answer for _spoolup._tcp on the local network (mDNS).
// Only real adapters answer: not VPNs or virtual ones, the same ones addresses() ranks
// last. The records name the PC, its port and version. The
// "discovery" setting (on unless false) turns it off; the window's Settings has it.
let advertiser = null;
function setDiscovery(on) {
  if (!on) {
    if (advertiser) { advertiser.stop(); advertiser = null; }
    return;
  }
  if (advertiser) return;
  advertiser = discovery.create({
    port: config.port, version: VERSION, name: os.hostname(),
    adapters: (name) => !VIRTUAL_ADAPTER.test(name),
    log: (msg) => console.log(msg),
  });
  advertiser.start();
}

server.listen(config.port, "0.0.0.0", () => {
  if (!UI) checkNetwork(); // with the window, sendReady checks
  setDiscovery(config.discovery !== false);
  const { all, ips, lan } = addresses();
  console.log(`\nSpoolUp${VERSION ? ` ${VERSION}` : ""} running${DRY_RUN ? " (DRY RUN: no keys are sent)" : ""}.`);
  if (UI) { sendReady(); return; }
  if (INSTALLED) console.log(`Settings: ${CONFIG_PATH}`);
  console.log("Open this on your tablet (same Wi-Fi/network):");
  if (config.host && !ips.includes(config.host)) console.log(`   http://${config.host}:${config.port}/?t=${config.token}   (the "host" setting)`);
  for (const a of all) {
    const note = a.virtual ? "   (VPN or virtual adapter: tablets usually can't reach this)" : "";
    console.log(`   ${a.name}: http://${a.ip}:${config.port}/?t=${config.token}${note}`);
  }
  if (lan) {
    console.log(`\nOr scan this with the tablet's camera (${lan}${config.host ? `, from the "host" setting` : ""}):`);
    console.log(`If the tablet can't open it, set "host" in ${CONFIG_PATH} to the address of your Wi-Fi or Ethernet.\n`);
    console.log(qr.toTerminal(`http://${lan}:${config.port}/?t=${config.token}`, { color: process.stdout.isTTY }));
  }
  console.log(`\nPairing code: ${config.token}`);
  console.log(`(You can also open http://${lan || "<this PC's IP>"}:${config.port} and type the code.)`);
  console.log(`\nLayout editor (on this PC): http://localhost:${config.port}/editor.html?t=${config.token}`);
  console.log(config.onlyWhenGameFocused
    ? "\nKeys are only sent while Star Citizen is the window in front.\n"
    : "\nKeep the game window focused; keys go to whatever window is in front.\n");
});

const shutdown = () => {
  for (const k of keyHolds.keys()) sendKey("u", k.split(":").map(Number));
  setDiscovery(false); // says goodbye, so the apps drop this PC from their lists
  setTimeout(() => process.exit(0), 100);
};
process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
// Closing the console window (SIGHUP) or Ctrl+Break also releases held keys first.
process.on("SIGHUP", shutdown);
process.on("SIGBREAK", shutdown);
// The app window asks the server to stop with "quit"; if the window itself goes away,
// stdin closes, which stops the server the same way.
// Commands from the app window, one per line: "quit", or "set <name> <value>" from its
// slider, checkboxes and settings dialog. Each setting is checked (SETTINGS), saved to
// config.json and applied at once; the window restarts the server after port or voice ones.
function applySetting(name, text) {
  const value = SETTINGS[name] ? SETTINGS[name](text) : undefined;
  if (value === undefined) { console.error(`Setting not changed: "${text}" isn't a valid ${name}.`); return; }
  saveSetting(name, value);
  // The port only changes when the server restarts, as the window does next.
  if (name !== "port") config[name] = value;
  // A new pairing code shuts out devices paired with the old one: they're disconnected
  // (releasing anything they hold) and show the pairing screen when they try again.
  if (name === "token" && tablets.size) {
    console.log(`New pairing code: disconnecting ${tablets.size === 1 ? "1 device" : `${tablets.size} devices`} paired with the old one.`);
    for (const c of tablets) c.close();
  }
  if (name === "voiceConfidence") {
    console.log(`Voice strictness set to ${Math.round(value * 100)}%.`);
    ui({ t: "voiceConfidence", value });
    return;
  }
  if (name === "gameFolders") {
    console.log(`Star Citizen folder: ${value[0] || "found automatically"}.`);
    gameChanged();
    sendReady();
    return;
  }
  console.log(`Saved ${name}: ${value === true ? "on" : value === false ? "off" : Array.isArray(value) ? value.map((l) => l.name).join(", ") || "none" : value}.`);
  if (name.startsWith("ptt")) {
    sendPushToTalk();
    if (voiceStatus) ui({ ...voiceStatus, ptt: config.pttKey ? config.pttKeyName || "on" : "" });
  }
  // The listener restarts with the new phrases.
  if (name === "shortEmotes" && voice) voice.kill();
  if (name === "discovery") setDiscovery(value);
  sendReady();
}

if (UI) {
  let pendingInput = "";
  process.stdin.setEncoding("utf8");
  process.stdin.on("data", (text) => {
    const lines = (pendingInput + text).split(/\r?\n/);
    pendingInput = lines.pop();
    for (const line of lines.map((l) => l.trim())) {
      if (line === "quit") { shutdown(); return; }
      const set = /^set (\w+) (.+)$/.exec(line);
      if (set) applySetting(set[1], set[2].trim());
    }
  });
  process.stdin.on("end", shutdown);
}

// Rewrites one setting in config.json, keeping the rest of the file as the player wrote it.
function saveSetting(key, value) {
  const saved = JSON.parse(fs.readFileSync(CONFIG_PATH, "utf8"));
  saved[key] = value;
  fs.writeFileSync(CONFIG_PATH, JSON.stringify(saved, null, 2) + "\n");
}
