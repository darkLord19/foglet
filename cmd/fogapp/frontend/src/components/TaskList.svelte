<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { formatRelativeTime } from "$lib/utils";
    import { GitPullRequest } from "@lucide/svelte";
    import Collapsible from "./common/Collapsible.svelte";

    let tasks = $derived(appState.runningSessions);

    function openSession(id: string) {
        appState.selectSession(id);
    }
</script>

<Collapsible title="Running" count={tasks.length}>
    {#if tasks.length > 0}
        <div class="rows">
            {#each tasks as task (task.id)}
                <button class="row" onclick={() => openSession(task.id)}>
                    <span class="row__main">
                        <span class="row__title">
                            {task.latest_run?.prompt || "Untitled task"}
                        </span>
                        <span class="row__meta">
                            <span class="truncate">{task.repo_name}</span>
                            <span aria-hidden="true">·</span>
                            <!-- Real timestamp. The previous build printed a
                                 hard-coded "Just now" on every row. -->
                            <time datetime={task.updated_at}>
                                {formatRelativeTime(task.updated_at)}
                            </time>
                            {#if task.pr_url}
                                <span aria-hidden="true">·</span>
                                <GitPullRequest size={11} />
                                <span>PR</span>
                            {/if}
                        </span>
                    </span>

                    <span class="badge badge--running">
                        <span class="badge__dot" aria-hidden="true"></span>
                        Running
                    </span>
                </button>
            {/each}
        </div>
    {:else}
        <div class="empty">
            <p class="empty__title">Nothing running</p>
            <p>Started runs appear here while they work.</p>
        </div>
    {/if}
</Collapsible>
