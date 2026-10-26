// Crew: share links, away mode, requests between decks and linked crewmates.
// The owner's deck gets a Crew section in Deck settings, a Share sheet and an inbox
// for requests; a guest on a share link gets a bar with who they're helping and an
// Ask button.
(() => {
  const D = window.Deck;
  if (!D || D.demo) return;
  const tr = D.tr;
  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const QUICK = ["BRB", "Cover me", "Take the helm", "Gear down", "Spool QT", "Need a hand"];

  // ---------- inbox: requests from guests and crew ----------

  const inbox = $("inbox");
  function showRequest(q) {
    if (inbox.querySelector(`[data-id="${q.id}"]`)) return;
    const card = document.createElement("div");
    card.className = "rq";
    card.dataset.id = q.id;
    const what = q.label ? tr("asks you to press {0}", `<b>${esc(tr(q.label))}</b>`) : "";
    card.innerHTML = `<div class="rq-from">${esc(q.from)}</div>
      <div class="rq-text">${what}${q.text ? `<span>${esc(q.text)}</span>` : ""}</div>
      <div class="rq-btns">${q.label ? `<button class="rq-do" type="button">${tr("Do it")}</button>` : `<button class="rq-do" type="button">${tr("On it")}</button>`}
      <button class="rq-no" type="button">${tr(q.label ? "No" : "Dismiss")}</button></div>`;
    card.querySelector(".rq-do").onclick = () => { D.buzz(); D.send({ t: "answer", id: q.id, do: true }); card.remove(); };
    card.querySelector(".rq-no").onclick = () => { D.send({ t: "answer", id: q.id, do: false }); card.remove(); };
    inbox.prepend(card);
    D.buzz([20, 40, 20]);
    while (inbox.children.length > 4) inbox.lastChild.remove();
  }
  D.on("request", (m) => showRequest(m.request));
  D.on("request-gone", (m) => inbox.querySelector(`[data-id="${m.id}"]`)?.remove());
  D.on("answered", (m) => D.toast(tr(m.result === "done" ? "Done: {0}" : "Declined: {0}", tr(m.label || m.text || "")), 2600));

  // ---------- guest bar ----------

  if (D.guest) {
    document.body.classList.add("guest");
    const bar = $("guestBar");
    let info = null, owner = "", afk = false;
    const left = () => {
      if (!info || !info.expires || info.expires.startsWith("0001")) return "";
      const min = Math.max(0, Math.round((new Date(info.expires) - Date.now()) / 60000));
      return min >= 60 ? `${Math.floor(min / 60)} h ${min % 60} min` : `${min} min`;
    };
    const draw = () => {
      if (!info) return;
      bar.hidden = false;
      const note = info.afkOnly && !afk ? ` · ${tr("works while {0} is AFK", esc(owner))}` : "";
      const time = left() ? ` · ${left()}` : "";
      bar.querySelector(".gb-text").innerHTML = `${tr("Helping {0}", `<b>${esc(owner)}</b>`)}${time}${note}`;
      bar.classList.toggle("waiting", Boolean(info.afkOnly && !afk));
    };
    D.on("hello", (m) => { info = m.guest; owner = m.name; afk = m.afk; draw(); });
    D.on("afk", (m) => { afk = m.on; draw(); D.toast(tr(m.on ? "{0} is AFK: you have the controls" : "{0} is back", m.name), 3500); });
    setInterval(draw, 30000);
    bar.querySelector(".gb-ask").onclick = () => openAsk(null, owner);
  }

  // ---------- ask (send a request) ----------

  let askTarget = null; // crewmate id, or null for a guest asking the owner
  function openAsk(target, name) {
    askTarget = target;
    $("askTitle").textContent = tr("Ask {0}", name || tr("your crew"));
    $("askText").value = "";
    const quick = $("askQuick");
    quick.innerHTML = "";
    for (const q of QUICK) {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = tr(q);
      b.onclick = () => sendAsk(tr(q));
      quick.appendChild(b);
    }
    $("askSheet").classList.add("open");
    setTimeout(() => $("askText").focus(), 50);
  }
  function sendAsk(text) {
    text = String(text || "").trim();
    if (!text) return;
    if (askTarget) D.send({ t: "crew-request", peer: askTarget, text });
    else D.send({ t: "request", text });
    D.buzz();
    $("askSheet").classList.remove("open");
  }
  $("askForm").onsubmit = (e) => { e.preventDefault(); sendAsk($("askText").value); };
  $("askClose").onclick = () => $("askSheet").classList.remove("open");

  if (D.guest) return; // the rest is for the owner's decks

  // ---------- away mode ----------

  const afkBtn = $("afkBtn");
  const setAfk = (on) => { afkBtn.setAttribute("aria-checked", String(on)); document.documentElement.toggleAttribute("data-afk", on); };
  afkBtn.onclick = () => { const on = afkBtn.getAttribute("aria-checked") !== "true"; D.send({ t: "afk", on }); setAfk(on); D.buzz(); };
  D.on("hello", (m) => {
    setAfk(Boolean(m.afk));
    for (const q of m.requests || []) showRequest(q);
    drawCrew(m.crew || []);
  });
  D.on("afk", (m) => setAfk(m.on));
  $("releaseAllBtn").onclick = () => { D.send({ t: "release-all" }); D.buzz([20, 30, 20]); };

  // ---------- crew list ----------

  function drawCrew(list) {
    const box = $("crewList");
    box.innerHTML = "";
    if (!list.length) {
      box.innerHTML = `<p class="sheet-note">${tr("No crew yet. Share controls with a friend, or add their link on the PC.")}</p>`;
      return;
    }
    for (const c of list) {
      const row = document.createElement("div");
      row.className = "crew-row" + (c.online ? " online" : "");
      const tag = c.afk ? `<i class="crew-tag">AFK</i>` : c.guest ? `<i class="crew-tag">${tr(c.peer ? "linked" : "guest")}</i>` : "";
      row.innerHTML = `<div class="crew-head"><span class="crew-dot"></span><b>${esc(c.name)}</b>${tag}
        <button class="crew-ask" type="button">${tr("Ask")}</button></div><div class="crew-btns"></div>`;
      row.querySelector(".crew-ask").onclick = () => openAsk(c.id, c.name);
      const btns = row.querySelector(".crew-btns");
      for (const b of c.buttons || []) {
        const el = document.createElement("button");
        el.type = "button";
        el.textContent = tr(b.label);
        el.disabled = !c.online || (c.afkOnly && !c.afk);
        el.onclick = () => { D.buzz(); D.send({ t: "crew-press", peer: c.id, id: b.id }); };
        btns.appendChild(el);
      }
      box.appendChild(row);
    }
  }
  D.on("crew", (m) => drawCrew(m.crew || []));

  // ---------- share sheet ----------

  const sheet = $("shareSheet");
  let scope = { all: false, panels: new Set() };
  let minutes = 60;
  function panelsOf(l) {
    if (!l) return [];
    if (l.pages) return l.pages.map((p) => ({ id: p.name, title: p.name }));
    const order = (l.regions || []).flatMap((r) => r.panels || []);
    for (const id of Object.keys(l.panels || {})) if (!order.includes(id)) order.push(id);
    return order.map((id) => ({ id, title: (l.panels[id] || {}).title || id }));
  }
  function chip(label, on, onClick) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "chip";
    b.textContent = label;
    b.setAttribute("aria-pressed", String(on));
    b.onclick = () => { onClick(); D.buzz(); };
    return b;
  }
  function drawScope() {
    const box = $("sharePanels");
    box.innerHTML = "";
    box.appendChild(chip(tr("Everything"), scope.all, () => { scope.all = !scope.all; drawScope(); }));
    for (const p of panelsOf(D.layout())) {
      const c = chip(tr(p.title), scope.all || scope.panels.has(p.id), () => {
        scope.all = false;
        if (scope.panels.has(p.id)) scope.panels.delete(p.id); else scope.panels.add(p.id);
        drawScope();
      });
      c.disabled = scope.all;
      box.appendChild(c);
    }
  }
  function drawTime() {
    const box = $("shareTime");
    box.innerHTML = "";
    for (const [m, label] of [[15, "15 min"], [60, "1 hour"], [240, "4 hours"], [0, "Until I turn it off"]]) {
      box.appendChild(chip(tr(label), minutes === m, () => { minutes = m; drawTime(); }));
    }
  }
  $("shareBtn").onclick = () => {
    D.closeSettings();
    sheet.classList.add("open");
    sheet.classList.remove("done");
    $("shareName").value = "";
    scope = { all: false, panels: new Set() };
    drawScope();
    drawTime();
  };
  $("shareClose").onclick = () => sheet.classList.remove("open");
  $("shareAfk").onclick = () => $("shareAfk").setAttribute("aria-checked", String($("shareAfk").getAttribute("aria-checked") !== "true"));
  $("shareForm").onsubmit = (e) => {
    e.preventDefault();
    const label = $("shareName").value.trim();
    if (!label) { D.toast("Who is it for?"); $("shareName").focus(); return; }
    if (!scope.all && !scope.panels.size) { D.toast("Pick what to share"); return; }
    D.send({ t: "share", label, minutes, all: scope.all, panels: [...scope.panels], afkOnly: $("shareAfk").getAttribute("aria-checked") === "true" });
  };
  D.on("shared", (m) => {
    const link = m.links.internet || m.links.lan;
    sheet.classList.add("done");
    $("shareLink").textContent = link;
    $("shareWhere").textContent = m.links.internet ? tr("Works from anywhere.") : tr("Works on your home network. Turn on internet links on the PC to share further.");
    drawQR($("shareQR"), link);
    $("shareCopy").onclick = () => {
      (navigator.clipboard ? navigator.clipboard.writeText(link) : Promise.reject()).then(() => D.toast("Copied"), () => D.toast(link, 5000));
    };
    $("shareSend").hidden = !navigator.share;
    $("shareSend").onclick = () => navigator.share({ title: "Tablet Deck", text: tr("Help me fly: {0}", m.label), url: link }).catch(() => {});
  });

  function drawQR(canvas, text) {
    const rows = window.DeckQR.matrix(text);
    const n = rows.length + 8, scale = Math.max(4, Math.floor(260 / n));
    canvas.width = canvas.height = n * scale;
    const g = canvas.getContext("2d");
    g.fillStyle = "#fff";
    g.fillRect(0, 0, canvas.width, canvas.height);
    g.fillStyle = "#000";
    rows.forEach((row, y) => row.forEach((dark, x) => { if (dark) g.fillRect((x + 4) * scale, (y + 4) * scale, scale, scale); }));
  }
})();
