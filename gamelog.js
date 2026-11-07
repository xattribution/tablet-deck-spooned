// Battle state from Star Citizen's Game.log, the text log the game writes next to its
// install for support and which other programs may read. It's read a few times a second
// while it grows. What it can tell:
//   - the ship you fly;
//   - your ship being hit, and by whom;
//   - your ship disabled or destroyed;
//   - you dying, and the players and NPCs you killed.
// It can't tell shield or hull levels, component damage or ammo: the game doesn't write
// those anywhere a program may read. The line formats are the game's and change between
// patches; a line that no longer matches is skipped.
//
// The server requires this (and the installer build inlines it, scripts/build.js), so it
// uses Node's built-in modules only.
"use strict";
const fs = require("fs");

const RE = {
  nick: /nickname="([^"]+)"/,
  handle: /Handle\[([^\]\s]+)\]/,
  driver: /<Vehicle Control Flow>.*Local client node \[\d+\] (requesting|granted|releasing) control token for '([^']+)'/,
  hit: /<Debug Hostility Events> \[OnHandleHit\] (Fake hit|Hit) FROM (\S+) TO (\S+)\. Being sent to child (\S+)/,
  destroy: /<Vehicle Destruction> .*Vehicle '([^']+)' .*advanced from destroy level (\d) to (\d) caused by '([^']*)'/,
  death: /<Actor Death> CActor::Kill: '([^']+)' .*killed by '([^']*)'/,
  entityNo: /_\d{6,}$/,
};

const POLL_MS = 250;
const FIND_MS = 30000;       // look for the log again this often while there's none
const AGAIN_MS = 1000;       // ... or this often while the one followed is missing
const FIGHT_MS = 8000;       // no hits for this long: the fight is over
const RECENT = 5;            // hits kept for the deck's "Last hits"
const BIG = 4 << 20;         // joining a log this big: read its start, then only its end
const HEAD = 1 << 20;
const TAIL = 256 << 10;
const CHUNK = 1 << 20;       // read at most this much per poll

// "PU_Pilots-Human-Criminal-Pilot_Light_6929783507558" -> "Criminal pilot",
// "ORIG_m80_200000777639" -> "ORIG m80". Names are what the game wrote, made readable.
function shortEntity(entity) {
  const e = String(entity || "").replace(RE.entityNo, "");
  const low = e.toLowerCase();
  if (low.includes("pirate")) return "Pirate";
  if (low.includes("criminal")) return "Criminal pilot";
  if (low.includes("security") || low.includes("advocacy")) return "Security";
  if (low.startsWith("pu_") || low.startsWith("npc_")) return "NPC";
  return e.replace(/_/g, " ").slice(0, 40);
}

const fresh = () => ({ ship: "", fire: false, from: "", hits: 0, level: 0, dead: false, by: "", kills: 0, killed: "", recent: [] });

// create({ files, onChange, log }): files() lists the logs to follow, [{ path, running }],
// best first, and again() looks for them anew; onChange(event, state) hears "ship", "hit", "clear", "ship-1" (disabled),
// "ship-2" (destroyed), "dead", "kill", "reset" (a new game session) and "sync" (the log
// found and read up to now). state() is what the decks show.
function create({ files, onChange = () => {}, log = () => {} }) {
  let nick = "";
  let shipEntity = "";
  let st = fresh();
  let fightTimer = null;
  let lastHit = 0;
  // Reading what the log already held when SpoolUp started: the name, the ship and the
  // kills count, quietly; old hits aren't news.
  let catchingUp = false;

  // `now` lets a deck whose clock differs say how long ago a hit was; `reading`: a log
  // was found.
  const snapshot = () => ({ ...st, recent: st.recent.map((h) => ({ ...h })), now: Date.now(), reading: Boolean(file) });
  const tell = (event) => { try { onChange(event, snapshot()); } catch (err) { log(`Game.log: ${err.message}`); } };
  // Hits come many a second in a fight: the ones read in one go are told once, and before
  // whatever came after them.
  let hitWaiting = false;
  const flushHits = () => { if (hitWaiting) { hitWaiting = false; tell("hit"); } };
  const changed = (event) => {
    if (catchingUp) return;
    if (event === "hit") { hitWaiting = true; return; }
    flushHits();
    tell(event);
  };

  function reset() {
    clearTimeout(fightTimer);
    fightTimer = null;
    shipEntity = "";
    st = fresh();
    changed("reset");
  }

  function line(text) {
    if (text.includes("nickname=") || (!nick && text.includes("Handle["))) {
      const m = RE.nick.exec(text) || RE.handle.exec(text);
      if (m) nick = m[1];
      return;
    }
    if (text.includes("<Vehicle Control Flow>")) {
      const m = RE.driver.exec(text);
      if (!m) return;
      if (m[1] === "releasing") {
        if (shipEntity !== m[2]) return;
        shipEntity = ""; st.ship = "";
      } else {
        if (shipEntity === m[2]) return;
        shipEntity = m[2]; st.ship = shortEntity(m[2]);
        st.level = 0; st.dead = false; st.by = "";
      }
      changed("ship");
      return;
    }
    if (text.includes("<Debug Hostility Events>")) {
      const m = RE.hit.exec(text);
      if (!m || !nick || m[4] !== nick) return;
      // A hit on a ship you're in says which ship it is, even without a control token line.
      if (m[3] !== shipEntity) { shipEntity = m[3]; st.ship = shortEntity(m[3]); }
      if (catchingUp) return;
      if (!st.fire) st.hits = 0;
      const from = shortEntity(m[2]);
      lastHit = Date.now();
      st.fire = true; st.from = from; st.hits++;
      st.dead = false; st.by = ""; // hit again: alive again
      const top = st.recent[0];
      if (top && top.from === from && lastHit - top.at < FIGHT_MS) { top.n++; top.at = lastHit; }
      else st.recent = [{ from, n: 1, at: lastHit }, ...st.recent].slice(0, RECENT);
      clearTimeout(fightTimer);
      fightTimer = setTimeout(() => {
        fightTimer = null;
        if (Date.now() - lastHit < FIGHT_MS - 50) return;
        st.fire = false;
        changed("clear");
      }, FIGHT_MS);
      if (fightTimer.unref) fightTimer.unref();
      changed("hit");
      return;
    }
    if (text.includes("<Vehicle Destruction>")) {
      const m = RE.destroy.exec(text);
      if (!m || !shipEntity || m[1] !== shipEntity) return;
      const level = Number(m[3]);
      if (level < 1 || level > 2 || level === st.level) return;
      st.level = level;
      changed(`ship-${level}`);
      return;
    }
    if (text.includes("<Actor Death>")) {
      const m = RE.death.exec(text);
      if (!m || !nick) return;
      if (m[1] === nick) {
        clearTimeout(fightTimer); fightTimer = null;
        st.dead = true; st.fire = false; st.by = m[2] === nick ? "" : shortEntity(m[2]);
        changed("dead");
      } else if (m[2] === nick) {
        st.kills++; st.killed = shortEntity(m[1]);
        changed("kill");
      }
    }
  }

  // Following the file: opened, read and closed at each poll that finds it grown, so the
  // game is never kept from rotating it, and a partial line waits for the rest.
  let file = "";          // the log followed, "" while it's missing
  let lastPath = "";      // the last log followed, kept while it's missing
  let id = "";            // its identity, so a new session's log is noticed
  let off = 0;            // bytes used
  let lastFind = 0;
  let timer = null;

  function pick() {
    lastFind = Date.now();
    let list = [];
    try { list = files() || []; } catch { list = []; }
    const there = list.filter((f) => f && f.path && isFile(f.path));
    const best = there.find((f) => f.running) || there[0];
    return best ? best.path : "";
  }

  function readRange(fd, from, length) {
    const buf = Buffer.alloc(length);
    const n = fs.readSync(fd, buf, 0, length, from);
    return buf.subarray(0, n);
  }

  // Feeds whole lines from buf; returns the bytes used.
  function feed(buf) {
    let used = 0;
    for (;;) {
      const nl = buf.indexOf(10, used);
      if (nl < 0) return used;
      line(buf.toString("utf8", used, nl));
      used = nl + 1;
    }
  }

  function poll() {
    // Which log: looked up again now and then, so starting the PTU follows its log.
    // A log that just went away (the game moves the old one aside as it starts) is looked
    // for again soon; it comes back as a new game session.
    if (!file || Date.now() - lastFind > FIND_MS) {
      if (!file && lastFind && Date.now() - lastFind < (lastPath ? AGAIN_MS : FIND_MS)) return;
      const next = pick();
      if (next && next !== lastPath) {
        // Another channel's log: what's shown was about the old one; read the new one quietly.
        if (lastPath) reset();
        lastPath = next; id = ""; off = 0;
        log(`Reading battle events from ${next}`);
      }
      file = next;
      if (!file) return;
    }
    let s;
    try { s = fs.statSync(file); } catch { file = ""; return; }
    if (id && `${s.birthtimeMs}:${s.ino}` === id && s.size === off) return;
    let fd;
    try { fd = fs.openSync(file, "r"); } catch { return; }
    try {
      const sid = `${s.birthtimeMs}:${s.ino}`;
      if (sid !== id || s.size < off) {
        // The first look reads what's there quietly; a log that's new or started over
        // after that is a new game session, read as it happens.
        catchingUp = id === "";
        if (!catchingUp) reset();
        id = sid; off = 0;
        if (s.size > BIG) {
          // A long session already running: its start has the player's name, its end the
          // ship they're in. What happened in between is old news.
          feed(readRange(fd, 0, HEAD));
          const tail = readRange(fd, s.size - TAIL, TAIL);
          const nl = tail.indexOf(10);
          off = s.size - TAIL + (nl < 0 ? tail.length : nl + 1);
        }
      }
      while (off < s.size) {
        const buf = readRange(fd, off, Math.min(CHUNK, s.size - off));
        if (!buf.length) break;
        const used = feed(buf);
        if (!used) {
          if (buf.length < CHUNK) break; // a partial line: wait for the rest
          off += buf.length;             // a line longer than a chunk: skip it
          continue;
        }
        off += used;
      }
    } catch (err) {
      log(`Game.log: ${err.message}`);
    } finally {
      fs.closeSync(fd);
      if (catchingUp) { catchingUp = false; hitWaiting = false; changed("sync"); }
      flushHits();
    }
  }

  return {
    start() { if (!timer) { timer = setInterval(poll, POLL_MS); if (timer.unref) timer.unref(); poll(); } },
    stop() { clearInterval(timer); timer = null; clearTimeout(fightTimer); fightTimer = null; },
    // Looks for the log again at the next poll, e.g. after the player picks another folder.
    again() { lastFind = 0; file = ""; },
    state: snapshot,
    line,
  };
}

function isFile(p) { try { return fs.statSync(p).isFile(); } catch { return false; } }

module.exports = { create, shortEntity };
