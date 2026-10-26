<script lang="ts">
  import { iconSVG } from "../lib/icons";
  import type { Control } from "../lib/types";
  import Key from "./Key.svelte";
  let { faces, gid, flex = 1 }: { faces: Record<string, Control | undefined>; gid: string; flex?: number } = $props();
</script>

<div class="spad" style="flex: {flex} 1 0">
  <div class="ring">
    <span class="ship">{@html iconSVG("ship")}</span>
    {#each ["front", "left", "right", "back"] as f (f)}
      {#if faces[f]}<Key c={faces[f]!} {gid} cls="face {f}" label={f} icon={`shield-${f}`} effect="shield" flex={0} />{/if}
    {/each}
    {#if faces.reset}<Key c={faces.reset} {gid} cls="hub" label="Reset" effect="shield" flex={0} />{/if}
  </div>
  {#if faces.top || faces.bottom}
    <div class="tb">
      {#if faces.top}<Key c={faces.top} {gid} label="Top" icon="shield-top" effect="shield" />{/if}
      {#if faces.bottom}<Key c={faces.bottom} {gid} label="Bottom" icon="shield-bottom" effect="shield" />{/if}
    </div>
  {/if}
</div>
