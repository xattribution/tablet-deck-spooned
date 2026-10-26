// The app's own files: from the network when the PC answers, from the cache when it
// doesn't (a reload while the PC restarts still opens the app).
const CACHE = "tabletdeck-app-1";
self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (e) => e.waitUntil(self.clients.claim()));
self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== "GET" || url.origin !== location.origin || url.pathname.startsWith("/api/") || url.pathname.startsWith("/ws")) return;
  const page = e.request.mode === "navigate";
  if (!page && !url.pathname.startsWith("/assets/") && !/\.(png|svg|webmanifest)$/.test(url.pathname)) return;
  e.respondWith(fetch(e.request).then((r) => {
    if (r.ok) { const copy = r.clone(); caches.open(CACHE).then((c) => c.put(page ? "/" : e.request, copy)); }
    return r;
  }).catch(() => caches.match(page ? "/" : e.request)));
});
