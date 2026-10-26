<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { go } from "../lib/nav.svelte";
  let { flex = 1 }: { flex?: number } = $props();
  const lines = $derived(S.chat.slice(-3));
</script>

<button type="button" class="transcript" style="flex: {flex} 1 0" onclick={() => go("comms")} aria-label="Talk to the agent">
  {#each lines as l, i (l.at + i)}
    <span class="ln {l.who}"><b>{l.who === "you" ? "›" : l.who === "agent" ? "‹" : "·"}</b>{l.text}</span>
  {:else}
    <span class="ln system"><b>·</b>{S.agent.ready ? `Say something to ${S.agent.name || "the agent"}` : "No agent set up"}</span>
  {/each}
  {#if S.busy}<span class="dots"><i></i><i></i><i></i></span>{/if}
</button>
