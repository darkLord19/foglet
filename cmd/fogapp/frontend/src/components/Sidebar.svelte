<script lang="ts">
  import { appState } from "$lib/stores.svelte";
  import { formatRelativeTime, truncatePrompt } from "$lib/utils";
  import { LayoutGrid, Settings, Plus } from "@lucide/svelte";

  const running = $derived(appState.runningSessions);
  const recent = $derived(appState.completedSessions.slice(0, 12));

  function go(view: "board" | "new" | "settings") {
    appState.setView(view);
    appState.selectedSessionID = "";
  }
</script>

<aside class="side">
  <div class="side__top">
    <button class="mark" onclick={() => go("board")}>
      <span class="mark__dot" aria-hidden="true">F</span>
      <span class="mark__name">Fog</span>
    </button>
  </div>

  <nav class="side__nav" aria-label="Main">
    <button
      class="nav"
      class:is-on={appState.currentView === "board"}
      aria-current={appState.currentView === "board"}
      onclick={() => go("board")}
    >
      <LayoutGrid size={14} />
      <span>Board</span>
    </button>

    <button
      id="nav-new"
      class="nav"
      class:is-on={appState.currentView === "new"}
      aria-current={appState.currentView === "new"}
      onclick={() => go("new")}
    >
      <Plus size={14} />
      <span>New session</span>
    </button>
  </nav>

  <div class="side__scroll scroll-y">
    {#if running.length > 0}
      <p class="side__label">Running</p>
      <div class="rows">
        {#each running as s (s.id)}
          <button
            class="row"
            data-active={appState.selectedSessionID === s.id}
            onclick={() => appState.selectSession(s.id)}
          >
            <span class="row__main">
              <span class="row__title">
                {truncatePrompt(s.latest_run?.prompt ?? s.id)}
              </span>
              <span class="row__meta">
                <span class="truncate">{s.repo_name}</span>
              </span>
            </span>
            <span class="badge badge--running">
              <span class="badge__dot" aria-hidden="true"></span>
            </span>
          </button>
        {/each}
      </div>
    {/if}

    <p class="side__label">Recent</p>
    {#if recent.length > 0}
      <div class="rows">
        {#each recent as s (s.id)}
          <button
            class="row"
            data-active={appState.selectedSessionID === s.id}
            onclick={() => appState.selectSession(s.id)}
          >
            <span class="row__main">
              <span class="row__title">
                {truncatePrompt(s.latest_run?.prompt ?? s.id)}
              </span>
              <span class="row__meta">
                <span class="truncate">{s.repo_name}</span>
                <span aria-hidden="true">·</span>
                <time datetime={s.updated_at}>
                  {formatRelativeTime(s.updated_at)}
                </time>
              </span>
            </span>
          </button>
        {/each}
      </div>
    {:else}
      <p class="side__none">No sessions yet.</p>
    {/if}
  </div>

  <div class="side__foot">
    <button
      id="nav-settings"
      class="nav"
      class:is-on={appState.currentView === "settings"}
      aria-current={appState.currentView === "settings"}
      onclick={() => go("settings")}
    >
      <Settings size={14} />
      <span>Settings</span>
    </button>
  </div>
</aside>

<style>
  .side {
    inline-size: var(--side-w);
    flex: none;
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    background: var(--color-paper-2);
    border-inline-end: var(--rule-hair) solid var(--color-rule);
  }

  .side__top {
    display: flex;
    align-items: center;
    block-size: var(--bar-h);
    padding-inline: var(--space-sm);
    flex: none;
  }

  .mark {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    padding: var(--space-2xs);
    background: none;
    border: none;
    border-radius: var(--radius);
    cursor: pointer;
    transition: background-color var(--dur-micro) var(--ease-out);
  }

  .mark:hover {
    background: var(--color-paper-3);
  }

  .mark:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: 1px;
  }

  .mark__dot {
    display: grid;
    place-items: center;
    inline-size: 1.125rem;
    block-size: 1.125rem;
    background: var(--color-accent);
    color: var(--color-accent-ink);
    border-radius: var(--radius-sm);
    font-size: 0.625rem;
    font-weight: 700;
  }

  .mark__name {
    font-size: var(--text-md);
    font-weight: 600;
    letter-spacing: var(--tracking-tight);
    color: var(--color-ink);
  }

  .side__nav,
  .side__foot {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: var(--space-2xs) var(--space-xs);
    flex: none;
  }

  .side__foot {
    border-block-start: var(--rule-hair) solid var(--color-rule);
  }

  .nav {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    inline-size: 100%;
    block-size: var(--row-h);
    padding-inline: var(--space-xs);
    background: transparent;
    border: none;
    border-radius: var(--radius);
    color: var(--color-ink-2);
    font-family: var(--font-body);
    font-size: var(--text-sm);
    text-align: start;
    white-space: nowrap;
    cursor: pointer;
    transition:
      background-color var(--dur-micro) var(--ease-out),
      color var(--dur-micro) var(--ease-out);
  }

  .nav:hover {
    background: var(--color-paper-3);
    color: var(--color-ink);
  }

  .nav:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -2px;
  }

  .nav.is-on {
    background: var(--color-paper-4);
    color: var(--color-ink);
    font-weight: 500;
  }

  .side__scroll {
    flex: 1;
    min-block-size: 0;
    padding-block-end: var(--space-sm);
  }

  .side__label {
    padding: var(--space-sm) var(--space-sm) var(--space-2xs);
    font-size: var(--text-2xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: var(--tracking-label);
    line-height: var(--leading-caps);
    color: var(--color-ink-3);
  }

  .side__none {
    padding: 0 var(--space-sm);
    font-size: var(--text-2xs);
    color: var(--color-ink-3);
  }

  /* Sidebar rows sit flush to the rail, so they lose the primitive's
     horizontal padding step and keep the accent edge tight to the panel. */
  .side__scroll .row {
    padding-inline: var(--space-sm);
  }

  .side__scroll .badge {
    padding-inline: var(--space-2xs);
  }
</style>
