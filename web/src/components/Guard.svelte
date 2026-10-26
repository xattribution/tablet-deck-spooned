<script lang="ts">
  import { drag } from "../lib/press";
  import { haptic } from "../lib/haptics";
  import type { Control } from "../lib/types";
  import Key from "./Key.svelte";
  let { c, gid, flex = 1 }: { c: Control; gid: string; flex?: number } = $props();

  // Lift the striped cover (tap it, or flick it up), then press. It closes again by
  // itself a few seconds later.
  let open = $state(false);
  let timer: ReturnType<typeof setTimeout>;
  const lift = () => {
    open = true;
    haptic("press");
    clearTimeout(timer);
    timer = setTimeout(() => (open = false), 5000);
  };
  const handlers = {
    start(e: PointerEvent) { return { y0: e.clientY }; },
    move() {},
    end(e: PointerEvent, s: { y0: number }, cancelled: boolean) { if (!cancelled) lift(); },
  };
  $effect(() => () => clearTimeout(timer));
</script>

<div class="guard" class:open style="flex: {flex} 1 0">
  <Key {c} {gid} effect="alarm" />
  <button type="button" class="cover" aria-label="Lift cover" use:drag={handlers}><i></i></button>
</div>
