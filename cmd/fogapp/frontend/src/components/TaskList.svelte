<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import {
        ChevronRight,
        ChevronDown,
        ExternalLink,
        Play,
        Activity,
        GitPullRequest,
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
        <div class="tasks-container">
            {#each tasks as task (task.id)}
                <div class="task-card">
                    <div class="card-header">
                        <div class="card-title-row">
                            <span
                                class="task-title"
                                title={task.latest_run?.prompt}
                            >
                                {task.latest_run?.prompt || "Untitled task"}
                            </span>
                            {#if task.pr_url}
                                <span class="pr-badge" title="PR Created">
                                    <GitPullRequest size={12} />
                                </span>
                            {/if}
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
                    <Activity size={24} class="text-muted mb-2" />
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
        background: #09090b; /* Solid hex */
        border: 1px solid var(--color-border);
        border-radius: 12px;
        padding: 16px;
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        /* transition removed */
    }

    .task-card:hover {
        border-color: var(--color-border-accent);
        /* transform removed */
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5); /* Stronger shadow */
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

    .pr-badge {
        display: flex;
        align-items: center;
        color: var(--color-success);
        margin-right: 4px;
    }

    .link-icon {
        color: var(--color-text-muted);
        background: none;
        border: none;
        cursor: pointer;
        padding: 0;
        transition: color 0.2s;
    }

    .link-icon:hover {
        color: var(--color-text);
    }

    .card-meta {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 11px;
        color: var(--color-text-muted);
    }

    .repo-tag {
        background: #1a1a1a; /* Solid equivalent of 0.05 white on black */
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
        background: #064e3b; /* Dark solid green (emerald-900 approx) */
        color: var(--color-success);
        border-color: #065f46;
    }

    .start-btn:hover {
        background: #065f46; /* Slightly lighter green */
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
        background: #450a0a; /* Solid dark red (red-950 approx) */
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
    :global(.text-muted) {
        color: var(--color-text-muted);
    }
    :global(.mb-2) {
        margin-bottom: 0.5rem;
    }
</style>
