<script lang="ts">
  // (i): the explanation shows on tap, not as text on the page.
  let { text }: { text: string } = $props();
  let open = $state(false);
</script>

<button type="button" class="info" class:open onclick={(e) => { e.stopPropagation(); open = !open; }} onblur={() => (open = false)} aria-label={text}>
  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9.5"/><path d="M12 11v6M12 7.5v.5" stroke-linecap="round"/></svg>
  {#if open}<span class="tip" role="tooltip">{text}</span>{/if}
</button>
