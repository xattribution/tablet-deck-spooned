// The PC's control panel: status, pairing, voice, devices and crew, keybinds,
// agents and settings. Only this PC can open it; it talks to the local API with the
// key in its address.
(() => {
  "use strict";
  const $ = (s) => document.querySelector(s);
  const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const key = new URLSearchParams(location.search).get("k") || "";
  const api = (path, opts = {}) => fetch(`/api/v1/${path}`, { ...opts, headers: { "X-Deck-Key": key, "Content-Type": "application/json", ...(opts.headers || {}) } })
    .then(async (r) => { const b = await r.json().catch(() => ({})); if (!r.ok) throw new Error(b.error || r.statusText); return b; });
  const put = (path, body) => api(path, { method: "PUT", body: JSON.stringify(body) });
  const post = (path, body) => api(path, { method: "POST", body: JSON.stringify(body || {}) });
  let toastT;
  const toast = (t) => { const el = $("#toast"); el.textContent = t; el.classList.add("show"); clearTimeout(toastT); toastT = setTimeout(() => el.classList.remove("show"), 2200); };
  const ago = (t) => { const s = Math.round((Date.now() - new Date(t)) / 1000); return s < 60 ? "NOW" : s < 3600 ? `${Math.round(s / 60)} MIN` : s < 86400 ? `${Math.round(s / 3600)} H` : `${Math.round(s / 86400)} D`; };

  let status = null, cfg = null, voice = null;

  function drawQR(canvas, text) {
    const rows = window.DeckQR.matrix(text), n = rows.length + 8, scale = Math.max(3, Math.floor(160 / n));
    canvas.width = canvas.height = n * scale;
    const g = canvas.getContext("2d");
    g.fillStyle = "#fff"; g.fillRect(0, 0, canvas.width, canvas.height); g.fillStyle = "#000";
    rows.forEach((row, y) => row.forEach((d, x) => { if (d) g.fillRect((x + 4) * scale, (y + 4) * scale, scale, scale); }));
  }

  function paintStatus() {
    const s = status;
    $("#ver").innerHTML = `<span>VERSION</span><span>${esc(s.version)}${s.dryRun ? " · DRY RUN" : ""}</span>`;
    const g = s.game || {};
    const decks = (s.clients || []).filter((c) => c.kind === "owner").length;
    const crew = (s.clients || []).length - decks;
    const v = s.voice || {};
    $("#readouts").innerHTML = [
      ["Game", g.running ? (g.focused ? "In front" : "Behind") : "Not running", g.running ? (s.keybinds.files || []).length + " KEYBIND FILES" : "START STAR CITIZEN", g.running ? (g.focused ? "good" : "caution") : "bad"],
      ["Decks", `${decks} linked`, crew ? `${crew} CREW · GUESTS` : "NO CREW CONNECTED", decks ? "good" : ""],
      ["Presses", Number(s.presses).toLocaleString(), `${s.blocked} BLOCKED`, ""],
      ["Voice", v.state === "ready" ? "Listening" : v.state || "off", v.reason ? String(v.reason).slice(0, 60) : (voice && voice.recognizer) || "", v.state === "ready" ? "good" : v.state === "unavailable" ? "bad" : ""],
    ].map(([l, val, sub, dot]) => `<div><span class="lbl">${l}</span><span class="val">${dot ? `<i class="dot ${dot}"></i>` : ""}${esc(val)}</span><span class="sub">${esc(sub)}</span></div>`).join("");
    if (s.pairing) {
      $("#code").textContent = s.pairing.code;
      const sel = $("#addr");
      const want = sel.value;
      sel.innerHTML = (s.pairing.addresses || []).map((a) => `<option value="${esc(a.ip)}">${esc(a.ip)} · ${esc(a.adapter)}${a.virtual ? " (VPN/virtual)" : ""}</option>`).join("");
      if (want) sel.value = want;
      paintLink();
    }
    $("#devrows").innerHTML = deviceRows();
    $("#mcpurl").textContent = `http://localhost:${s.port}/mcp`;
    $("#mcpjson").textContent = JSON.stringify({ mcpServers: { tabletdeck: { command: "C:\\Program Files\\Tablet Deck\\TabletDeck.exe", args: ["mcp"] } } }, null, 2);
    const kb = s.keybinds || {};
    $("#kb").innerHTML = `<span class="k1">Files in use</span><span class="mono" style="font-size:12px">${(kb.files || []).map(esc).join("<br>") || "—"}</span>
      <span class="k1">Actions read</span><span>${kb.actions || 0}</span>
      <span class="k1">Installs found</span><span class="mono" style="font-size:12px">${(kb.installs || []).map((i) => `${esc(i.channel)}${i.running ? " · RUNNING" : ""} · ${esc(i.dir)}`).join("<br>") || "None yet: start the game once, or set its folder in config.json (gameDirs)"}</span>`;
  }
  function linkFor(ip) { return `http://${ip}:${status.port}/?t=${status.pairing.code}`; }
  function paintLink() { const ip = $("#addr").value || (status.pairing.addresses[0] || {}).ip; if (ip) drawQR($("#qr"), linkFor(ip)); }
  $("#addr").onchange = paintLink;

  function deviceRows() {
    const rows = [];
    const online = new Map((status.clients || []).map((c) => [c.name, c]));
    for (const d of devices) {
      const c = [...online.values()].find((x) => x.name === d.name && x.kind === "owner");
      rows.push(`<tr><td><i class="dot ${c ? "good" : ""}"></i><b>${esc(d.name)}</b></td><td class="mono">DECK</td><td>${esc(c ? c.layout : "")}</td><td class="mono">${c ? "NOW" : ago(d.lastSeen)}</td>
        <td style="text-align:right"><button type="button" class="sbtn" data-unpair="${esc(d.id)}">Unpair</button></td></tr>`);
    }
    for (const c of status.clients || []) {
      if (c.kind === "owner" && devices.some((d) => d.name === c.name)) continue;
      rows.push(`<tr><td><i class="dot good"></i><b>${esc(c.name)}</b></td><td class="mono">${esc(c.kind === "owner" ? (c.local ? "THIS PC" : "DECK") : c.kind)}</td><td>${esc(c.layout)}</td><td class="mono">NOW</td><td></td></tr>`);
    }
    return rows.join("") || `<tr><td colspan="5" class="mono">No devices yet. Scan the code above with a tablet or phone.</td></tr>`;
  }

  let devices = [], grants = [], crewLinks = [];
  function paintGrants() {
    $("#grants").innerHTML = grants.length ? grants.map((g) => {
      const left = g.expires && !String(g.expires).startsWith("0001") ? `${Math.max(0, Math.round((new Date(g.expires) - Date.now()) / 60000))} MIN LEFT` : "UNTIL OFF";
      const link = (g.links && (g.links.internet || g.links.lan)) || "";
      return `<div class="list-row"><b>${esc(g.label)}</b><span class="inline" style="grid-row:span 2"><button type="button" class="sbtn" data-copy="${esc(link)}">Copy link</button><button type="button" class="sbtn danger" data-revoke="${esc(g.id)}">Revoke</button></span>
        <span class="mono">${g.all ? "EVERYTHING" : `${(g.buttons || []).length} CONTROLS`} · ${g.afkOnly ? "WHILE AFK · " : ""}${left} · ${g.presses} PRESSES · ${g.connected} CONNECTED</span></div>`;
    }).join("") : `<p class="note">No share links. Make one from a deck's Share button.</p>`;
    $("#crewlinks").innerHTML = crewLinks.map((l) => `<div class="list-row"><b>${esc(l.name)}</b><button type="button" class="sbtn danger" style="grid-row:span 2" data-unlink="${esc(l.id)}">Remove</button><span class="mono">${esc(l.url)}</span></div>`).join("");
  }

  function paintVoice() {
    const v = voice;
    if (!v) return;
    $("#vstate").textContent = (v.status && v.status.state) || "";
    $("#vrec").textContent = v.engine === "native" ? `${v.recognizer} · gorelaxor` : v.engine === "windows" ? "Windows speech (fallback)" : (v.status && v.status.reason) || "Off";
    const m = v.models || {};
    $("#vfound").textContent = `${(m.stt || []).length} speech-to-text · ${m.vad ? "MarbleNet" : "no VAD"} · ${m.tts ? "Pocket TTS" : "no TTS"} · ${(v.voices || []).length} voices`;
    const tts = $("#tts");
    if (document.activeElement !== tts) {
      tts.innerHTML = (v.voices || []).map((n) => `<option value="${esc(n)}">${esc(n.replace(/_/g, " "))}</option>`).join("") || `<option value="">No voices found</option>`;
      if (cfg) tts.value = cfg.ttsVoice;
    }
    const mic = $("#mic");
    if (document.activeElement !== mic) {
      mic.innerHTML = (v.mics && v.mics.length ? v.mics : [{ id: -1, name: "Windows default" }]).map((d) => `<option value="${d.id}">${esc(d.name)}</option>`).join("");
      if (cfg) mic.value = String(cfg.micDevice);
    }
  }
  function paintMeter(level) {
    const n = Math.round((level || 0) * 24);
    $("#meter").innerHTML = Array.from({ length: 24 }, (_, i) => `<i class="${i < n ? (i > 18 ? "hot" : "on") : ""}"></i>`).join("");
  }

  function paintSettings() {
    const c = cfg;
    $("#sname").value = c.name; $("#tapms").value = c.tapMs; $("#gapms").value = c.comboGapMs; $("#port").value = c.port;
    $("#wake").value = c.wakeWord; $("#models").value = c.modelsDir || "";
    $("#focus").innerHTML = [["strict", "Strict"], ["local", "This PC"], ["always", "Always"], ["off", "Off"]].map(([v, l]) => `<button type="button" data-focus="${v}" aria-pressed="${c.focusMode === v}">${l}</button>`).join("");
    $("#replies").innerHTML = [["off", "Off"], ["short", "Short"], ["full", "Full"]].map(([v, l]) => `<button type="button" data-replies="${v}" aria-pressed="${c.replies === v}">${l}</button>`).join("");
    for (const [id, on] of [["#relfocus", c.releaseOnFocusLoss], ["#tunnel", c.tunnel === "cloudflare"], ["#discovery", c.discovery]]) {
      $(id).setAttribute("aria-checked", String(!!on)); $(id).querySelector(".mono").textContent = on ? "ON" : "OFF";
    }
    const t = status && status.tunnel;
    if (t) {
      $("#tunnelmsg").textContent = t.state === "up" ? t.url : t.error || (t.state !== "off" ? t.state : "");
      $("#cfinstall").hidden = t.installed;
    }
  }

  async function refresh() {
    try {
      [status, cfg, devices, grants] = await Promise.all([api("status"), api("config"), api("devices"), api("grants")]);
      const crew = await api("crew");
      crewLinks = crew.links || [];
      paintStatus(); paintGrants(); paintSettings();
      api("nearby").then((list) => { $("#nearby").textContent = list.length ? list.map((n) => `${n.name} · ${n.address}`).join("   ") : "None found."; });
    } catch (e) { toast(e.message); }
  }
  async function refreshVoice() { try { voice = await api("voice"); paintVoice(); paintMeter(voice.level); } catch {} }

  async function saveCfg(patch) {
    const next = Object.assign({}, cfg, patch);
    try {
      const r = await put("config", next);
      cfg = next; $("#saveerr").textContent = "";
      toast(r.restart ? "Saved. Restart Tablet Deck for the new port." : "Saved");
      refresh(); setTimeout(refreshVoice, 800);
    } catch (e) { $("#saveerr").textContent = e.message; }
  }

  document.addEventListener("click", async (e) => {
    const t = e.target.closest("button");
    if (!t) return;
    if (t.dataset.focus) saveCfg({ focusMode: t.dataset.focus });
    else if (t.dataset.replies) saveCfg({ replies: t.dataset.replies });
    else if (t.dataset.unpair) { await api(`devices/${t.dataset.unpair}`, { method: "DELETE" }); refresh(); }
    else if (t.dataset.revoke) { await api(`grants/${t.dataset.revoke}`, { method: "DELETE" }); refresh(); }
    else if (t.dataset.unlink) { await api(`crew/${t.dataset.unlink}`, { method: "DELETE" }); refresh(); }
    else if (t.dataset.copy) { navigator.clipboard.writeText(t.dataset.copy).then(() => toast("Copied")); }
  });
  $("#relfocus").onclick = () => saveCfg({ releaseOnFocusLoss: !cfg.releaseOnFocusLoss });
  $("#discovery").onclick = () => saveCfg({ discovery: !cfg.discovery });
  $("#tunnel").onclick = async () => { try { await post("tunnel", { on: cfg.tunnel !== "cloudflare" }); refresh(); } catch (e) { toast(e.message); } };
  $("#cfinstall").onclick = async () => { toast("Downloading cloudflared…"); try { await post("tunnel/install"); toast("cloudflared installed"); refresh(); } catch (e) { toast(e.message); } };
  $("#savesettings").onclick = () => saveCfg({ name: $("#sname").value.trim(), tapMs: +$("#tapms").value, comboGapMs: +$("#gapms").value, port: +$("#port").value });
  $("#wake").onchange = () => saveCfg({ wakeWord: $("#wake").value.trim().toLowerCase() });
  $("#tts").onchange = () => saveCfg({ ttsVoice: $("#tts").value });
  $("#mic").onchange = () => saveCfg({ micDevice: +$("#mic").value });
  $("#modelssave").onclick = () => saveCfg({ modelsDir: $("#models").value.trim() });
  $("#ttstest").onclick = () => post("voice/say", { text: "Landing gear down. Ready when you are.", voice: $("#tts").value }).catch((e) => toast(e.message));
  $("#newcode").onclick = () => saveCfg({ token: Math.random().toString(16).slice(2, 10) });
  $("#copylink").onclick = () => navigator.clipboard.writeText(linkFor($("#addr").value)).then(() => toast("Copied"));
  $("#showkey").onclick = () => { $("#apikey").textContent = status.apiKey; };
  $("#release").onclick = () => post("release-all").then(() => toast("All keys released"));
  $("#editor").onclick = () => window.open(status.editor, "_blank");
  $("#touch").onclick = () => window.open(status.pairing.local + "&name=This%20PC", "touchdeck", "width=1024,height=768");
  $("#installprofile").onclick = async () => {
    try { const r = await post("keybinds/install"); $("#kbmsg").textContent = `Installed to ${r.written.length} folder(s). In game: Options › Keybindings › Control Profiles.`; }
    catch (e) { $("#kbmsg").textContent = e.message; }
  };
  $("#crewform").onsubmit = async (e) => {
    e.preventDefault();
    try { await post("crew", { name: $("#cname").value.trim(), link: $("#clink").value.trim() }); $("#cname").value = $("#clink").value = ""; refresh(); }
    catch (err) { toast(err.message); }
  };

  // Section highlight in the side menu while scrolling.
  const links = [...document.querySelectorAll("nav.side a")];
  const io = new IntersectionObserver((ents) => {
    for (const en of ents) if (en.isIntersecting) links.forEach((a) => a.setAttribute("aria-current", String(a.getAttribute("href") === `#${en.target.id}`)));
  }, { rootMargin: "-10% 0px -80% 0px" });
  document.querySelectorAll("main section[id]").forEach((s) => io.observe(s));

  // Live activity.
  const log = $("#log");
  function addLog(ev) {
    const d = ev.data || {};
    let src = "", text = "";
    switch (ev.t) {
      case "press": src = (d.src && (d.src.kind === "deck" ? d.src.who : d.src.kind)) || ""; text = `${d.label} · ${String(d.keys || "").toUpperCase()}${d.ok ? "" : ` · BLOCKED (${d.reason})`}`; break;
      case "heard": src = "VOICE"; text = `"${d.text}"${d.label ? ` → ${d.label}` : d.accepted === false ? " · not matched" : ""}`; break;
      case "agent": src = "AGENT"; text = `${d.tool}${d.ok ? "" : " · failed"}`; break;
      case "request": src = "CREW"; text = `${d.from}: ${d.label || ""} ${d.text || ""}`; break;
      case "keybinds": src = "PC"; text = `Keybinds changed: ${d.actions} actions`; break;
      case "afk": src = "PC"; text = d.on ? "Away mode on" : "Away mode off"; break;
      case "view": src = "VIEW"; text = `${d.view} → ${d.device || "all decks"} (${d.decks})`; break;
      default: return;
    }
    const row = document.createElement("div");
    row.innerHTML = `<span class="t">${new Date(ev.time).toLocaleTimeString([], { hour12: false })}</span><span>${esc(String(src).toUpperCase().slice(0, 10))}</span><span>${esc(text)}</span>`;
    log.prepend(row);
    while (log.children.length > 200) log.lastChild.remove();
  }
  api("log").then((list) => list.forEach(addLog)).catch(() => {});
  const es = new EventSource(`/api/v1/events?k=${encodeURIComponent(key)}`);
  es.onmessage = (m) => { try { const ev = JSON.parse(m.data); addLog(ev); if (["clients", "devices", "grants", "crew", "game", "tunnel", "voice", "keybinds"].includes(ev.t)) refresh(); } catch {} };

  refresh();
  refreshVoice();
  setInterval(refreshVoice, 400);
  setInterval(refresh, 15000);
})();
