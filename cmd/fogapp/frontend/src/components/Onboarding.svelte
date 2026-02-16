<script lang="ts">
    import { onMount } from "svelte";
    import { fade, fly, slide } from "svelte/transition";
    import {
        Check,
        ChevronRight,
        Loader2,
        Search,
        Github,
        RefreshCw,
        Terminal,
    } from "@lucide/svelte";
    import { appState } from "$lib/stores.svelte";
    import {
        updateSettings,
        discoverRepos,
        importRepos,
        fetchGhStatus,
    } from "$lib/api";
    import type { DiscoveredRepo, GhStatus } from "$lib/types";
    import { getModelsForTool } from "$lib/constants";
    import Dropdown from "./Dropdown.svelte";

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
    let selectedRepos = $state<string[]>([]); // repo full names
    let repoSearch = $state("");

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
            error = "Failed to check GitHub CLI status";
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
                    throw new Error("GitHub CLI must be installed to continue");
                }
                if (!ghStatus?.authenticated) {
                    throw new Error(
                        "Please authenticate with GitHub CLI to continue",
                    );
                }
                // Proceed to tool selection
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
                if (!selectedTool)
                    throw new Error("Please select a default AI tool");
                if (!selectedModel)
                    throw new Error("Please select a default model");

                // Save preferences
                await updateSettings({
                    default_tool: selectedTool,
                    default_model: selectedModel,
                    default_models: { [selectedTool]: selectedModel },
                });

                // Start discovery for next step
                step = 2;
                await loadRepos();
            }
        } catch (err) {
            error = err instanceof Error ? err.message : "An error occurred";
        } finally {
            loading = false;
        }
    }

    async function loadRepos() {
        loading = true;
        try {
            discovered = await discoverRepos();
        } catch (err) {
            error = "Failed to discover repositories";
        } finally {
            loading = false;
        }
    }

    function toggleRepo(fullName: string) {
        if (selectedRepos.includes(fullName)) {
            selectedRepos = selectedRepos.filter((p) => p !== fullName);
        } else {
            selectedRepos = [...selectedRepos, fullName];
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
                err instanceof Error ? err.message : "Failed to complete setup";
            loading = false;
        }
    }

    let filteredRepos = $derived(
        discovered.filter((r) =>
            r.nameWithOwner.toLowerCase().includes(repoSearch.toLowerCase()),
        ),
    );
</script>

<div class="onboarding-overlay" transition:fade>
    <div class="onboarding-card" in:fly={{ y: 20, duration: 400 }}>
        <div class="progress-bar">
            <div
                class="progress-fill"
                style="width: {(step + 1) * 33.33}%"
            ></div>
        </div>

        <div class="step-content">
            {#if step === 0}
                <div class="step-header">
                    <div class="icon-circle">
                        <Github size={24} />
                    </div>
                    <h2>GitHub CLI Setup</h2>
                    <p>
                        Fog uses the GitHub CLI <code>gh</code> for authentication
                        and repository access.
                    </p>
                </div>

                <div class="gh-status-container">
                    {#if checkingStatus}
                        <div class="status-check">
                            <Loader2 class="spin" size={20} />
                            <span>Checking status...</span>
                        </div>
                    {:else if ghStatus}
                        <div class="status-item">
                            <div class="status-label">GitHub CLI Installed</div>
                            <div
                                class="status-value {ghStatus.installed
                                    ? 'success'
                                    : 'error'}"
                            >
                                {#if ghStatus.installed}
                                    <Check size={18} /> Installed
                                {:else}
                                    Not Found
                                {/if}
                            </div>
                        </div>

                        {#if !ghStatus.installed}
                            <div class="install-instructions">
                                <p>To install GitHub CLI:</p>
                                <div class="code-block">
                                    <Terminal size={14} />
                                    <code>
                                        {#if ghStatus.os === "darwin"}
                                            brew install gh
                                        {:else}
                                            sudo apt install gh
                                        {/if}
                                    </code>
                                </div>
                            </div>
                        {:else}
                            <div class="status-item">
                                <div class="status-label">Authenticated</div>
                                <div
                                    class="status-value {ghStatus.authenticated
                                        ? 'success'
                                        : 'warning'}"
                                >
                                    {#if ghStatus.authenticated}
                                        <Check size={18} /> Authenticated
                                    {:else}
                                        Not Authenticated
                                    {/if}
                                </div>
                            </div>

                            {#if !ghStatus.authenticated}
                                <div class="install-instructions">
                                    <p>To authenticate:</p>
                                    <div class="code-block">
                                        <Terminal size={14} />
                                        <code>gh auth login</code>
                                    </div>
                                    <p class="sub-hint">
                                        Run this in your terminal, then refresh.
                                    </p>
                                </div>
                            {/if}
                        {/if}

                        <button
                            class="btn btn-secondary refresh-btn"
                            onclick={checkGhStatus}
                            disabled={checkingStatus}
                        >
                            <RefreshCw
                                size={16}
                                class={checkingStatus ? "spin" : ""}
                            />
                            Refresh Status
                        </button>
                    {:else}
                        <div class="error-message">
                            Could not determine status.
                        </div>
                        <button
                            class="btn btn-secondary refresh-btn"
                            onclick={checkGhStatus}
                        >
                            <RefreshCw size={16} /> Retry
                        </button>
                    {/if}
                </div>
            {:else if step === 1}
                <div class="step-header">
                    <div class="icon-circle">
                        <div class="ai-icon">✨</div>
                    </div>
                    <h2>Select AI Model</h2>
                    <p>
                        Choose your preferred AI tool and model for code
                        generation.
                    </p>
                </div>

                <div class="input-group">
                    <label for="tool">Default Tool</label>
                    <Dropdown
                        bind:value={selectedTool}
                        options={availableTools}
                        placeholder="Select Tool..."
                        class="full-width"
                    />
                </div>

                <div class="input-group">
                    <label for="model">Default Model</label>
                    {#if !selectedTool || availableModels.length > 0}
                        <Dropdown
                            bind:value={selectedModel}
                            options={availableModels}
                            placeholder={selectedTool
                                ? "Select Model..."
                                : "Select Tool First..."}
                            disabled={!selectedTool}
                            class="full-width"
                        />
                    {:else}
                        <input
                            id="model"
                            type="text"
                            bind:value={selectedModel}
                            placeholder="e.g. gpt-4"
                            class="text-input"
                        />
                    {/if}
                </div>
            {:else if step === 2}
                <div class="step-header">
                    <div class="icon-circle">
                        <Search size={24} />
                    </div>
                    <h2>Add Repositories</h2>
                    <p>Select GitHub repositories to import into Fog.</p>
                </div>

                <div class="repo-selection">
                    <div class="search-bar">
                        <Search size={16} class="search-icon" />
                        <input
                            type="text"
                            bind:value={repoSearch}
                            placeholder="Filter repositories..."
                        />
                    </div>

                    <div class="repo-list">
                        {#if loading && discovered.length === 0}
                            <div class="loading-state">
                                <Loader2 class="spin" size={24} />
                                <span>Fetching from GitHub...</span>
                            </div>
                        {:else}
                            {#each filteredRepos as repo}
                                <button
                                    class="repo-item {selectedRepos.includes(
                                        repo.nameWithOwner,
                                    )
                                        ? 'selected'
                                        : ''}"
                                    onclick={() =>
                                        toggleRepo(repo.nameWithOwner)}
                                >
                                    <div class="repo-info">
                                        <span class="repo-name"
                                            >{repo.nameWithOwner}</span
                                        >
                                        <span class="repo-path">{repo.url}</span
                                        >
                                    </div>
                                    {#if selectedRepos.includes(repo.nameWithOwner)}
                                        <Check size={18} class="check-icon" />
                                    {/if}
                                </button>
                            {/each}
                        {/if}
                    </div>
                    <p class="hint-text">{selectedRepos.length} selected</p>
                </div>
            {/if}
        </div>

        {#if error}
            <div class="error-banner" transition:slide>
                {error}
            </div>
        {/if}

        <div class="actions">
            {#if step === 2}
                <button
                    class="btn btn-primary full-width"
                    onclick={finish}
                    disabled={loading}
                >
                    {#if loading}
                        <Loader2 class="spin" size={16} />
                        Finishing...
                    {:else}
                        Finish Setup
                    {/if}
                </button>
            {:else if step === 0}
                <button
                    class="btn btn-primary full-width"
                    onclick={nextStep}
                    disabled={!ghStatus?.authenticated}
                >
                    Continue <ChevronRight size={16} />
                </button>
            {:else}
                <button
                    class="btn btn-primary full-width"
                    onclick={nextStep}
                    disabled={loading}
                >
                    {#if loading}
                        <Loader2 class="spin" size={16} />
                        Saving...
                    {:else}
                        Continue <ChevronRight size={16} />
                    {/if}
                </button>
            {/if}
            {#if step > 0 && !loading && step < 2}
                <button class="btn btn-ghost" onclick={() => step--}
                    >Back</button
                >
            {/if}
        </div>
    </div>
</div>

<style>
    .onboarding-overlay {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.8);
        backdrop-filter: blur(8px);
        z-index: 9999;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 20px;
    }

    .onboarding-card {
        background: var(--color-bg-elevated);
        border: 1px solid var(--color-border);
        border-radius: 16px;
        width: 100%;
        max-width: 480px;
        box-shadow: var(--shadow-xl);
        position: relative;
        display: flex;
        flex-direction: column;
        max-height: 90vh;
    }

    .progress-bar {
        height: 4px;
        background: var(--color-bg-surface);
        width: 100%;
        border-top-left-radius: 16px;
        border-top-right-radius: 16px;
        overflow: hidden;
    }

    .progress-fill {
        height: 100%;
        background: var(--color-accent);
        transition: width 0.3s ease;
    }

    .step-content {
        padding: 32px;
        /* Remove overflow-y: auto to allow dropdowns to fly out */
    }

    .step-header {
        text-align: center;
        margin-bottom: 32px;
        display: flex;
        flex-direction: column;
        align-items: center;
    }

    .icon-circle {
        width: 64px;
        height: 64px;
        border-radius: 50%;
        background: var(--color-bg-surface);
        display: flex;
        align-items: center;
        justify-content: center;
        margin-bottom: 16px;
        color: var(--color-text);
        border: 1px solid var(--color-border);
    }

    .ai-icon {
        font-size: 24px;
    }

    h2 {
        font-size: 24px;
        font-weight: 600;
        color: var(--color-text);
        margin-bottom: 8px;
    }

    p {
        font-size: 14px;
        color: var(--color-text-secondary);
        line-height: 1.5;
    }

    /* GH Status Styles */
    .gh-status-container {
        display: flex;
        flex-direction: column;
        gap: 16px;
    }

    .status-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 12px 16px;
        background: var(--color-bg-surface);
        border: 1px solid var(--color-border);
        border-radius: 8px;
    }

    .status-label {
        font-size: 14px;
        font-weight: 500;
        color: var(--color-text);
    }

    .status-value {
        font-size: 13px;
        font-weight: 600;
        display: flex;
        align-items: center;
        gap: 6px;
    }

    .status-value.success {
        color: var(--color-success);
    }
    .status-value.error {
        color: var(--color-danger);
    }
    .status-value.warning {
        color: var(--color-warning);
    }

    .install-instructions {
        background: var(--color-bg-surface);
        border: 1px solid var(--color-border);
        border-radius: 8px;
        padding: 16px;
        font-size: 13px;
    }

    .install-instructions p {
        margin-bottom: 8px;
        font-weight: 500;
        color: var(--color-text);
    }

    .code-block {
        background: #111;
        color: #eee;
        padding: 10px 12px;
        border-radius: 6px;
        display: flex;
        align-items: center;
        gap: 8px;
        font-family: var(--font-mono);
        font-size: 12px;
        border: 1px solid rgba(255, 255, 255, 0.1);
    }

    .sub-hint {
        margin-top: 8px;
        font-size: 12px;
        color: var(--color-text-muted);
    }

    .refresh-btn {
        margin-top: 8px;
        width: 100%;
    }

    .status-check {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 10px;
        padding: 20px;
        color: var(--color-text-muted);
        font-size: 14px;
    }

    /* Form Styles */
    .input-group {
        margin-bottom: 20px;
    }

    .input-group label {
        display: block;
        font-size: 13px;
        font-weight: 500;
        color: var(--color-text);
        margin-bottom: 8px;
    }

    .text-input {
        width: 100%;
        background: var(--color-bg);
        border: 1px solid var(--color-border);
        border-radius: 8px;
        padding: 10px 12px;
        color: var(--color-text);
        font-size: 14px;
        outline: none;
        transition: border-color 0.2s;
    }

    .text-input:focus {
        border-color: var(--color-accent);
    }

    .actions {
        padding: 24px 32px;
        border-top: 1px solid var(--color-border);
        background: var(--color-bg-surface);
        display: flex;
        flex-direction: column;
        gap: 12px;
        border-bottom-left-radius: 16px;
        border-bottom-right-radius: 16px;
    }

    .btn {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 8px;
        padding: 10px 16px;
        border-radius: 8px;
        font-size: 14px;
        font-weight: 500;
        cursor: pointer;
        transition: all 0.2s;
        border: none;
    }

    .btn-primary {
        background: var(--color-accent);
        color: #000;
    }

    .btn-primary:hover:not(:disabled) {
        opacity: 0.9;
    }

    .btn-primary:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .btn-secondary {
        background: var(--color-bg-hover);
        color: var(--color-text);
        border: 1px solid var(--color-border);
    }

    .btn-secondary:hover:not(:disabled) {
        background: var(--color-bg-active);
    }

    .btn-ghost {
        background: transparent;
        color: var(--color-text-secondary);
    }

    .btn-ghost:hover {
        color: var(--color-text);
        background: var(--color-bg-hover);
    }

    .full-width {
        width: 100%;
    }

    :global(.spin) {
        animation: spin 1s linear infinite;
    }

    @keyframes spin {
        from {
            transform: rotate(0deg);
        }
        to {
            transform: rotate(360deg);
        }
    }

    /* Repo Selection */
    .repo-selection {
        display: flex;
        flex-direction: column;
        height: 300px;
    }

    .search-bar {
        position: relative;
        margin-bottom: 12px;
    }

    :global(.search-icon) {
        position: absolute;
        left: 10px;
        top: 50%;
        transform: translateY(-50%);
        color: var(--color-text-muted);
    }

    .search-bar input {
        width: 100%;
        background: var(--color-bg-surface);
        border: 1px solid var(--color-border);
        border-radius: 8px;
        padding: 8px 12px 8px 36px;
        color: var(--color-text);
        font-size: 13px;
        outline: none;
    }

    .search-bar input:focus {
        border-color: var(--color-accent);
    }

    .repo-list {
        flex: 1;
        overflow-y: auto;
        border: 1px solid var(--color-border);
        border-radius: 8px;
        background: var(--color-bg);
    }

    .loading-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 100%;
        color: var(--color-text-muted);
        gap: 12px;
        font-size: 13px;
    }

    .repo-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        padding: 10px 12px;
        border: none;
        background: transparent;
        border-bottom: 1px solid var(--color-border);
        cursor: pointer;
        text-align: left;
        transition: background 0.1s;
    }

    .repo-item:last-child {
        border-bottom: none;
    }

    .repo-item:hover {
        background: var(--color-bg-hover);
    }

    .repo-item.selected {
        background: rgba(250, 204, 21, 0.05);
    }

    .repo-info {
        display: flex;
        flex-direction: column;
        gap: 2px;
        overflow: hidden;
    }

    .repo-name {
        font-size: 13px;
        font-weight: 500;
        color: var(--color-text);
    }

    .repo-path {
        font-size: 11px;
        color: var(--color-text-muted);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    :global(.check-icon) {
        color: var(--color-accent);
        flex-shrink: 0;
    }

    .hint-text {
        text-align: right;
        font-size: 12px;
        color: var(--color-text-muted);
        margin-top: 8px;
    }

    .error-banner {
        background: rgba(239, 68, 68, 0.1);
        color: var(--color-danger);
        padding: 10px;
        text-align: center;
        font-size: 13px;
        border-top: 1px solid rgba(239, 68, 68, 0.2);
    }
</style>
