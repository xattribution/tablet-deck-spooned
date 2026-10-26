<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { press } from "../lib/press";
  import { fx, effectFor } from "../lib/fx";
  import { iconFor, iconSVG, badge } from "../lib/icons";
  import { short } from "../lib/resolve";
  import type { Control } from "../lib/types";

  let { c, gid, flex = 1, label, icon, hero = false, wide = false, effect: fxName, spool = false, seg = false, cls = "", extra }:
    { c: Control; gid: string; flex?: number; label?: string; icon?: string; hero?: boolean; wide?: boolean; effect?: string; spool?: boolean; seg?: boolean; cls?: string; extra?: any } = $props();

  let el: HTMLElement;
  const hold = $derived(c.mode === "hold");
  const locked = $derived(S.mode === "guest" && !!c.locked);
  const lit = $derived(!!(c.state && S.lit[c.state.group]) && !locked);
  const bd = $derived(c.unbound ? "BIND" : badge(c.label));
  const svg = $derived(iconSVG(icon || iconFor(c, gid)));

  // A light that changes by itself (another deck, voice, the agent) still glows.
  let was: boolean | undefined;
  $effect(() => {
    const now = lit;
    if (was !== undefined && now && !was && el) fx.play("lit", el);
    was = now;
  });
  // Refused by the PC: a short shake.
  $effect(() => {
    const t = S.blocked[c.id];
    if (!t || !el) return;
    el.classList.remove("shake"); void el.offsetWidth; el.classList.add("shake");
  });
</script>

<button bind:this={el} type="button" class="k {cls}" class:hold class:hero class:wide class:on={lit} class:unbound={c.unbound} class:locked class:spool class:segk={seg}
  data-id={c.id} aria-label={c.label} aria-pressed={c.state ? lit : undefined} style="flex: {flex} 1 0"
  use:press={{ id: c.id, hold, locked, effect: effectFor(c.cid || c.id, fxName) }}>
  {#if bd && !seg}<i class="bd" class:ub={c.unbound}>{bd}</i>{/if}
  <span class="kin"><span class="ic">{@html svg}</span><span class="l">{label || short(c.label)}</span></span>
  {#if c.state && !seg}<i class="led"></i>{/if}
  {#if hold}
    {#if spool}
      <svg class="hold-ring" viewBox="0 0 100 100"><circle cx="50" cy="50" r="46" pathLength="1" /></svg>
    {:else}
      <svg class="hold-ring" viewBox="0 0 100 100" preserveAspectRatio="none"><rect x="1.5" y="1.5" width="97" height="97" rx="3" pathLength="1" vector-effect="non-scaling-stroke" /></svg>
    {/if}
  {/if}
  {@render extra?.()}
</button>
