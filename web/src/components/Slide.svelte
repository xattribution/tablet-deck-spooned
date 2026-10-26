<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { drag } from "../lib/press";
  import { tap, send } from "../lib/net";
  import { haptic } from "../lib/haptics";
  import { fx } from "../lib/fx";
  import { iconSVG } from "../lib/icons";
  import type { Control } from "../lib/types";
  let { c, flex = 1 }: { c: Control; flex?: number } = $props();

  // Slide the handle the whole length of the rail to fire. Let go early and it
  // springs back.
  let rail: HTMLElement, handle: HTMLElement, label: HTMLElement, root: HTMLElement;
  let fired = $state(false);
  const set = (x: number, live: boolean, max: number) => {
    handle.classList.toggle("back", !live);
    handle.style.transform = `translate3d(${x}px,0,0)`;
    label.style.opacity = String(1 - Math.min(1, (x / Math.max(1, max)) * 1.4));
  };
  type St = { x0: number; max: number; d: number; last: number };
  const handlers = {
    start(e: PointerEvent): St | null {
      if (S.mode === "guest" && c.locked) { send({ t: "request", id: c.id }); return null; }
      if (!(e.target as Element).closest(".handle")) return null;
      haptic("tap");
      return { x0: e.clientX, max: rail.clientWidth - handle.offsetWidth - 8, d: 0, last: 0 };
    },
    move(e: PointerEvent, s: St) {
      s.d = Math.max(0, Math.min(s.max, e.clientX - s.x0));
      const notch = Math.floor((s.d / s.max) * 6);
      if (notch !== s.last) { s.last = notch; haptic("tick"); }
      set(s.d, true, s.max);
    },
    end(e: PointerEvent, s: St, cancelled: boolean) {
      if (!cancelled && s.d >= s.max * 0.94) {
        tap(c.id);
        haptic("long");
        fx.play("alarm", root);
        fired = true;
        setTimeout(() => (fired = false), 1300);
      }
      set(0, false, s.max);
    },
  };
</script>

<div class="slide" class:fired bind:this={root} data-id={c.id} style="flex: {flex} 1 0">
  <div class="rail" bind:this={rail} use:drag={handlers}>
    <span class="handle" bind:this={handle}>{@html iconSVG("exit-seat")}</span>
    <span class="l" bind:this={label}>{c.label}</span>
  </div>
</div>
