<script lang="ts">
  import { onMount } from "svelte";
  import { S } from "../lib/state.svelte";
  import { bindStage, go, nav, watch } from "../lib/nav.svelte";
  import Pager from "./Pager.svelte";
  import Chin from "./Chin.svelte";
  import Home from "./Home.svelte";
  import Comms from "./Comms.svelte";

  let stage: HTMLElement;
  let comms: HTMLElement | null = $state(null);
  let deck: HTMLElement;
  let home: HTMLElement | null = $state(null);
  const owner = $derived(S.mode === "deck");

  onMount(() => {
    bindStage(stage, { comms, deck, home });
    go("deck", { instant: true });
    watch();
    // Turning the tablet keeps you on the same section.
    const ro = new ResizeObserver(() => go(nav.section, { instant: true }));
    ro.observe(stage);
    return () => ro.disconnect();
  });
</script>

<div class="stage" bind:this={stage}>
  {#if owner}<section class="section comms-s" bind:this={comms} aria-label="Comms"><Comms /></section>{/if}
  <section class="section deck" bind:this={deck} aria-label="Deck">
    <Pager />
    <Chin />
  </section>
  {#if owner}<section class="section home-s" bind:this={home} aria-label="Home"><Home /></section>{/if}
</div>
