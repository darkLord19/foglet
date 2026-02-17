<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { fetchBranches, createSession, importRepos } from "$lib/api";
    import { getModelsForTool } from "$lib/constants";
    import type {
        CreateSessionPayload,
        DiscoveredRepo,
        Branch,
    } from "$lib/types";
    import { toast } from "svelte-sonner";
    import {
        ChevronDown,
        LayoutGrid,
        GitBranch,
        ArrowRight,
        Check,
        Hammer,
        Bot,
        Cpu,
        Zap,
        GitPullRequest,
        Settings,
        X,
    } from "@lucide/svelte";

    import Dropdown from "./Dropdown.svelte";

    let { onSessionCreated }: { onSessionCreated?: () => void } = $props();

    let prompt = $state("");
    let repo = $state("");
    let branch = $state("");
    let branches = $state<Branch[]>([]);
    let submitting = $state(false);
    let expanded = $state(false);
    let modeOpen = $state(false);

    // Defaults
    let tool = $state("");
    let model = $state("");
    let mode = $state<"plan" | "build">("build"); // Mapped to internal logic

    // PR State
    let createPR = $state(false);
    let showPRConfig = $state(false);
    let prBranch = $state("");
    let prTitle = $state("");
    let tempPrBranch = $state("");
    let tempPrTitle = $state("");

    // Models available for the currently-selected tool
    let availableModels = $derived(getModelsForTool(tool));

    // Load defaults from settings (runs on every settings change, which is fine
    // since manual selections are sent immediately on submit anyway)
    $effect(() => {
        if (appState.settings?.default_tool) {
            tool = appState.settings.default_tool;
        }
        if (appState.settings?.default_model) {
            model = appState.settings.default_model;
        }
    });

    // When tool changes, auto-select the per-tool default model (or reset)
    $effect(() => {
        if (!tool) return;
        const perToolDefault = appState.settings?.default_models?.[tool] ?? "";
        if (perToolDefault && availableModels.includes(perToolDefault)) {
            model = perToolDefault;
        } else if (
            model &&
            availableModels.length > 0 &&
            !availableModels.includes(model)
        ) {
            model = "";
        }
    });

    // Auto-select repo if only one
    $effect(() => {
        if (appState.repos.length === 1 && !repo) {
            repo = appState.repos[0].name;
        }
    });

    // Fetch branches when repo changes
    $effect(() => {
        if (repo) {
            loadBranches(repo);
        } else {
            branches = [];
            branch = "";
        }
    });

    async function loadBranches(repoName: string) {
        try {
            branches = await fetchBranches(repoName);
            // Auto-select default
            const def = branches.find((b) => b.is_default);
            if (def && !branch) {
                branch = def.name;
            }
        } catch (err) {
            console.error("Failed to fetch branches", err);
        }
    }

    function handleFocus() {
        expanded = true;
        appState.chatExpanded = true;
    }

    async function handleSubmit() {
        if (!prompt.trim() || !repo) return;

        submitting = true;
        try {
            // Ensure imported
            const isImported = appState.repos.some((r) => r.name === repo);
            if (!isImported) {
                await importRepos([repo]);
                await appState.refreshRepos();
            }

            const payload: CreateSessionPayload = {
                repo,
                prompt: prompt.trim(),
                async: true,
                autopr:
                    createPR || (appState.settings?.default_autopr ?? false),
                pr_title: prTitle,
                branch_name: prBranch,
            };

            if (branch) payload.base_branch = branch;
            if (tool) payload.tool = tool;
            if (model) payload.model = model;

            // Mode logic: if "plan", trigger specific prompt prefix?
            // For now, just passing the prompt as is, but could prepend context.
            if (mode === "plan") {
                payload.prompt = "[PLAN MODE] " + payload.prompt;
            }

            const out = await createSession(payload);
            toast.success(`Session started: ${out.session_id}`);
            prompt = "";

            // Notify parent to refresh/nav
            if (onSessionCreated) onSessionCreated();

            // Auto-open detail
            await appState.refreshSessions();
            await appState.selectSession(out.session_id, true);
        } catch (err) {
            toast.error(
                "Failed: " +
                    (err instanceof Error ? err.message : "Unknown error"),
            );
        } finally {
            submitting = false;
        }
    }

    function handleKeydown(e: KeyboardEvent) {
        if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            handleSubmit();
        }
    }

    function openPRConfig() {
        tempPrBranch = prBranch;
        tempPrTitle = prTitle;
        showPRConfig = true;
    }

    function savePRConfig() {
        prBranch = tempPrBranch;
        prTitle = tempPrTitle;
        showPRConfig = false;
    }
</script>

<div class="chat-container {expanded ? 'expanded' : ''}">
    <div class="chat-orb">
        <!-- Header: Selectors -->
        <div class="chat-header">
            <!-- Repo Selector -->
            <!-- Repo Selector -->
            {#snippet repoIcon()}
                <LayoutGrid size={12} class="text-muted" />
            {/snippet}

            <Dropdown
                bind:value={repo}
                options={appState.repos.map((r) => r.name)}
                placeholder="Select Repository..."
                icon={repoIcon}
                class="min-w-[180px]"
            />

            <!-- Branch Selector -->
            {#if repo}
                {#snippet branchIcon()}
                    <GitBranch size={12} class="text-muted" />
                {/snippet}
                <div>
                    <Dropdown
                        bind:value={branch}
                        options={branches.map((b) => b.name)}
                        placeholder="default"
                        icon={branchIcon}
                        class="min-w-[120px]"
                    />
                </div>
            {/if}

            <!-- Tool Selector -->
        </div>

        <!-- Body: Input -->
        <div class="chat-body">
            <textarea
                id="chat-prompt"
                bind:value={prompt}
                onfocus={handleFocus}
                onkeydown={handleKeydown}
                placeholder="Ask Fog to work on a task"
                class="chat-input"
                spellcheck="false"
            ></textarea>
        </div>

        <!-- Footer: Controls -->
        <div class="chat-footer {expanded ? 'visible' : ''}">
            <div class="footer-left">
                <!-- Add functionality like attachments here later -->

                <!-- Tool Selector -->
                <!-- Tool Selector -->
                {#snippet toolIcon()}
                    <Cpu size={12} class="text-muted" />
                {/snippet}
                <Dropdown
                    bind:value={tool}
                    options={[
                        { value: "", label: "Auto (Default)" },
                        ...(appState.settings?.available_tools || []),
                    ]}
                    placeholder="Select Tool..."
                    icon={toolIcon}
                    class="min-w-[140px]"
                />

                <!-- Model Selector -->
                {#if tool}
                    {#snippet modelIcon()}
                        <Zap size={12} class="text-muted" />
                    {/snippet}
                    <div>
                        <Dropdown
                            bind:value={model}
                            options={[
                                { value: "", label: "Default Model" },
                                ...availableModels.map((m) => ({
                                    value: m,
                                    label: m,
                                })),
                            ]}
                            placeholder="Default Model"
                            icon={modelIcon}
                            class="min-w-[160px]"
                        />
                    </div>
                {/if}

                <!-- Create PR Toggle -->
                <div class="divider"></div>
                <button
                    class="icon-btn {createPR ? 'active' : ''}"
                    onclick={() => (createPR = !createPR)}
                    title="Create PR"
                >
                    <GitPullRequest size={14} />
                    <span class="btn-label">Create PR</span>
                </button>

                {#if createPR}
                    <button
                        class="icon-btn"
                        onclick={openPRConfig}
                        title="Configure PR"
                    >
                        <Settings size={14} />
                    </button>

                    {#if showPRConfig}
                        <div
                            class="pr-backdrop"
                            onclick={() => (showPRConfig = false)}
                        ></div>
                        <div class="pr-config-modal">
                            <div class="pr-header">
                                <span>PR Configuration</span>
                                <button
                                    class="close-btn"
                                    onclick={() => (showPRConfig = false)}
                                >
                                    <X size={12} />
                                </button>
                            </div>
                            <div class="pr-field">
                                <label for="pr-branch">Branch Name</label>
                                <input
                                    id="pr-branch"
                                    type="text"
                                    bind:value={tempPrBranch}
                                    placeholder="fog/feature-name"
                                    class="pr-input"
                                />
                            </div>
                            <div class="pr-field">
                                <label for="pr-title">PR Title</label>
                                <input
                                    id="pr-title"
                                    type="text"
                                    bind:value={tempPrTitle}
                                    placeholder="feat: description"
                                    class="pr-input"
                                />
                            </div>
                            <div class="pr-actions">
                                <button
                                    class="btn btn-primary btn-sm w-full"
                                    onclick={savePRConfig}
                                >
                                    OK
                                </button>
                            </div>
                        </div>
                    {/if}
                {/if}
            </div>

            <div class="footer-right">
                <!-- Mode Toggle -->
                <!-- Mode Toggle -->
                <div class="mode-toggle-wrapper">
                    <button
                        class="mode-btn"
                        onclick={() => (modeOpen = !modeOpen)}
                    >
                        {#if mode === "plan"}
                            <Bot size={14} class="text-accent" />
                            <span>Interactive plan</span>
                        {:else}
                            <Hammer size={14} class="text-accent" />
                            <span>Build</span>
                        {/if}
                        <ChevronDown size={10} />
                    </button>

                    {#if modeOpen}
                        <div class="mode-dropdown glass">
                            <button
                                class="mode-option {mode === 'plan'
                                    ? 'selected'
                                    : ''}"
                                onclick={() => {
                                    mode = "plan";
                                    modeOpen = false;
                                }}
                            >
                                <Bot size={14} />
                                <div class="mode-info">
                                    <span class="mode-title"
                                        >Interactive plan</span
                                    >
                                    <span class="mode-desc"
                                        >Chat to verify goals</span
                                    >
                                </div>
                                {#if mode === "plan"}<Check
                                        size={12}
                                        class="check"
                                    />{/if}
                            </button>
                            <button
                                class="mode-option {mode === 'build'
                                    ? 'selected'
                                    : ''}"
                                onclick={() => {
                                    mode = "build";
                                    modeOpen = false;
                                }}
                            >
                                <Hammer size={14} />
                                <div class="mode-info">
                                    <span class="mode-title">Build</span>
                                    <span class="mode-desc"
                                        >Start immediately</span
                                    >
                                </div>
                                {#if mode === "build"}<Check
                                        size={12}
                                        class="check"
                                    />{/if}
                            </button>
                        </div>
                    {/if}
                </div>

                <!-- Submit -->
                <button
                    id="chat-submit"
                    class="submit-btn"
                    disabled={submitting || !prompt.trim() || !repo}
                    onclick={handleSubmit}
                >
                    {#if submitting}
                        <div class="spinner"></div>
                    {:else}
                        <ArrowRight size={16} />
                    {/if}
                </button>
            </div>
        </div>
    </div>
</div>

<style>
    .chat-container {
        width: 100%;
        max-width: 800px;
        margin: 0 auto;
        /* transition removed */
        position: relative;
        z-index: 100; /* Force top */
    }

    .chat-orb {
        background: #09090b; /* Solid black/dark */
        border: 1px solid var(--color-border);
        border-radius: 20px;
        padding: 0;
        position: relative;
        /* transition removed */
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.8); /* Solid strong shadow */
    }

    .chat-container.expanded .chat-orb {
        border-color: var(--color-border-accent);
        box-shadow: 0 8px 30px rgba(0, 0, 0, 0.9);
    }

    .chat-header {
        padding: 12px 16px 4px;
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .chat-body {
        padding: 8px 16px 12px;
    }

    .chat-input {
        width: 100%;
        background: #09090b;
        border: none;
        outline: none;
        resize: none;
        font-size: 16px;
        line-height: 1.5;
        color: var(--color-text);
        min-height: 56px;
        transition: min-height 0.3s;
        font-family: inherit;
    }

    .chat-container.expanded .chat-input {
        min-height: 120px;
    }

    .chat-input::placeholder {
        color: var(--color-text-muted);
    }

    .chat-footer {
        padding: 12px 16px;
        border-top: 1px solid var(--color-border);
        display: flex;
        align-items: center;
        justify-content: space-between;
        /* opacity removed */
    }

    /* Optional: hide footer when collapsed? 
     Jules keeps it visible but minimal. 
     Let's keep it visible. */

    .footer-left {
        display: flex;
        align-items: center;
        gap: 8px;
        position: relative;
    }

    .footer-right {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .mode-toggle-wrapper {
        position: relative;
    }

    .mode-btn {
        display: flex;
        align-items: center;
        gap: 6px;
        background: #09090b;
        border: none;
        color: var(--color-text);
        font-size: 13px;
        font-weight: 500;
        cursor: pointer;
        padding: 6px 10px;
        border-radius: 6px;
    }
    .mode-btn:hover {
        background: var(--color-bg-hover);
    }

    .mode-dropdown {
        position: absolute;
        top: calc(100% + 4px);
        right: 0;
        width: 200px;
        margin-top: 4px;
        background: #09090b;
        border: 1px solid var(--color-border);
        border-radius: 12px;
        padding: 4px;
        display: flex;
        flex-direction: column;
        gap: 2px;
        z-index: 9999;
    }

    .mode-option {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 8px 12px;
        text-align: left;
        background: #09090b;
        border: none;
        border-radius: 8px;
        color: var(--color-text-secondary);
        cursor: pointer;
        width: 100%;
    }

    .mode-option:hover {
        background: var(--color-bg-hover);
        color: var(--color-text);
    }

    /* Highlight selected option */
    .mode-option.selected {
        background: #1e1b00;
        color: var(--color-accent);
        border: 1px solid var(--color-accent);
    }

    .mode-info {
        flex: 1;
        display: flex;
        flex-direction: column;
    }

    .mode-title {
        font-size: 13px;
        font-weight: 600;
        color: inherit;
    }

    .mode-desc {
        font-size: 11px;
        color: var(--color-text-muted);
    }

    :global(.text-accent) {
        color: var(--color-accent);
    }

    .submit-btn {
        width: 36px;
        height: 36px;
        border-radius: 10px;
        display: flex;
        align-items: center;
        justify-content: center;
        background: var(--color-bg-active);
        color: var(--color-text);
        border: 1px solid var(--color-border);
        cursor: pointer;
        transition: all 0.2s;
    }

    .submit-btn:hover:not(:disabled) {
        background: var(--color-accent);
        color: white;
        border-color: var(--color-accent);
    }

    .submit-btn:disabled {
        filter: brightness(0.4);
        cursor: not-allowed;
    }

    .spinner {
        width: 14px;
        height: 14px;
        border: 2px solid #333;
        border-top-color: white;
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }

    .divider {
        width: 1px;
        height: 16px;
        background: var(--color-border);
        margin: 0 4px;
    }

    .icon-btn {
        display: flex;
        align-items: center;
        gap: 6px;
        background: #09090b;
        border: none;
        color: var(--color-text-muted);
        padding: 6px;
        border-radius: 6px;
        cursor: pointer;
        transition: all 0.2s;
    }

    .icon-btn:hover {
        background: var(--color-bg-hover);
        color: var(--color-text);
    }

    .icon-btn.active {
        color: var(--color-accent);
        background: #0c1a3d;
    }

    .btn-label {
        font-size: 11px;
        font-weight: 600;
    }

    .pr-backdrop {
        position: fixed;
        inset: 0;
        background: #000000;
        z-index: 99;
    }

    .pr-config-modal {
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        width: 320px;
        background: var(--color-bg-elevated);
        border: 1px solid var(--color-border-accent);
        border-radius: 16px;
        padding: 16px;
        display: flex;
        flex-direction: column;
        gap: 16px;
        z-index: 100;
        box-shadow: 0 20px 50px #000000;
    }

    .pr-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        font-size: 12px;
        font-weight: 600;
        color: var(--color-text);
    }

    .close-btn {
        background: #09090b;
        border: none;
        color: var(--color-text-muted);
        cursor: pointer;
        padding: 4px;
        border-radius: 4px;
    }
    .close-btn:hover {
        background: var(--color-bg-hover);
        color: var(--color-text);
    }

    .pr-field {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .pr-field label {
        font-size: 11px;
        color: var(--color-text-secondary);
    }

    .pr-input {
        background: var(--color-bg-input);
        border: 1px solid var(--color-border);
        border-radius: 6px;
        padding: 6px 8px;
        font-size: 12px;
        color: var(--color-text);
        outline: none;
    }

    .pr-input:focus {
        border-color: var(--color-accent);
    }

    .pr-actions {
        margin-top: 8px;
    }

    .w-full {
        width: 100%;
    }

    :global(.btn-sm) {
        padding: 6px 12px;
        font-size: 12px;
    }
</style>
