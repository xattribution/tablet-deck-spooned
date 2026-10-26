<script lang="ts">
  import { onMount } from "svelte";
  import { S } from "../lib/state.svelte";
  import { layoutPages } from "../lib/pages";
  import Pane from "./Pane.svelte";

  let el: HTMLElement;
  let W = $state(0), H = $state(0);
  const pages = $derived(layoutPages(S.groups, W, H));
  $effect(() => { S.pages = Math.max(1, pages.length); if (S.page >= pages.length) S.page = Math.max(0, pages.length - 1); });

  onMount(() => {
    const ro = new ResizeObserver(([e]) => { W = e.contentRect.width; H = e.contentRect.height; });
    ro.observe(el);
    let raf = 0;
    const onScroll = () => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        const p = el.scrollLeft / Math.max(1, el.clientWidth);
        document.documentElement.style.setProperty("--pagepos", String(p));
        const i = Math.round(p);
        if (i !== S.page) S.page = i;
      });
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => { ro.disconnect(); el.removeEventListener("scroll", onScroll); };
  });

  // Asked to go to a page (the chin's page bar): glide there.
  export function goPage(i: number) { el?.scrollTo({ left: i * el.clientWidth, behavior: "smooth" }); }
  $effect(() => {
    const want = S.pageWant;
    if (want != null && el) { goPage(want); S.pageWant = null; }
  });
  // A new view starts on its first page.
  let lastView = "";
  $effect(() => {
    const v = S.view?.id || "";
    if (v !== lastView && el) { lastView = v; el.scrollTo({ left: 0, behavior: "instant" as ScrollBehavior }); }
  });
</script>

<div class="pager" bind:this={el}>
  {#each pages as page, pi (pi)}
    <div class="page">
      {#each page as col, ci (ci)}
        <div class="pcol" style="flex: 1 1 0">
          {#each col.groups as g, gi (g.id)}
            <Pane {g} weight={col.weights[gi]} />
          {/each}
        </div>
      {/each}
    </div>
  {:else}
    <div class="page"><div class="empty-page">{S.online ? "No panels on this screen" : "Connecting…"}</div></div>
  {/each}
</div>
