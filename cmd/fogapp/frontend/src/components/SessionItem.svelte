<script lang="ts">
    import type { SessionSummary } from "$lib/types";
    import { appState } from "$lib/stores.svelte";
    import { formatRelativeTime, truncatePrompt } from "$lib/utils";
    import { GitPullRequest, MessageSquare } from "@lucide/svelte";

    let { session }: { session: SessionSummary } = $props();

    const isActive = $derived(appState.selectedSessionID === session.id);
    const isBusy = $derived(session.busy);
    const prompt = $derived(session.latest_run?.prompt ?? session.id);
    const age = $derived(formatRelativeTime(session.updated_at));

    function select() {
        appState.selectSession(session.id);
    }
</script>

<button class="row" data-active={isActive} onclick={select}>
    <span class="row__glyph" aria-hidden="true">
        {#if session.pr_url}
            <GitPullRequest size={14} />
        {:else}
            <MessageSquare size={14} />
        {/if}
    </span>

    <span class="row__main">
        <span class="row__title">{truncatePrompt(prompt)}</span>
        <span class="row__meta">
            <time datetime={session.updated_at}>{age}</time>
            {#if session.pr_url}
                <span aria-hidden="true">·</span>
                <span class="row__pr">PR open</span>
            {/if}
        </span>
    </span>

    {#if isBusy}
        <!-- Colour is never the only signal: the label carries it too. -->
        <span class="badge badge--running">
            <span class="badge__dot" aria-hidden="true"></span>
            Running
        </span>
    {/if}
</button>

<style>
    .row__glyph {
        display: flex;
        flex: none;
        color: var(--color-ink-3);
    }

    .row[data-active="true"] .row__glyph {
        color: var(--color-accent);
    }

    .row__pr {
        color: var(--color-signal-add);
    }
</style>
