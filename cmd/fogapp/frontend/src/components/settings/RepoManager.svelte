<script lang="ts">
    import { toast } from "svelte-sonner";
    import { appState } from "$lib/stores.svelte";
    import { discoverRepos, importRepos } from "$lib/api";
    import type { DiscoveredRepo } from "$lib/types";
    import { Plus, Search } from "@lucide/svelte";

    let discoveryLoading = $state(false);
    let discovered = $state<DiscoveredRepo[]>([]);
    let importingRepos = $state<string[]>([]);

    type DiscoveredRepoGroup = { owner: string; repos: DiscoveredRepo[] };

    function groupDiscoveredRepos(
        repos: DiscoveredRepo[],
    ): DiscoveredRepoGroup[] {
        const groups: DiscoveredRepoGroup[] = [];
        const byOwner = new Map<string, DiscoveredRepoGroup>();

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

    let discoveredGroups = $derived(groupDiscoveredRepos(discovered));
    let firstDiscoveredName = $derived(discovered[0]?.nameWithOwner ?? "");

    async function handleDiscover() {
        discoveryLoading = true;
        try {
            const allDiscovered = await discoverRepos();
            const existingNames = new Set(appState.repos.map((r) => r.name));
            discovered = allDiscovered.filter(
                (d) => !existingNames.has(d.nameWithOwner),
            );

            // Only surface a message when the result is empty — a populated
            // list is its own feedback.
            if (discovered.length === 0) {
                toast.info(
                    allDiscovered.length > 0
                        ? "Everything on GitHub is already imported"
                        : "No repositories found",
                );
            }
        } catch (err) {
            toast.error(
                "Discovery failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            discoveryLoading = false;
        }
    }

    async function handleImport(repoName: string) {
        if (importingRepos.includes(repoName)) return;
        importingRepos = [...importingRepos, repoName];
        try {
            await importRepos([repoName]);
            // The row leaving the list and appearing above is the feedback.
            discovered = discovered.filter((d) => d.nameWithOwner !== repoName);
            await appState.refreshRepos();
        } catch (err) {
            toast.error(
                `Couldn't import ${repoName}: ` +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            importingRepos = importingRepos.filter((r) => r !== repoName);
        }
    }
</script>

<div class="panel">
    <div class="panel__head">
        <span class="panel__title">Repositories</span>
        <button
            id="settings-discover"
            class="btn btn-secondary rm__discover"
            data-state={discoveryLoading ? "loading" : undefined}
            disabled={discoveryLoading}
            onclick={handleDiscover}
        >
            <Search size={13} />
            <span>Discover</span>
        </button>
    </div>

    {#if appState.repos.length > 0}
        <ul class="rm__list">
            {#each appState.repos as repo (repo.name)}
                <li class="rm__row">
                    <span class="rm__name truncate">{repo.name}</span>
                    <span class="rm__path mono truncate">
                        {repo.base_worktree_path}
                    </span>
                </li>
            {/each}
        </ul>
    {:else}
        <div class="rm__pad">
            <div class="empty">
                <p class="empty__title">No repositories</p>
                <p>Discover pulls the list from your GitHub account.</p>
            </div>
        </div>
    {/if}

    {#if discovered.length > 0}
        <div class="rm__found">
            <p class="rm__found-title">Available to import</p>

            {#each discoveredGroups as group (group.owner)}
                <div class="rm__group">
                    <p class="rm__owner mono">{group.owner}</p>
                    <ul class="rm__list">
                        {#each group.repos as d (d.nameWithOwner)}
                            <li class="rm__row rm__row--action">
                                <span class="rm__name truncate">
                                    {d.nameWithOwner}
                                </span>
                                <button
                                    id={d.nameWithOwner === firstDiscoveredName
                                        ? "settings-import"
                                        : undefined}
                                    class="btn btn-secondary rm__import"
                                    data-state={importingRepos.includes(
                                        d.nameWithOwner,
                                    )
                                        ? "loading"
                                        : undefined}
                                    disabled={importingRepos.includes(
                                        d.nameWithOwner,
                                    )}
                                    onclick={() => handleImport(d.nameWithOwner)}
                                >
                                    <Plus size={12} />
                                    <span>Import</span>
                                </button>
                            </li>
                        {/each}
                    </ul>
                </div>
            {/each}
        </div>
    {/if}
</div>

<style>
    .rm__discover {
        block-size: 2rem;
        font-size: var(--text-2xs);
    }

    .rm__list {
        margin: 0;
        padding: 0;
        list-style: none;
    }

    .rm__row {
        display: grid;
        grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr);
        align-items: center;
        gap: var(--space-md);
        padding: var(--space-xs) var(--space-md);
        border-block-end: var(--rule-hair) solid var(--color-rule);
        min-inline-size: 0;
    }

    .rm__row:last-child {
        border-block-end: none;
    }

    .rm__row--action {
        grid-template-columns: minmax(0, 1fr) max-content;
    }

    .rm__name {
        font-size: var(--text-sm);
        color: var(--color-ink);
    }

    .rm__path {
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }

    .rm__import {
        block-size: 1.75rem;
        font-size: var(--text-2xs);
        padding-inline: var(--space-xs);
    }

    .rm__pad {
        padding: var(--space-md);
    }

    .rm__found {
        border-block-start: var(--rule-hair) solid var(--color-rule);
    }

    .rm__found-title {
        padding: var(--space-xs) var(--space-md);
        font-size: var(--text-2xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        color: var(--color-accent);
    }

    .rm__owner {
        padding: var(--space-2xs) var(--space-md);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
        background: var(--color-paper-3);
    }

    /* Narrow panes stack the repo name above its path. */
    @container (max-width: 34rem) {
        .rm__row {
            grid-template-columns: minmax(0, 1fr);
            gap: var(--space-3xs);
        }

        .rm__row--action {
            grid-template-columns: minmax(0, 1fr) max-content;
        }
    }
</style>
