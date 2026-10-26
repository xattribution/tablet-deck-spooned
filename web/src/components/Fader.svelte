<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { drag } from "../lib/press";
  import { tap, down, up, send } from "../lib/net";
  import { haptic } from "../lib/haptics";
  import { fx } from "../lib/fx";
  import { iconSVG } from "../lib/icons";
  import Key from "./Key.svelte";

  let { node, gid, cat }: { node: any; gid: string; cat: string } = $props();
  const n = $derived(node);
  const hold = $derived(((n.up || n.down)?.mode) === "hold");
  const locked = $derived(S.mode === "guest" && !!((n.up && n.up.locked) || (n.down && n.down.locked)));
  let track: HTMLElement, thumb: HTMLElement, fill: HTMLElement;
  let active = $state(false);

  // Each detent dragged past is one press of up or down; a throttle (hold controls)
  // holds the key while it's off center. A tap above or below center is one press.
  // The thumb follows the finger and springs back to center (the game doesn't say
  // where the level is).
  type St = { len: number; half: number; step: number; x0: number; y0: number; n: number; moved: boolean; at: number; held: string };
  const set = (d: number, half: number, live: boolean) => {
    thumb.classList.toggle("back", !live); fill.classList.toggle("back", !live);
    const p = half ? d / half : 0;
    if (n.horizontal) {
      thumb.style.transform = `translate3d(${d}px,0,0)`;
      fill.style.transform = `scaleX(${p})`;
    } else {
      thumb.style.transform = `translate3d(0,${-d}px,0)`;
      fill.style.transform = `scaleY(${-p})`;
    }
  };
  const handlers = {
    start(e: PointerEvent): St | null {
      if (locked) { send({ t: "request", id: (n.up || n.down).id }); return null; }
      const r = track.getBoundingClientRect();
      const len = n.horizontal ? r.width : r.height;
      active = true;
      haptic("tap");
      return { len, half: len / 2 - 16, step: Math.max(18, len / 12), x0: e.clientX, y0: e.clientY, n: 0, moved: false,
        at: n.horizontal ? (e.clientX - r.left) / r.width : (e.clientY - r.top) / r.height, held: "" };
    },
    move(e: PointerEvent, s: St) {
      let d = n.horizontal ? e.clientX - s.x0 : s.y0 - e.clientY;
      if (Math.abs(d) > 6) s.moved = true;
      d = Math.max(-s.half, Math.min(s.half, d));
      set(d, s.half, true);
      if (hold) {
        const want = d > s.step ? n.up?.id : d < -s.step ? n.down?.id : "";
        if ((want || "") !== s.held) {
          if (s.held) up(s.held);
          if (want) { down(want); haptic("press"); }
          s.held = want || "";
        }
        return;
      }
      const k = Math.trunc(d / s.step);
      while (s.n !== k) {
        const upward = k > s.n;
        if (upward ? s.n >= 0 : s.n <= 0) {
          const id = upward ? n.up?.id : n.down?.id;
          if (id) { tap(id); haptic("tick"); fx.play("tick", thumb); }
        }
        s.n += upward ? 1 : -1;
      }
    },
    end(e: PointerEvent, s: St, cancelled: boolean) {
      active = false;
      if (s.held) up(s.held);
      if (!s.moved && !cancelled) {
        const upward = n.horizontal ? s.at > 0.5 : s.at < 0.5;
        const id = upward ? n.up?.id : n.down?.id;
        if (id) { tap(id); haptic("press"); fx.play("tick", thumb); }
      }
      set(0, s.half, false);
    },
  };
</script>

<div class="fader" class:horiz={n.horizontal} class:active class:locked data-cat={n.cat || cat} style="flex: {n.flex} 1 0">
  {#if n.max}<Key c={n.max} {gid} cls="cap max" label="Max" icon="chev-up" flex={0} />{/if}
  <div class="track" bind:this={track} use:drag={handlers} role="slider" aria-label={n.label} aria-valuenow={0} tabindex="0">
    <i class="ticks"></i><i class="mid"></i><i class="fill" bind:this={fill}></i><span class="thumb" bind:this={thumb}></span>
  </div>
  {#if n.min}<Key c={n.min} {gid} cls="cap min" label="Min" icon="chev-down" flex={0} />{/if}
  <span class="fl"><span class="ic">{@html iconSVG(n.icon || "sliders")}</span><span class="l">{n.label}</span></span>
</div>
