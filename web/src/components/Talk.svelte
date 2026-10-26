<script lang="ts">
  import { S, local } from "../lib/state.svelte";
  import { drag } from "../lib/press";
  import { talkDown, talkUp, micSVG, stopTalk } from "../lib/voice";
  import { send } from "../lib/net";
  let { flex = 1 }: { flex?: number } = $props();
  const on = $derived(S.talkMode === "live" ? S.live : S.listening);
  const label = $derived(S.talkMode === "live" ? (S.live ? "Live · tap to stop" : "Off air · tap to go live") : S.talkMode === "hold" ? "Hold to talk" : S.listening ? "Listening · tap to stop" : "Tap to talk");
  const handlers = { start() { talkDown(); return {}; }, move() {}, end() { talkUp(); } };
  function mode(m: "tap" | "hold" | "live") {
    if (S.listening) stopTalk();
    if (m !== "live" && S.live) { send({ t: "voice-live", on: false }); S.live = false; }
    S.talkMode = m;
    local.set("talk-mode", m);
  }
</script>

<div class="talk" style="flex: {flex} 1 0" data-cat="crew">
  <button type="button" class="k hero mic hold" class:on use:drag={handlers} aria-label={label} aria-pressed={on}>
    <span class="kin"><span class="ic">{@html micSVG}</span><span class="l">{label}</span></span><i class="vu"></i>
  </button>
  <div class="seg local">
    {#each [["tap", "Tap"], ["hold", "Hold"], ["live", "Live"]] as [v, l] (v)}
      <button type="button" class:on={S.talkMode === v} onclick={() => mode(v as any)}>{l}</button>
    {/each}
  </div>
</div>
