<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { slide } from "svelte/transition";
    import {
        ChevronRight,
        ChevronDown,
        ExternalLink,
        Play,
        Pencil,
        X,
        Activity,
    } from "@lucide/svelte";

    let expanded = $state(true);

    // Filter running sessions
    let tasks = $derived(appState.runningSessions);

    function openSession(id: string) {
        appState.selectSession(id);
    }
</script>

<div class="task-list">
    <button class="list-header" onclick={() => (expanded = !expanded)}>
        <div class="header-left">
            <div class="chevron-btn">
                {#if expanded}
                    <ChevronDown size={14} />
                {:else}
                    <ChevronRight size={14} />
                {/if}
            </div>
            <span class="header-title">Ongoing tasks</span>
            <span class="count-badge">{tasks.length}</span>
        </div>
    </button>

    {#if expanded}
        <div class="tasks-container" transition:slide>
            {#each tasks as task (task.id)}
                <div class="task-card glass">
                    <div class="card-header">
                        <div class="card-title-row">
                            <span
                                class="task-title"
                                title={task.latest_run?.prompt}
                            >
                                {task.latest_run?.prompt || "Untitled task"}
                            </span>
                            <button
                                class="link-icon"
                                onclick={() => openSession(task.id)}
                            >
                                <ExternalLink size={12} />
                            </button>
                        </div>
                        <div class="card-meta">
                            <span class="repo-tag">{task.repo_name}</span>
                            <span class="time-tag">Just now</span>
                        </div>
                    </div>

                    <div class="card-actions">
                        <button
                            class="action-btn start-btn"
                            onclick={() => openSession(task.id)}
                        >
                            <Play size={12} fill="currentColor" />
                            <span>Open</span>
                        </button>
                    </div>
                </div>
            {/each}

            {#if tasks.length === 0}
                <div class="empty-state">
                    <Activity size={24} class="opacity-30 mb-2" />
                    <span>No active tasks</span>
                </div>
            {/if}
        </div>
    {/if}
</div>

<style>
    .task-list {
        margin-top: 32px;
        width: 100%;
        max-width: 800px;
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

    .count-badge {
        background: var(--color-bg-elevated);
        padding: 2px 6px;
        border-radius: 99px;
        font-size: 10px;
        font-weight: 700;
    }

    .tasks-container {
        display: grid;
        gap: 12px;
        margin-top: 12px;
    }

    .task-card {
        background: var(--color-bg-elevated);
        border: 1px solid var(--color-border);
        border-radius: 12px;
        padding: 16px;
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        transition: all 0.2s;
    }

    .task-card:hover {
        border-color: var(--color-border-accent);
        transform: translateY(-1px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    }

    .card-header {
        flex: 1;
        min-width: 0;
        margin-right: 16px;
    }

    .card-title-row {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 4px;
    }

    .task-title {
        font-size: 14px;
        font-weight: 500;
        color: var(--color-text);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .link-icon {
        color: var(--color-text-muted);
        opacity: 0;
        transition: opacity 0.2s;
        background: none;
        border: none;
        cursor: pointer;
        padding: 0;
    }

    .task-card:hover .link-icon {
        opacity: 1;
    }

    .card-meta {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 11px;
        color: var(--color-text-muted);
    }

    .repo-tag {
        background: rgba(255, 255, 255, 0.05);
        padding: 2px 6px;
        border-radius: 4px;
    }

    .card-actions {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .action-btn {
        display: flex;
        align-items: center;
        gap: 6px;
        height: 28px;
        padding: 0 10px;
        border-radius: 6px;
        border: 1px solid transparent;
        cursor: pointer;
        font-size: 12px;
        font-weight: 500;
        transition: all 0.2s;
    }

    .start-btn {
        background: rgba(16, 185, 129, 0.1);
        color: var(--color-success);
        border-color: rgba(16, 185, 129, 0.2);
    }

    .start-btn:hover {
        background: rgba(16, 185, 129, 0.2);
    }

    :global(.icon-only) {
        width: 28px;
        padding: 0;
        justify-content: center;
        background: transparent;
        color: var(--color-text-muted);
    }

    :global(.icon-only:hover) {
        background: var(--color-bg-hover);
        color: var(--color-text);
    }

    :global(.icon-only.danger:hover) {
        color: var(--color-danger);
        background: rgba(239, 68, 68, 0.1);
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
        border-radius: 12px;
        margin-top: 12px;
    }

    /* Utility classes */
    :global(.opacity-30) {
        opacity: 0.3;
    }
    :global(.mb-2) {
        margin-bottom: 0.5rem;
    }
</style>
