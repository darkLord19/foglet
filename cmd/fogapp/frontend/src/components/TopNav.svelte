<script lang="ts">
  import { appState } from "$lib/stores.svelte";
  import { Settings } from "@lucide/svelte";

  function showHome() {
    appState.setView("new");
    appState.selectedSessionID = "";
  }

  function showSettings() {
    appState.setView("settings");
    appState.selectedSessionID = "";
  }
</script>

<!-- N7 brutal slab: full-bleed bar, opaque, 2px structural rule below. -->
<nav class="nav">
  <button class="mark" onclick={showHome} aria-label="Fog — home">
    <span class="mark__glyph" aria-hidden="true">▚</span>
    <span class="mark__name">FOG</span>
  </button>

  <button
    id="nav-settings"
    class="btn btn-ghost btn-icon nav__action"
    class:is-current={appState.currentView === "settings"}
    onclick={showSettings}
    aria-current={appState.currentView === "settings"}
    title="Settings"
    aria-label="Settings"
  >
    <Settings size={18} />
  </button>
</nav>

<style>
  .nav {
    position: fixed;
    inset-block-start: 0;
    inset-inline: 0;
    z-index: var(--z-sticky);
    block-size: var(--bar-h);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    padding-inline: var(--gutter);
    /* Opaque. The previous build applied a `.glass` class that was never
       defined anywhere, so content scrolled under a transparent bar. */
    background: var(--color-paper);
    border-block-end: var(--rule-hair) solid var(--color-rule);
  }

  .mark {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    padding: var(--space-2xs) var(--space-xs);
    background: none;
    border: var(--rule-hair) solid transparent;
    cursor: pointer;
    transition: border-color var(--dur-micro) var(--ease-out);
  }

  .mark:hover {
    border-color: var(--color-rule);
  }

  .mark:focus-visible {
    outline: var(--rule-hair) solid var(--color-focus);
    outline-offset: var(--rule-hair);
  }

  /* The wordmark is the accent's anchor point — a drawn glyph, not a
     rounded orb with a leftover blue glow. */
  .mark__glyph {
    display: grid;
    place-items: center;
    inline-size: 1.5rem;
    block-size: 1.5rem;
    background: var(--color-accent);
    color: var(--color-accent-ink);
    font-size: var(--text-sm);
    line-height: 1;
  }

  .mark__name {
    font-family: var(--font-body);
    font-weight: 800;
    font-size: var(--text-md);
    letter-spacing: 0.02em;
    line-height: var(--leading-caps);
    color: var(--color-ink);
  }

  /* Active chrome is a rule, not a fill — keeps the accent budget for the
     one primary action per surface. */
  .nav__action.is-current {
    color: var(--color-accent);
    border-block-end: var(--rule-active) solid var(--color-accent);
  }
</style>
