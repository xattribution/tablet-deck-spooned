// The voice key, wherever it is (the Comms panel or the chin): tap to talk (and tap
// again to stop; it also stops by itself when you stop talking), hold to talk, or
// live (always listening, the key turns it on and off).
import { S } from "./state.svelte";
import { send } from "./net";
import { haptic } from "./haptics";

let backstop: ReturnType<typeof setTimeout> | undefined;

export function talkDown() {
  haptic("press");
  if (S.talkMode === "live") { send({ t: "voice-live", on: !S.live }); S.live = !S.live; return; }
  if (S.talkMode === "tap") {
    if (S.listening) { stopTalk(); return; }
    send({ t: "voice", on: true, mode: "tap" });
    S.listening = true;
    clearTimeout(backstop);
    backstop = setTimeout(() => { if (S.listening) stopTalk(); }, 13000); // the PC stops at 12 s; never stay lit
    return;
  }
  send({ t: "voice", on: true });
  S.listening = true;
}
export function talkUp() {
  if (S.talkMode !== "hold") return;
  send({ t: "voice", on: false });
  S.listening = false;
}
export function stopTalk() {
  clearTimeout(backstop);
  send({ t: "voice", on: false, mode: "tap" });
  S.listening = false;
}
export const micSVG = `<svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><rect x="17" y="6" width="14" height="24" rx="7"/><path d="M10 22a14 14 0 0 0 28 0M24 36v7M17 43h14"/></svg>`;
