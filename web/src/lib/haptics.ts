// Haptics: the Android app's real vibration motor when it's there, the web's vibrate
// elsewhere (Android browsers). iPadOS has neither for web pages.
export type Haptic = "tick" | "tap" | "press" | "heavy" | "error" | "long";

const native = (window as any).TabletDeckNative;
const PATTERN: Record<Haptic, number | number[]> = { tick: 4, tap: 8, press: 14, heavy: [22, 30, 22], error: [30, 50, 30], long: [60, 40, 60, 40, 120] };

let last = 0;
export function haptic(kind: Haptic) {
  const now = performance.now();
  if (kind === "tick" && now - last < 28) return; // a fast drag shouldn't buzz continuously
  last = now;
  try {
    if (native && native.haptic) { native.haptic(kind); return; }
    if (navigator.vibrate) navigator.vibrate(PATTERN[kind]);
  } catch {}
}
