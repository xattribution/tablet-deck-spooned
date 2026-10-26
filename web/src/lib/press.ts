// How a key is pressed. A tap key lights at once and presses when the finger lifts
// without having travelled, so a swipe that starts on a key turns pages instead
// (the browser takes the touch for scrolling and cancels the press). A hold key goes
// down at once and up when the finger lifts, wherever it lifts, even if the key was
// redrawn meanwhile.
import { down, up, tap, send } from "./net";
import { fx, effectFor } from "./fx";
import { haptic } from "./haptics";

export interface PressOpts {
  id: string;
  hold?: boolean;
  locked?: boolean;
  effect?: string;
  onTap?: () => void; // instead of pressing a control
  onDown?: () => void;
  onUp?: () => void;
}

const active = new Map<number, () => void>();
function finish(e: PointerEvent) {
  const r = active.get(e.pointerId);
  if (r) { active.delete(e.pointerId); r(); }
}
// Bubble phase, so the key's own handler (which fires the tap) runs first; this only
// catches a lift the key never saw (it was redrawn or the finger left the window).
addEventListener("pointerup", finish);
addEventListener("pointercancel", finish);
const releaseAll = () => { for (const [id, r] of [...active]) { active.delete(id); r(); } };
addEventListener("blur", releaseAll);
addEventListener("pagehide", releaseAll);
document.addEventListener("visibilitychange", () => { if (document.hidden) releaseAll(); });

export function press(node: HTMLElement, opts: PressOpts) {
  let o = opts;
  let shown = 0;
  let holding = false;
  let pid = -1;
  let x0 = 0, y0 = 0, moved = false;

  const lift = () => {
    const left = Math.max(0, 80 - (performance.now() - shown));
    setTimeout(() => node.classList.remove("down", "charging"), left);
  };

  function onDown(e: PointerEvent) {
    if (e.button > 0 || pid !== -1) return;
    pid = e.pointerId;
    x0 = e.clientX; y0 = e.clientY; moved = false;
    shown = performance.now();
    node.classList.add("down");
    haptic("tap");
    if (o.hold && !o.locked && !o.onTap) {
      holding = true;
      try { node.setPointerCapture(e.pointerId); } catch {}
      node.classList.add("charging");
      down(o.id);
      o.onDown?.();
      fx.play(effectFor(o.id, o.effect), node, e.clientX, e.clientY);
    }
    active.set(e.pointerId, () => end(false));
  }
  function onMove(e: PointerEvent) {
    if (e.pointerId !== pid || holding) return;
    if (Math.abs(e.clientX - x0) > 12 || Math.abs(e.clientY - y0) > 12) { moved = true; node.classList.remove("down"); }
  }
  function onUp(e: PointerEvent) {
    if (e.pointerId !== pid) return;
    active.delete(e.pointerId);
    end(!moved, e);
  }
  function onCancel(e: PointerEvent) {
    if (e.pointerId !== pid) return;
    active.delete(e.pointerId);
    end(false);
  }
  function end(fire: boolean, e?: PointerEvent) {
    if (pid === -1) return;
    pid = -1;
    if (holding) {
      holding = false;
      up(o.id);
      o.onUp?.();
    } else if (fire) {
      if (o.onTap) o.onTap();
      else if (o.locked) send({ t: "request", id: o.id });
      else {
        tap(o.id);
        fx.play(effectFor(o.id, o.effect), node, e?.clientX, e?.clientY);
      }
      haptic("press");
    }
    lift();
  }

  node.addEventListener("pointerdown", onDown);
  node.addEventListener("pointermove", onMove);
  node.addEventListener("pointerup", onUp);
  node.addEventListener("pointercancel", onCancel);
  node.addEventListener("contextmenu", (e) => e.preventDefault());
  return {
    update(n: PressOpts) { o = n; },
    destroy() {
      if (holding) { up(o.id); holding = false; }
      if (pid !== -1) active.delete(pid);
      node.removeEventListener("pointerdown", onDown);
      node.removeEventListener("pointermove", onMove);
      node.removeEventListener("pointerup", onUp);
      node.removeEventListener("pointercancel", onCancel);
    },
  };
}

// A drag control (fader, lever, dial, slide): the control owns the pointer from the
// first touch (touch-action: none in its CSS), and gets move and end calls.
export interface DragHandlers<T> {
  start(e: PointerEvent): T | null;
  move(e: PointerEvent, s: T): void;
  end(e: PointerEvent, s: T, cancelled: boolean): void;
}
export function drag<T>(node: HTMLElement, h: DragHandlers<T>) {
  let handlers = h;
  let pid = -1;
  let st: T | null = null;
  function onDown(e: PointerEvent) {
    if (pid !== -1 || e.button > 0) return;
    const s = handlers.start(e);
    if (s == null) return;
    e.preventDefault();
    e.stopPropagation();
    pid = e.pointerId; st = s;
    try { node.setPointerCapture(pid); } catch {}
    active.set(pid, () => { if (st != null) handlers.end(e, st, true); pid = -1; st = null; });
  }
  function onMove(e: PointerEvent) {
    if (e.pointerId !== pid || st == null) return;
    handlers.move(e, st);
  }
  function onEnd(e: PointerEvent) {
    if (e.pointerId !== pid || st == null) return;
    active.delete(pid);
    const s = st;
    pid = -1; st = null;
    handlers.end(e, s, e.type === "pointercancel");
  }
  node.addEventListener("pointerdown", onDown);
  node.addEventListener("pointermove", onMove);
  node.addEventListener("pointerup", onEnd);
  node.addEventListener("pointercancel", onEnd);
  return {
    update(n: DragHandlers<T>) { handlers = n; },
    destroy() {
      if (pid !== -1) { const r = active.get(pid); active.delete(pid); r?.(); }
      node.removeEventListener("pointerdown", onDown);
      node.removeEventListener("pointermove", onMove);
      node.removeEventListener("pointerup", onEnd);
      node.removeEventListener("pointercancel", onEnd);
    },
  };
}
