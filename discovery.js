// Lets the SpoolUp apps (Android, iOS) find this PC on the home network by themselves:
// a small DNS-SD responder over multicast DNS (RFC 6762 and 6763), the way printers and
// speakers are found. It answers questions for the service type _spoolup._tcp with this
// PC's name, the deck's port and its address, and announces itself when it starts.
//
// mDNS packets go to 224.0.0.251 on the PC's network adapters (the server leaves out VPN
// and virtual ones). Questions from addresses outside those networks get no answer. The
// records carry the PC's name and SpoolUp's version; an app that finds the PC then pairs
// with the code (typed, or from the QR code) like any browser.
//
//   const advertiser = discovery.create({ port, version, name, adapters, log });
//   advertiser.start(); ... advertiser.stop(); // stop() says goodbye, so apps drop it at once
//
// Node built-ins only. Records (TTLs as RFC 6762 suggests):
//   _spoolup._tcp.local        PTR  <name>._spoolup._tcp.local
//   <name>._spoolup._tcp.local SRV  0 0 <port> spoolup-<host>.local
//   <name>._spoolup._tcp.local TXT  "v=<version>" "name=<name>"
//   spoolup-<host>.local       A    the address on the adapter the question came in on
// The host name has its own "spoolup-" label so it never collides with the <host>.local
// that Windows itself answers for.
"use strict";

const dgram = require("dgram");
const os = require("os");

const GROUP = "224.0.0.251";
const MDNS_PORT = 5353;
const SERVICE = ["_spoolup", "_tcp", "local"];
const META = ["_services", "_dns-sd", "_udp", "local"]; // "which services are here?"
const TYPE = { A: 1, PTR: 12, TXT: 16, SRV: 33, ANY: 255 };
const IN = 1, FLUSH = 0x8000, QU = 0x8000;
const TTL_HOST = 120, TTL_OTHER = 4500, TTL_LEGACY = 10;
const MAX_PACKET = 9000;
const RESPONSE_GAP_MS = 1000;   // at most one multicast answer per question and adapter a second
const RESPONSES_PER_SECOND = 40; // and a cap across everything, so a flood can't make it chatty
const ADAPTER_CHECK_MS = 30000; // new Wi-Fi or a cable plugged in: join that network too

// ---------- names ----------

const sameName = (a, b) => a.length === b.length && a.every((l, i) => l.toLowerCase() === b[i].toLowerCase());

// A DNS label holds up to 63 bytes; cut on a character boundary.
function label(text, fallback) {
  let s = String(text || "").replace(/[\x00-\x1f\x7f]/g, "").trim() || fallback;
  while (Buffer.byteLength(s) > 63) s = s.slice(0, -1);
  return s;
}
const hostLabel = (host) => label("spoolup-" + String(host || "").toLowerCase().replace(/[^a-z0-9-]+/g, "-").replace(/^-+|-+$/g, ""), "spoolup").replace(/-+$/, "");

// ---------- reading questions ----------

// A name at off, following compression pointers (with a guard against loops).
function readName(buf, off) {
  const labels = [];
  let end = -1, jumps = 0;
  for (;;) {
    if (off >= buf.length) throw new Error("name past the end");
    const len = buf[off];
    if (len === 0) { off++; break; }
    if ((len & 0xc0) === 0xc0) {
      if (off + 1 >= buf.length || ++jumps > 20) throw new Error("bad pointer");
      if (end < 0) end = off + 2;
      off = ((len & 0x3f) << 8) | buf[off + 1];
      continue;
    }
    if (len > 63 || off + 1 + len > buf.length) throw new Error("bad label");
    labels.push(buf.toString("utf8", off + 1, off + 1 + len));
    off += 1 + len;
    if (labels.length > 128) throw new Error("name too long");
  }
  return { labels, next: end >= 0 ? end : off };
}

// The questions in a query, plus the PTR answers it already knows (known-answer
// suppression: an app that already has our PTR with plenty of TTL left needn't hear it).
// Null for responses and anything malformed.
function parseQuery(buf) {
  if (buf.length < 12 || buf.length > MAX_PACKET) return null;
  const flags = buf.readUInt16BE(2);
  if (flags & 0x8000) return null;         // a response, maybe our own
  if ((flags >> 11) & 0xf) return null;    // opcode other than QUERY
  const qd = buf.readUInt16BE(4), an = buf.readUInt16BE(6);
  if (qd === 0 || qd > 64) return null;
  try {
    let off = 12;
    const questions = [];
    for (let i = 0; i < qd; i++) {
      const n = readName(buf, off);
      off = n.next;
      if (off + 4 > buf.length) return null;
      const cls = buf.readUInt16BE(off + 2);
      questions.push({ name: n.labels, type: buf.readUInt16BE(off), unicast: Boolean(cls & QU), cls: cls & ~QU });
      off += 4;
    }
    const known = [];
    for (let i = 0; i < Math.min(an, 64); i++) {
      const n = readName(buf, off);
      off = n.next;
      if (off + 10 > buf.length) break;
      const type = buf.readUInt16BE(off), ttl = buf.readUInt32BE(off + 4), len = buf.readUInt16BE(off + 8);
      off += 10;
      if (off + len > buf.length) break;
      if (type === TYPE.PTR) known.push({ name: n.labels, ttl, target: readName(buf, off).labels });
      off += len;
    }
    return { id: buf.readUInt16BE(0), questions, known };
  } catch {
    return null;
  }
}

// ---------- writing answers ----------

function nameBytes(labels) {
  const parts = [];
  for (const l of labels) { const b = Buffer.from(l, "utf8"); parts.push(Buffer.from([b.length]), b); }
  parts.push(Buffer.from([0]));
  return Buffer.concat(parts);
}

function record({ name, type, flush, ttl, data }) {
  const head = Buffer.alloc(10);
  head.writeUInt16BE(type, 0);
  head.writeUInt16BE(IN | (flush ? FLUSH : 0), 2);
  head.writeUInt32BE(ttl, 4);
  head.writeUInt16BE(data.length, 8);
  return Buffer.concat([nameBytes(name), head, data]);
}

// A response: answers and additional records; legacy (unicast, from a port other than
// 5353) ones echo the query's id and questions and use short TTLs without cache-flush.
function packet(answers, additionals, legacy) {
  const head = Buffer.alloc(12);
  head.writeUInt16BE(legacy ? legacy.id : 0, 0);
  head.writeUInt16BE(0x8400, 2); // response, authoritative
  const parts = [head];
  if (legacy) {
    head.writeUInt16BE(legacy.questions.length, 4);
    for (const q of legacy.questions) {
      const t = Buffer.alloc(4);
      t.writeUInt16BE(q.type, 0);
      t.writeUInt16BE(IN, 2);
      parts.push(nameBytes(q.name), t);
    }
  }
  const fix = (r) => (legacy ? { ...r, flush: false, ttl: Math.min(r.ttl, TTL_LEGACY) } : r);
  head.writeUInt16BE(answers.length, 6);
  head.writeUInt16BE(additionals.length, 10);
  for (const r of answers) parts.push(record(fix(r)));
  for (const r of additionals) parts.push(record(fix(r)));
  return Buffer.concat(parts);
}

function txtData(pairs) {
  const parts = [];
  for (const p of pairs) {
    let b = Buffer.from(p, "utf8");
    if (b.length > 255) b = b.subarray(0, 255);
    parts.push(Buffer.from([b.length]), b);
  }
  return Buffer.concat(parts);
}

// ---------- adapters ----------

const toInt = (ip) => ip.split(".").reduce((n, p) => ((n << 8) | (+p & 255)) >>> 0, 0);

// IPv4 adapters to answer on: real, non-internal, with an address that isn't link-local,
// and accepted by the caller's filter (the server leaves out VPNs and virtual adapters).
function listAdapters(accept) {
  const list = [];
  for (const [name, infos] of Object.entries(os.networkInterfaces())) {
    for (const i of infos || []) {
      if (i.family !== "IPv4" && i.family !== 4) continue;
      if (i.internal || i.address.startsWith("169.254.")) continue;
      if (accept && !accept(name, i.address)) continue;
      list.push({ name, ip: i.address, net: toInt(i.address) & toInt(i.netmask), mask: toInt(i.netmask) });
    }
  }
  return list;
}

// ---------- the responder ----------

function create(options) {
  const port = options.port;
  const log = options.log || (() => {});
  const instance = [label(options.name, "SpoolUp PC"), ...SERVICE];
  const host = [hostLabel(options.host || os.hostname()), "local"];
  const txt = txtData([`v=${options.version || ""}`, `name=${label(options.name, "SpoolUp PC")}`]);

  let socket = null;
  let adapters = [];
  let joined = new Set();
  let timer = null;
  let announceTimer = null;
  const lastSent = new Map(); // "adapter|question" -> time
  let budget = { second: 0, used: 0 };
  const queue = []; // multicast sends, one at a time: each picks its adapter first
  let sending = false;

  const srvData = () => {
    const t = nameBytes(host);
    const b = Buffer.alloc(6);
    b.writeUInt16BE(0, 0); // priority
    b.writeUInt16BE(0, 2); // weight
    b.writeUInt16BE(port, 4);
    return Buffer.concat([b, t]);
  };
  const aData = (ip) => Buffer.from(ip.split(".").map(Number));

  const records = (adapter, ttlScale = 1) => ({
    ptr: { name: SERVICE, type: TYPE.PTR, flush: false, ttl: TTL_OTHER * ttlScale, data: nameBytes(instance) },
    meta: { name: META, type: TYPE.PTR, flush: false, ttl: TTL_OTHER * ttlScale, data: nameBytes(SERVICE) },
    srv: { name: instance, type: TYPE.SRV, flush: true, ttl: TTL_HOST * ttlScale, data: srvData() },
    txt: { name: instance, type: TYPE.TXT, flush: true, ttl: TTL_OTHER * ttlScale, data: txt },
    a: { name: host, type: TYPE.A, flush: true, ttl: TTL_HOST * ttlScale, data: aData(adapter.ip) },
  });

  const adapterFor = (ip) => {
    if (!/^\d+\.\d+\.\d+\.\d+$/.test(ip)) return null;
    const n = toInt(ip);
    return adapters.find((a) => (n & a.mask) === a.net) || null;
  };

  function spend() {
    const s = Math.floor(Date.now() / 1000);
    if (budget.second !== s) budget = { second: s, used: 0 };
    return ++budget.used <= RESPONSES_PER_SECOND;
  }

  function pump() {
    if (sending || !queue.length) return;
    const { sock, adapter, buf } = queue.shift();
    sending = true;
    try {
      sock.setMulticastInterface(adapter.ip);
      sock.send(buf, MDNS_PORT, GROUP, () => { sending = false; pump(); });
    } catch {
      sending = false; // closed meanwhile, or the adapter went away
      pump();
    }
  }
  const multicast = (adapter, buf, sock = socket) => { if (sock) { queue.push({ sock, adapter, buf }); pump(); } };

  // The answers to one query, for the adapter it came in on.
  function answer(query, adapter) {
    const r = records(adapter);
    const answers = [], extra = [];
    const add = (list, rec) => { if (!list.includes(rec)) list.push(rec); };
    const keys = [];
    for (const q of query.questions) {
      if (q.cls !== IN && q.cls !== 255) continue;
      const any = q.type === TYPE.ANY;
      if (sameName(q.name, SERVICE) && (any || q.type === TYPE.PTR)) {
        const known = query.known.some((k) => sameName(k.name, SERVICE) && sameName(k.target, instance) && k.ttl > TTL_OTHER / 2);
        if (known) continue;
        add(answers, r.ptr); add(extra, r.srv); add(extra, r.txt); add(extra, r.a);
      } else if (sameName(q.name, META) && (any || q.type === TYPE.PTR)) {
        add(answers, r.meta);
      } else if (sameName(q.name, instance)) {
        if (any || q.type === TYPE.SRV) { add(answers, r.srv); add(extra, r.a); }
        if (any || q.type === TYPE.TXT) add(answers, r.txt);
      } else if (sameName(q.name, host) && (any || q.type === TYPE.A)) {
        add(answers, r.a);
      } else continue;
      keys.push(`${q.name.join(".").toLowerCase()}|${q.type}|${q.unicast ? "u" : "m"}`);
    }
    // Something answered as an answer needn't repeat as an additional record.
    return { answers, extra: extra.filter((x) => !answers.includes(x)), keys };
  }

  function onMessage(buf, rinfo) {
    if (!socket) return;
    const query = parseQuery(buf);
    if (!query) return;
    // Only questions from this PC's own networks (RFC 6762 section 11).
    const adapter = adapterFor(rinfo.address);
    if (!adapter) return;
    const { answers, extra, keys } = answer(query, adapter);
    if (!answers.length) return;
    // A one-shot ("legacy") query from another port gets a direct, normal-DNS-style answer.
    if (rinfo.port !== MDNS_PORT) {
      if (spend()) socket.send(packet(answers, extra, query), rinfo.port, rinfo.address);
      return;
    }
    const now = Date.now();
    const fresh = keys.some((k) => now - (lastSent.get(`${adapter.ip}|${k}`) || 0) >= RESPONSE_GAP_MS);
    if (!fresh || !spend()) return;
    for (const k of keys) lastSent.set(`${adapter.ip}|${k}`, now);
    if (lastSent.size > 500) for (const [k, t] of lastSent) if (now - t > 10 * RESPONSE_GAP_MS) lastSent.delete(k);
    const reply = packet(answers, extra, null);
    // Questions asking for a unicast reply get one (the first question after joining a network).
    if (query.questions.every((q) => q.unicast)) socket.send(reply, MDNS_PORT, rinfo.address);
    else multicast(adapter, reply);
  }

  // Unsolicited announcements of every record, on the given adapters (TTL 0 says goodbye).
  function announce(list, goodbye, sock = socket) {
    for (const adapter of list) {
      const r = records(adapter, goodbye ? 0 : 1);
      multicast(adapter, packet([r.ptr, r.srv, r.txt, r.a], [], null), sock);
    }
  }

  function refreshAdapters() {
    if (!socket) return;
    adapters = listAdapters(options.adapters);
    const now = new Set(adapters.map((a) => a.ip));
    const added = adapters.filter((a) => !joined.has(a.ip));
    for (const ip of joined) if (!now.has(ip)) { try { socket.dropMembership(GROUP, ip); } catch { /* gone with its adapter */ } }
    for (const a of added) {
      try { socket.addMembership(GROUP, a.ip); } catch (err) { log(`Discovery: can't listen on ${a.name}: ${err.message}`); }
    }
    joined = now;
    // RFC 6762 asks for at least two announcements, a second apart.
    if (added.length) {
      announce(added, false);
      clearTimeout(announceTimer);
      announceTimer = setTimeout(() => announce(added.filter((a) => joined.has(a.ip)), false), 1000);
      announceTimer.unref?.();
    }
  }

  return {
    start() {
      if (socket) return;
      const s = dgram.createSocket({ type: "udp4", reuseAddr: true });
      socket = s;
      s.on("message", onMessage);
      s.on("error", (err) => {
        log(`Discovery is off: ${err.code === "EADDRINUSE" || err.code === "EACCES" ? "UDP port 5353 is taken" : err.message}. Apps can still connect by address or QR code.`);
        if (socket === s) { socket = null; clearInterval(timer); clearTimeout(announceTimer); }
        try { s.close(); } catch { /* already closed */ }
      });
      s.bind(MDNS_PORT, () => {
        if (socket !== s) return;
        try {
          s.setMulticastTTL(255);
          s.setMulticastLoopback(true);
        } catch { /* defaults work too */ }
        joined = new Set();
        refreshAdapters();
        timer = setInterval(refreshAdapters, ADAPTER_CHECK_MS);
        timer.unref?.();
      });
      s.unref?.();
    },
    stop() {
      const s = socket;
      if (!s) return;
      socket = null;
      clearInterval(timer);
      clearTimeout(announceTimer);
      announce(adapters, true, s);
      // Close once the goodbyes are out (or after a moment if one stalls).
      let closed = false;
      const close = () => {
        if (closed) return;
        closed = true;
        clearInterval(wait);
        for (let i = queue.length - 1; i >= 0; i--) if (queue[i].sock === s) queue.splice(i, 1);
        try { s.close(); } catch { /* closed */ }
      };
      const wait = setInterval(() => { if (!queue.some((q) => q.sock === s) && !sending) close(); }, 20);
      wait.unref?.();
      setTimeout(close, 500).unref?.();
    },
    get running() { return Boolean(socket); },
  };
}

module.exports = { create, SERVICE_TYPE: "_spoolup._tcp", parseQuery, packet };
