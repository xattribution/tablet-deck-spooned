// Effects for the console: what a press looks like when it lands. Flares burst, noise
// rings out, missiles streak off the panel, quantum warps the whole screen, scanners
// ping, lasers fire, lights bloom, targets lock. One canvas over everything, drawn only
// while something is moving. Off when the system asks for reduced motion.
(() => {
  "use strict";
  const cv = document.createElement("canvas");
  cv.id = "fx";
  cv.setAttribute("aria-hidden", "true");
  document.body.appendChild(cv);
  const g = cv.getContext("2d");
  let W = 0, H = 0, dpr = 1;
  function resize() {
    dpr = Math.min(2, window.devicePixelRatio || 1);
    W = innerWidth; H = innerHeight;
    cv.width = W * dpr; cv.height = H * dpr;
    cv.style.width = `${W}px`; cv.style.height = `${H}px`;
  }
  resize();
  const reduced = () => matchMedia("(prefers-reduced-motion: reduce)").matches;

  // Each effect is a list of drawables with a lifetime; the loop runs while any live.
  const live = [];
  let raf = 0, last = 0;
  function add(d) { d.t = 0; live.push(d); if (!raf) { last = performance.now(); raf = requestAnimationFrame(loop); } }
  function loop(now) {
    const dt = Math.min(0.05, (now - last) / 1000);
    last = now;
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    g.clearRect(0, 0, W, H);
    for (let i = live.length - 1; i >= 0; i--) {
      const d = live[i];
      d.t += dt;
      if (d.t >= d.life) { live.splice(i, 1); continue; }
      g.save();
      d.draw(d.t / d.life, dt, d.t);
      g.restore();
    }
    raf = live.length ? requestAnimationFrame(loop) : 0;
    if (!raf) g.clearRect(0, 0, W, H);
  }

  const css = (el, name, fb) => (el && getComputedStyle(el).getPropertyValue(name).trim()) || fb;
  const center = (el, e) => {
    const r = el.getBoundingClientRect();
    return { x: e && e.clientX ? e.clientX : r.left + r.width / 2, y: e && e.clientY ? e.clientY : r.top + r.height / 2, r };
  };
  const rnd = (a, b) => a + Math.random() * (b - a);
  const ease = (p) => 1 - Math.pow(1 - p, 3);

  // ---------- effects ----------
  const FX = {
    // A ring from the touch point: every press shows where it landed.
    press(el, e) {
      const { x, y } = center(el, e), col = css(el, "--cat", "#fff");
      add({ life: 0.38, draw(p) { g.globalAlpha = 0.55 * (1 - p); g.strokeStyle = col; g.lineWidth = 2; g.beginPath(); g.arc(x, y, 8 + ease(p) * 46, 0, 7); g.stroke(); } });
    },
    // Flares: hot sparks thrown out and falling, with a white flash at the launcher.
    flares(el, e) {
      const { r } = center(el, e), x = r.left + r.width / 2, y = r.top + r.height / 2;
      add({ life: 0.25, draw(p) { g.globalCompositeOperation = "lighter"; const rad = g.createRadialGradient(x, y, 0, x, y, 120); rad.addColorStop(0, `rgba(255,240,210,${0.6 * (1 - p)})`); rad.addColorStop(1, "rgba(255,160,60,0)"); g.fillStyle = rad; g.fillRect(x - 120, y - 120, 240, 240); } });
      for (let i = 0; i < 34; i++) {
        const a = rnd(-Math.PI, 0) + rnd(-0.4, 0.4), v = rnd(180, 460);
        let px = x, py = y, vx = Math.cos(a) * v, vy = Math.sin(a) * v;
        const life = rnd(0.7, 1.25), hot = Math.random() < 0.3;
        add({ life, draw(p, dt) {
          vy += 520 * dt; vx *= 0.985;
          const nx = px + vx * dt, ny = py + vy * dt;
          g.globalCompositeOperation = "lighter";
          g.strokeStyle = hot ? `rgba(255,255,235,${1 - p})` : `rgba(255,${Math.round(190 - 110 * p)},${Math.round(90 - 80 * p)},${1 - p})`;
          g.lineWidth = hot ? 2.6 : 1.8; g.lineCap = "round";
          g.beginPath(); g.moveTo(px, py); g.lineTo(nx, ny); g.stroke();
          px = nx; py = ny;
        } });
      }
    },
    // Noise: jagged rings of interference spreading from the key.
    noise(el, e) {
      const { r } = center(el, e), x = r.left + r.width / 2, y = r.top + r.height / 2, col = css(el, "--cat", "#fff");
      for (let k = 0; k < 4; k++) {
        const seed = Math.random() * 10;
        add({ life: 0.9 + k * 0.12, draw(p, dt, t) {
          if (t < k * 0.1) return;
          const R = 20 + ease(p) * 260;
          g.globalAlpha = 0.7 * (1 - p); g.strokeStyle = col; g.lineWidth = 1.6;
          g.beginPath();
          for (let i = 0; i <= 72; i++) { const a = (i / 72) * Math.PI * 2, j = Math.sin(a * 9 + seed + t * 30) * 6 + Math.sin(a * 23 + seed) * 3; g.lineTo(x + Math.cos(a) * (R + j), y + Math.sin(a) * (R + j)); }
          g.stroke();
        } });
      }
    },
    // A missile leaves the panel trailing smoke.
    missile(el, e) {
      const { r } = center(el, e), x0 = r.left + r.width / 2, y0 = r.top + r.height / 2;
      const x1 = x0 + rnd(-120, 120), y1 = -60, cx = x0 + rnd(-200, 200), cy = y0 - 120;
      const at = (p) => ({ x: (1 - p) * (1 - p) * x0 + 2 * (1 - p) * p * cx + p * p * x1, y: (1 - p) * (1 - p) * y0 + 2 * (1 - p) * p * cy + p * p * y1 });
      let lastPuff = 0;
      add({ life: 0.75, draw(p, dt, t) {
        const q = p * p, h = at(q);
        if (t - lastPuff > 0.016) {
          lastPuff = t;
          const sx = h.x, sy = h.y, s0 = rnd(4, 8);
          add({ life: rnd(0.8, 1.3), draw(pp) { g.globalAlpha = 0.35 * (1 - pp); g.fillStyle = "#9aa3ad"; g.beginPath(); g.arc(sx + pp * rnd(-1, 1) * 6, sy + pp * 10, s0 + pp * 18, 0, 7); g.fill(); } });
        }
        g.globalCompositeOperation = "lighter";
        const rad = g.createRadialGradient(h.x, h.y, 0, h.x, h.y, 22); rad.addColorStop(0, "rgba(255,250,230,1)"); rad.addColorStop(0.3, "rgba(255,170,80,.8)"); rad.addColorStop(1, "rgba(255,90,40,0)");
        g.fillStyle = rad; g.fillRect(h.x - 22, h.y - 22, 44, 44);
      } });
    },
    // Quantum: the stars stretch into lines towards the edges and the screen flashes.
    warp(el) {
      const cx = W / 2, cy = H / 2, col = css(el, "--cat", "#B48CFF");
      const stars = Array.from({ length: 160 }, () => ({ a: rnd(0, Math.PI * 2), r: rnd(20, Math.max(W, H) * 0.5), s: rnd(0.6, 1.6) }));
      add({ life: 1.0, draw(p) {
        g.globalCompositeOperation = "lighter";
        const k = ease(p);
        for (const st of stars) {
          const r0 = st.r * (1 + k * 2.2 * st.s), r1 = r0 + 20 + k * 260 * st.s;
          g.strokeStyle = `rgba(220,215,255,${0.75 * (1 - p)})`; g.lineWidth = 1.2 * st.s;
          g.beginPath(); g.moveTo(cx + Math.cos(st.a) * r0, cy + Math.sin(st.a) * r0); g.lineTo(cx + Math.cos(st.a) * r1, cy + Math.sin(st.a) * r1); g.stroke();
        }
        g.globalCompositeOperation = "source-over";
        g.globalAlpha = Math.max(0, 0.5 - p) ; g.fillStyle = col; g.fillRect(0, 0, W, H);
      } });
    },
    // A scanner ping: a sweeping wedge and an expanding ring.
    scan(el, e) {
      const { r } = center(el, e), x = r.left + r.width / 2, y = r.top + r.height / 2, col = css(el, "--cat", "#fff");
      add({ life: 1.1, draw(p) {
        const R = 30 + ease(p) * 300, a = p * Math.PI * 2.2;
        g.globalAlpha = 0.8 * (1 - p); g.strokeStyle = col; g.lineWidth = 2;
        g.beginPath(); g.arc(x, y, R, 0, 7); g.stroke();
        g.globalAlpha = 0.25 * (1 - p); g.fillStyle = col;
        g.beginPath(); g.moveTo(x, y); g.arc(x, y, R, a - 0.5, a); g.closePath(); g.fill();
      } });
    },
    // The mining laser: a flickering beam out of the top of the screen.
    laser(el, e) {
      const { r } = center(el, e), x = r.left + r.width / 2, y = r.top, col = css(el, "--cat", "#8CE05A");
      add({ life: 0.6, draw(p, dt, t) {
        g.globalCompositeOperation = "lighter";
        const w = (6 + Math.sin(t * 90) * 3) * (1 - p);
        const lg = g.createLinearGradient(x - w * 3, 0, x + w * 3, 0);
        lg.addColorStop(0, "rgba(0,0,0,0)"); lg.addColorStop(0.5, col); lg.addColorStop(1, "rgba(0,0,0,0)");
        g.fillStyle = lg; g.globalAlpha = 0.9 * (1 - p); g.fillRect(x - w * 3, 0, w * 6, y);
        g.fillStyle = "#fff"; g.fillRect(x - w * 0.3, 0, w * 0.6, y);
      } });
    },
    // Lights: a warm bloom from the switch.
    lights(el, e) {
      const { r } = center(el, e), x = r.left + r.width / 2, y = r.top + r.height / 2;
      add({ life: 0.8, draw(p) {
        g.globalCompositeOperation = "lighter";
        const R = 60 + ease(p) * 340, rad = g.createRadialGradient(x, y, 0, x, y, R);
        rad.addColorStop(0, `rgba(255,236,190,${0.45 * (1 - p)})`); rad.addColorStop(1, "rgba(255,200,120,0)");
        g.fillStyle = rad; g.fillRect(x - R, y - R, R * 2, R * 2);
      } });
    },
    // Doors: two leaves slide apart over the key.
    doors(el) {
      const { r } = center(el), col = css(el, "--cat", "#fff");
      add({ life: 0.7, draw(p) {
        const o = ease(p) * r.width * 0.5;
        g.globalAlpha = 0.55 * (1 - p); g.fillStyle = col;
        g.fillRect(r.left - o, r.top, r.width / 2, r.height); g.fillRect(r.left + r.width / 2 + o, r.top, r.width / 2, r.height);
      } });
    },
    // Shields: the face flares and a hex shell ripples out from it.
    shield(el, e) {
      el.classList.remove("ripple"); void el.offsetWidth; el.classList.add("ripple");
      const { r } = center(el, e), x = r.left + r.width / 2, y = r.top + r.height / 2, col = css(el, "--cat", "#3CC4D6");
      const hex = (cx, cy, s) => { g.beginPath(); for (let i = 0; i < 6; i++) { const a = Math.PI / 3 * i + Math.PI / 6; g.lineTo(cx + Math.cos(a) * s, cy + Math.sin(a) * s); } g.closePath(); g.stroke(); };
      add({ life: 0.7, draw(p) {
        g.strokeStyle = col; g.lineWidth = 1.4;
        const R = 20 + ease(p) * 120;
        for (let i = 0; i < 18; i++) { const a = (i / 18) * Math.PI * 2; g.globalAlpha = 0.7 * (1 - p) * (0.5 + 0.5 * Math.sin(i * 1.7)); hex(x + Math.cos(a) * R, y + Math.sin(a) * R, 7 + p * 5); }
      } });
    },
    // Target cycling: brackets close on the key.
    lock(el) {
      const { r } = center(el), col = css(el, "--cat", "#fff");
      add({ life: 0.45, draw(p) {
        const o = (1 - ease(Math.min(1, p * 1.6))) * 26, L = Math.min(r.width, r.height) * 0.22;
        g.globalAlpha = p > 0.7 ? (1 - p) / 0.3 : 1; g.strokeStyle = col; g.lineWidth = 2.5;
        const c = (x, y, sx, sy) => { g.beginPath(); g.moveTo(x + sx * L, y); g.lineTo(x, y); g.lineTo(x, y + sy * L); g.stroke(); };
        c(r.left - o, r.top - o, 1, 1); c(r.right + o, r.top - o, -1, 1); c(r.left - o, r.bottom + o, 1, -1); c(r.right + o, r.bottom + o, -1, -1);
      } });
    },
    // Flight ready: a light sweeps the console and the panels power on in turn.
    power() {
      const con = document.getElementById("console");
      if (!con) return;
      con.classList.remove("powerup"); void con.offsetWidth; con.classList.add("powerup");
      setTimeout(() => con.classList.remove("powerup"), 1000);
      [...con.querySelectorAll(".page .pane")].forEach((p, j) => { p.classList.remove("boot"); p.style.setProperty("--d", `${j * 50}ms`); void p.offsetWidth; p.classList.add("boot"); });
    },
    // Self-destruct and eject: the console strobes red.
    alarm() {
      const con = document.getElementById("console");
      if (!con) return;
      con.classList.remove("alarmed"); void con.offsetWidth; con.classList.add("alarmed");
    },
    quantum() {},
  };
  FX.streak = FX.press; FX.dial = FX.press;

  window.DeckFX3 = {
    play(name, el, e) {
      if (!el || reduced()) return;
      (FX[name] || FX.press)(el, e);
      if (name !== "press" && FX[name] && !["power", "alarm", "quantum", "warp"].includes(name)) FX.press(el, e);
    },
    // A state that changed by itself (another deck, voice, the agent) still shows.
    state(el, on) {
      if (reduced() || !on) return;
      const id = el.dataset.id || "";
      if (/^lights$/.test(id)) FX.lights(el);
      else FX.press(el);
    },
    resize,
  };
})();
