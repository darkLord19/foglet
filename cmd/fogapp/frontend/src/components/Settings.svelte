<script lang="ts">
    import { toast } from "svelte-sonner";
    import { appState } from "$lib/stores.svelte";
    import { discoverRepos, updateSettings, importRepos } from "$lib/api";
    import { getModelsForTool } from "$lib/constants";
    import type { DiscoveredRepo, UpdateSettingsPayload } from "$lib/types";
    import { fade, slide } from "svelte/transition";
    import {
        Cpu,
        GitBranch,
        Github,
        Boxes,
        Save,
        Zap,
        Plus,
        Search,
        Layers,
    } from "@lucide/svelte";
    import Dropdown from "./Dropdown.svelte";

    let loading = $state(false);
    let discoveryLoading = $state(false);
    let discovered = $state<DiscoveredRepo[]>([]);

    // Local state for settings form
    let defaultTool = $state(appState.settings?.default_tool || "");
    let defaultModels = $state<Record<string, string>>({
        ...(appState.settings?.default_models ?? {}),
    });
    let defaultModel = $state("");
    let defaultAutoPR = $state(appState.settings?.default_autopr || false);
    let defaultNotify = $state(appState.settings?.default_notify || false);
    let branchPrefix = $state(appState.settings?.branch_prefix || "fog/");

    // Models available for the currently-selected default tool
    let availableModels = $derived(getModelsForTool(defaultTool));

    // When tool changes, load the per-tool default model (or reset)
    $effect(() => {
        if (!defaultTool) return;
        const stored = defaultModels[defaultTool] ?? "";
        if (stored && availableModels.includes(stored)) {
            defaultModel = stored;
        } else if (
            defaultModel &&
            availableModels.length > 0 &&
            !availableModels.includes(defaultModel)
        ) {
            defaultModel = "";
        }
    });

    // When model changes, write back to the per-tool map
    $effect(() => {
        if (defaultTool) {
            defaultModels[defaultTool] = defaultModel;
        }
    });

    async function saveAll() {
        loading = true;
        try {
            const payload: UpdateSettingsPayload = {
                default_autopr: defaultAutoPR,
                default_notify: defaultNotify,
                default_model: defaultModel,
                default_models: defaultModels,
            };
            if (defaultTool.trim()) {
                payload.default_tool = defaultTool.trim();
            }
            if (branchPrefix.trim()) {
                payload.branch_prefix = branchPrefix.trim();
            }

            const newSettings = await updateSettings(payload);
            appState.settings = newSettings;
            toast.success("Settings saved");
        } catch (err) {
            toast.error(
                "Failed to save settings: " +
                    (err instanceof Error ? err.message : "Error"),
            );
        } finally {
            loading = false;
        }
    }

    async function handleDiscover() {
        discoveryLoading = true;
        try {
            discovered = await discoverRepos();
            if (discovered.length === 0) {
                toast.info("No repositories found");
            } else if (discovered.length === 1) {
                toast.success("Found 1 repository");
            } else {
                toast.success(`Found ${discovered.length} repositories`);
            }
        } catch (err) {
            toast.error(
                "Discovery failed: " +
                    (err instanceof Error ? err.message : "Error"),
            );
        } finally {
            discoveryLoading = false;
        }
    }

    async function handleImport(repoName: string) {
        try {
            await importRepos([repoName]);
            toast.success(`Imported ${repoName}`);
            discovered = discovered.filter((d) => d.nameWithOwner !== repoName);
            await appState.refreshRepos();
        } catch (err) {
            toast.error(
                "Import failed: " +
                    (err instanceof Error ? err.message : "Error"),
            );
        }
    }
</script>

<div class="settings-container animate-fade-in" in:fade>
    <div class="settings-header">
        <h1 class="text-shimmer">Settings</h1>
        <p>Fine-tune your intelligence experience</p>
    </div>

    <div class="settings-grid">
        <!-- AI Intelligence -->
        <section class="settings-group">
            <div class="group-header">
                <Cpu size={16} />
                <h2>Intelligence Tuning</h2>
            </div>
            <div class="glass card section-content">
                <div class="field-group">
                    <label for="default-tool" class="label-v2"
                        >Default Intelligence Tool</label
                    >
                    {#snippet toolIcon()}
                        <Cpu size={14} class="opacity-50" />
                    {/snippet}
                    <Dropdown
                        bind:value={defaultTool}
                        options={appState.settings?.available_tools || []}
                        placeholder="Select a tool..."
                        icon={toolIcon}
                        class="w-full"
                    />
                </div>

                <div class="field-group">
                    <label for="default-model" class="label-v2"
                        >Model Override (Optional)</label
                    >
                    {#snippet modelIcon()}
                        <Zap size={14} class="opacity-50" />
                    {/snippet}
                    <Dropdown
                        bind:value={defaultModel}
                        options={[
                            { value: "", label: "Default (Auto)" },
                            ...availableModels.map((m) => ({
                                value: m,
                                label: m,
                            })),
                        ]}
                        placeholder="Default (Auto)"
                        icon={modelIcon}
                        class="w-full"
                    />
                    <span class="input-hint"
                        >Overrides the default model used by the tool</span
                    >
                </div>
            </div>
        </section>

        <!-- Automation & Workflow -->
        <section class="settings-group">
            <div class="group-header">
                <Zap size={16} />
                <h2>Automation & Flow</h2>
            </div>
            <div class="glass card section-content">
                <div class="switch-field">
                    <div class="text-block">
                        <label for="auto-pr">Autonomous PR Creation</label>
                        <p>
                            Automatically initialize draft pull requests for all
                            successful sessions.
                        </p>
                    </div>
                    <button
                        id="auto-pr"
                        class="toggle-switch {defaultAutoPR ? 'checked' : ''}"
                        onclick={() => (defaultAutoPR = !defaultAutoPR)}
                        role="switch"
                        aria-checked={defaultAutoPR}
                        aria-label="Autonomous PR Creation"
                    >
                        <span class="toggle-thumb"></span>
                    </button>
                </div>

                <div class="divider"></div>

                <div class="switch-field">
                    <div class="text-block">
                        <label for="notify">Desktop Notifications</label>
                        <p>
                            Receive system-level alerts when a long-running
                            intelligence task completes.
                        </p>
                    </div>
                    <button
                        id="notify"
                        class="toggle-switch {defaultNotify ? 'checked' : ''}"
                        onclick={() => (defaultNotify = !defaultNotify)}
                        role="switch"
                        aria-checked={defaultNotify}
                        aria-label="Desktop Notifications"
                    >
                        <span class="toggle-thumb"></span>
                    </button>
                </div>
            </div>
        </section>

        <!-- Infrastructure -->
        <section class="settings-group">
            <div class="group-header">
                <Layers size={16} />
                <h2>Infrastructure</h2>
            </div>
            <div class="glass card section-content">
                <div class="field-group">
                    <div class="label-v2">
                        <GitBranch size={14} />
                        <label for="settings-branch-prefix"
                            >Git Branch Prefix</label
                        >
                    </div>
                    <input
                        id="settings-branch-prefix"
                        bind:value={branchPrefix}
                        class="input"
                        placeholder="fog/"
                    />
                </div>
                <!-- Removed GitHub PAT section -->
                <div class="field-group">
                    <div class="label-v2">
                        <Github size={14} />
                        <label for="gh-status">GitHub CLI Status</label>
                    </div>
                    <div class="gh-status-display">
                        <div class="status-row">
                            <div
                                class="status-indicator {appState.settings
                                    ?.gh_installed
                                    ? 'success'
                                    : 'error'}"
                            ></div>
                            <span
                                >{appState.settings?.gh_installed
                                    ? "Installed"
                                    : "Not Installed"}</span
                            >
                        </div>
                        <div class="status-row">
                            <div
                                class="status-indicator {appState.settings
                                    ?.gh_authenticated
                                    ? 'success'
                                    : 'warning'}"
                            ></div>
                            <span
                                >{appState.settings?.gh_authenticated
                                    ? "Authenticated"
                                    : "Not Authenticated"}</span
                            >
                        </div>
                    </div>
                    <p class="input-hint">
                        Managed via <code>gh</code> CLI tool.
                    </p>
                </div>
            </div>
        </section>

        <!-- Resource Management -->
        <section class="settings-group">
            <div class="group-header">
                <Boxes size={16} />
                <h2>Connected Resources</h2>
            </div>
            <div class="glass card section-content">
                {#if appState.repos.length > 0}
                    <div class="resource-grid">
                        {#each appState.repos as repo}
                            <div class="resource-item">
                                <div class="res-icon"><Github size={12} /></div>
                                <div class="res-info">
                                    <span class="res-name">{repo.name}</span>
                                    <span class="res-path"
                                        >{repo.base_worktree_path}</span
                                    >
                                </div>
                            </div>
                        {/each}
                    </div>
                {:else}
                    <div class="empty-resources">
                        <Search size={20} />
                        <p>No repositories imported</p>
                    </div>
                {/if}

                <div class="discovery-pane">
                    <button
                        id="settings-discover"
                        class="btn btn-secondary discover-trigger"
                        disabled={discoveryLoading}
                        onclick={handleDiscover}
                    >
                        {#if discoveryLoading}
                            <div class="mini-loader"></div>
                            <span>Fetching from GitHub...</span>
                        {:else}
                            <Search size={14} />
                            <span>Discover GitHub Repositories</span>
                        {/if}
                    </button>

                    {#if discovered.length > 0}
                        <div class="discovery-results" transition:slide>
                            <div class="res-header">Available to Import</div>
                            <div class="res-list">
                                {#each discovered as d, i}
                                    <div class="res-item">
                                        <span class="res-name"
                                            >{d.nameWithOwner}</span
                                        >
                                        <button
                                            id={i === 0
                                                ? "settings-import"
                                                : undefined}
                                            class="btn-import"
                                            onclick={() =>
                                                handleImport(d.nameWithOwner)}
                                        >
                                            <Plus size={12} />
                                            <span>Import</span>
                                        </button>
                                    </div>
                                {/each}
                            </div>
                        </div>
                    {/if}
                </div>
            </div>
        </section>
    </div>

    <div class="action-footer">
        <button
            id="settings-save"
            class="btn btn-primary save-action"
            disabled={loading}
            onclick={saveAll}
        >
            {#if loading}
                <div class="mini-loader"></div>
                <span>Saving State...</span>
            {:else}
                <Save size={18} />
                <span>Save Configuration</span>
            {/if}
        </button>
    </div>
</div>

<style>
    .settings-container {
        max-width: 900px;
        margin: 0 auto;
        padding: 60px 40px 160px;
    }

    .settings-header {
        text-align: left;
        margin-bottom: 56px;
    }

    .settings-header h1 {
        font-size: 40px;
        font-weight: 800;
        margin-bottom: 8px;
        letter-spacing: -0.02em;
    }

    .settings-header p {
        font-size: 16px;
        color: var(--color-text-secondary);
        font-weight: 500;
    }

    .settings-grid {
        display: flex;
        flex-direction: column;
        gap: 40px;
    }

    .settings-group {
        display: flex;
        flex-direction: column;
        gap: 16px;
    }

    .group-header {
        display: flex;
        align-items: center;
        gap: 10px;
        color: var(--color-accent);
        opacity: 0.9;
        padding-left: 4px;
    }

    .group-header h2 {
        font-size: 14px;
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: 0.1em;
        color: var(--color-text-secondary);
    }

    .section-content {
        padding: 24px;
        display: flex;
        flex-direction: column;
        gap: 24px;
        border-radius: 20px;
    }

    .switch-field {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 32px;
    }

    .text-block label {
        display: block;
        font-size: 14px;
        font-weight: 600;
        margin-bottom: 4px;
    }

    .text-block p {
        font-size: 13px;
        color: var(--color-text-secondary);
        line-height: 1.4;
    }

    .divider {
        height: 1px;
        background: var(--color-border);
    }

    .resource-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
        gap: 12px;
    }

    .resource-item {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px;
        background: rgba(255, 255, 255, 0.02);
        border: 1px solid var(--color-border);
        border-radius: 12px;
    }

    .res-icon {
        width: 24px;
        height: 24px;
        background: var(--color-bg-active);
        border-radius: 6px;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--color-accent);
    }

    .res-info {
        display: flex;
        flex-direction: column;
        min-width: 0;
    }

    .res-name {
        font-size: 13px;
        font-weight: 600;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .res-path {
        font-size: 11px;
        color: var(--color-text-muted);
        font-family: var(--font-mono);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .empty-resources {
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 32px;
        color: var(--color-text-muted);
        gap: 12px;
        border: 1px dashed var(--color-border);
        border-radius: 16px;
    }

    .discovery-pane {
        margin-top: 8px;
    }

    .discover-trigger {
        width: 100%;
        height: 44px;
        border-radius: var(--radius-md);
        font-weight: 700;
    }

    .discovery-results {
        margin-top: 20px;
        background: rgba(0, 0, 0, 0.1);
        border-radius: 12px;
        padding: 16px;
        border: 1px solid var(--color-border);
    }

    .res-header {
        font-size: 11px;
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--color-text-muted);
        margin-bottom: 12px;
    }

    .res-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .res-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 12px;
        background: rgba(255, 255, 255, 0.03);
        border-radius: 8px;
    }

    .btn-import {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 4px 10px;
        background: var(--color-bg-active);
        border: 1px solid var(--color-border-strong);
        border-radius: 6px;
        color: var(--color-text);
        font-size: 11px;
        font-weight: 700;
        cursor: pointer;
        transition: all 0.2s;
    }

    .btn-import:hover {
        background: var(--color-accent);
        border-color: transparent;
    }

    .action-footer {
        position: fixed;
        bottom: 0;
        left: 0;
        right: 0;
        padding: 24px;
        background: linear-gradient(to top, var(--color-bg) 60%, transparent);
        display: flex;
        justify-content: center;
        pointer-events: none;
    }

    .save-action {
        pointer-events: auto;
        width: 400px;
        height: 56px;
        border-radius: 16px;
        box-shadow: 0 20px 40px rgba(0, 0, 0, 0.5);
    }

    .mini-loader {
        width: 16px;
        height: 16px;
        border: 2px solid rgba(255, 255, 255, 0.2);
        border-top-color: white;
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
        margin-right: 8px;
    }

    .gh-status-display {
        display: flex;
        flex-direction: column;
        gap: 8px;
        padding: 12px;
        background: var(--color-bg-surface);
        border-radius: 8px;
        border: 1px solid var(--color-border);
    }

    .status-row {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 13px;
    }

    .status-indicator {
        width: 8px;
        height: 8px;
        border-radius: 50%;
    }

    .status-indicator.success {
        background-color: var(--color-success);
    }
    .status-indicator.error {
        background-color: var(--color-danger);
    }
    .status-indicator.warning {
        background-color: var(--color-warning);
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
</style>
