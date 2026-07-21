<script lang="ts">
    import { toast } from "svelte-sonner";
    import { appState } from "$lib/stores.svelte";
    import { firstPromptLine, openExternal } from "$lib/utils";
    import { followUp, cancelSession, openInEditor, forkSession } from "$lib/api";
    import Timeline from "./Timeline.svelte";
    import DiffView from "./DiffView.svelte";
    import LogsView from "./LogsView.svelte";
    import StatsView from "./StatsView.svelte";
    import SessionHeader from "./session/SessionHeader.svelte";
    import FollowUpBar from "./session/FollowUpBar.svelte";
    import { Terminal, FileCode, Activity, History } from "@lucide/svelte";

    let followupPrompt = $state("");
    let submitting = $state(false);
    let activeTab = $state("timeline");
    let isWide = $state(false);

    const session = $derived(appState.detailSession);
    const runs = $derived(appState.detailRuns ?? []);
    const latestRun = $derived(runs[0]);
    const isBusy = $derived(session?.busy ?? false);
    const titleText = $derived(firstPromptLine(latestRun?.prompt));

    const ALL_TABS = [
        { id: "timeline", label: "Timeline", icon: History },
        { id: "diff", label: "Diff", icon: FileCode },
        { id: "logs", label: "Logs", icon: Terminal },
        { id: "stats", label: "Stats", icon: Activity },
    ];

    // Wide windows pin the timeline to its own pane, so it drops out of the
    // tab strip — the tabs then only drive the right-hand pane.
    const tabs = $derived(
        isWide ? ALL_TABS.filter((t) => t.id !== "timeline") : ALL_TABS,
    );

    // Keep the selection valid when the layout flips between one and two panes.
    $effect(() => {
        if (isWide && activeTab === "timeline") activeTab = "diff";
    });

    $effect(() => {
        const mq = window.matchMedia("(min-width: 100rem)");
        const sync = () => (isWide = mq.matches);
        sync();
        mq.addEventListener("change", sync);
        return () => mq.removeEventListener("change", sync);
    });

    async function handleFollowup() {
        if (!followupPrompt.trim() || !session) return;
        submitting = true;
        try {
            await followUp(session.id, followupPrompt.trim());
            followupPrompt = "";
            await appState.loadDetail();
        } catch (err) {
            toast.error(
                "Follow-up failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            submitting = false;
        }
    }

    async function handleRerun() {
        if (!session || !latestRun) return;
        try {
            await followUp(session.id, latestRun.prompt);
            await appState.loadDetail();
        } catch (err) {
            toast.error(
                "Re-run failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        }
    }

    async function handleStop() {
        if (!session) return;
        try {
            await cancelSession(session.id);
            await appState.loadDetail();
        } catch (err) {
            toast.error(
                "Couldn't stop the run: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        }
    }

    async function handleOpen() {
        if (!session) return;
        try {
            await openInEditor(session.id);
        } catch (err) {
            toast.error(
                "Couldn't open the editor: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        }
    }

    async function handleFork() {
        if (!session || !latestRun) return;
        try {
            const out = await forkSession(session.id, latestRun.prompt);
            // Off-screen result — the new session isn't visible from here, so
            // this one genuinely warrants a toast.
            toast.success(`Forked to session ${out.session_id.substring(0, 8)}`);
            await appState.refreshSessions();
        } catch (err) {
            toast.error(
                "Fork failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        }
    }

    function openPR() {
        openExternal(session?.pr_url);
    }
</script>

{#if session}
    <div class="sv">
        <SessionHeader
            {session}
            {latestRun}
            {isBusy}
            title={titleText}
            onRerun={handleRerun}
            onFork={handleFork}
            onStop={handleStop}
            onOpenEditor={handleOpen}
            onOpenPR={openPR}
        />

        <div class="sv__panes" class:is-split={isWide}>
            {#if isWide}
                <!-- Ultrawide: the timeline stays visible while you read the
                     diff or the logs, instead of being tabbed away. -->
                <section class="sv__pane sv__pane--rail" aria-label="Timeline">
                    <div class="sv__pane-head">
                        <span class="panel__title">Timeline</span>
                    </div>
                    <div class="sv__pane-body scroll-y">
                        <Timeline />
                    </div>
                </section>
            {/if}

            <section class="sv__pane" aria-label="Session detail">
                <div class="sv__pane-head">
                    <div class="tabs" role="tablist">
                        {#each tabs as tab (tab.id)}
                            <button
                                role="tab"
                                class="tabs__tab"
                                class:is-active={activeTab === tab.id}
                                aria-selected={activeTab === tab.id}
                                onclick={() => (activeTab = tab.id)}
                            >
                                <tab.icon size={14} />
                                <span>{tab.label}</span>
                            </button>
                        {/each}
                    </div>
                </div>

                <div class="sv__pane-body scroll-y">
                    {#if activeTab === "timeline"}
                        <Timeline />
                    {:else if activeTab === "diff"}
                        <DiffView />
                    {:else if activeTab === "logs"}
                        <LogsView />
                    {:else if activeTab === "stats"}
                        <StatsView />
                    {/if}
                </div>
            </section>
        </div>

        {#if !isBusy}
            <FollowUpBar
                bind:value={followupPrompt}
                {submitting}
                onsubmit={handleFollowup}
            />
        {/if}
    </div>
{:else}
    <div class="sv__none">
        <div class="sv__none-card">
            <p class="empty__title">No session selected</p>
            <p>Pick a run from the list to read its timeline, diff and logs.</p>
        </div>
    </div>
{/if}

<style>
    .sv {
        display: flex;
        flex-direction: column;
        block-size: 100%;
        min-block-size: 0;
        background: var(--color-paper);
    }

    .sv__panes {
        flex: 1;
        display: flex;
        min-block-size: 0;
        min-inline-size: 0;
    }

    .sv__panes.is-split {
        display: grid;
        grid-template-columns: minmax(0, 22rem) minmax(0, 1fr);
    }

    .sv__pane {
        display: flex;
        flex-direction: column;
        min-inline-size: 0;
        min-block-size: 0;
        flex: 1;
    }

    .sv__pane--rail {
        border-inline-end: var(--rule-hair) solid var(--color-rule);
    }

    .sv__pane-head {
        flex: none;
        padding-inline: var(--gutter);
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .sv__pane--rail .sv__pane-head {
        padding-block: var(--space-sm);
    }

    .sv__pane-body {
        flex: 1;
        padding: var(--space-md) var(--gutter);
        min-block-size: 0;
    }

    /* ── Tabs ────────────────────────────────────────────────────────
       Underline-rule tabs. The active rule is the same 3px accent used
       for every "this one" signal in the app. */
    .tabs {
        display: flex;
        gap: var(--space-lg);
    }

    .tabs__tab {
        display: flex;
        align-items: center;
        gap: var(--space-2xs);
        padding: var(--space-sm) 0;
        background: none;
        border: none;
        border-block-end: var(--rule-active) solid transparent;
        color: var(--color-ink-3);
        font-size: var(--text-xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        line-height: var(--leading-caps);
        white-space: nowrap;
        cursor: pointer;
        transition:
            color var(--dur-micro) var(--ease-out),
            border-color var(--dur-micro) var(--ease-out);
    }

    .tabs__tab:hover {
        color: var(--color-ink);
    }

    .tabs__tab:focus-visible {
        outline: var(--rule-hair) solid var(--color-focus);
        outline-offset: calc(var(--rule-hair) * -1);
    }

    .tabs__tab.is-active {
        color: var(--color-accent);
        border-block-end-color: var(--color-accent);
    }

    /* ── Empty ──────────────────────────────────────────────────────── */
    .sv__none {
        display: grid;
        place-items: center;
        block-size: 100%;
        padding: var(--gutter);
    }

    .sv__none-card {
        display: flex;
        flex-direction: column;
        gap: var(--space-2xs);
        max-inline-size: 32rem;
        padding: var(--space-lg);
        border: var(--rule-hair) dashed var(--color-rule);
        color: var(--color-ink-3);
        font-size: var(--text-sm);
    }
</style>
