<script lang="ts">
    import Dropdown from "../Dropdown.svelte";

    let {
        selectedTool = $bindable(""),
        selectedModel = $bindable(""),
        availableTools,
        availableModels,
    }: {
        selectedTool?: string;
        selectedModel?: string;
        availableTools: { value: string; label: string }[];
        availableModels: { value: string; label: string }[];
    } = $props();
</script>

<div class="step">
    <div class="section-head">
        <h2 class="section-head__title">Pick an agent</h2>
        <p class="section-head__sub">
            The tool and model new sessions use by default. Both can be changed
            per run.
        </p>
    </div>

    <div class="field">
        <span class="label">Tool</span>
        <Dropdown
            bind:value={selectedTool}
            options={availableTools}
            placeholder="Select a tool"
            class="step__wide"
        />
    </div>

    <div class="field">
        <span class="label">Model</span>
        {#if !selectedTool || availableModels.length > 0}
            <Dropdown
                bind:value={selectedModel}
                options={availableModels}
                placeholder={selectedTool ? "Select a model" : "Pick a tool first"}
                disabled={!selectedTool}
                class="step__wide"
            />
        {:else}
            <input
                type="text"
                bind:value={selectedModel}
                placeholder="Model name"
                class="input input-mono step__wide"
                aria-label="Model"
            />
        {/if}
    </div>
</div>

<style>
    .step {
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
        min-inline-size: 0;
    }

    :global(.step__wide) {
        inline-size: 100%;
        max-inline-size: none;
    }
</style>
