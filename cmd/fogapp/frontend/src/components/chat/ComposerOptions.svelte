<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { Cpu, Zap, GitPullRequest, SlidersHorizontal } from "@lucide/svelte";
    import Dropdown from "../Dropdown.svelte";

    let {
        tool = $bindable(""),
        model = $bindable(""),
        createPR = $bindable(false),
        availableModels,
        onConfigurePR,
    }: {
        tool?: string;
        model?: string;
        createPR?: boolean;
        availableModels: string[];
        onConfigurePR: () => void;
    } = $props();
</script>

{#snippet toolIcon()}<Cpu size={12} />{/snippet}
{#snippet modelIcon()}<Zap size={12} />{/snippet}

<div class="opts">
    <Dropdown
        bind:value={tool}
        options={[
            { value: "", label: "Auto" },
            ...(appState.settings?.available_tools || []),
        ]}
        placeholder="Tool"
        icon={toolIcon}
    />

    {#if tool}
        <Dropdown
            bind:value={model}
            options={[
                { value: "", label: "Default model" },
                ...availableModels.map((m) => ({ value: m, label: m })),
            ]}
            placeholder="Default model"
            icon={modelIcon}
        />
    {/if}

    <span class="opts__sep" aria-hidden="true"></span>

    <button
        type="button"
        class="opts__pr"
        class:is-on={createPR}
        aria-pressed={createPR}
        onclick={() => (createPR = !createPR)}
    >
        <GitPullRequest size={14} />
        <span>Open PR</span>
    </button>

    {#if createPR}
        <button
            type="button"
            class="btn btn-ghost btn-icon"
            onclick={onConfigurePR}
            title="Configure the pull request"
            aria-label="Configure the pull request"
        >
            <SlidersHorizontal size={14} />
        </button>
    {/if}
</div>

<style>
    .opts {
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        flex-wrap: wrap;
        min-inline-size: 0;
    }

    .opts__sep {
        inline-size: var(--rule-hair);
        block-size: 1rem;
        background: var(--color-rule);
        flex: none;
    }

    .opts__pr {
        display: flex;
        align-items: center;
        gap: var(--space-2xs);
        padding: var(--space-2xs) var(--space-xs);
        background: transparent;
        border: var(--rule-hair) solid var(--color-rule-2);
        border-radius: var(--radius);
        color: var(--color-ink-2);
        font-size: var(--text-xs);
        font-weight: 600;
        white-space: nowrap;
        cursor: pointer;
        transition:
            color var(--dur-micro) var(--ease-out),
            border-color var(--dur-micro) var(--ease-out);
    }

    .opts__pr:hover {
        border-color: var(--color-rule-2);
        color: var(--color-ink);
    }

    .opts__pr:focus-visible {
        outline: var(--rule-hair) solid var(--color-focus);
        outline-offset: var(--rule-hair);
    }

    .opts__pr.is-on {
        border-color: var(--color-accent);
        color: var(--color-accent);
    }
</style>
