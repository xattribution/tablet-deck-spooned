<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { send } from "../lib/net";
  import { go } from "../lib/nav.svelte";
  import { emblem, catOf } from "../lib/icons";
  import { drag } from "../lib/press";
  import { talkDown, talkUp, micSVG } from "../lib/voice";
  import { haptic } from "../lib/haptics";

  const owner = $derived(S.mode === "deck");
  const em = $derived(emblem(S.theme));
  const me = $derived(S.cockpit?.consoles?.find((c) => c.id === S.cockpit?.me));
  const sub = $derived(S.mode === "guest" ? `Shared by ${S.owner || "a friend"}` : me ? me.name : "This screen");

  // The ticker: what just happened (a crewmate's press, the agent, voice), for a moment.
  let showTick = $state(false);
  let tickTimer: ReturnType<typeof setTimeout>;
  $effect(() => {
    if (!S.ticker) return;
    S.ticker.n;
    showTick = true;
    clearTimeout(tickTimer);
    tickTimer = setTimeout(() => (showTick = false), 2600);
  });

  const link = $derived(!S.online ? "bad pulse" : S.latency > 150 ? "caution" : "good");
  const game = $derived(S.game.running ? (S.game.focused ? "good" : "caution") : "");
  const talkOn = $derived(S.talkMode === "live" ? S.live : S.listening);
  let micDown = $state(false);
  const mic = { start() { micDown = true; talkDown(); return {}; }, move() {}, end() { micDown = false; talkUp(); } };

  function pick(id: string) {
    if (S.view?.id === id) return;
    haptic("tap");
    send({ t: "view", id });
  }
  function swatch(gs: string[]) { return gs.slice(0, 3).map((g) => catOf(g)); }
  let dock: HTMLElement | undefined = $state();
  // Keep the picked view in sight in the dock.
  $effect(() => {
    const id = S.view?.id;
    if (!dock || !id) return;
    const b = dock.querySelector<HTMLElement>(`[data-v="${CSS.escape(id)}"]`);
    // Scroll only the dock (scrollIntoView would also scroll the stage).
    if (b && (b.offsetLeft < dock.scrollLeft || b.offsetLeft + b.offsetWidth > dock.scrollLeft + dock.clientWidth))
      dock.scrollTo({ left: b.offsetLeft - 8, behavior: "smooth" });
  });
</script>

<div class="chin">
  <button type="button" class="home" onclick={() => owner && go("home", { tab: "views" })} aria-label="Home">
    <span class="em"><svg viewBox="0 0 64 64">{@html em.emblem}</svg></span>
    <span class="t"><b>{S.view?.name || em.name}</b><span>{sub}</span></span>
  </button>

  <div class="mid">
    {#if S.pages > 1}
      <div class="pagebar" aria-label="Pages">
        {#each Array(S.pages) as _, i (i)}
          <button type="button" aria-label="Page {i + 1}" onclick={() => { S.pageWant = i; haptic("tick"); }}><i></i></button>
        {/each}
        <span class="ind" style="transform: translateX(calc(var(--pagepos, 0) * 40px))"></span>
      </div>
    {/if}
    {#if owner}
      <div class="dock" bind:this={dock}>
        {#each S.views as v (v.id)}
          <button type="button" data-v={v.id} class:on={S.view?.id === v.id} onclick={() => pick(v.id)}>
            <span class="sw">{#each swatch(v.groups || []) as c, i (i)}<i data-cat={c}></i>{/each}</span>{v.name}
          </button>
        {/each}
      </div>
    {/if}
    <div class="ticker" class:show={showTick} aria-live="polite">
      {#if S.ticker}<b>{S.ticker.who}</b><span>{S.ticker.what}</span>{/if}
    </div>
  </div>

  <div class="right">
    {#if S.battle && (S.battle.fire || S.battle.dead || S.battle.level > 0)}
      <span class="lampb bad pulse"><i></i><span>Fire</span></span>
    {/if}
    {#if owner}
      <button type="button" class="lampb {S.afk ? 'caution' : ''}" onclick={() => { send({ t: "afk", on: !S.afk }); haptic("tap"); }} aria-pressed={S.afk}><i></i><span>AFK</span></button>
    {/if}
    <span class="lampb {game}" title={S.game.running ? (S.game.focused ? "Game in front" : "Game in back") : "Game not running"}><i></i><span>Game</span></span>
    <span class="lampb {link}" title={S.online ? `${S.latency} ms` : "Offline"}><i></i><span>{S.online ? `${S.latency}ms` : "Link"}</span></span>
    {#if owner}
      <button type="button" class="micb" class:on={talkOn} class:down={micDown} use:drag={mic} aria-label="Talk" aria-pressed={talkOn}>{@html micSVG}</button>
    {/if}
  </div>
</div>
