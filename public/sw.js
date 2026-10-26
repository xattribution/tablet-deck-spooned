// Keeps the deck's own files (scripts, styles, icons) for quick starts and a deck that
// still opens when the PC is briefly away. Layouts, pairing and presses always go to
// the PC. __VERSION__ is filled in by the server, so a new version replaces the cache.
const CACHE = "tabletdeck-__VERSION__";
const SHELL = ["/", "/index.html", "/themes.css", "/deck.css", "/hud.css", "/fx.css", "/intro.css", "/crew.css",
  "/icons.js", "/i18n.js", "/fx.js", "/canvas.js", "/qr.js", "/app.js", "/crew.js", "/intro.js", "/awake.js",
  "/icon.svg", "/icon-192.png", "/manifest.webmanifest"];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
    .then(() => self.clients.claim()));
});

self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== "GET" || url.origin !== location.origin) return;
  // Pages and the deck's files: the network first (always current), the cache when it's away.
  const page = e.request.mode === "navigate";
  const file = SHELL.includes(url.pathname);
  if (!page && !file) return;
  e.respondWith(fetch(e.request).then((r) => {
    if (r.ok && file) { const copy = r.clone(); caches.open(CACHE).then((c) => c.put(e.request, copy)); }
    return r;
  }).catch(() => caches.match(page ? "/index.html" : e.request, { ignoreSearch: page })));
});
