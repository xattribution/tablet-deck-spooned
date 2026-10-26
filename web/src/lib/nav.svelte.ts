// Where the stage is: comms above the deck, home below it.
export const nav = $state({ section: "deck" as "comms" | "deck" | "home", tab: "views" as string });

let stage: HTMLElement | null = null;
const els: Record<string, HTMLElement | null> = {};
export function bindStage(s: HTMLElement, parts: Record<string, HTMLElement | null>) {
  stage = s;
  Object.assign(els, parts);
}
export function go(section: "comms" | "deck" | "home", opts: { tab?: string; instant?: boolean } = {}) {
  if (opts.tab) nav.tab = opts.tab;
  const el = els[section];
  if (!stage || !el) return;
  stage.scrollTo({ top: el.offsetTop, behavior: opts.instant ? "instant" : "smooth" } as ScrollToOptions);
}
export function watch() {
  if (!stage) return;
  const s = stage;
  const pick = () => {
    const mid = s.scrollTop + s.clientHeight / 2;
    for (const k of ["comms", "deck", "home"] as const) {
      const el = els[k];
      if (el && mid >= el.offsetTop && mid < el.offsetTop + el.offsetHeight) { if (nav.section !== k) nav.section = k; return; }
    }
  };
  s.addEventListener("scroll", () => requestAnimationFrame(pick), { passive: true });
}

// iPad Safari doesn't shrink the page for the keyboard; tell the CSS how much it covers.
if (window.visualViewport) {
  const vv = window.visualViewport;
  const set = () => document.documentElement.style.setProperty("--kb", `${Math.max(0, Math.round(window.innerHeight - vv.height - vv.offsetTop))}px`);
  vv.addEventListener("resize", set);
  vv.addEventListener("scroll", set);
}
