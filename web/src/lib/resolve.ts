// Turns a panel's layout (docs/PANELS.md) and its controls into a tree the components
// draw. Every control in the panel appears exactly once: anything the layout doesn't
// place lands in a grid at the end, or where the layout says "rest". A dangerous
// control always gets a cover.
import type { Control, Group } from "./types";

export type N =
  | { k: "box"; dir: "row" | "col" | "flip"; hazard?: boolean; flex: number; kids: N[] }
  | { k: "key"; c: Control; flex: number; label?: string; icon?: string; size?: string; effect?: string; spool?: boolean; toggle?: boolean }
  | { k: "lever"; c: Control; flex: number }
  | { k: "fader"; up?: Control; down?: Control; max?: Control; min?: Control; label: string; icon?: string; cat?: string; horizontal?: boolean; flex: number }
  | { k: "pair"; cs: Control[]; names: string[]; flex: number }
  | { k: "dial"; cs: Control[]; center?: Control; flex: number; store: string }
  | { k: "shield"; faces: Record<string, Control | undefined>; flex: number }
  | { k: "guard"; c: Control; flex: number }
  | { k: "slide"; c: Control; flex: number }
  | { k: "grid"; cs: Control[]; small?: boolean; flex: number }
  | { k: "modes"; store: string; options: { name: string; value: string }[]; current: string; inner: N | null; flex: number }
  | { k: "talk"; flex: number }
  | { k: "route"; flex: number }
  | { k: "transcript"; flex: number }
  | { k: "radio"; discord?: Control; game?: Control; flex: number };

const SUB = ["up", "down", "max", "min", "front", "back", "left", "right", "top", "bottom", "reset", "discord", "game"];

// The controls a layout places (like ControlRefs on the PC): not labels, icons or options.
function refs(n: any): string[] {
  const out: string[] = [];
  const add = (v: any) => {
    if (typeof v === "string") { if (v !== "rest") out.push(v); }
    else if (Array.isArray(v)) v.forEach((x) => typeof x === "string" && out.push(x));
    else if (v && typeof v === "object") SUB.forEach((k) => typeof v[k] === "string" && out.push(v[k]));
  };
  const walk = (m: any) => {
    if (!m || typeof m !== "object") return;
    for (const [k, v] of Object.entries(m)) {
      if (k === "row" || k === "col" || k === "flip") (v as any[] || []).forEach(walk);
      else if (k === "options") (v as any[] || []).forEach((o) => walk(o && o.layout));
      else if (["key", "toggle", "lever", "guard", "slide", "pair", "dial", "center", "grid", "fader", "shieldpad", "radio"].includes(k)) add(v);
    }
  };
  walk(n);
  return out;
}

export function resolve(g: Group, modes: Record<string, string>, guest: boolean): N {
  const by = new Map<string, Control>();
  for (const c of g.controls) by.set(c.cid || c.id, c);
  const used = new Set<Control>();
  const take = (id?: string): Control | undefined => {
    if (!id) return undefined;
    const c = by.get(`${g.id}.${id}`) || by.get(id);
    if (c && !used.has(c)) { used.add(c); return c; }
    return undefined;
  };
  const peek = (id?: string) => (id ? by.get(`${g.id}.${id}`) || by.get(id) : undefined);
  let restAt: { flex: number; small: boolean } | null = null;
  const REST = { k: "grid", cs: [], flex: 1 } as N;

  const node = (n: any): N | null => {
    if (!n || typeof n !== "object") return null;
    const flex = typeof n.flex === "number" ? n.flex : 1;
    for (const dir of ["row", "col", "flip"] as const) {
      if (Array.isArray(n[dir])) {
        const kids = n[dir].map(node).filter(Boolean) as N[];
        return kids.length ? { k: "box", dir, hazard: n.style === "hazard", flex, kids } : null;
      }
    }
    const keyish = n.key ?? n.toggle;
    if (keyish != null) {
      const p = peek(keyish);
      if (p && p.danger && !used.has(p)) { const c = take(keyish)!; return { k: "guard", c, flex }; }
      const c = take(keyish);
      return c ? { k: "key", c, flex, label: n.label, icon: n.icon, size: n.size, effect: n.fx, spool: !!n.spool, toggle: n.toggle != null } : null;
    }
    if (n.lever) { const c = take(n.lever); return c ? { k: "lever", c, flex } : null; }
    if (n.fader) {
      const f = n.fader;
      const upc = take(f.up), dn = take(f.down);
      if (!upc && !dn) return null;
      return { k: "fader", up: upc, down: dn, max: take(f.max), min: take(f.min), label: f.label || "", icon: f.icon, cat: f.cat, horizontal: !!f.horizontal, flex };
    }
    if (n.pair) { const cs = (n.pair as string[]).map(take).filter(Boolean) as Control[]; return cs.length ? { k: "pair", cs, names: n.names || [], flex } : null; }
    if (n.dial) {
      const cs = (n.dial as string[]).map(take).filter(Boolean) as Control[];
      return cs.length ? { k: "dial", cs, center: take(n.center), flex, store: `dial:${g.id}` } : null;
    }
    if (n.shieldpad) {
      const faces: Record<string, Control | undefined> = {};
      for (const f of ["front", "back", "left", "right", "top", "bottom", "reset"]) faces[f] = take(n.shieldpad[f]);
      return { k: "shield", faces, flex };
    }
    if (n.guard) { const c = take(n.guard); return c ? { k: "guard", c, flex } : null; }
    if (n.slide) { const c = take(n.slide); return c ? { k: "slide", c, flex } : null; }
    if (n.grid) {
      if (n.grid === "rest") { restAt = { flex, small: !!n.small }; return REST; }
      const cs = (n.grid as string[]).map(take).filter(Boolean) as Control[];
      return cs.length ? { k: "grid", cs, small: !!n.small, flex } : null;
    }
    if (n.modes) {
      const opts = (n.options || []) as any[];
      const store = `mode:${n.modes}`;
      const cur = modes[store] || (opts[0] && opts[0].value) || "";
      const pick = opts.find((o) => o.value === cur) || opts[0];
      for (const o of opts) if (o !== pick) refs(o.layout).forEach((id) => take(id)); // the hidden options' controls aren't "rest"
      return { k: "modes", store, options: opts.map((o) => ({ name: o.name, value: o.value })), current: pick ? pick.value : "", inner: pick ? node(pick.layout) : null, flex };
    }
    if (n.talk) return guest ? null : { k: "talk", flex };
    if (n.route) return guest ? null : { k: "route", flex };
    if (n.transcript) return guest ? null : { k: "transcript", flex };
    if (n.radio) {
      const d = take(n.radio.discord), gm = take(n.radio.game);
      return d || gm ? { k: "radio", discord: d, game: gm, flex } : null;
    }
    return null;
  };

  let tree = g.layout ? node(g.layout) : null;
  const rest = g.controls.filter((c) => !used.has(c));
  if (restAt) {
    const r = restAt as { flex: number; small: boolean };
    Object.assign(REST, { cs: rest, small: r.small, flex: r.flex });
    if (!rest.length && tree) tree = prune(tree);
    return tree || { k: "grid", cs: rest, flex: 1 };
  }
  if (!tree) return { k: "grid", cs: rest, flex: 1, small: rest.length > 12 };
  if (rest.length) return { k: "box", dir: "col", flex: 1, kids: [{ ...tree, flex: 4 } as N, { k: "grid", cs: rest, flex: rest.length > 4 ? 2 : 1 }] };
  return tree;
}

function prune(n: N): N | null {
  if (n.k === "grid" && !n.cs.length) return null;
  if (n.k === "box") {
    const kids = n.kids.map(prune).filter(Boolean) as N[];
    return kids.length ? { ...n, kids } : null;
  }
  return n;
}

// A short label: "Toggle landing gear" -> "Landing gear".
export const short = (l: string) => String(l).replace(/^(toggle|request|engage|master:?)\s+/i, "").replace(/^prev /i, "");
