<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { LayoutGrid, Plus, Settings } from "@lucide/svelte";
    import TaskList from "./TaskList.svelte";
    import SessionHistory from "./SessionHistory.svelte";
</script>

<nav class="sidebar" aria-label="Application navigation">
    <!-- Brand mark — sits in the titlebar-inset zone on macOS, not a button -->
    <div class="sidebar__brand">
        <span class="sidebar__brand-name">Fog</span>
    </div>

    <!-- Primary nav -->
    <div class="sidebar__nav">
        <button
            class="row"
            data-active={appState.currentView === "board"}
            aria-current={appState.currentView === "board" ? "page" : undefined}
            onclick={() => appState.setView("board")}
        >
            <span class="row__glyph" aria-hidden="true">
                <LayoutGrid size={14} />
            </span>
            <span class="row__main">
                <span class="row__title">Board</span>
            </span>
        </button>
        <button
            class="row"
            data-active={appState.currentView === "new"}
            aria-current={appState.currentView === "new" ? "page" : undefined}
            onclick={() => appState.setView("new")}
        >
            <span class="row__glyph" aria-hidden="true">
                <Plus size={14} />
            </span>
            <span class="row__main">
                <span class="row__title">New session</span>
            </span>
        </button>
    </div>

    <!-- Running + finished sessions -->
    <div class="sidebar__sessions scroll-y">
        <TaskList />
        <SessionHistory />
    </div>

    <!-- Settings at the bottom -->
    <div class="sidebar__foot">
        <button
            class="row"
            data-active={appState.currentView === "settings"}
            aria-current={appState.currentView === "settings" ? "page" : undefined}
            onclick={() => appState.setView("settings")}
        >
            <span class="row__glyph" aria-hidden="true">
                <Settings size={14} />
            </span>
            <span class="row__main">
                <span class="row__title">Settings</span>
            </span>
        </button>
    </div>
</nav>

<style>
    .sidebar {
        display: flex;
        flex-direction: column;
        block-size: 100%;
        background: var(--color-paper-2);
        border-inline-end: var(--rule-hair) solid var(--color-rule);
        overflow: hidden;
    }

    /* Brand zone — spans the titlebar-inset area on macOS so the "Fog"
       wordmark sits where the title would appear, with the traffic lights
       to its left. Non-interactive by design. */
    .sidebar__brand {
        flex: none;
        display: flex;
        align-items: flex-end;
        padding-inline: var(--space-sm);
        padding-block-end: var(--space-xs);
        padding-block-start: calc(var(--titlebar-inset, 0px) + var(--space-xs));
        block-size: calc(var(--bar-h) + var(--titlebar-inset, 0px));
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .sidebar__brand-name {
        font-size: var(--text-md);
        font-weight: 700;
        letter-spacing: var(--tracking-tight);
        color: var(--color-ink);
    }

    /* Primary nav — Board, New session */
    .sidebar__nav {
        flex: none;
        border-block-end: var(--rule-hair) solid var(--color-rule);
        padding-block: var(--space-2xs);
    }

    /* Running + finished sessions — scrollable middle region */
    .sidebar__sessions {
        flex: 1;
        min-block-size: 0;
    }

    /* Give collapsible section headers horizontal breathing room */
    .sidebar__sessions :global(.sec__toggle) {
        padding-inline: var(--space-sm);
    }

    /* Settings footer */
    .sidebar__foot {
        flex: none;
        border-block-start: var(--rule-hair) solid var(--color-rule);
        padding-block: var(--space-2xs);
    }

    /* Icon slot before the row title — tinted to match row active state */
    .row__glyph {
        display: flex;
        flex: none;
        color: var(--color-ink-3);
    }

    .row[data-active="true"] .row__glyph {
        color: var(--color-accent);
    }
</style>
