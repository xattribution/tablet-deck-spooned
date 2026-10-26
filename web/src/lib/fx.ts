// Effects: soft points of light and glows drawn on one canvas over the deck, added
// together like light ("lighter"), so overlapping glows brighten instead of piling up
// as shapes. Everything moves by physics (velocity, drag, gravity) and fades with an
// ease, and the loop only runs while something is alive.

type P = {
  x: number; y: number; vx: number; vy: number;
  age: number; life: number; size: number; grow: number;
  drag: number; g: number; color: string; stretch: number; alpha: number;
  delay: number; swirl?: { cx: number; cy: number; r: number; a: number; va: number; vr: number };
  emit?: (p: P, dt: number) => void;
};

const reduced = matchMedia("(prefers-reduced-motion: reduce)");
let canvas: HTMLCanvasElement | null = null;
let ctx: CanvasRenderingContext2D | null = null;
let dpr = 1;
let W = 0, H = 0;
const parts: P[] = [];
let running = false;
let lastT = 0;
const sprites = new Map<string, HTMLCanvasElement>();

function sprite(color: string): HTMLCanvasElement {
  let s = sprites.get(color);
  if (s) return s;
  s = document.createElement("canvas");
  s.width = s.height = 128;
  const g = s.getContext("2d")!;
  const grad = g.createRadialGradient(64, 64, 0, 64, 64, 64);
  // A white-hot core into the color, falling off smoothly: reads as light, not a disc.
  grad.addColorStop(0, "rgba(255,255,255,1)");
  grad.addColorStop(0.12, color);
  grad.addColorStop(0.35, withAlpha(color, 0.45));
  grad.addColorStop(0.65, withAlpha(color, 0.12));
  grad.addColorStop(1, withAlpha(color, 0));
  g.fillStyle = grad;
  g.fillRect(0, 0, 128, 128);
  sprites.set(color, s);
  return s;
}

function withAlpha(color: string, a: number): string {
  const c = parseColor(color);
  return `rgba(${c[0]},${c[1]},${c[2]},${a})`;
}
const colorCache = new Map<string, [number, number, number]>();
function parseColor(color: string): [number, number, number] {
  const hit = colorCache.get(color);
  if (hit) return hit;
  let out: [number, number, number] = [200, 200, 200];
  const m = color.trim().match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (m) {
    const h = m[1].length === 3 ? m[1].split("").map((x) => x + x).join("") : m[1];
    out = [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
  } else {
    const r = color.match(/rgba?\(([^)]+)\)/);
    if (r) { const v = r[1].split(",").map((x) => parseFloat(x)); out = [v[0] | 0, v[1] | 0, v[2] | 0]; }
  }
  colorCache.set(color, out);
  return out;
}

function ensure() {
  if (canvas) return;
  canvas = document.createElement("canvas");
  canvas.id = "fx";
  canvas.setAttribute("aria-hidden", "true");
  document.body.appendChild(canvas);
  ctx = canvas.getContext("2d", { alpha: true })!;
  resize();
  addEventListener("resize", resize);
}

function resize() {
  if (!canvas || !ctx) return;
  dpr = Math.min(2, window.devicePixelRatio || 1);
  W = innerWidth; H = innerHeight;
  canvas.width = Math.round(W * dpr);
  canvas.height = Math.round(H * dpr);
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}

function add(p: Partial<P> & { x: number; y: number }) {
  parts.push({ vx: 0, vy: 0, age: 0, life: 600, size: 10, grow: 0, drag: 0, g: 0, color: "#ffffff", stretch: 0, alpha: 1, delay: 0, ...p });
  if (!running) { running = true; lastT = performance.now(); requestAnimationFrame(frame); }
}

const ease = (t: number) => 1 - t * t * (3 - 2 * t); // smoothstep fade-out

function frame(t: number) {
  if (!ctx) return;
  const dt = Math.min(34, t - lastT);
  lastT = t;
  ctx.clearRect(0, 0, W, H);
  ctx.globalCompositeOperation = "lighter";
  for (let i = parts.length - 1; i >= 0; i--) {
    const p = parts[i];
    if (p.delay > 0) { p.delay -= dt; continue; }
    p.age += dt;
    if (p.age >= p.life) { parts.splice(i, 1); continue; }
    const k = dt / 1000;
    if (p.swirl) {
      const s = p.swirl;
      s.a += s.va * k; s.r = Math.max(0, s.r + s.vr * k);
      const nx = s.cx + Math.cos(s.a) * s.r, ny = s.cy + Math.sin(s.a) * s.r;
      p.vx = (nx - p.x) / Math.max(k, 0.001); p.vy = (ny - p.y) / Math.max(k, 0.001);
      p.x = nx; p.y = ny;
    } else {
      const d = Math.pow(1 - p.drag, dt / 16.67);
      p.vx *= d; p.vy = p.vy * d + p.g * k;
      p.x += p.vx * k; p.y += p.vy * k;
    }
    if (p.emit) p.emit(p, dt);
    const life = p.age / p.life;
    const a = p.alpha * ease(life) * Math.min(1, p.age / 60); // a quick fade in, a smooth fade out
    const size = p.size * (1 + p.grow * life);
    const img = sprite(p.color);
    ctx.globalAlpha = a;
    if (p.stretch > 0) {
      const sp = Math.hypot(p.vx, p.vy);
      const len = Math.max(size, size + sp * p.stretch);
      ctx.save();
      ctx.translate(p.x, p.y);
      ctx.rotate(Math.atan2(p.vy, p.vx));
      ctx.drawImage(img, -len + size / 2, -size / 2, len, size);
      ctx.restore();
    } else {
      ctx.drawImage(img, p.x - size / 2, p.y - size / 2, size, size);
    }
  }
  ctx.globalAlpha = 1;
  if (parts.length) requestAnimationFrame(frame);
  else { running = false; ctx.clearRect(0, 0, W, H); }
}

// ---------- effects ----------

const rnd = (a: number, b: number) => a + Math.random() * (b - a);
function catColor(el: Element | null): string {
  if (!el) return "#c9a54c";
  const c = getComputedStyle(el).getPropertyValue("--cat").trim() || getComputedStyle(el).getPropertyValue("--accent").trim();
  return c || "#c9a54c";
}
function centerOf(el: Element) {
  const r = el.getBoundingClientRect();
  return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: r.width, h: r.height, r };
}

function bloom(x: number, y: number, size: number, color: string, life = 420, alpha = 0.55, delay = 0) {
  add({ x, y, size, color, life, alpha, grow: 0.35, delay });
}
function motes(x: number, y: number, n: number, color: string, speed: number, opts: Partial<P> = {}) {
  for (let i = 0; i < n; i++) {
    const a = rnd(0, Math.PI * 2), s = speed * rnd(0.5, 1);
    add({ x, y, vx: Math.cos(a) * s, vy: Math.sin(a) * s, size: rnd(5, 10), color, life: rnd(380, 620), drag: 0.08, ...opts });
  }
}
function ring(x: number, y: number, n: number, color: string, speed: number, opts: Partial<P> = {}) {
  for (let i = 0; i < n; i++) {
    const a = (i / n) * Math.PI * 2;
    add({ x, y, vx: Math.cos(a) * speed, vy: Math.sin(a) * speed, size: 7, color, life: 650, drag: 0.04, ...opts });
  }
}

const FX: Record<string, (el: Element, x: number, y: number) => void> = {
  press(el, x, y) {
    const c = catColor(el), { w, h } = centerOf(el);
    bloom(x, y, Math.min(260, Math.max(w, h) * 1.1), c, 380, 0.42);
    motes(x, y, 7, c, 120, { size: 6, life: 420 });
  },
  flares(el) {
    const { x, y } = centerOf(el);
    const colors = ["#ffe2a0", "#ffb04a", "#ff7a2e"];
    for (let b = 0; b < 3; b++) {
      for (let i = 0; i < 12; i++) {
        const a = rnd(-Math.PI * 0.95, -Math.PI * 0.05), s = rnd(380, 720);
        add({ x, y, vx: Math.cos(a) * s, vy: Math.sin(a) * s, g: 900, drag: 0.02, size: rnd(7, 12), stretch: 0.03, color: colors[i % 3], life: rnd(700, 1100), delay: b * 130 });
      }
      bloom(x, y, 180, "#ffb04a", 300, 0.5, b * 130);
    }
  },
  noise(el) {
    const { x, y } = centerOf(el);
    for (let r = 0; r < 3; r++) ring(x, y, 36, r % 2 ? "#7fe7ff" : "#a783ff", 260, { delay: r * 140, life: 700 });
    bloom(x, y, 220, "#7fe7ff", 500, 0.35);
  },
  missile(el) {
    const { x, y } = centerOf(el);
    const c = catColor(el);
    bloom(x, y, 200, "#ffd9a0", 260, 0.6);
    add({
      x, y, vx: rnd(-40, 40), vy: -260, g: -1400, drag: 0, size: 16, color: "#fff2d0", life: 900,
      emit(p) {
        add({ x: p.x + rnd(-2, 2), y: p.y + rnd(2, 6), vx: rnd(-20, 20), vy: rnd(40, 90), size: rnd(8, 14), drag: 0.06, color: Math.random() < 0.5 ? "#ffb04a" : c, life: rnd(300, 520), alpha: 0.7 });
      },
    });
  },
  quantum(el) {
    const { x, y, w, h } = centerOf(el);
    const R = Math.max(w, h) * 0.75, c = "#a783ff";
    for (let i = 0; i < 44; i++) {
      const a = (i / 44) * Math.PI * 2;
      add({ x: x + Math.cos(a) * R, y: y + Math.sin(a) * R, size: rnd(6, 10), color: i % 3 ? c : "#7fb8ff", life: 1150, swirl: { cx: x, cy: y, r: R, a, va: 5.5, vr: -R / 1.1 } });
    }
    bloom(x, y, Math.max(w, h) * 1.6, c, 600, 0.7, 1050);
    motes(x, y, 0, c, 0);
    setTimeout(() => motes(x, y, 22, "#cbb8ff", 420, { stretch: 0.02, life: 700 }), 1080);
  },
  warp(el) {
    const { x, y } = centerOf(el);
    for (let i = 0; i < 34; i++) {
      const a = rnd(0, Math.PI * 2), s = rnd(500, 1100);
      add({ x, y, vx: Math.cos(a) * s, vy: Math.sin(a) * s, size: rnd(5, 9), stretch: 0.05, color: i % 2 ? "#a783ff" : "#ffffff", life: 520, drag: 0 });
    }
    bloom(x, y, 360, "#a783ff", 700, 0.6);
  },
  shield(el) {
    const { x, y, w, h } = centerOf(el);
    const c = catColor(el);
    bloom(x, y, Math.max(w, h) * 1.5, c, 520, 0.5);
    for (let i = 0; i < 26; i++) {
      const a = (i / 26) * Math.PI * 2;
      add({ x: x + Math.cos(a) * w * 0.35, y: y + Math.sin(a) * h * 0.35, vx: Math.cos(a) * 90, vy: Math.sin(a) * 90, size: 6, color: c, life: 600, drag: 0.05 });
    }
  },
  lock(el) {
    const { x, y, w, h } = centerOf(el);
    const c = "#ff5a52";
    for (const [dx, dy] of [[-1, -1], [1, -1], [-1, 1], [1, 1]]) {
      const sx = x + dx * w * 0.9, sy = y + dy * h * 0.9;
      add({ x: sx, y: sy, vx: (x - sx) * 2.2, vy: (y - sy) * 2.2, drag: 0.09, size: 14, color: c, life: 520 });
    }
    bloom(x, y, Math.max(w, h), c, 420, 0.5, 240);
  },
  power(el) {
    const { x, y, w, h } = centerOf(el);
    const c = catColor(el);
    bloom(x, y, Math.max(w, h) * 1.4, c, 700, 0.55);
    for (let i = 0; i < 16; i++) add({ x: x + rnd(-w / 3, w / 3), y: y + rnd(0, h / 3), vx: rnd(-10, 10), vy: rnd(-160, -60), size: rnd(5, 9), color: c, life: rnd(700, 1100), drag: 0.02 });
  },
  lights(el) {
    const { x, y } = centerOf(el);
    bloom(x, y, 520, "#fff3d6", 650, 0.45);
  },
  doors(el) {
    const { x, y, w } = centerOf(el);
    const c = catColor(el);
    add({ x, y, vx: -w * 1.4, size: 30, color: c, life: 520, drag: 0.08, stretch: 0.02 });
    add({ x, y, vx: w * 1.4, size: 30, color: c, life: 520, drag: 0.08, stretch: 0.02 });
  },
  scan(el) {
    const { x, y } = centerOf(el);
    ring(x, y, 54, "#a783ff", 340, { life: 1000, drag: 0.01, size: 6 });
    ring(x, y, 54, "#7fb8ff", 220, { life: 1000, drag: 0.01, size: 5, delay: 160 });
  },
  laser(el) {
    const { x, y, h } = centerOf(el);
    for (let i = 0; i < 26; i++) add({ x: x + rnd(-3, 3), y: y - i * (h / 10), size: rnd(8, 14), color: i % 2 ? "#ff5a52" : "#ffd1c9", life: 380, delay: i * 6 });
  },
  alarm(el) {
    const { x, y } = centerOf(el);
    bloom(x, y, 420, "#ff4a3d", 420, 0.7);
    bloom(x, y, 420, "#ff4a3d", 420, 0.7, 450);
  },
  tick(el, x, y) {
    add({ x, y, size: 16, color: catColor(el), life: 220, alpha: 0.7 });
  },
  lit(el) {
    const { x, y, w, h } = centerOf(el);
    bloom(x, y, Math.max(w, h) * 1.2, catColor(el), 520, 0.4);
  },
};

export const fx = {
  play(name: string, el: Element, x?: number, y?: number) {
    if (reduced.matches) return;
    ensure();
    const c = centerOf(el);
    (FX[name] || FX.press)(el, x ?? c.x, y ?? c.y);
  },
  press(el: Element, x: number, y: number) { fx.play("press", el, x, y); },
  flashControl(id: string) {
    const el = document.querySelector(`[data-id="${CSS.escape(id)}"]`);
    if (el) fx.play("press", el);
  },
  hit() {
    if (reduced.matches) return;
    ensure();
    for (let i = 0; i < 18; i++) {
      const side = i % 4, t = Math.random();
      const x = side === 0 ? 0 : side === 1 ? W : t * W, y = side === 2 ? 0 : side === 3 ? H : t * H;
      add({ x, y, vx: (W / 2 - x) * 0.6, vy: (H / 2 - y) * 0.6, drag: 0.07, size: rnd(14, 26), color: "#ff3b30", life: 520, alpha: 0.8 });
    }
  },
};

// Which effect a control gets, by what it is.
const BY_ID: [RegExp, string][] = [
  [/decoy|panic/, "flares"], [/noise/, "noise"], [/missile/, "missile"], [/scan/, "scan"], [/fire-laser|fracture/, "laser"],
  [/^lights$/, "lights"], [/door/, "doors"], [/^shields-/, "shield"], [/^prev-|target/, "lock"], [/flight-ready|power-on/, "power"],
  [/self-d|eject/, "alarm"], [/quantum/, "quantum"],
];
export function effectFor(id: string, explicit?: string): string {
  if (explicit) return explicit;
  for (const [re, n] of BY_ID) if (re.test(id)) return n;
  return "press";
}
