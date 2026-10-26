<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { pairWith } from "../lib/net";
  import { emblem } from "../lib/icons";
  let code = $state("");
  const em = $derived(emblem(S.theme));
  function go(e: Event) {
    e.preventDefault();
    if (code.trim()) pairWith(code);
  }
</script>

<div class="pairscreen">
  <form class="pairbox" onsubmit={go} novalidate>
    <span class="em"><svg viewBox="0 0 64 64">{@html em.emblem}</svg></span>
    {#if S.mode === "gone"}
      <h1>Link closed</h1>
      <p class="err">{S.gone}</p>
    {:else}
      <h1>Pair this device</h1>
      <input class="field code" bind:value={code} autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" maxlength="32" placeholder="code on your PC" aria-label="Code on your PC" />
      {#if S.pairError}<p class="err" role="alert">{S.pairError}</p>{/if}
      <button class="btn primary">Connect</button>
    {/if}
  </form>
</div>
