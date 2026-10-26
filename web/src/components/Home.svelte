<script lang="ts">
  import { S, toast } from "../lib/state.svelte";
  import { send, api } from "../lib/net";
  import { go, nav } from "../lib/nav.svelte";
  import { catOf, emblem, THEMES, iconSVG, iconFor } from "../lib/icons";
  import { haptic } from "../lib/haptics";
  import Info from "./Info.svelte";

  const TABS = [["views", "Views"], ["panels", "Panels"], ["build", "Build"], ["cockpit", "Cockpit"], ["settings", "Settings"]] as const;
  const CATS = [["flight", "Flight"], ["nav", "Nav"], ["eng", "Power"], ["def", "Defense"], ["combat", "Combat"], ["ind", "Industry"], ["crew", "Crew"], ["ui", "UI"], ["emerg", "Emergency"]];

  // ---------- data from the PC, fetched when a tab opens ----------
  let catalog = $state<{ groups: any[]; controls: any[] }>({ groups: [], controls: [] });
  let cockpit = $state<any>(null);
  let cfg = $state<any>(null);
  let devices = $state<any[]>([]);
  let status = $state<any>(null);
  const load = {
    async catalog() { try { const c = await api("catalog"); catalog = { groups: c.groups || [], controls: c.controls || [] }; } catch (e: any) { toast(e.message); } },
    async cockpit() { try { cockpit = await api("cockpit"); } catch (e: any) { toast(e.message); } },
    async settings() {
      try {
        const [c, d, s] = await Promise.all([api("config"), api("devices"), api("status")]);
        cfg = c; devices = d || []; status = s;
      } catch (e: any) { toast(e.message); }
    },
  };
  $effect(() => {
    if (nav.section !== "home") return;
    const t = nav.tab;
    if ((t === "panels" || t === "build") && !catalog.groups.length) load.catalog();
    if (t === "cockpit") load.cockpit();
    if (t === "settings") load.settings();
  });
  // The cockpit changed somewhere: keep the list current while it's open.
  $effect(() => { S.cockpit; if (nav.tab === "cockpit" && nav.section === "home") load.cockpit(); });

  // ---------- views ----------
  function pickView(id: string) { send({ t: "view", id }); haptic("press"); go("deck"); }

  // ---------- panels on this screen ----------
  const onScreen = $derived(new Set(S.groups.map((g) => g.id)));
  const addable = $derived(catalog.groups.filter((g) => !onScreen.has(g.id) && !g.game));
  const addGame = $derived(catalog.groups.filter((g) => !onScreen.has(g.id) && g.game));
  function addPanel(id: string) { send({ t: "cockpit-add", id }); haptic("press"); }
  function removePanel(id: string) { send({ t: "cockpit-remove", id }); haptic("tap"); }

  // ---------- build a panel ----------
  let bName = $state(""), bCat = $state("flight"), bFind = $state(""), bPick = $state<string[]>([]), bGroup = $state("");
  const byId = $derived(Object.fromEntries(catalog.controls.map((c) => [c.id, c])));
  const byGroup = $derived.by(() => {
    const m: Record<string, any[]> = {};
    for (const c of catalog.controls) (m[c.group] ||= []).push(c);
    return m;
  });
  const glyph = (c: any) => iconSVG(c.icon || iconFor(c, c.group));
  const found = $derived.by(() => {
    const q = bFind.trim().toLowerCase();
    return catalog.controls.filter((c) => (bGroup ? c.group === bGroup : true) && (!q || c.label.toLowerCase().includes(q))).slice(0, 120);
  });
  function toggle(id: string) { bPick = bPick.includes(id) ? bPick.filter((x) => x !== id) : [...bPick, id]; haptic("tick"); }
  async function savePanel() {
    if (!bName.trim() || !bPick.length) return;
    try {
      const p = await api("panels", { method: "POST", body: JSON.stringify({ name: bName.trim(), category: bCat, controls: bPick }) });
      send({ t: "cockpit-add", id: p.id });
      toast(`Saved ${p.name}`);
      haptic("press");
      bName = ""; bPick = []; bFind = "";
      load.catalog();
      go("deck");
    } catch (e: any) { toast(e.message); haptic("error"); }
  }

  // ---------- cockpit ----------
  const meId = $derived(S.cockpit?.me || "");
  async function cock(body: any) {
    try { cockpit = await api("cockpit", { method: "POST", body: JSON.stringify(body) }); } catch (e: any) { toast(e.message); haptic("error"); }
  }
  function solo(on: boolean) { send({ t: "cockpit-solo", on }); haptic("press"); setTimeout(load.cockpit, 400); }
  let renaming = $state(""), newName = $state("");

  // ---------- settings ----------
  function setTheme(id: string) {
    haptic("tap");
    if (S.member) send({ t: "theme", id });
    S.theme = id;
  }
  let saveT: ReturnType<typeof setTimeout>;
  function saveCfg() {
    clearTimeout(saveT);
    saveT = setTimeout(async () => {
      try { await api("config", { method: "PUT", body: JSON.stringify(cfg) }); } catch (e: any) { toast(e.message); haptic("error"); }
    }, 500);
  }
  async function bindAll() {
    try { const r = await api("keybinds/bind-all", { method: "POST" }); toast(r.added ? `Bound ${r.added} actions · restart the game to load them` : "Every action already has a key"); haptic("press"); } catch (e: any) { toast(e.message); }
  }
  async function unpair(id: string) {
    try { await api(`devices/${id}`, { method: "DELETE" }); devices = devices.filter((d) => d.id !== id); haptic("tap"); } catch (e: any) { toast(e.message); }
  }

  // The keyboard covers the bottom of the screen on iPad: keep the field in sight.
  let pane: HTMLElement | undefined = $state();
  function keepInSight(e: FocusEvent) {
    const el = e.target as HTMLElement;
    if (!el.matches("input, textarea, select") || !pane) return;
    setTimeout(() => {
      if (!pane) return;
      const kb = parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--kb")) || 0;
      const r = el.getBoundingClientRect(), pr = pane.getBoundingClientRect();
      const visible = Math.min(pr.bottom, window.innerHeight - kb) - pr.top;
      const want = r.top - pr.top - visible / 3;
      if (r.bottom > pr.top + visible - 12 || r.top < pr.top) pane.scrollBy({ top: want, behavior: "smooth" });
    }, 320);
  }
</script>

<div class="grab" aria-hidden="true"></div>
<div class="tabs" role="tablist">
  {#each TABS as [id, label] (id)}
    <button type="button" role="tab" class:on={nav.tab === id} aria-selected={nav.tab === id} onclick={() => { nav.tab = id; haptic("tick"); }}>{label}</button>
  {/each}
</div>

<div class="pane2" bind:this={pane} onfocusin={keepInSight}>
  {#if nav.tab === "views"}
    <div class="tiles">
      {#each S.views as v (v.id)}
        <button type="button" class="tile" class:on={S.view?.id === v.id} onclick={() => pickView(v.id)}>
          <b>{v.name}</b>
          <span class="sw">{#each (v.groups || []).slice(0, 8) as g (g)}<i data-cat={catOf(g)}></i>{/each}</span>
          <small>{(v.groups || []).length} panels</small>
        </button>
      {/each}
    </div>
    {#if S.member}<p class="note"><Info text="A view picked here changes every console in the cockpit." /> Cockpit view</p>{/if}

  {:else if nav.tab === "panels"}
    <div class="h2">On this screen</div>
    <div class="ptiles">
      {#each S.groups as g (g.id)}
        <div class="ptile on-screen" data-cat={catOf(g.id, g.category)}>
          <span class="pl"><i></i><b>{g.name}</b><em>{g.code || ""}</em></span>
          <span class="gl">{#each g.controls.slice(0, 4) as c (c.id)}<span>{@html iconSVG(c.icon || iconFor(c, g.id))}</span>{/each}</span>
          <button type="button" class="rm" onclick={() => removePanel(g.id)} aria-label="Take {g.name} off this screen">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M5 12h14"/></svg>
          </button>
        </div>
      {/each}
    </div>
    <div class="h2">Add <Info text="Adds a panel to this screen only. In a cockpit the other consoles keep theirs." /></div>
    <div class="ptiles">
      {#each addable as g (g.id)}
        <button type="button" class="ptile" data-cat={catOf(g.id, g.category)} onclick={() => addPanel(g.id)}>
          <span class="pl"><i></i><b>{g.name}</b><em>{g.code || ""}</em></span>
          <span class="gl">{#each (byGroup[g.id] || []).slice(0, 4) as c (c.id)}<span>{@html glyph(c)}</span>{/each}</span>
          <span class="add" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg></span>
        </button>
      {/each}
    </div>
    {#if addGame.length}
      <div class="h2">Game menus <Info text="Every action in each of the game's own keybind menus." /></div>
      <div class="ptiles">
        {#each addGame as g (g.id)}
          <button type="button" class="ptile" data-cat={catOf(g.id, g.category)} onclick={() => addPanel(g.id)}>
          <span class="pl"><i></i><b>{g.name.replace(/^Game:\s*/i, "")}</b><em>{g.code || ""}</em></span>
          <span class="gl">{#each (byGroup[g.id] || []).slice(0, 4) as c (c.id)}<span>{@html glyph(c)}</span>{/each}</span>
          <span class="add" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg></span>
        </button>
        {/each}
      </div>
    {/if}
    <div class="row-end"><button type="button" class="btn" onclick={() => (nav.tab = "build")}>Build a panel</button></div>

  {:else if nav.tab === "build"}
    <div class="build">
      <input class="field" bind:value={bName} placeholder="Panel name" aria-label="Panel name" maxlength="40" enterkeyhint="done" />
      <div class="seg local wide">
        {#each CATS as [id, l] (id)}<button type="button" class:on={bCat === id} data-cat={id} onclick={() => (bCat = id)}>{l}</button>{/each}
      </div>
      {#if bPick.length}
        <div class="h2">{bPick.length} picked</div>
        <div class="kcs">
          {#each bPick as id, i (id)}
            <button type="button" class="kc on" onclick={() => toggle(id)}><em>{i + 1}</em><span class="ic">{@html byId[id] ? glyph(byId[id]) : ""}</span><span class="l">{byId[id]?.label || id}</span></button>
          {/each}
        </div>
      {/if}
      <div class="h2">Controls</div>
      <div class="findrow">
        <input class="field" bind:value={bFind} placeholder="Find" aria-label="Find a control" enterkeyhint="search" />
        <select class="field" bind:value={bGroup} aria-label="From panel">
          <option value="">All panels</option>
          {#each catalog.groups as g (g.id)}<option value={g.id}>{g.name}</option>{/each}
        </select>
      </div>
      <div class="kcs">
        {#each found as c (c.id)}
          <button type="button" class="kc" class:on={bPick.includes(c.id)} data-cat={catOf(c.group)} onclick={() => toggle(c.id)}>
            {#if c.unbound}<em class="ub">BIND</em>{/if}<span class="ic">{@html glyph(c)}</span><span class="l">{c.label}</span>
          </button>
        {/each}
      </div>
      <div class="savebar">
        <button type="button" class="btn primary" disabled={!bName.trim() || !bPick.length} onclick={savePanel}>Save and add</button>
      </div>
    </div>

  {:else if nav.tab === "cockpit"}
    {#if cockpit}
      <div class="h2">Consoles <Info text="Every paired tablet that isn't solo is a console. Panels split across them; a lost console's panels move to its neighbor." /></div>
      <div class="cons">
        {#each cockpit.consoles as c (c.id)}
          <div class="con" class:me={c.id === meId} class:off={!c.online}>
            <i class="lamp" class:good={c.online && !c.solo} class:caution={c.solo} class:bad={c.lost}></i>
            {#if renaming === c.id}
              <form onsubmit={(e) => { e.preventDefault(); cock({ action: "console", id: c.id, name: newName }); renaming = ""; }}>
                <input class="field" bind:value={newName} aria-label="Console name" maxlength="24" enterkeyhint="done" />
              </form>
            {:else}
              <button type="button" class="nm" onclick={() => { renaming = c.id; newName = c.name; }}><b>{c.name}</b>{#if c.id === meId}<small>this screen</small>{/if}</button>
            {/if}
            <small>{c.solo ? "solo" : c.online ? `${(c.panels || []).length} panels` : c.hostedBy ? `hosted by ${cockpit.consoles.find((x: any) => x.id === c.hostedBy)?.name || "?"}` : "offline"}</small>
            <span class="acts">
              {#if c.online}<button type="button" class="btn sm" onclick={() => send({ t: "identify-ask", id: c.id })}>Identify</button>{/if}
              {#if !c.online}<button type="button" class="btn sm" onclick={() => cock({ action: "forget", id: c.id })}>Forget</button>{/if}
            </span>
          </div>
        {/each}
      </div>
      <div class="row-end">
        {#if S.member}
          <button type="button" class="btn" onclick={() => solo(true)}>Go solo</button>
        {:else}
          <button type="button" class="btn primary" onclick={() => solo(false)}>Join the cockpit</button>
        {/if}
        <button type="button" class="btn" onclick={() => cock({ action: "reset" })}>Even split</button>
      </div>
    {:else}
      <p class="note">Loading…</p>
    {/if}

  {:else if nav.tab === "settings"}
    <div class="h2">Theme</div>
    <div class="tiles themes">
      {#each THEMES as t (t)}
        <button type="button" class="tile theme-t" class:on={S.theme === t} data-theme={t} onclick={() => setTheme(t)}>
          <span class="em"><svg viewBox="0 0 64 64">{@html emblem(t).emblem}</svg></span><b>{emblem(t).name}</b>
        </button>
      {/each}
    </div>

    {#if cfg}
      <div class="h2">Voice</div>
      <div class="kv">
        <label for="v-on">Voice</label>
        <div class="seg local"><button type="button" class:on={cfg.voice} onclick={() => { cfg.voice = true; saveCfg(); }}>On</button><button type="button" class:on={!cfg.voice} onclick={() => { cfg.voice = false; saveCfg(); }}>Off</button></div>
        <label for="v-wake">Wake word</label>
        <input id="v-wake" class="field" bind:value={cfg.wakeWord} oninput={saveCfg} autocomplete="off" autocapitalize="none" />
        <label for="v-rep">Replies</label>
        <div class="seg local">{#each ["off", "short", "full"] as r (r)}<button type="button" class:on={cfg.replies === r} onclick={() => { cfg.replies = r; saveCfg(); }}>{r}</button>{/each}</div>
        <label for="v-voice">Voice of replies</label>
        <input id="v-voice" class="field" bind:value={cfg.ttsVoice} oninput={saveCfg} autocomplete="off" autocapitalize="none" />
        <span class="k1">State</span><span class="v">{S.voice.state}{S.voice.reason ? ` · ${S.voice.reason}` : ""}</span>
      </div>

      <div class="h2">Agent <Info text="Any OpenAI-compatible API. The key stays on the PC." /></div>
      <div class="kv">
        <label for="a-name">Name</label><input id="a-name" class="field" bind:value={cfg.agentName} oninput={saveCfg} autocomplete="off" />
        <label for="a-url">API</label><input id="a-url" class="field" bind:value={cfg.agentUrl} oninput={saveCfg} inputmode="url" autocomplete="off" autocapitalize="none" placeholder="https://api.x.ai/v1" />
        <label for="a-model">Model</label><input id="a-model" class="field" bind:value={cfg.agentModel} oninput={saveCfg} autocomplete="off" autocapitalize="none" />
        <label for="a-key">Key</label><input id="a-key" class="field" type="password" bind:value={cfg.agentKey} oninput={saveCfg} autocomplete="off" />
        <span class="k1">Style</span>
        <div class="seg local">{#each [["fast", "Fast"], ["crew", "Crew"]] as [v, l] (v)}<button type="button" class:on={cfg.agentStyle === v} onclick={() => { cfg.agentStyle = v; saveCfg(); }}>{l}</button>{/each}</div>
      </div>
    {/if}

    <div class="h2">Keys</div>
    <div class="row-end left">
      <button type="button" class="btn" onclick={bindAll}>Bind every action</button>
      <button type="button" class="btn" onclick={() => send({ t: "release-all" })}>Release all keys</button>
      <button type="button" class="btn" onclick={() => send({ t: "reset-states" })}>Reset lights</button>
    </div>

    {#if devices.length}
      <div class="h2">Devices</div>
      {#each devices as d (d.id)}
        <div class="rowi"><i class="lamp" class:good={d.online}></i><b>{d.name}</b><small>{d.platform || ""}{d.native ? " · app" : ""}</small>
          <button type="button" class="btn sm" onclick={() => unpair(d.id)}>Unpair</button></div>
      {/each}
    {/if}
    {#if status?.pairing}
      <div class="h2">Pair another</div>
      <div class="kv"><span class="k1">Code</span><span class="v code">{status.pairing.code}</span></div>
    {/if}
  {/if}
</div>
<button type="button" class="back up" onclick={() => go("deck")} aria-label="Back to the deck">
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M6 15l6-6 6 6"/></svg>
</button>
