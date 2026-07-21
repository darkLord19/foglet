<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { formatRelativeTime, openExternal } from "$lib/utils";
    import { Check, GitPullRequest } from "@lucide/svelte";
    import Collapsible from "./common/Collapsible.svelte";

    let sessions = $derived(appState.completedSessions);

    function openSession(id: string) {
        appState.selectSession(id);
    }

    function openPR(e: MouseEvent, url?: string) {
        e.stopPropagation();
        openExternal(url);
    }
</script>

<Collapsible title="Finished" count={sessions.length}>
    {#if sessions.length > 0}
        <div class="rows">
            {#each sessions as session (session.id)}
                <!-- The PR link sits beside the row rather than inside it: the
                     previous build nested a <button> within a <button>, which
                     is invalid markup and unreachable by keyboard. -->
                <div class="pair">
                    <button
                        class="row"
                        onclick={() => openSession(session.id)}
                    >
                        <span class="row__glyph" aria-hidden="true">
                            <Check size={14} />
                        </span>
                        <span class="row__main">
                            <span class="row__title">
                                {session.latest_run?.prompt || "Untitled task"}
                            </span>
                            <span class="row__meta">
                                <span class="truncate">{session.repo_name}</span>
                                <span aria-hidden="true">·</span>
                                <time datetime={session.created_at}>
                                    {formatRelativeTime(session.created_at)}
                                </time>
                            </span>
                        </span>
                    </button>

                    {#if session.pr_url}
                        <button
                            class="btn btn-ghost btn-icon pair__pr"
                            onclick={(e) => openPR(e, session.pr_url)}
                            title="Open pull request"
                            aria-label="Open pull request for this session"
                        >
                            <GitPullRequest size={14} />
                        </button>
                    {/if}
                </div>
            {/each}
        </div>
    {:else}
        <div class="empty">
            <p class="empty__title">No finished runs</p>
            <p>Completed sessions collect here.</p>
        </div>
    {/if}
</Collapsible>

<style>
    .pair {
        display: flex;
        align-items: stretch;
        border-block-end: var(--rule-hair) solid var(--color-rule);
        min-inline-size: 0;
    }

    .pair:last-child {
        border-block-end: none;
    }

    .pair .row {
        border-block-end: none;
    }

    .pair__pr {
        flex: none;
        color: var(--color-signal-add);
        block-size: auto;
    }

    .row__glyph {
        display: flex;
        flex: none;
        color: var(--color-signal-add);
    }
</style>
