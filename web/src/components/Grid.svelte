<script lang="ts">
  import type { Control } from "../lib/types";
  import Key from "./Key.svelte";
  import Guard from "./Guard.svelte";
  let { cs, gid, flex = 1, small = false }: { cs: Control[]; gid: string; flex?: number; small?: boolean } = $props();

  // Columns that keep keys close to square and as big as the box allows; worked out
  // only when the box changes size.
  let el: HTMLElement;
  let cols = $state(1);
  $effect(() => {
    const n = cs.length;
    const ro = new ResizeObserver(([e]) => {
      const W = e.contentRect.width, H = e.contentRect.height, gap = 6;
      let best = 1, score = 0;
      for (let c = 1; c <= n; c++) {
        const r = Math.ceil(n / c), w = (W - gap * (c - 1)) / c, h = (H - gap * (r - 1)) / r;
        const s = Math.min(w, h * 1.5) * (r * c - n > c ? 0.85 : 1);
        if (s > score) { score = s; best = c; }
      }
      if (best !== cols) cols = best;
    });
    ro.observe(el);
    return () => ro.disconnect();
  });
  const rows = $derived(Math.ceil(cs.length / cols));
</script>

<div class="grid" class:small bind:this={el} style="flex: {flex} 1 0; grid-template-columns: repeat({cols}, minmax(0, 1fr)); grid-template-rows: repeat({rows}, minmax(0, 1fr))">
  {#each cs as c (c.id)}
    {#if c.danger}<Guard {c} {gid} />{:else}<Key {c} {gid} />{/if}
  {/each}
</div>
