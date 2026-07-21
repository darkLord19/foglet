<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import RunEntry from "./session/RunEntry.svelte";

    const runs = $derived(appState.detailRuns ?? []);

    /** Only ai_output events – logs already live in LogsView */
    const aiOutputEvents = $derived(
        appState.detailEvents.filter((e) => e.type === "ai_output"),
    );

    function isTerminal(state: string) {
        switch (state.trim()) {
            case "COMPLETED":
            case "FAILED":
            case "CANCELLED":
                return true;
            default:
                return false;
        }
    }
</script>

<div class="tl">
    {#if runs.length === 0}
        <div class="empty">
            <p class="empty__title">No runs yet</p>
            <p>This session hasn&rsquo;t executed anything.</p>
        </div>
    {:else}
        {#each runs as run, i (run.id)}
            <RunEntry
                {run}
                index={runs.length - i}
                isSelected={run.id === appState.selectedRunID}
                isLast={i === runs.length - 1}
                inProgress={!isTerminal(run.state)}
                outputEvents={run.id === appState.selectedRunID
                    ? aiOutputEvents
                    : []}
                onselect={() => (appState.selectedRunID = run.id)}
            />
        {/each}
    {/if}
</div>

<style>
    .tl {
        display: flex;
        flex-direction: column;
        min-inline-size: 0;
    }
</style>
