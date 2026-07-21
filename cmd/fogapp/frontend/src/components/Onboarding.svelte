<script lang="ts">
    import { onMount } from "svelte";
    import { ChevronRight } from "@lucide/svelte";
    import { appState } from "$lib/stores.svelte";
    import {
        updateSettings,
        discoverRepos,
        importRepos,
        fetchGhStatus,
    } from "$lib/api";
    import type { DiscoveredRepo, GhStatus } from "$lib/types";
    import { getModelsForTool } from "$lib/constants";
    import StepGitHub from "./onboarding/StepGitHub.svelte";
    import StepAgent from "./onboarding/StepAgent.svelte";
    import StepRepos from "./onboarding/StepRepos.svelte";

    const STEPS = ["GitHub", "Agent", "Repositories"];

    let step = $state(0);
    let loading = $state(false);
    let error = $state("");

    // Step 0: GH Status
    let ghStatus = $state<GhStatus | null>(null);
    let checkingStatus = $state(false);

    // Step 1: Tools
    let selectedTool = $state("");
    let selectedModel = $state("");

    // Step 2: Repos
    let discovered = $state<DiscoveredRepo[]>([]);
    let discoveredTotal = $state(0);
    let selectedRepos = $state<string[]>([]);

    let availableTools = $derived(
        appState.settings?.available_tools.map((t) => ({
            value: t,
            label: t,
        })) ?? [],
    );

    let availableModels = $derived(
        selectedTool
            ? getModelsForTool(selectedTool).map((m) => ({
                  value: m,
                  label: m,
              }))
            : [],
    );

    onMount(async () => {
        await checkGhStatus();
    });

    async function checkGhStatus() {
        checkingStatus = true;
        error = "";
        try {
            ghStatus = await fetchGhStatus();
        } catch (err) {
            error = "Couldn't check the GitHub CLI status";
            console.error(err);
        } finally {
            checkingStatus = false;
        }
    }

    async function nextStep() {
        error = "";
        loading = true;

        try {
            if (step === 0) {
                if (!ghStatus?.installed) {
                    throw new Error("The GitHub CLI must be installed to continue");
                }
                if (!ghStatus?.authenticated) {
                    throw new Error("Sign in with the GitHub CLI to continue");
                }
                step = 1;

                // Pre-select if available
                if (appState.settings?.default_tool) {
                    selectedTool = appState.settings.default_tool;
                }
                if (
                    selectedTool &&
                    appState.settings?.default_models?.[selectedTool]
                ) {
                    selectedModel =
                        appState.settings.default_models[selectedTool];
                }
            } else if (step === 1) {
                if (!selectedTool) throw new Error("Pick a default tool");
                if (!selectedModel) throw new Error("Pick a default model");

                await updateSettings({
                    default_tool: selectedTool,
                    default_model: selectedModel,
                    default_models: { [selectedTool]: selectedModel },
                });

                step = 2;
                await loadRepos();
            }
        } catch (err) {
            error = err instanceof Error ? err.message : "Something went wrong";
        } finally {
            loading = false;
        }
    }

    async function loadRepos() {
        loading = true;
        try {
            const allDiscovered = await discoverRepos();
            discoveredTotal = allDiscovered.length;
            const existingNames = new Set(appState.repos.map((r) => r.name));
            discovered = allDiscovered.filter(
                (d) => !existingNames.has(d.nameWithOwner),
            );
        } catch (err) {
            error = "Couldn't discover repositories";
        } finally {
            loading = false;
        }
    }

    async function finish() {
        error = "";
        loading = true;
        try {
            if (selectedRepos.length > 0) {
                await importRepos(selectedRepos);
            }
            // Refresh settings to clear onboarding_required
            await appState.refreshAll();
        } catch (err) {
            console.error("Onboarding finish error:", err);
            error =
                err instanceof Error ? err.message : "Couldn't complete setup";
            loading = false;
        }
    }
</script>

<div class="ob">
    <div class="ob__card">
        <!-- Stepped rule, not a percentage-width bar. Each step is a discrete
             segment, so progress reads as "2 of 3" rather than "66.66%". -->
        <ol class="steps">
            {#each STEPS as label, i (label)}
                <li
                    class="steps__item"
                    class:is-done={i < step}
                    class:is-current={i === step}
                    aria-current={i === step ? "step" : undefined}
                >
                    <span class="steps__rule" aria-hidden="true"></span>
                    <span class="steps__label">{label}</span>
                </li>
            {/each}
        </ol>

        <div class="ob__body">
            {#if step === 0}
                <StepGitHub
                    {ghStatus}
                    checking={checkingStatus}
                    onrefresh={checkGhStatus}
                />
            {:else if step === 1}
                <StepAgent
                    bind:selectedTool
                    bind:selectedModel
                    {availableTools}
                    {availableModels}
                />
            {:else}
                <StepRepos
                    {discovered}
                    {discoveredTotal}
                    bind:selectedRepos
                    {loading}
                    onreload={loadRepos}
                />
            {/if}
        </div>

        {#if error}
            <p class="ob__error" role="alert">{error}</p>
        {/if}

        <div class="ob__actions">
            {#if step > 0 && step < 2}
                <button
                    class="btn btn-ghost"
                    onclick={() => step--}
                    disabled={loading}
                >
                    Back
                </button>
            {/if}

            {#if step === 2}
                <button
                    class="btn btn-primary ob__go"
                    onclick={finish}
                    data-state={loading ? "loading" : undefined}
                    disabled={loading}
                >
                    <span>Finish setup</span>
                </button>
            {:else}
                <button
                    class="btn btn-primary ob__go"
                    onclick={nextStep}
                    data-state={loading ? "loading" : undefined}
                    disabled={loading || (step === 0 && !ghStatus?.authenticated)}
                >
                    <span>Continue</span>
                    <ChevronRight size={15} />
                </button>
            {/if}
        </div>
    </div>
</div>

<style>
    .ob {
        position: fixed;
        inset: 0;
        z-index: var(--z-modal);
        display: grid;
        place-items: center;
        padding: var(--space-md);
        /* Flat scrim — no backdrop blur. Glass isn't in this system. */
        background: oklch(8% 0.004 92 / 0.88);
    }

    .ob__card {
        inline-size: 100%;
        max-inline-size: 34rem;
        max-block-size: min(90dvh, 48rem);
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
        padding: var(--space-lg);
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule-2);
        overflow-y: auto;
    }

    /* ── Step indicator ─────────────────────────────────────────────── */
    .steps {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: var(--space-xs);
        margin: 0;
        padding: 0;
        list-style: none;
    }

    .steps__item {
        display: flex;
        flex-direction: column;
        gap: var(--space-2xs);
        min-inline-size: 0;
    }

    .steps__rule {
        block-size: var(--rule-active);
        background: var(--color-rule);
        transition: background-color var(--dur-short) var(--ease-out);
    }

    .steps__item.is-done .steps__rule {
        background: var(--color-rule-2);
    }

    .steps__item.is-current .steps__rule {
        background: var(--color-accent);
    }

    .steps__label {
        font-size: var(--text-2xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        line-height: var(--leading-caps);
        color: var(--color-ink-3);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .steps__item.is-current .steps__label {
        color: var(--color-accent);
    }

    .ob__body {
        flex: 1;
        min-block-size: 0;
        display: flex;
        flex-direction: column;
    }

    .ob__error {
        padding: var(--space-xs) var(--space-sm);
        background: var(--color-signal-del-wash);
        border-inline-start: var(--rule-active) solid var(--color-signal-del);
        color: var(--color-signal-del);
        font-size: var(--text-sm);
    }

    .ob__actions {
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        padding-block-start: var(--space-sm);
        border-block-start: var(--rule-hair) solid var(--color-rule);
    }

    .ob__go {
        flex: 1;
    }
</style>
