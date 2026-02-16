<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { slide } from "svelte/transition";
    import {
        ChevronRight,
        ChevronDown,
        CheckCircle2,
        Clock,
        MoreHorizontal,
    } from "@lucide/svelte";

    let expanded = $state(true);

    // Filter completed sessions
    let sessions = $derived(appState.completedSessions);

    function openSession(id: string) {
        appState.selectSession(id);
    }
</script>

<div class="session-history">
    <button class="list-header" onclick={() => (expanded = !expanded)}>
        <div class="header-left">
            <div class="chevron-btn">
                {#if expanded}
                    <ChevronDown size={14} />
                {:else}
                    <ChevronRight size={14} />
                {/if}
            </div>
            <span class="header-title">Finished tasks</span>
        </div>
    </button>

    {#if expanded}
        <div class="sessions-grid" transition:slide>
            {#each sessions as session (session.id)}
                <button
                    class="session-card glass"
                    onclick={() => openSession(session.id)}
                >
                    <div class="card-icon">
                        <CheckCircle2 size={16} class="text-success" />
                    </div>
                    <div class="card-content">
                        <span
                            class="session-prompt"
                            title={session.latest_run?.prompt}
                        >
                            {session.latest_run?.prompt || "Untitled task"}
                        </span>
                        <div class="session-meta">
                            <span class="repo-name">{session.repo_name}</span>
                            <span class="dot">•</span>
                            <span class="time-ago">
                                {new Date(
                                    session.created_at,
                                ).toLocaleDateString()}
                            </span>
                        </div>
                    </div>
                    <div class="card-menu">
                        <MoreHorizontal size={14} />
                    </div>
                </button>
            {/each}

            {#if sessions.length === 0}
                <div class="empty-state">
                    <Clock size={24} class="opacity-30 mb-2" />
                    <span>No finished tasks yet</span>
                </div>
            {/if}
        </div>
    {/if}
</div>

<style>
    .session-history {
        margin-top: 24px;
        width: 100%;
        max-width: 800px;
        padding-bottom: 40px;
    }

    .list-header {
        display: flex;
        align-items: center;
        width: 100%;
        background: none;
        border: none;
        text-align: left;
        gap: 8px;
        padding: 8px 0;
        cursor: pointer;
        user-select: none;
        color: var(--color-text-secondary);
        margin-bottom: 8px;
        font-family: inherit;
    }

    .header-left {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .list-header:hover {
        color: var(--color-text);
    }

    .chevron-btn {
        background: none;
        border: none;
        padding: 0;
        color: inherit;
        cursor: pointer;
        display: flex;
        align-items: center;
    }

    .header-title {
        font-size: 14px;
        font-weight: 600;
    }

    .sessions-grid {
        display: grid;
        gap: 8px;
    }

    .session-card {
        display: flex;
        align-items: center;
        gap: 12px;
        width: 100%;
        text-align: left;
        background: transparent;
        border: 1px solid transparent;
        border-radius: 8px;
        padding: 10px 12px;
        cursor: pointer;
        transition: all 0.2s;
    }

    .session-card:hover {
        background: var(--color-bg-elevated);
        border-color: var(--color-border);
    }

    .card-icon {
        color: var(--color-success);
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .card-content {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .session-prompt {
        font-size: 13px;
        font-weight: 500;
        color: var(--color-text);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .session-meta {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 11px;
        color: var(--color-text-muted);
    }

    .repo-name {
        font-weight: 500;
    }

    .card-menu {
        color: var(--color-text-muted);
        opacity: 0;
        transition: opacity 0.2s;
    }

    .session-card:hover .card-menu {
        opacity: 1;
    }

    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 32px;
        color: var(--color-text-muted);
        font-size: 13px;
        border: 1px dashed var(--color-border);
        border-radius: 8px;
    }

    /* Utility classes from global css reference */
    :global(.text-success) {
        color: var(--color-success);
    }
    :global(.opacity-30) {
        opacity: 0.3;
    }
    :global(.mb-2) {
        margin-bottom: 0.5rem;
    }
</style>
