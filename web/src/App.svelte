<script lang="ts">
  import { S, local } from "./lib/state.svelte";
  import { applyArt, registerCats } from "./lib/icons";
  import Pair from "./components/Pair.svelte";
  import Stage from "./components/Stage.svelte";
  import Overlays from "./components/Overlays.svelte";

  S.theme = local.get("theme") || "rsi";
  try { S.modes = JSON.parse(local.get("modes") || "{}"); } catch {}
  S.talkMode = (local.get("talk-mode") as any) || "tap";
  S.radioCh = (local.get("radio-ch") as any) || "discord";

  $effect(() => {
    document.documentElement.dataset.theme = S.theme;
    local.set("theme", S.theme);
    applyArt(S.theme);
    const meta = document.querySelector('meta[name="theme-color"]') as HTMLMetaElement | null;
    if (meta) requestAnimationFrame(() => (meta.content = getComputedStyle(document.documentElement).getPropertyValue("--surface").trim() || "#0A111C"));
  });
  $effect(() => { local.set("modes", JSON.stringify(S.modes)); });
  $effect(() => { registerCats(S.groups); });
</script>

{#if S.mode === "pair" || S.mode === "gone"}
  <Pair />
{:else if S.mode === "deck" || S.mode === "guest"}
  <Stage />
{:else}
  <div class="pairscreen"></div>
{/if}
<Overlays />
