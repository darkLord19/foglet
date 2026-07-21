<script lang="ts">
    import type { RunSummary, RunEvent } from "$lib/types";
    import { formatRelativeTime } from "$lib/utils";
    import { ChevronDown } from "@lucide/svelte";

    let {
        run,
        index,
        isSelected,
        isLast,
        outputEvents = [],
        inProgress,
        onselect,
    }: {
        run: RunSummary;
        index: number;
        isSelected: boolean;
        isLast: boolean;
        outputEvents?: RunEvent[];
        inProgress: boolean;
        onselect: () => void;
    } = $props();

    let statusKind = $derived(
        run.state === "COMPLETED"
            ? "done"
            : run.state === "FAILED"
              ? "failed"
              : run.state === "CANCELLED"
                ? "warn"
                : inProgress
                  ? "running"
                  : "idle",
    );

    let showOutput = $derived(isSelected && outputEvents.length > 0);
    let showPending = $derived(isSelected && inProgress);
</script>

<article class="run" class:is-selected={isSelected}>
    <div class="run__rail">
        <span class="run__num mono">{index}</span>
        {#if !isLast}
            <span class="run__line" aria-hidden="true"></span>
        {/if}
    </div>

    <div class="run__body">
        <header class="run__head">
            <time class="run__time mono" datetime={run.created_at}>
                {formatRelativeTime(run.created_at)}
            </time>
            <span class="badge badge--{statusKind}">
                <span class="badge__dot" aria-hidden="true"></span>
                {run.state.replace("AI_", "")}
            </span>
        </header>

        <div class="blk">
            <p class="blk__label">Prompt</p>
            <p class="blk__text">{run.prompt}</p>
        </div>

        {#if showOutput}
            {#each outputEvents as evt (evt.id)}
                <div class="blk blk--out">
                    <p class="blk__label">Output</p>
                    <p class="blk__text">{evt.message || evt.data}</p>
                </div>
            {/each}
        {:else if showPending}
            <div class="blk blk--out">
                <p class="blk__label">Output</p>
                <p class="blk__pending">
                    <span class="spinner" aria-hidden="true"></span>
                    Working…
                </p>
            </div>
        {/if}

        {#if !isSelected}
            <button class="run__more" onclick={onselect}>
                <ChevronDown size={13} />
                <span>Show output</span>
            </button>
        {/if}
    </div>
</article>

<style>
    .run {
        display: flex;
        gap: var(--space-md);
        min-inline-size: 0;
    }

    .run__rail {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: var(--space-2xs);
        flex: none;
        inline-size: 1.75rem;
    }

    /* Square numeral, not a round status orb. */
    .run__num {
        display: grid;
        place-items: center;
        inline-size: 1.75rem;
        block-size: 1.75rem;
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule);
        font-size: var(--text-2xs);
        font-weight: 700;
        color: var(--color-ink-3);
    }

    .run.is-selected .run__num {
        background: var(--color-accent);
        border-color: var(--color-accent);
        color: var(--color-accent-ink);
    }

    .run__line {
        inline-size: var(--rule-hair);
        flex: 1;
        min-block-size: var(--space-md);
        background: var(--color-rule);
    }

    .run__body {
        flex: 1;
        min-inline-size: 0;
        display: flex;
        flex-direction: column;
        gap: var(--space-xs);
        padding-block-end: var(--space-lg);
    }

    .run__head {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-sm);
        min-block-size: 1.75rem;
    }

    .run__time {
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }

    /* ── Content blocks ──────────────────────────────────────────────
       A labelled block with a leading rule, rather than a chat bubble
       with an avatar. */
    .blk {
        padding: var(--space-xs) var(--space-sm);
        background: var(--color-paper-2);
        border-inline-start: var(--rule-hair) solid var(--color-rule);
        min-inline-size: 0;
    }

    .blk--out {
        border-inline-start-color: var(--color-accent);
    }

    .blk__label {
        font-size: var(--text-2xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        line-height: var(--leading-caps);
        color: var(--color-ink-3);
        margin-block-end: var(--space-2xs);
    }

    .blk__text {
        font-size: var(--text-sm);
        line-height: var(--leading-body);
        color: var(--color-ink-2);
        white-space: pre-wrap;
        overflow-wrap: anywhere;
        max-inline-size: var(--measure);
    }

    .blk--out .blk__text {
        color: var(--color-ink);
    }

    .blk__pending {
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        font-size: var(--text-sm);
        color: var(--color-ink-3);
    }

    .run__more {
        align-self: flex-start;
        display: flex;
        align-items: center;
        gap: var(--space-2xs);
        padding: var(--space-2xs) 0;
        background: none;
        border: none;
        color: var(--color-ink-3);
        font-size: var(--text-2xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        cursor: pointer;
        transition: color var(--dur-micro) var(--ease-out);
    }

    .run__more:hover {
        color: var(--color-accent);
    }

    .run__more:focus-visible {
        outline: var(--rule-hair) solid var(--color-focus);
        outline-offset: var(--rule-hair);
    }
</style>
