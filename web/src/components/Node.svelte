<script lang="ts">
  import type { N } from "../lib/resolve";
  import Node from "./Node.svelte";
  import Key from "./Key.svelte";
  import Fader from "./Fader.svelte";
  import Lever from "./Lever.svelte";
  import Dial from "./Dial.svelte";
  import ShieldPad from "./ShieldPad.svelte";
  import Guard from "./Guard.svelte";
  import Slide from "./Slide.svelte";
  import Grid from "./Grid.svelte";
  import Modes from "./Modes.svelte";
  import Talk from "./Talk.svelte";
  import Route from "./Route.svelte";
  import Transcript from "./Transcript.svelte";
  import Radio from "./Radio.svelte";

  let { n, gid, cat }: { n: N; gid: string; cat: string } = $props();
</script>

{#if n.k === "box"}
  <div class="bx {n.dir}" class:hazard={n.hazard} style="flex: {n.flex} 1 0">
    {#each n.kids as kid, i (i)}<Node n={kid} {gid} {cat} />{/each}
  </div>
{:else if n.k === "key"}
  <Key c={n.c} {gid} flex={n.flex} label={n.label} icon={n.icon} hero={n.size === "hero"} wide={n.size === "wide"} effect={n.effect} spool={n.spool} />
{:else if n.k === "lever"}
  <Lever c={n.c} {gid} flex={n.flex} />
{:else if n.k === "fader"}
  <Fader node={n} {gid} {cat} />
{:else if n.k === "pair"}
  <div class="seg" style="flex: {n.flex} 1 0">
    {#each n.cs as c, i (c.id)}<Key {c} {gid} label={n.names[i]} flex={1} seg />{/each}
  </div>
{:else if n.k === "dial"}
  <Dial node={n} {gid} />
{:else if n.k === "shield"}
  <ShieldPad faces={n.faces} {gid} flex={n.flex} />
{:else if n.k === "guard"}
  <Guard c={n.c} {gid} flex={n.flex} />
{:else if n.k === "slide"}
  <Slide c={n.c} flex={n.flex} />
{:else if n.k === "grid"}
  <Grid cs={n.cs} {gid} flex={n.flex} small={n.small} />
{:else if n.k === "modes"}
  <Modes node={n} {gid} {cat} />
{:else if n.k === "talk"}
  <Talk flex={n.flex} />
{:else if n.k === "route"}
  <Route flex={n.flex} />
{:else if n.k === "transcript"}
  <Transcript flex={n.flex} />
{:else if n.k === "radio"}
  <Radio node={n} {gid} />
{/if}
