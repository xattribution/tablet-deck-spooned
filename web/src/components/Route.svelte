<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { send } from "../lib/net";
  import { haptic } from "../lib/haptics";
  let { flex = 1 }: { flex?: number } = $props();
  const route = $derived(S.agent.route || "command");
  function pick(m: string) { send({ t: "voice-route", mode: m }); S.agent = { ...S.agent, route: m }; haptic("tap"); }
</script>

<div class="route" style="flex: {flex} 1 0">
  <div class="seg local">
    {#each [["command", "Commands"], ["auto", "Auto"], ["agent", S.agent.name || "Agent"]] as [v, l] (v)}
      <button type="button" class:on={route === v} onclick={() => pick(v)}>{l}</button>
    {/each}
  </div>
</div>
