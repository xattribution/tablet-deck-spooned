// The deck's icon set and category colors (shared with the classic deck).
import "../legacy/icons.js";
import "../legacy/icons2.js";
import "../legacy/emblems.js";
import "../legacy/art.js";
import type { Control } from "./types";

const w = window as any;

export function iconSVG(name: string): string {
  return w.deckIcon ? w.deckIcon(name) : "";
}
export function catOf(groupId: string, category?: string): string {
  return category || (w.DeckCat ? w.DeckCat.of(groupId) : "ui");
}
export function iconFor(c: Control, groupId: string): string {
  return w.DeckCat ? w.DeckCat.icon(c.cid ? { ...c, id: c.cid } : c, groupId) : "default";
}
export function badge(label: string): string {
  return w.DeckCat ? w.DeckCat.badge(label) : "";
}
export function registerCats(groups: { id: string; category?: string }[]) {
  w.DeckCat?.set?.(groups);
}
export function emblem(theme: string): { name: string; full: string; emblem: string } {
  return (w.DECK_EMBLEMS || {})[theme] || { name: "Tablet Deck", full: "Tablet Deck", emblem: "" };
}
export function applyArt(theme: string) {
  w.DeckArt?.apply?.(theme);
}
export const THEMES = ["rsi", "hud", "mobiglas", "drake", "origin", "crusader", "aegis", "anvil", "misc", "argo", "cnou", "banu", "aopoa", "esperia"];
