// One WebSocket to the PC, reconnecting by itself. Presses go out immediately; every
// message from the PC lands in S.
import { S, local, ticker, toast } from "./state.svelte";
import { haptic } from "./haptics";
import { fx } from "./fx";

const q = new URLSearchParams(location.search);
export const GUEST = (location.pathname.match(/^\/g\/([a-f0-9]{20,64})/) || [])[1] || "";
const urlCode = (q.get("t") || "").trim().toLowerCase();
export const device = q.get("device") || (Math.min(screen.width, screen.height) < 500 ? "phone" : "tablet");
export const nativeApp = q.get("native") || ((window as any).TabletDeckNative ? "android" : "");
const did = q.get("did") || "";

let token = GUEST ? "" : urlCode && urlCode !== local.get("pairCode") ? urlCode : local.get("token") || urlCode || "";
export const auth = () => (GUEST ? `g=${GUEST}` : `t=${encodeURIComponent(token)}`);

let ws: WebSocket | null = null;
let backoff = 400;
let lastPong = 0;

export function send(obj: Record<string, unknown>) {
  if (ws && ws.readyState === 1) ws.send(JSON.stringify(obj));
}

// ---------- holds ----------
// Two fingers on one control send one down and one up; a lost link forgets them all
// (the PC lets go of everything a deck held when it drops).
const downs = new Map<string, number>();
export function down(id: string) {
  const n = (downs.get(id) || 0) + 1;
  downs.set(id, n);
  if (n === 1) send({ t: "down", id });
}
export function up(id: string) {
  const n = (downs.get(id) || 0) - 1;
  if (n > 0) { downs.set(id, n); return; }
  downs.delete(id);
  send({ t: "up", id });
}
export function tap(id: string) { send({ t: "tap", id }); }

// ---------- connecting ----------

export function start() {
  if (!GUEST && !token) { S.mode = "pair"; return; }
  // Check the code (or token) first, so a wrong one shows pairing with a reason.
  fetch(GUEST ? `/layout.json?${auth()}` : `/api/v1/views?${auth()}`, { cache: "no-store" })
    .then((r) => {
      if (r.ok) {
        if (!GUEST && token !== local.get("token")) { local.set("token", token); local.set("pairCode", ""); }
        S.mode = GUEST ? "guest" : "deck";
        connect();
        return;
      }
      if (GUEST) { S.mode = "gone"; S.gone = r.status === 410 ? "This share link ran out." : "This share link isn't valid any more."; return; }
      S.mode = "pair";
      S.pairError = r.status === 429 ? "Too many wrong codes. Wait a minute." : token ? "That code didn't match the one on your PC." : "";
    })
    .catch(() => setTimeout(start, 1500));
}

export function pairWith(code: string) {
  token = code.trim().toLowerCase();
  local.set("pairCode", token);
  S.pairError = "";
  start();
}

function connect() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const extra = new URLSearchParams({ device });
  if (nativeApp) extra.set("native", nativeApp);
  if (did) extra.set("did", did);
  if (q.get("name")) extra.set("name", q.get("name")!);
  const sock = new WebSocket(`${proto}://${location.host}/${GUEST ? "gws" : "ws"}?${auth()}&${extra}`);
  ws = sock;
  sock.onopen = () => {
    backoff = 400;
    lastPong = Date.now();
    S.online = true;
    ping();
    const info = (window as any).TabletDeckNative?.info?.();
    if (info) send({ t: "hello-native", text: info });
  };
  sock.onmessage = (e) => {
    let m: any;
    try { m = JSON.parse(e.data); } catch { return; }
    handle(m);
  };
  sock.onclose = () => {
    if (sock !== ws) return;
    S.online = false;
    S.listening = false;
    downs.clear();
    if ((sock as any).dead) return;
    setTimeout(connect, backoff);
    backoff = Math.min(backoff * 1.6, 6000);
  };
}

function ping() { send({ t: "ping", n: performance.now() }); }
setInterval(() => {
  ping();
  if (ws && ws.readyState === 1 && Date.now() - lastPong > 6000) ws.close();
}, 1500);

// Coming back from the background: reconnect at once instead of waiting out a backoff.
document.addEventListener("visibilitychange", () => {
  if (!document.hidden && S.mode !== "pair" && S.mode !== "gone" && (!ws || ws.readyState > 1)) { backoff = 0; connect(); }
});

export function stop() {
  if (ws) { (ws as any).dead = true; ws.close(); ws = null; }
}

const BLOCKED: Record<string, string> = {
  "not-running": "Game not running", "not-focused": "Game not in front", "focus-failed": "Game won't come forward",
  denied: "Not shared", "not-afk": "Only while AFK", "rate-limited": "Slow down", expired: "Link ran out", unbound: "Not bound · bind it in Settings",
};

function handle(m: any) {
  switch (m.t) {
    case "pong": lastPong = Date.now(); S.latency = Math.round(performance.now() - m.n); break;
    case "hello":
      S.name = m.name || ""; S.owner = m.name || ""; S.lit = m.states || {}; S.game = m.game || S.game; S.afk = !!m.afk;
      S.listening = false;
      if (m.guest) { S.guest = m.guest; S.groups = m.guest.groups || []; S.mode = "guest"; break; }
      S.views = m.views || []; S.crew = m.crew || []; S.requests = m.requests || [];
      S.voice = m.voiceStatus || S.voice; S.chat = m.chat || []; S.live = !!m.live; S.agent = m.agent || {};
      S.member = !!m.cockpit; S.battle = m.battle || null;
      if (m.cockpitTheme) S.theme = m.cockpitTheme;
      if (!S.member) send({ t: "view", id: local.get(`view-${device}`) || "flight" });
      break;
    case "paired": token = m.token; local.set("token", token); local.set("pairCode", ""); break;
    case "view":
      S.view = m.view; S.groups = m.groups || []; S.cockpit = m.cockpit || null; S.member = !!m.cockpit;
      if (!m.view.id.startsWith("agent-")) local.set(`view-${device}`, m.view.id);
      break;
    case "views": S.views = m.views || []; break;
    case "state": S.lit[m.group] = m.on; break;
    case "states": S.lit = m.states || {}; break;
    case "game": S.game = m; break;
    case "afk": S.afk = m.on; break;
    case "crew": S.crew = m.crew || []; break;
    case "request": S.requests = [...S.requests, m.request]; haptic("heavy"); break;
    case "request-gone": S.requests = S.requests.filter((r) => r.id !== m.id); break;
    case "answered": ticker(m.result === "done" ? "Done" : "Declined", m.label || m.text || ""); break;
    case "pressed": if (m.kind !== "deck") { ticker(m.who || m.kind, m.label || ""); fx.flashControl(m.id); } break;
    case "agent": ticker("Agent", String(m.tool || "").replace(/_/g, " ")); break;
    case "blocked":
      toast(BLOCKED[m.reason] || "Not pressed");
      haptic("error");
      S.blocked = { ...S.blocked, [m.id]: Date.now() };
      break;
    case "released":
      downs.delete(m.id);
      if (m.reason === "focus-lost") toast("Released · game lost focus");
      break;
    case "theme": S.theme = m.id || S.theme; break;
    case "voice-status": S.voice = m; break;
    case "listening": S.listening = !!m.on; break;
    case "voice-live": S.live = !!m.on; break;
    case "voice-off": S.listening = false; toast(m.reason || "Voice is off"); break;
    case "heard": ticker("Heard", m.text || m.label || ""); break;
    case "unheard": ticker("Voice", "didn't catch that"); break;
    case "chat": S.chat = [...S.chat, m.line].slice(-40); if (m.line.who === "agent") ticker(S.agent.name || "Agent", m.line.text); break;
    case "chat-clear": S.chat = []; break;
    case "agent-busy": S.busy = !!m.on; break;
    case "agent-info": S.agent = m.agent || {}; break;
    case "battle": S.battle = m.state || null; if (m.event === "hit") { haptic("heavy"); fx.hit(); } else if (/ship-|dead/.test(m.event)) haptic("long"); break;
    case "cockpit-lost": S.lost = [...S.lost.filter((l) => l.console !== m.console), m]; haptic("long"); break;
    case "cockpit-back": S.lost = S.lost.filter((l) => l.console !== m.console); ticker("Cockpit", `${m.name} is back`); break;
    case "identify": S.identify = m.name; setTimeout(() => { if (S.identify === m.name) S.identify = ""; }, 2600); break;
    case "toast": toast(m.text); break;
    case "reload": send({ t: "view", id: S.view ? S.view.id : "flight", mode: S.member ? "init" : "" }); break;
    case "unpaired": local.set("token", ""); token = ""; stop(); S.mode = "pair"; S.pairError = "This device was unpaired on the PC."; break;
    case "revoked": stop(); S.mode = "gone"; S.gone = m.reason === "expired" ? "This share link ran out." : "This share link was turned off."; break;
  }
}

// The PC's REST API, with this deck's token.
export async function api<T = any>(path: string, opts: RequestInit = {}): Promise<T> {
  const sep = path.includes("?") ? "&" : "?";
  const r = await fetch(`/api/v1/${path}${sep}${auth()}`, { ...opts, headers: { "Content-Type": "application/json", ...(opts.headers || {}) } });
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error((b && b.error) || r.statusText);
  return b as T;
}
