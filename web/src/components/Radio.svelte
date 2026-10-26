<script lang="ts">
  import { S, local } from "../lib/state.svelte";
  import { drag } from "../lib/press";
  import { down, up, send } from "../lib/net";
  import { haptic } from "../lib/haptics";
  import { iconSVG } from "../lib/icons";
  let { node, gid }: { node: any; gid: string } = $props();

  const d = $derived(node.discord), g = $derived(node.game);
  const ch = $derived(S.radioCh === "game" && g ? "game" : S.radioCh === "both" && d && g ? "both" : d ? "discord" : "game");
  const ids = $derived((ch === "discord" ? [d] : ch === "game" ? [g] : [d, g]).filter(Boolean).map((c: any) => c.id));
  const locked = $derived(S.mode === "guest" && [d, g].some((c: any) => c && c.locked));
  let tx = $state(false);

  // A squelch at each end, made on the spot (no sound files).
  let audio: AudioContext | null = null;
  function squelch(open: boolean) {
    try {
      audio = audio || new AudioContext();
      if (audio.state === "suspended") audio.resume();
      const t = audio.currentTime, len = open ? 0.06 : 0.12, n = Math.floor(audio.sampleRate * len);
      const buf = audio.createBuffer(1, n, audio.sampleRate), data = buf.getChannelData(0);
      for (let i = 0; i < n; i++) data[i] = (Math.random() * 2 - 1) * (1 - i / n);
      const src = audio.createBufferSource(), bp = audio.createBiquadFilter(), gain = audio.createGain();
      src.buffer = buf; bp.type = "bandpass"; bp.frequency.value = open ? 2400 : 1600; bp.Q.value = 0.8;
      gain.gain.setValueAtTime(0.12, t); gain.gain.exponentialRampToValueAtTime(0.001, t + len);
      src.connect(bp).connect(gain).connect(audio.destination); src.start(t);
    } catch {}
  }
  let held: string[] = [];
  const handlers = {
    start() {
      if (locked) { ids.forEach((id) => send({ t: "request", id })); return null; }
      held = ids;
      held.forEach(down);
      tx = true;
      haptic("press");
      squelch(true);
      return {};
    },
    move() {},
    end() { held.forEach(up); held = []; tx = false; haptic("tap"); squelch(false); },
  };
  function pick(v: "discord" | "game" | "both") { S.radioCh = v; local.set("radio-ch", v); haptic("tap"); }
</script>

<div class="radio" style="flex: {node.flex} 1 0" data-cat="crew">
  <button type="button" class="k hero radio-key hold" class:charging={tx} use:drag={handlers} aria-label="Push to talk">
    <i class="tx">TX</i>
    <span class="kin"><span class="ic">{@html iconSVG("radio")}</span><span class="l">{ch === "both" ? "Discord + game" : ch === "game" ? "Game push to talk" : "Discord push to talk"}</span></span>
  </button>
  <div class="seg local">
    {#if d}<button type="button" class:on={ch === "discord"} onclick={() => pick("discord")}>Discord</button>{/if}
    {#if g}<button type="button" class:on={ch === "game"} onclick={() => pick("game")}>Game</button>{/if}
    {#if d && g}<button type="button" class:on={ch === "both"} onclick={() => pick("both")}>Both</button>{/if}
  </div>
</div>
