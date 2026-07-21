<script lang="ts">
    import type { DiscoveredRepo } from "$lib/types";
    import { Check, RefreshCw, Search } from "@lucide/svelte";

    let {
        discovered,
        discoveredTotal,
        selectedRepos = $bindable<string[]>([]),
        loading,
        onreload,
    }: {
        discovered: DiscoveredRepo[];
        discoveredTotal: number;
        selectedRepos?: string[];
        loading: boolean;
        onreload: () => void;
    } = $props();

    let repoSearch = $state("");

    type RepoGroup = { owner: string; repos: DiscoveredRepo[] };

    function groupReposByOwner(repos: DiscoveredRepo[]): RepoGroup[] {
        const groups: RepoGroup[] = [];
        const byOwner = new Map<string, RepoGroup>();

        for (const repo of repos) {
            const owner =
                repo.owner?.login ||
                repo.nameWithOwner.split("/")[0] ||
                "unknown";
            let group = byOwner.get(owner);
            if (!group) {
                group = { owner, repos: [] };
                byOwner.set(owner, group);
                groups.push(group);
            }
            group.repos.push(repo);
        }

        return groups;
    }

    let filteredRepos = $derived(
        discovered.filter((r) =>
            r.nameWithOwner.toLowerCase().includes(repoSearch.toLowerCase()),
        ),
    );

    let filteredGroups = $derived(groupReposByOwner(filteredRepos));

    function toggleRepo(fullName: string) {
        if (selectedRepos.includes(fullName)) {
            selectedRepos = selectedRepos.filter((p) => p !== fullName);
        } else {
            selectedRepos = [...selectedRepos, fullName];
        }
    }
</script>

<div class="step">
    <div class="section-head">
        <h2 class="section-head__title">Add repositories</h2>
        <p class="section-head__sub">
            Pick what Fog can work in. You can add more later in Settings.
        </p>
    </div>

    <div class="srch">
        <Search size={14} aria-hidden="true" />
        <input
            type="text"
            class="srch__input"
            bind:value={repoSearch}
            placeholder="Filter"
            aria-label="Filter repositories"
        />
    </div>

    <div class="list">
        {#if loading && discovered.length === 0}
            <p class="list__loading">
                <span class="spinner" aria-hidden="true"></span>
                Fetching from GitHub…
            </p>
        {:else if filteredRepos.length === 0}
            <div class="empty">
                {#if discovered.length === 0}
                    <p class="empty__title">
                        {discoveredTotal > 0
                            ? "Everything is already imported"
                            : "No repositories found"}
                    </p>
                    <button
                        class="btn btn-secondary list__retry"
                        onclick={onreload}
                        data-state={loading ? "loading" : undefined}
                        disabled={loading}
                    >
                        <RefreshCw size={13} />
                        <span>Refresh</span>
                    </button>
                {:else}
                    <p class="empty__title">No matches</p>
                    <p>Nothing matches “{repoSearch}”.</p>
                {/if}
            </div>
        {:else}
            {#each filteredGroups as group (group.owner)}
                <p class="list__owner mono">{group.owner}</p>
                {#each group.repos as repo (repo.nameWithOwner)}
                    {@const on = selectedRepos.includes(repo.nameWithOwner)}
                    <button
                        class="row"
                        role="checkbox"
                        aria-checked={on}
                        data-active={on}
                        onclick={() => toggleRepo(repo.nameWithOwner)}
                    >
                        <span class="row__main">
                            <span class="row__title">{repo.nameWithOwner}</span>
                        </span>
                        {#if on}
                            <Check size={15} class="list__check" />
                        {/if}
                    </button>
                {/each}
            {/each}
        {/if}
    </div>

    <p class="hint">{selectedRepos.length} selected</p>
</div>

<style>
    .step {
        display: flex;
        flex-direction: column;
        gap: var(--space-sm);
        min-block-size: 0;
        min-inline-size: 0;
    }

    .srch {
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        padding: var(--space-2xs) var(--space-sm);
        background: var(--color-paper);
        border: var(--rule-hair) solid var(--color-rule-2);
        color: var(--color-ink-3);
        transition: border-color var(--dur-micro) var(--ease-out);
    }

    .srch:focus-within {
        border-color: var(--color-accent);
    }

    .srch__input {
        flex: 1;
        min-inline-size: 0;
        background: none;
        border: none;
        outline: none;
        color: var(--color-ink);
        font-family: var(--font-body);
        font-size: var(--text-sm);
    }

    .srch__input::placeholder {
        color: var(--color-ink-3);
    }

    .list {
        flex: 1;
        min-block-size: 8rem;
        max-block-size: 40dvh;
        overflow-y: auto;
        overscroll-behavior: contain;
        border: var(--rule-hair) solid var(--color-rule-2);
    }

    .list__owner {
        position: sticky;
        inset-block-start: 0;
        padding: var(--space-2xs) var(--space-md);
        background: var(--color-paper-3);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }

    .list__loading {
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        padding: var(--space-md);
        font-size: var(--text-sm);
        color: var(--color-ink-3);
    }

    .list :global(.list__check) {
        flex: none;
        color: var(--color-accent);
    }

    .list__retry {
        align-self: flex-start;
        margin-block-start: var(--space-xs);
    }

    .empty {
        border: none;
    }
</style>
