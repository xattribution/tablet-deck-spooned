<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { press, drag } from "../lib/press";
  import { tap } from "../lib/net";
  import { haptic } from "../lib/haptics";
  import { fx } from "../lib/fx";
  import { iconFor, iconSVG } from "../lib/icons";
  import { short } from "../lib/resolve";
  import type { Control } from "../lib/types";
  import Key from "./Key.svelte";

  let { node, gid }: { node: any; gid: string } = $props();
  const cs: Control[] = $derived(node.cs);
  let face: HTMLElement;
  let D = $state(200);

  // Positions sit evenly around the whole circle, starting at the top, sized so
  // neighbors never touch.
  const step = $derived(360 / Math.max(1, cs.length));
  const R = $derived(D * 0.36);
  const size = $derived(Math.min(D * 0.25, ((2 * Math.PI * R) / Math.max(1, cs.length)) * 0.8));
  const angleOf = (i: number) => -90 + i * step;
  const sel = $derived(Math.max(0, cs.findIndex((c) => c.id === S.modes[node.store])));
  let near = $state(-1);

  // The rotor turns the short way round, so going from the last mode to the first
  // doesn't spin it all the way back.
  let rot = $state(0);
  let live = $state(false);
  const turnTo = (deg: number) => {
    let d = ((deg - rot) % 360 + 540) % 360 - 180;
    rot = rot + d;
  };
  $effect(() => { if (!live) turnTo(angleOf(sel) + 90); });

  function choose(i: number, e?: PointerEvent) {
    const c = cs[i];
    if (!c) return;
    S.modes = { ...S.modes, [node.store]: c.id };
    tap(c.id);
    haptic("press");
    const el = face?.querySelectorAll(".pos")[i];
    if (el) fx.play("press", el, e?.clientX, e?.clientY);
  }

  const handlers = {
    start(e: PointerEvent) {
      if ((e.target as Element).closest(".pos, .hub")) return null;
      live = true;
      return { last: -1 };
    },
    move(e: PointerEvent, s: { last: number }) {
      const r = face.getBoundingClientRect();
      const a = (Math.atan2(e.clientY - (r.top + r.height / 2), e.clientX - (r.left + r.width / 2)) * 180) / Math.PI;
      turnTo(a + 90);
      const i = ((Math.round((a + 90) / step) % cs.length) + cs.length) % cs.length;
      if (i !== s.last) { s.last = i; near = i; haptic("tick"); }
    },
    end(e: PointerEvent, s: { last: number }, cancelled: boolean) {
      live = false;
      near = -1;
      if (!cancelled && s.last >= 0) choose(s.last, e);
      else turnTo(angleOf(sel) + 90);
    },
  };

  $effect(() => {
    const ro = new ResizeObserver(([en]) => (D = Math.min(en.contentRect.width, en.contentRect.height)));
    ro.observe(face);
    return () => ro.disconnect();
  });
  const arcLen = $derived(Math.min(80, step * 0.55));
  const arcPath = $derived(() => {
    const r = 37, a0 = ((-90 - arcLen / 2) * Math.PI) / 180, a1 = ((-90 + arcLen / 2) * Math.PI) / 180;
    return `M ${r * Math.cos(a0)} ${r * Math.sin(a0)} A ${r} ${r} 0 0 1 ${r * Math.cos(a1)} ${r * Math.sin(a1)}`;
  });
</script>

<div class="dial" style="flex: {node.flex} 1 0">
  <div class="face" bind:this={face} use:drag={handlers}>
    <div class="rotor" class:live style="transform: rotate({rot}deg)">
      <svg viewBox="-50 -50 100 100"><path class="arc" d={arcPath()} stroke-width="3.2" /><circle class="tip" cx="0" cy="-48.5" r="2.2" /></svg>
    </div>
    {#each cs as c, i (c.id)}
      {@const a = (angleOf(i) * Math.PI) / 180}
      <div class="posw" style="width: {size}px; height: {size}px; transform: translate3d({R * Math.cos(a) - size / 2}px, {R * Math.sin(a) - size / 2}px, 0)">
        <button type="button" class="k pos" class:sel={i === sel} class:near={i === near} data-id={c.id} aria-label={c.label} aria-pressed={i === sel}
          use:press={{ id: c.id, onTap: () => choose(i) }}>
          <span class="kin"><span class="ic">{@html iconSVG(iconFor(c, gid))}</span><span class="l">{short(c.label).replace(/ mode$/i, "")}</span></span>
        </button>
      </div>
    {/each}
    {#if node.center}<Key c={node.center} {gid} cls="hub" flex={0} />{/if}
  </div>
</div>
