// Panels into pages: columns that fit the screen, filled top to bottom, each panel
// getting room by its weight but never less than its minimum. The same groups on the
// same screen always land the same way, so nothing reshuffles while you play.
import type { Group } from "./types";

export type Col = { groups: Group[]; weights: number[] };
export type Page = Col[];

export function layoutPages(groups: Group[], W: number, H: number): Page[] {
  if (!groups.length || W < 10 || H < 10) return [];
  const cols = W >= 1050 ? 3 : W >= 640 ? 2 : 1;
  const scale = Math.min(1.25, Math.max(0.6, H / 760));
  const minOf = (g: Group) => Math.max(110, Math.round((g.size?.min || 60 + 70 * Math.ceil(g.controls.length / 4)) * scale));
  const weightOf = (g: Group) => g.size?.weight || Math.max(2, g.controls.length * 0.7);
  const pages: Page[] = [];
  let page: Page = [];
  let col: Col = { groups: [], weights: [] };
  let used = 0;
  const flushCol = () => {
    if (col.groups.length) page.push(col);
    col = { groups: [], weights: [] };
    used = 0;
    if (page.length === cols) { pages.push(page); page = []; }
  };
  for (const g of groups) {
    const m = Math.min(H, minOf(g));
    if (col.groups.length && used + m > H + 2) flushCol();
    col.groups.push(g);
    col.weights.push(weightOf(g));
    used += m + 2;
  }
  flushCol();
  if (page.length) pages.push(page);
  return pages;
}
