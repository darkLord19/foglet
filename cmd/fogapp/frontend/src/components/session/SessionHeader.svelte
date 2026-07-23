<script lang="ts">
    import type { SessionSummary, RunSummary } from "$lib/types";
    import {
        RefreshCcw,
        GitFork,
        Code,
        Square,
        GitPullRequest,
        ArrowLeft,
    } from "@lucide/svelte";

    let {
        session,
        latestRun,
        isBusy,
        title,
        onBack,
        onRerun,
        onFork,
        onStop,
        onOpenEditor,
        onOpenPR,
    }: {
        session: SessionSummary;
        latestRun?: RunSummary;
        isBusy: boolean;
        title: string;
        onBack: () => void;
        onRerun: () => void;
        onFork: () => void;
        onStop: () => void;
        onOpenEditor: () => void;
        onOpenPR: () => void;
    } = $props();

    let statusKind = $derived(
        isBusy
            ? "running"
            : session.status === "failed" || session.status === "error"
              ? "failed"
              : session.status === "done" || session.status === "completed"
                ? "done"
                : "idle",
    );
</script>

<header class="head">
    <div class="head__top">
        <button
            class="btn btn-ghost btn-icon head__back"
            title="Back to board"
            aria-label="Back to board"
            onclick={onBack}
        >
            <ArrowLeft size={16} />
        </button>
        <div class="head__id">
            <p class="head__crumb mono">
                <span class="truncate">{session.repo_name}</span>
                <span class="head__sep" aria-hidden="true">/</span>
                <span>{session.id.substring(0, 8)}</span>
            </p>
            <h1 id="detail-title" class="head__title truncate">{title}</h1>
        </div>

        <div class="head__status">
            <span class="badge badge--{statusKind}">
                <span class="badge__dot" aria-hidden="true"></span>
                {session.status}
            </span>
        </div>
    </div>

    <div class="head__actions">
        {#if isBusy}
            <button
                id="detail-stop"
                class="btn btn-danger"
                onclick={onStop}
            >
                <Square size={14} />
                <span>Stop</span>
            </button>
        {:else}
            <button
                id="detail-rerun"
                class="btn btn-secondary"
                onclick={onRerun}
                disabled={!latestRun}
            >
                <RefreshCcw size={14} />
                <span>Re-run</span>
            </button>
        {/if}

        <button
            id="detail-fork"
            class="btn btn-secondary"
            onclick={onFork}
            disabled={isBusy || !latestRun}
        >
            <GitFork size={14} />
            <span>Fork</span>
        </button>

        {#if session.pr_url}
            <button class="btn btn-secondary head__pr" onclick={onOpenPR}>
                <GitPullRequest size={14} />
                <span>View PR</span>
            </button>
        {/if}

        <!-- The one primary action on this surface. -->
        <button id="detail-open" class="btn btn-primary" onclick={onOpenEditor}>
            <Code size={14} />
            <span>Open in editor</span>
        </button>
    </div>
</header>

<style>
    .head {
        display: flex;
        flex-direction: column;
        gap: var(--space-sm);
        padding: var(--space-md) var(--gutter);
        border-block-end: var(--rule-hair) solid var(--color-rule);
        min-inline-size: 0;
        container-type: inline-size;
    }

    .head__top {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        gap: var(--space-md);
        min-inline-size: 0;
    }

    .head__back {
        flex: none;
        margin-block-start: var(--space-3xs);
    }

    .head__id {
        display: flex;
        flex-direction: column;
        gap: var(--space-3xs);
        min-inline-size: 0;
    }

    .head__crumb {
        display: flex;
        align-items: center;
        gap: var(--space-2xs);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
        min-inline-size: 0;
    }

    .head__sep {
        color: var(--color-rule-2);
    }

    .head__title {
        font-size: var(--text-md);
        line-height: var(--leading-tight);
        max-inline-size: 60ch;
    }

    .head__status {
        flex: none;
    }

    .head__actions {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--space-2xs);
    }

    .head__pr {
        color: var(--color-signal-add);
        border-color: var(--color-signal-add);
    }

    /* Tight panes drop the button labels' padding rather than wrapping the
       labels themselves — affordances stay single-line. */
    @container (max-width: 44rem) {
        .head__actions {
            justify-content: flex-start;
        }

        .head__actions :global(.btn) {
            padding-inline: var(--space-sm);
        }
    }
</style>
