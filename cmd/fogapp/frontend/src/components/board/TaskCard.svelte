<script lang="ts">
    import type { Task } from "$lib/types";
    import { appState } from "$lib/stores.svelte";
    import { formatRelativeTime, openExternal } from "$lib/utils";
    import { Play, GitBranch, ExternalLink } from "@lucide/svelte";

    let {
        task,
        dragging = false,
        onstart,
        onopen,
        ondragstart,
        ondragend,
    }: {
        task: Task;
        dragging?: boolean;
        onstart: (task: Task) => void;
        onopen: (task: Task) => void;
        ondragstart: (task: Task) => void;
        ondragend: () => void;
    } = $props();

    const session = $derived(
        task.session_id
            ? appState.sessions.find((s) => s.id === task.session_id)
            : undefined,
    );

    const isRunning = $derived(!!session && appState.isSessionRunning(session));

    /**
     * A card sitting in a working column with nothing running is usually one
     * that arrived from a tracker: remote moves reclassify but deliberately
     * never launch an agent, so a human still has to start it.
     *
     * In Review only offers the button once an implementation exists — the
     * reviewer reads that worktree, so there is nothing to review without it.
     */
    const needsStart = $derived(
        (task.status === "in_progress" && !isRunning && !task.session_id) ||
            (task.status === "in_review" && !isRunning && !!task.session_id),
    );

    const startLabel = $derived(
        task.status === "in_review" ? "Review" : "Start",
    );

    function openIssue(e: MouseEvent) {
        e.stopPropagation();
        openExternal(task.external_url);
    }
</script>

<div
    class="card"
    class:is-dragging={dragging}
    draggable="true"
    role="button"
    tabindex="0"
    aria-roledescription="Draggable task"
    ondragstart={() => ondragstart(task)}
    ondragend={ondragend}
    onclick={() => onopen(task)}
    onkeydown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onopen(task);
        }
    }}
>
    <p class="card__title">{task.title}</p>

    {#if task.body}
        <p class="card__body">{task.body}</p>
    {/if}

    <div class="card__foot">
        {#if task.repo_name}
            <span class="card__repo truncate">
                <GitBranch size={11} />
                {task.repo_name}
            </span>
        {:else}
            <span class="card__warn">No repo</span>
        {/if}

        {#if isRunning}
            <span class="badge badge--running">
                <span class="badge__dot" aria-hidden="true"></span>
                Running
            </span>
        {:else if needsStart}
            <button
                class="btn btn-secondary btn-sm"
                onclick={(e) => {
                    e.stopPropagation();
                    onstart(task);
                }}
            >
                <Play size={11} />
                <span>{startLabel}</span>
            </button>
        {:else}
            <time class="card__time" datetime={task.updated_at}>
                {formatRelativeTime(task.updated_at)}
            </time>
        {/if}
    </div>

    {#if task.external_key}
        <button class="card__ext" onclick={openIssue} title="Open in tracker">
            <span class="mono">{task.external_key}</span>
            <ExternalLink size={10} />
        </button>
    {/if}
</div>

<style>
    .card {
        position: relative;
        display: flex;
        flex-direction: column;
        gap: var(--space-2xs);
        padding: var(--space-sm);
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule);
        border-radius: var(--radius);
        cursor: grab;
        transition:
            background-color var(--dur-micro) var(--ease-out),
            border-color var(--dur-micro) var(--ease-out);
    }

    .card:hover {
        background: var(--color-paper-3);
        border-color: var(--color-rule-2);
    }

    .card:focus-visible {
        outline: 2px solid var(--color-focus);
        outline-offset: 1px;
    }

    /* The card stays in place and dims; a lifted, rotated clone is a tell
       and fights the direction's flatness. */
    .card.is-dragging {
        opacity: 0.4;
        cursor: grabbing;
    }

    .card__title {
        font-size: var(--text-sm);
        line-height: var(--leading-tight);
        color: var(--color-ink);
        overflow-wrap: anywhere;
    }

    .card__body {
        font-size: var(--text-2xs);
        line-height: var(--leading-tight);
        color: var(--color-ink-3);
        display: -webkit-box;
        -webkit-line-clamp: 2;
        line-clamp: 2;
        -webkit-box-orient: vertical;
        overflow: hidden;
    }

    .card__foot {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-xs);
        margin-block-start: var(--space-3xs);
        min-inline-size: 0;
    }

    .card__repo {
        display: flex;
        align-items: center;
        gap: var(--space-3xs);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
        min-inline-size: 0;
    }

    .card__warn {
        font-size: var(--text-2xs);
        color: var(--color-signal-warn);
    }

    .card__time {
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
        font-variant-numeric: tabular-nums;
        flex: none;
    }

    .card__ext {
        display: flex;
        align-items: center;
        gap: var(--space-3xs);
        align-self: flex-start;
        padding: 0;
        background: none;
        border: none;
        color: var(--color-ink-3);
        font-size: var(--text-2xs);
        cursor: pointer;
    }

    .card__ext:hover {
        color: var(--color-accent);
    }

    .card__ext:focus-visible {
        outline: 2px solid var(--color-focus);
        outline-offset: 1px;
    }
</style>
