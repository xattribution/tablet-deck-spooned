<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { drag } from "../lib/press";
  import { tap, send } from "../lib/net";
  import { haptic } from "../lib/haptics";
  import { fx } from "../lib/fx";
  import { iconSVG } from "../lib/icons";
  import { short } from "../lib/resolve";
  import type { Control } from "../lib/types";

  let { c, gid, flex = 1 }: { c: Control; gid: string; flex?: number } = $props();
  const lit = $derived(!!(c.state && S.lit[c.state.group]));
  let slot: HTMLElement, knob: HTMLElement, el: HTMLElement;
  let transit = $state(false);
  let travel = 0;

  // The knob rests at the top (gear up) or bottom (down). Drag it toward the other end
  // past halfway, or tap it, to throw it: one press. Its position follows the lit
  // state, so the game (or another deck) moving the gear moves the lever too.
  const place = (offset: number, live: boolean) => {
    if (!slot || !knob) return;
    travel = Math.max(0, slot.clientHeight - knob.offsetHeight);
    knob.classList.toggle("back", !live);
    const base = lit ? travel : 0;
    knob.style.transform = `translate3d(0,${Math.max(0, Math.min(travel, base + offset))}px,0)`;
  };
  $effect(() => { lit; place(0, false); });
  $effect(() => {
    const ro = new ResizeObserver(() => place(0, false));
    ro.observe(slot);
    return () => ro.disconnect();
  });
  const handlers = {
    start(e: PointerEvent) {
      if (S.mode === "guest" && c.locked) { send({ t: "request", id: c.id }); return null; }
      haptic("tap");
      return { y0: e.clientY, d: 0 };
    },
    move(e: PointerEvent, s: { y0: number; d: number }) { s.d = e.clientY - s.y0; place(s.d, true); },
    end(e: PointerEvent, s: { y0: number; d: number }, cancelled: boolean) {
      const thrown = Math.abs(s.d) < 10 || (lit ? s.d < -travel / 2 : s.d > travel / 2);
      if (!cancelled && thrown) {
        tap(c.id);
        haptic("heavy");
        fx.play("press", knob);
        transit = false; requestAnimationFrame(() => (transit = true));
        setTimeout(() => (transit = false), 1700);
      }
      place(0, false);
    },
  };
</script>

<div class="lever" class:on={lit} class:transit bind:this={el} data-id={c.id} style="flex: {flex} 1 0" use:drag={handlers} role="button" tabindex="0" aria-label={c.label} aria-pressed={lit}>
  <span class="slot" bind:this={slot}><span class="knob" bind:this={knob}>{@html iconSVG("gear")}</span></span>
  <span class="greens"><i></i><i></i><i></i></span>
  <span class="l">{short(c.label)}</span>
</div>
