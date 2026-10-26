<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { send } from "../lib/net";
  import { haptic } from "../lib/haptics";

  let toastOn = $state(false), text = $state("");
  let t: ReturnType<typeof setTimeout>;
  $effect(() => {
    if (!S.toast) return;
    text = S.toast.text;
    toastOn = true;
    clearTimeout(t);
    t = setTimeout(() => (toastOn = false), 2400);
  });
  const others = (lost: string) => (S.cockpit?.consoles || []).filter((c) => c.online && c.id !== lost);
  function sendTo(l: { console: string }, to: string) {
    send({ t: "cockpit-fallback", id: l.console, text: to });
    S.lost = S.lost.filter((x) => x.console !== l.console);
    haptic("tap");
  }
  const b = $derived(S.battle);
  const fighting = $derived(!!b && (b.fire || b.dead || b.level > 0));
  const severe = $derived(!!b && (b.level >= 1 || b.dead));
</script>

<div class="toast" class:show={toastOn} role="status">{text}</div>

{#each S.lost as l (l.console)}
  <div class="lost" role="alert">
    <b>{l.name} lost</b>
    <span>Its panels moved to {l.toName}.</span>
    <div class="row">
      {#each others(l.console).filter((c) => c.id !== l.to) as c (c.id)}
        <button type="button" class="btn sm" onclick={() => sendTo(l, c.id)}>Send to {c.name}</button>
      {/each}
      <button type="button" class="btn sm primary" onclick={() => (S.lost = S.lost.filter((x) => x.console !== l.console))}>OK</button>
    </div>
  </div>
{/each}

{#if S.identify}
  {#key S.identify}<div class="ident"><b>{S.identify}</b></div>{/key}
{/if}

<div class="battle-frame" class:on={fighting} class:severe></div>
{#if fighting && b}
  <div class="battlebar">
    {b.dead ? "Dead" : b.level >= 2 ? "Ship destroyed" : b.level >= 1 ? "Ship disabled" : "Under fire"}
    {#if b.from}<span>{b.from}</span>{/if}
    {#if b.hits}<span>{b.hits} hits</span>{/if}
  </div>
{/if}
