// Generated art for each maker: the hull behind the panels and the wear on them. Seeded
// noise, flow fields and simple physical rules (seams, rivets, rust that runs down from
// them, scratches that follow the way hands move) so every panel looks used, not
// printed. Drawn once per theme and screen size, then set as CSS images:
//   --art-surface: the hull or view behind the panels (frosted panels blur it)
//   --art-wear:    a tile of scratches, chips, grime or glass smudges over every panel
(() => {
  "use strict";

  // ---------- seeded randomness and noise ----------
  function rng(seed) {
    let a = seed >>> 0;
    return () => { a |= 0; a = (a + 0x6D2B79F5) | 0; let t = Math.imul(a ^ (a >>> 15), 1 | a); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
  }
  function noise2(seed) {
    const r = rng(seed), p = new Uint8Array(512), perm = [...Array(256).keys()];
    for (let i = 255; i > 0; i--) { const j = Math.floor(r() * (i + 1)); [perm[i], perm[j]] = [perm[j], perm[i]]; }
    for (let i = 0; i < 512; i++) p[i] = perm[i & 255];
    const grad = (h, x, y) => { const v = h & 7; const u = v < 4 ? x : y, w = v < 4 ? y : x; return ((v & 1) ? -u : u) + ((v & 2) ? -2 * w : 2 * w); };
    const fade = (t) => t * t * t * (t * (t * 6 - 15) + 10);
    const n = (x, y) => {
      const X = Math.floor(x) & 255, Y = Math.floor(y) & 255; x -= Math.floor(x); y -= Math.floor(y);
      const u = fade(x), v = fade(y), A = p[X] + Y, B = p[X + 1] + Y;
      const l = (a, b, t) => a + t * (b - a);
      return l(l(grad(p[A], x, y), grad(p[B], x - 1, y), u), l(grad(p[A + 1], x, y - 1), grad(p[B + 1], x - 1, y - 1), u), v) * 0.25;
    };
    n.fbm = (x, y, o = 5) => { let s = 0, a = 0.5, f = 1; for (let i = 0; i < o; i++) { s += a * n(x * f, y * f); f *= 2.03; a *= 0.5; } return s; };
    return n;
  }
  const hash = (s) => [...s].reduce((h, c) => Math.imul(h ^ c.charCodeAt(0), 16777619), 2166136261) >>> 0;

  function canvas(w, h) { const c = document.createElement("canvas"); c.width = w; c.height = h; return [c, c.getContext("2d")]; }
  // Paint a field function into the canvas pixel by pixel (fn returns [r,g,b,a 0..255]).
  function field(ctx, w, h, fn) {
    const img = ctx.getImageData(0, 0, w, h), d = img.data;
    for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
      const c = fn(x, y); if (!c) continue;
      const i = (y * w + x) * 4, a = c[3] / 255;
      d[i] = d[i] * (1 - a) + c[0] * a; d[i + 1] = d[i + 1] * (1 - a) + c[1] * a; d[i + 2] = d[i + 2] * (1 - a) + c[2] * a; d[i + 3] = Math.max(d[i + 3], c[3]);
    }
    ctx.putImageData(img, 0, 0);
  }

  // ---------- shared pieces ----------
  function stars(ctx, w, h, r, n, tint = "255,255,255") {
    for (let i = 0; i < n; i++) {
      const s = r() ** 3 * 1.6 + 0.3;
      ctx.fillStyle = `rgba(${tint},${0.2 + r() * 0.7})`;
      ctx.beginPath(); ctx.arc(r() * w, r() * h, s, 0, 7); ctx.fill();
    }
  }
  // Panel plates: a recursive split of the hull, seams cut in, rivets along the seams.
  function plates(ctx, w, h, r, o) {
    const rects = [];
    const split = (x, y, ww, hh, d) => {
      if (d === 0 || (ww < 220 && hh < 220)) { rects.push([x, y, ww, hh]); return; }
      if (ww > hh) { const k = ww * (0.35 + r() * 0.3); split(x, y, k, hh, d - 1); split(x + k, y, ww - k, hh, d - 1); }
      else { const k = hh * (0.35 + r() * 0.3); split(x, y, ww, k, d - 1); split(x, y + k, ww, hh - k, d - 1); }
    };
    split(0, 0, w, h, o.depth || 3);
    for (const [x, y, ww, hh] of rects) {
      const shade = (r() - 0.5) * (o.vary || 10);
      ctx.fillStyle = `rgba(${shade > 0 ? "255,255,255" : "0,0,0"},${Math.abs(shade) / 100})`;
      ctx.fillRect(x, y, ww, hh);
      ctx.strokeStyle = "rgba(0,0,0,.55)"; ctx.lineWidth = 2; ctx.strokeRect(x + 1, y + 1, ww - 2, hh - 2);
      ctx.strokeStyle = "rgba(255,255,255,.07)"; ctx.lineWidth = 1; ctx.strokeRect(x + 3, y + 3, ww - 6, hh - 6);
      if (o.rivets) for (const [ax, ay, bx, by] of [[x, y, x + ww, y], [x, y + hh, x + ww, y + hh], [x, y, x, y + hh], [x + ww, y, x + ww, y + hh]]) {
        const len = Math.hypot(bx - ax, by - ay), n = Math.floor(len / o.rivets);
        for (let i = 1; i < n; i++) {
          const px = ax + ((bx - ax) * i) / n + (ax === bx ? (ax === x ? 9 : -9) : 0), py = ay + ((by - ay) * i) / n + (ay === by ? (ay === y ? 9 : -9) : 0);
          ctx.fillStyle = "rgba(0,0,0,.5)"; ctx.beginPath(); ctx.arc(px + 0.8, py + 1, 3.2, 0, 7); ctx.fill();
          ctx.fillStyle = o.rivetColor || "rgba(170,170,160,.55)"; ctx.beginPath(); ctx.arc(px, py, 2.6, 0, 7); ctx.fill();
          ctx.fillStyle = "rgba(255,255,255,.35)"; ctx.beginPath(); ctx.arc(px - 0.8, py - 0.8, 0.9, 0, 7); ctx.fill();
          // Rust runs down from some rivets.
          if (o.rust && r() < o.rust) {
            const L = 20 + r() * 90, lg = ctx.createLinearGradient(px, py, px, py + L);
            lg.addColorStop(0, "rgba(120,58,20,.38)"); lg.addColorStop(1, "rgba(120,58,20,0)");
            ctx.fillStyle = lg; ctx.fillRect(px - 1.5 - r(), py, 3 + r() * 2, L);
          }
        }
      }
    }
  }
  // Scratches follow a few dominant directions, the way hands and tools actually move.
  function scratches(ctx, w, h, r, n, a0, alpha, light = true) {
    ctx.lineCap = "round";
    for (let i = 0; i < n; i++) {
      const a = a0 + (r() - 0.5) * 0.9 + (r() < 0.2 ? Math.PI / 2 : 0), L = 6 + r() ** 2 * 90, x = r() * w, y = r() * h;
      ctx.strokeStyle = light ? `rgba(255,255,255,${alpha * (0.3 + r())})` : `rgba(0,0,0,${alpha * (0.3 + r())})`;
      ctx.lineWidth = 0.4 + r() * 0.9;
      ctx.beginPath(); ctx.moveTo(x, y);
      ctx.quadraticCurveTo(x + Math.cos(a) * L * 0.5 + (r() - 0.5) * 6, y + Math.sin(a) * L * 0.5 + (r() - 0.5) * 6, x + Math.cos(a) * L, y + Math.sin(a) * L);
      ctx.stroke();
    }
  }
  // Grime and paint wear from layered noise.
  function grime(ctx, w, h, n, scale, color, amount, cut = 0.05) {
    field(ctx, w, h, (x, y) => {
      const v = n.fbm(x / scale, y / scale, 5) + 0.12 * n(x / 7, y / 7);
      if (v < cut) return null;
      return [color[0], color[1], color[2], Math.min(255, (v - cut) * amount * 255)];
    });
  }
  // Glass: faint fingerprints and a wipe arc.
  function smudges(ctx, w, h, r, k) {
    for (let i = 0; i < 4; i++) {
      const cx = r() * w, cy = r() * h, rx = 10 + r() * 8, ry = 13 + r() * 9, rot = r() * Math.PI;
      ctx.save(); ctx.translate(cx, cy); ctx.rotate(rot);
      for (let j = 1; j < 9; j++) { ctx.strokeStyle = `rgba(255,255,255,${0.035 * k})`; ctx.lineWidth = 0.8; ctx.beginPath(); ctx.ellipse(0, 0, rx * j / 8, ry * j / 8, 0, 0.2 + r(), Math.PI * 2 - r()); ctx.stroke(); }
      ctx.restore();
    }
    ctx.strokeStyle = `rgba(255,255,255,${0.03 * k})`; ctx.lineWidth = 26;
    ctx.beginPath(); ctx.arc(r() * w, h * 1.2, w * 0.7, Math.PI * 1.15, Math.PI * 1.55); ctx.stroke();
  }
  function flow(ctx, w, h, n, r, o) {
    ctx.lineWidth = o.width || 1;
    for (let i = 0; i < o.count; i++) {
      let x = r() * w, y = r() * h;
      ctx.strokeStyle = o.colors[Math.floor(r() * o.colors.length)];
      ctx.beginPath(); ctx.moveTo(x, y);
      for (let s = 0; s < o.steps; s++) { const a = n.fbm(x / o.scale, y / o.scale, 3) * Math.PI * o.turn; x += Math.cos(a) * 2.2; y += Math.sin(a) * 2.2; ctx.lineTo(x, y); }
      ctx.stroke();
    }
  }
  function contours(ctx, w, h, n, o) {
    const c = 6, cols = Math.ceil(w / c) + 1, rows = Math.ceil(h / c) + 1, v = new Float32Array(cols * rows);
    for (let j = 0; j < rows; j++) for (let i = 0; i < cols; i++) v[j * cols + i] = n.fbm(i * c / o.scale, j * c / o.scale, 4);
    ctx.lineWidth = o.width || 1;
    for (let L = 0; L < o.levels; L++) {
      const t = -0.5 + (L + 0.5) / o.levels;
      ctx.strokeStyle = o.color(L); ctx.beginPath();
      for (let j = 0; j < rows - 1; j++) for (let i = 0; i < cols - 1; i++) {
        const a = v[j * cols + i], b = v[j * cols + i + 1], d = v[(j + 1) * cols + i], e = v[(j + 1) * cols + i + 1];
        const pts = [], x = i * c, y = j * c;
        const edge = (p, q, x0, y0, x1, y1) => { if ((p < t) !== (q < t)) { const k = (t - p) / (q - p); pts.push([x0 + (x1 - x0) * k, y0 + (y1 - y0) * k]); } };
        edge(a, b, x, y, x + c, y); edge(b, e, x + c, y, x + c, y + c); edge(d, e, x, y + c, x + c, y + c); edge(a, d, x, y, x, y + c);
        if (pts.length >= 2) { ctx.moveTo(pts[0][0], pts[0][1]); ctx.lineTo(pts[1][0], pts[1][1]); }
      }
      ctx.stroke();
    }
  }
  function hazard(ctx, x, y, w, h, n, chip) {
    ctx.save(); ctx.beginPath(); ctx.rect(x, y, w, h); ctx.clip();
    for (let i = -h; i < w + h; i += 36) { ctx.fillStyle = "rgba(227,178,31,.85)"; ctx.beginPath(); ctx.moveTo(x + i, y); ctx.lineTo(x + i + 18, y); ctx.lineTo(x + i + 18 - h, y + h); ctx.lineTo(x + i - h, y + h); ctx.fill(); }
    ctx.restore();
    // Chipped: bare metal shows through where the paint has worn.
    field(ctx, Math.ceil(x + w), Math.ceil(y + h), (px, py) => (py < y || px < x || n.fbm(px / 18, py / 18, 4) < chip ? null : [70, 70, 64, 210]));
  }

  // ---------- each maker ----------
  // Each returns [surface(ctx, w, h, r, n), wear(ctx, s, r, n)]; the wear tile is s x s.
  const M = {
    rsi: [(c, w, h, r, n) => {
      const lg = c.createLinearGradient(0, 0, 0, h); lg.addColorStop(0, "#0B1626"); lg.addColorStop(1, "#060B14"); c.fillStyle = lg; c.fillRect(0, 0, w, h);
      grime(c, w, h, n, 260, [40, 80, 150], 0.5, -0.05);
      stars(c, w, h, r, 260, "200,220,255");
      c.strokeStyle = "rgba(201,165,76,.09)"; c.lineWidth = 1;
      for (let i = 0; i < 3; i++) { c.beginPath(); c.ellipse(w * (0.3 + r() * 0.4), h * 1.1, w * (0.5 + i * 0.25), h * (0.35 + i * 0.18), -0.12, Math.PI, Math.PI * 2); c.stroke(); }
    }, (c, s, r, n) => { scratches(c, s, s, r, 70, -0.3, 0.05); smudges(c, s, s, r, 1); }],
    hud: [(c, w, h, r, n) => {
      c.fillStyle = "#03070D"; c.fillRect(0, 0, w, h);
      grime(c, w, h, n, 300, [20, 60, 120], 0.6, -0.1);
      stars(c, w, h, r, 300, "190,215,255");
      c.strokeStyle = "rgba(76,163,255,.06)"; c.lineWidth = 1;
      for (let x = 0; x < w; x += 32) { c.beginPath(); c.moveTo(x + 0.5, 0); c.lineTo(x + 0.5, h); c.stroke(); }
      for (let y = 0; y < h; y += 32) { c.beginPath(); c.moveTo(0, y + 0.5); c.lineTo(w, y + 0.5); c.stroke(); }
      c.strokeStyle = "rgba(76,163,255,.14)"; c.lineWidth = 1.5;
      const R = Math.min(w, h) * 0.42; c.beginPath(); c.arc(w / 2, h / 2, R, -2.6, -0.5); c.stroke(); c.beginPath(); c.arc(w / 2, h / 2, R, 0.55, 2.6); c.stroke();
      for (let a = 0; a < 72; a++) { const t = (a / 72) * Math.PI * 2, L = a % 6 ? 6 : 14; c.beginPath(); c.moveTo(w / 2 + Math.cos(t) * (R + 4), h / 2 + Math.sin(t) * (R + 4)); c.lineTo(w / 2 + Math.cos(t) * (R + 4 + L), h / 2 + Math.sin(t) * (R + 4 + L)); c.stroke(); }
    }, (c, s, r) => {
      c.fillStyle = "rgba(120,190,255,.03)"; for (let y = 0; y < s; y += 3) c.fillRect(0, y, s, 1);
    }],
    mobiglas: [(c, w, h, r, n) => {
      c.fillStyle = "#021312"; c.fillRect(0, 0, w, h);
      grime(c, w, h, n, 220, [0, 200, 170], 0.7, -0.02);
      const S = 26, hx = S * Math.sqrt(3);
      for (let y = 0, row = 0; y < h + S; y += S * 1.5, row++) for (let x = row % 2 ? hx / 2 : 0; x < w + hx; x += hx) {
        const v = n.fbm(x / 300, y / 300, 3) + 0.2;
        if (v <= 0) continue;
        c.strokeStyle = `rgba(0,240,200,${Math.min(0.22, v * 0.45)})`; c.lineWidth = 1;
        c.beginPath(); for (let i = 0; i < 6; i++) { const a = Math.PI / 3 * i + Math.PI / 6; c.lineTo(x + Math.cos(a) * S * 0.92, y + Math.sin(a) * S * 0.92); } c.closePath(); c.stroke();
      }
    }, (c, s, r) => { smudges(c, s, s, r, 1.2); }],
    drake: [(c, w, h, r, n) => {
      c.fillStyle = "#24251F"; c.fillRect(0, 0, w, h);
      plates(c, w, h, r, { depth: 3, rivets: 26, rust: 0.12, vary: 14 });
      grime(c, w, h, n, 140, [12, 11, 8], 1.4, 0.02);
      scratches(c, w, h, r, 900, 0.2, 0.07);
      hazard(c, 0, h - 26, w, 26, n, 0.06);
      hazard(c, 0, 0, w * 0.35, 14, n, 0.0);
    }, (c, s, r, n) => { grime(c, s, s, n, 60, [8, 8, 6], 1.1, 0.06); scratches(c, s, s, r, 160, 0.2, 0.1); scratches(c, s, s, r, 40, 1.4, 0.12, false); }],
    anvil: [(c, w, h, r, n) => {
      c.fillStyle = "#353A2B"; c.fillRect(0, 0, w, h);
      plates(c, w, h, r, { depth: 3, rivets: 22, rust: 0.05, vary: 10, rivetColor: "rgba(140,146,112,.6)" });
      grime(c, w, h, n, 70, [150, 150, 136], 0.9, 0.18);
      grime(c, w, h, n, 200, [16, 18, 10], 1.2, 0.03);
      scratches(c, w, h, r, 500, -0.2, 0.06);
    }, (c, s, r, n) => { grime(c, s, s, n, 40, [170, 170, 150], 0.8, 0.2); scratches(c, s, s, r, 100, -0.2, 0.09); }],
    aegis: [(c, w, h, r, n) => {
      c.fillStyle = "#121417"; c.fillRect(0, 0, w, h);
      for (let y = 0; y < h; y += 6) for (let x = (y / 6) % 2 ? 3 : 0; x < w; x += 6) { c.fillStyle = "rgba(255,255,255,.025)"; c.fillRect(x, y, 3, 6); }
      c.strokeStyle = "rgba(0,0,0,.6)"; c.lineWidth = 2;
      for (let i = 0; i < 6; i++) { const x = r() * w; c.beginPath(); c.moveTo(x, 0); c.lineTo(x + h * 0.6, h); c.stroke(); }
      c.fillStyle = "rgba(200,53,62,.5)"; c.fillRect(0, h - 4, w * 0.22, 4);
    }, (c, s, r) => { scratches(c, s, s, r, 50, 0.8, 0.04); }],
    misc: [(c, w, h, r, n) => {
      c.fillStyle = "#B4B7B1"; c.fillRect(0, 0, w, h);
      for (let y = 0; y < h; y += 90 + Math.floor(r() * 60)) { c.fillStyle = "rgba(0,0,0,.12)"; c.fillRect(0, y, w, 2); c.fillStyle = "rgba(255,255,255,.3)"; c.fillRect(0, y + 2, w, 1); }
      for (let i = 0; i < 140; i++) { const x = r() * w, L = 30 + r() * 160, lg = c.createLinearGradient(x, 0, x, L); lg.addColorStop(0, "rgba(60,60,50,.12)"); lg.addColorStop(1, "rgba(60,60,50,0)"); c.fillStyle = lg; c.fillRect(x, r() * h, 1.5, L); }
      grime(c, w, h, n, 180, [80, 80, 70], 0.5, 0.05);
    }, (c, s, r) => { scratches(c, s, s, r, 60, 0, 0.05); }],
    argo: [(c, w, h, r, n) => {
      c.fillStyle = "#151515"; c.fillRect(0, 0, w, h);
      plates(c, w, h, r, { depth: 2, rivets: 40, vary: 6 });
      hazard(c, 0, 0, w, 22, n, 0.08); hazard(c, 0, h - 22, w, 22, n, 0.08);
      scratches(c, w, h, r, 500, 0, 0.06);
    }, (c, s, r, n) => { scratches(c, s, s, r, 120, 0, 0.08); grime(c, s, s, n, 50, [10, 10, 10], 0.9, 0.1); }],
    cnou: [(c, w, h, r, n) => {
      c.fillStyle = "#E7DFD0"; c.fillRect(0, 0, w, h);
      field(c, w, h, (x, y) => (r() < 0.04 ? [120, 100, 80, 40] : null));
      const cols = ["#E2662F", "#F2A541", "#F5D06B"];
      cols.forEach((col, i) => { c.strokeStyle = col; c.lineWidth = 18; c.beginPath(); c.moveTo(-40, h * 0.78 + i * 20); c.bezierCurveTo(w * 0.3, h * 0.6 + i * 20, w * 0.6, h * 0.95 + i * 20, w + 40, h * 0.7 + i * 20); c.stroke(); });
    }, (c, s, r) => { scratches(c, s, s, r, 50, 0.3, 0.05, false); }],
    origin: [(c, w, h, r, n) => {
      const lg = c.createLinearGradient(0, 0, 0, h); lg.addColorStop(0, "#F7F6F2"); lg.addColorStop(1, "#D9D5CB"); c.fillStyle = lg; c.fillRect(0, 0, w, h);
      for (let i = 0; i < 3; i++) { const y = h * (0.15 + i * 0.3), g = c.createLinearGradient(0, y - 60, 0, y + 60); g.addColorStop(0, "rgba(255,255,255,0)"); g.addColorStop(0.5, "rgba(255,255,255,.55)"); g.addColorStop(1, "rgba(255,255,255,0)"); c.fillStyle = g; c.fillRect(0, y - 60, w, 120); }
      c.strokeStyle = "rgba(185,151,91,.5)"; c.lineWidth = 1; c.beginPath(); c.moveTo(0, h * 0.62); c.lineTo(w, h * 0.58); c.stroke();
    }, (c, s, r) => { smudges(c, s, s, r, 0.8); }],
    crusader: [(c, w, h, r, n) => {
      const lg = c.createLinearGradient(0, 0, 0, h); lg.addColorStop(0, "#7FB6E6"); lg.addColorStop(0.6, "#C7DFF4"); lg.addColorStop(1, "#F6E6D4"); c.fillStyle = lg; c.fillRect(0, 0, w, h);
      const sun = c.createRadialGradient(w * 0.78, h * 0.2, 0, w * 0.78, h * 0.2, w * 0.5); sun.addColorStop(0, "rgba(255,250,235,.85)"); sun.addColorStop(1, "rgba(255,250,235,0)"); c.fillStyle = sun; c.fillRect(0, 0, w, h);
      // Cumulus: noise thresholded softly, brighter on top.
      field(c, w, h, (x, y) => { const v = n.fbm(x / 240, y / 120, 6) + (y / h) * 0.35 - 0.12; if (v < 0) return null; const t = Math.min(1, v * 3.2); const lift = Math.max(0, n.fbm(x / 240, (y - 8) / 120, 4) - n.fbm(x / 240, y / 120, 4)) * 300; return [Math.min(255, 238 + lift), Math.min(255, 242 + lift), 250, t * 235]; });
      // Orison's platforms, far off in the haze.
      for (let i = 0; i < 5; i++) { const x = r() * w, y = h * (0.7 + r() * 0.2), s = 30 + r() * 80; c.fillStyle = "rgba(120,140,170,.25)"; c.beginPath(); c.ellipse(x, y, s, s * 0.12, 0, 0, 7); c.fill(); c.fillRect(x - 2, y, 4, s * 0.6); }
    }, (c, s, r) => { smudges(c, s, s, r, 0.9); }],
    banu: [(c, w, h, r, n) => {
      c.fillStyle = "#21150C"; c.fillRect(0, 0, w, h);
      flow(c, w, h, n, r, { count: 2200, steps: 60, scale: 380, turn: 4, width: 1, colors: ["rgba(214,129,43,.07)", "rgba(40,190,170,.06)", "rgba(190,60,150,.06)", "rgba(240,200,140,.05)"] });
      grime(c, w, h, n, 160, [10, 6, 2], 0.9, 0.05);
    }, (c, s, r, n) => { grime(c, s, s, n, 50, [255, 200, 130], 0.25, 0.12); }],
    aopoa: [(c, w, h, r, n) => {
      c.fillStyle = "#DDE9E3"; c.fillRect(0, 0, w, h);
      contours(c, w, h, n, { scale: 420, levels: 14, width: 1.2, color: (L) => `rgba(31,${110 + L * 6},90,${0.08 + (L % 3 === 0 ? 0.1 : 0)})` });
    }, (c, s, r) => { smudges(c, s, s, r, 0.6); }],
    esperia: [(c, w, h, r, n) => {
      c.fillStyle = "#100A19"; c.fillRect(0, 0, w, h);
      field(c, w, h, (x, y) => { const v = n.fbm(x / 300, y / 300, 5); if (v < -0.05) return null; const k = Math.min(1, (v + 0.05) * 2.2); return [80 + 140 * k, 40 + 30 * k, 110 + 30 * k, k * 120]; });
      stars(c, w, h, r, 200, "255,190,170");
      c.strokeStyle = "rgba(255,122,92,.18)"; c.lineWidth = 2; c.beginPath(); c.arc(w * 1.05, h * 0.45, h * 0.6, Math.PI * 0.6, Math.PI * 1.4); c.stroke();
      c.strokeStyle = "rgba(168,139,255,.14)"; c.beginPath(); c.arc(-w * 0.05, h * 0.4, h * 0.55, -Math.PI * 0.4, Math.PI * 0.4); c.stroke();
    }, (c, s, r) => { smudges(c, s, s, r, 0.8); }],
  };

  // ---------- apply ----------
  const urls = {};
  let job = 0;
  async function blobURL(c, type, q) { return new Promise((res) => c.toBlob((b) => res(b ? URL.createObjectURL(b) : ""), type, q)); }
  async function apply(theme, el = document.documentElement) {
    const def = M[theme];
    const id = ++job;
    if (!def) { el.style.removeProperty("--art-surface"); el.style.removeProperty("--art-wear"); return; }
    // Surfaces are drawn at 60% and scaled up: they sit behind frosted or textured panels.
    const k = 0.6, w = Math.max(320, Math.round(innerWidth * k)), h = Math.max(320, Math.round(innerHeight * k));
    const key = `${theme}:${w}x${h}`;
    if (!urls[key]) {
      const seed = hash(theme);
      await new Promise((r) => setTimeout(r, 0));
      const [sc, sx] = canvas(w, h); def[0](sx, w, h, rng(seed), noise2(seed));
      const [wc, wx] = canvas(384, 384); def[1](wx, 384, rng(seed + 1), noise2(seed + 1));
      urls[key] = { surface: await blobURL(sc, "image/jpeg", 0.86), wear: await blobURL(wc, "image/png") };
    }
    if (id !== job) return;
    el.style.setProperty("--art-surface", `url("${urls[key].surface}")`);
    el.style.setProperty("--art-wear", `url("${urls[key].wear}")`);
  }

  window.DeckArt = { apply };
})();
