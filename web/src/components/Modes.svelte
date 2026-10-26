<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { haptic } from "../lib/haptics";
  import Node from "./Node.svelte";
  let { node, gid, cat }: { node: any; gid: string; cat: string } = $props();
  const pick = (v: string) => { S.modes = { ...S.modes, [node.store]: v }; haptic("tap"); };
</script>

<div class="bx col" style="flex: {node.flex} 1 0">
  <div class="seg local" style="flex: 0 0 clamp(40px, 16%, 56px)">
    {#each node.options as o (o.value)}
      <button type="button" class:on={o.value === node.current} onclick={() => pick(o.value)}>{o.name}</button>
    {/each}
  </div>
  {#if node.inner}<div class="bx col" style="flex: 1 1 0"><Node n={node.inner} {gid} {cat} /></div>{/if}
</div>
