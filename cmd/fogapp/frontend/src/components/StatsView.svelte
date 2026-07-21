<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { formatRelativeTime } from "$lib/utils";

    const runs = $derived(appState.detailRuns ?? []);
    const latestRun = $derived(runs[0]);
    const session = $derived(appState.detailSession);

    // Session facts, not metrics. These are key–value pairs, so they render as
    // a definition list rather than a grid of stat cards — a card grid implies
    // measurement that isn't happening here.
    const facts = $derived([
        { label: "Runs", value: String(runs.length), mono: true },
        { label: "Status", value: session?.status ?? "—", mono: false },
        { label: "Latest state", value: latestRun?.state.replace("AI_", "") ?? "—", mono: false },
        { label: "Tool", value: session?.tool ?? "—", mono: false },
        { label: "Model", value: session?.model || "—", mono: true },
        { label: "Branch", value: session?.branch ?? "—", mono: true },
        { label: "Session", value: session?.id ?? "—", mono: true },
        {
            label: "Updated",
            value: session ? formatRelativeTime(session.updated_at) : "—",
            mono: false,
        },
        {
            label: "Created",
            value: session ? formatRelativeTime(session.created_at) : "—",
            mono: false,
        },
    ]);
</script>

<div class="sv">
    <div class="panel">
        <div class="panel__head">
            <span class="panel__title">Session</span>
        </div>

        <dl class="facts">
            {#each facts as fact (fact.label)}
                <div class="facts__row">
                    <dt class="facts__key">{fact.label}</dt>
                    <dd class="facts__val" class:is-mono={fact.mono}>
                        {fact.value}
                    </dd>
                </div>
            {/each}
        </dl>
    </div>

    {#if session?.worktree_path}
        <div class="panel">
            <div class="panel__head">
                <span class="panel__title">Worktree</span>
            </div>
            <p class="sv__path mono">{session.worktree_path}</p>
        </div>
    {/if}
</div>

<style>
    .sv {
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
        max-inline-size: 52rem;
        min-inline-size: 0;
    }

    .facts {
        margin: 0;
        display: flex;
        flex-direction: column;
    }

    .facts__row {
        display: grid;
        grid-template-columns: minmax(6rem, 12rem) minmax(0, 1fr);
        gap: var(--space-md);
        padding: var(--space-xs) var(--space-md);
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .facts__row:last-child {
        border-block-end: none;
    }

    .facts__key {
        font-size: var(--text-2xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        line-height: 1.9;
        color: var(--color-ink-3);
    }

    .facts__val {
        margin: 0;
        font-size: var(--text-sm);
        color: var(--color-ink);
        overflow-wrap: anywhere;
        min-inline-size: 0;
    }

    .facts__val.is-mono {
        font-family: var(--font-mono);
        font-variant-numeric: tabular-nums;
    }

    /* Narrow panes stack the key above its value instead of squeezing the
       value into a few characters. */
    @container (max-width: 28rem) {
        .facts__row {
            grid-template-columns: minmax(0, 1fr);
            gap: var(--space-3xs);
        }
    }

    .sv__path {
        padding: var(--space-sm) var(--space-md);
        font-size: var(--text-xs);
        color: var(--color-ink-2);
        overflow-wrap: anywhere;
    }
</style>
