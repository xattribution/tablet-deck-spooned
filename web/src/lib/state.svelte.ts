// The deck's state, kept in sync with the PC over one WebSocket. Components read it
// directly; only what changes redraws.
import type { Battle, ChatLine, Cockpit, Crewmate, Group, Request, View } from "./types";

export type Mode = "connecting" | "pair" | "deck" | "guest" | "gone";

export const S = $state({
  mode: "connecting" as Mode,
  online: false,
  latency: 0,
  name: "",
  version: "",
  views: [] as View[],
  view: null as View | null,
  groups: [] as Group[],
  lit: {} as Record<string, boolean>,
  game: { running: false, focused: false } as { running: boolean; focused: boolean },
  afk: false,
  crew: [] as Crewmate[],
  requests: [] as Request[],
  voice: { state: "off" } as { state: string; reason?: string },
  listening: false,
  live: false,
  chat: [] as ChatLine[],
  busy: false,
  agent: {} as { ready?: boolean; name?: string; style?: string; route?: string },
  cockpit: null as Cockpit | null,
  member: false,
  battle: null as Battle | null,
  theme: "rsi",
  gone: "",
  pairError: "",
  owner: "",
  guest: null as any,
  ticker: null as null | { who: string; what: string; n: number },
  toast: null as null | { text: string; n: number },
  blocked: {} as Record<string, number>, // control id -> time it was refused (shakes)
  lost: [] as { console: string; name: string; to: string; toName: string }[],
  identify: "",
  modes: {} as Record<string, string>, // local switches: landing/docking, dial positions
  page: 0,
  pages: 1,
  pageWant: null as number | null,
  talkMode: "tap" as "tap" | "hold" | "live",
  radioCh: "discord" as "discord" | "game" | "both",
});

let tick = 0;
export function ticker(who: string, what: string) {
  S.ticker = { who, what, n: ++tick };
}
export function toast(text: string) {
  S.toast = { text, n: ++tick };
}

// Mode positions picked on a dial are remembered here (the game doesn't say which
// operator mode is on).
export const local = {
  get(k: string): string | null {
    try { return localStorage.getItem(k); } catch { return null; }
  },
  set(k: string, v: string | null) {
    try { if (v == null || v === "") localStorage.removeItem(k); else localStorage.setItem(k, v); } catch {}
  },
};
