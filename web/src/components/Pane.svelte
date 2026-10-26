<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { resolve } from "../lib/resolve";
  import { catOf } from "../lib/icons";
  import type { Group } from "../lib/types";
  import Node from "./Node.svelte";

  let { g, weight }: { g: Group; weight: number } = $props();
  const tree = $derived(resolve(g, S.modes, S.mode === "guest"));
  const cat = $derived(catOf(g.id, g.category));
  const min = $derived(Math.max(110, Math.round((g.size?.min || 120) * 0.6)));
</script>

<section class="pane" data-cat={cat} data-panel={g.id} style="flex: {weight} 1 0; min-height: {min}px" aria-label={g.name}>
  <header class="plate"><i class="lamp"></i><span class="nm">{g.name}</span><span class="no">{g.code || ""}</span></header>
  <div class="pbody"><Node n={tree} gid={g.id} {cat} /></div>
</section>
