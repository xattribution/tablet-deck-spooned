<script lang="ts">
  import { S } from "../lib/state.svelte";
  import { send } from "../lib/net";
  import { go } from "../lib/nav.svelte";
  import { haptic } from "../lib/haptics";
  import { talkDown, talkUp, micSVG } from "../lib/voice";
  import { drag } from "../lib/press";

  let text = $state("");
  let log: HTMLElement | undefined = $state();
  $effect(() => { S.chat.length; S.busy; if (log) requestAnimationFrame(() => log!.scrollTo({ top: log!.scrollHeight, behavior: "smooth" })); });
  function ask(e: Event) {
    e.preventDefault();
    const t = text.trim();
    if (!t) return;
    send({ t: "chat", text: t });
    text = "";
    haptic("tap");
  }
  function answer(id: string, d: boolean) { send({ t: "answer", id, do: d }); haptic(d ? "press" : "tap"); }
  const talkOn = $derived(S.talkMode === "live" ? S.live : S.listening);
  let down = $state(false);
  const mic = { start() { down = true; talkDown(); return {}; }, move() {}, end() { down = false; talkUp(); } };
</script>

<div class="comms">
  {#if S.requests.length}
    <div class="h2">Requests</div>
    <div class="reqs">
      {#each S.requests as r (r.id)}
        <div class="rowi req">
          <b>{r.from}</b><span>{r.label || r.text}</span>
          <button type="button" class="btn sm" onclick={() => answer(r.id, false)}>Decline</button>
          <button type="button" class="btn sm primary" onclick={() => answer(r.id, true)}>Do it</button>
        </div>
      {/each}
    </div>
  {/if}

  <div class="h2">{S.agent.name || "Agent"}
    {#if S.chat.length}<button type="button" class="btn sm" onclick={() => send({ t: "chat-clear" })}>Clear</button>{/if}
  </div>
  <div class="chatlog" bind:this={log}>
    {#each S.chat as l, i (i)}
      <p class="ln {l.who}">{l.text}</p>
    {:else}
      <p class="ln system">{S.agent.ready ? `Ask ${S.agent.name || "the agent"} anything, or tell it what to press.` : "Set up the agent in Settings."}</p>
    {/each}
    {#if S.busy}<span class="dots"><i></i><i></i><i></i></span>{/if}
  </div>
  <form class="ask" onsubmit={ask}>
    <button type="button" class="micb" class:on={talkOn} class:down use:drag={mic} aria-label="Talk">{@html micSVG}</button>
    <input class="field" bind:value={text} enterkeyhint="send" placeholder="Message" aria-label="Message" autocomplete="off" />
    <button class="btn primary" disabled={!text.trim()}>Send</button>
  </form>

  {#if S.crew.length}
    <div class="h2">Crew</div>
    <div class="crew">
      {#each S.crew as c (c.id)}
        <span class="rowi"><i class="lamp" class:good={c.online && !c.afk} class:caution={c.afk}></i><b>{c.name}</b>{#if c.afk}<small>AFK</small>{/if}</span>
      {/each}
    </div>
  {/if}
  <button type="button" class="back" onclick={() => go("deck")} aria-label="Back to the deck">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M6 9l6 6 6-6"/></svg>
  </button>
</div>
