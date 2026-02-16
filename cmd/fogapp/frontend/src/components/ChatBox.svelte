<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { fetchBranches, createSession, importRepos } from "$lib/api";
    import { CURATED_MODELS } from "$lib/constants";
    import type {
        CreateSessionPayload,
        DiscoveredRepo,
        Branch,
    } from "$lib/types";
    import { toast } from "svelte-sonner";
    import {
        Sparkles,
        ChevronDown,
        LayoutGrid,
        GitBranch,
        ArrowRight,
        Check,
        Play,
        Hammer,
        Bot,
        Cpu,
        Zap,
    } from "@lucide/svelte";
    import { slide } from "svelte/transition";

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
                autopr: appState.settings?.default_autopr ?? false,
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
</script>

<div class="chat-container {expanded ? 'expanded' : ''}">
    <div class="chat-orb glass">
        <!-- Header: Selectors -->
        <div class="chat-header">
            <!-- Repo Selector -->
            <!-- Repo Selector -->
            <div class="selector-pill">
                <select id="chat-repo" bind:value={repo} class="select-reset">
                    <option value="" disabled>Select Repository...</option>
                    {#each appState.repos as r}
                        <option value={r.name}>{r.name}</option>
                    {/each}
                </select>
                <LayoutGrid size={12} class="icon-muted pointer-events-none" />
                <div class="select-wrapper pointer-events-none">
                    <span class="select-label"
                        >{repo || "Select Repository..."}</span
                    >
                </div>
                <ChevronDown
                    size={10}
                    class="icon-chevron pointer-events-none"
                />
            </div>

            <!-- Branch Selector -->
            {#if repo}
                <div
                    class="selector-pill"
                    transition:slide={{ axis: "x", duration: 200 }}
                >
                    <select bind:value={branch} class="select-reset">
                        {#each branches as b}
                            <option value={b.name}>{b.name}</option>
                        {/each}
                    </select>
                    <GitBranch
                        size={12}
                        class="icon-muted pointer-events-none"
                    />
                    <div class="select-wrapper pointer-events-none">
                        <span class="select-label max-w-[120px] truncate">
                            {branch || "default"}
                        </span>
                    </div>
                    <ChevronDown
                        size={10}
                        class="icon-chevron pointer-events-none"
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
                <div class="selector-pill">
                    <select
                        id="chat-tool"
                        bind:value={tool}
                        class="select-reset"
                    >
                        <option value="" disabled>Select Tool...</option>
                        {#each appState.settings?.available_tools || [] as t}
                            <option value={t}>{t}</option>
                        {/each}
                    </select>
                    <Cpu size={12} class="icon-muted pointer-events-none" />
                    <div class="select-wrapper pointer-events-none">
                        <span class="select-label">{tool || "auto"}</span>
                    </div>
                    <ChevronDown
                        size={10}
                        class="icon-chevron pointer-events-none"
                    />
                </div>

                <!-- Model Selector -->
                {#if tool}
                    <div
                        class="selector-pill"
                        transition:slide={{ axis: "x", duration: 200 }}
                    >
                        <select
                            id="chat-model"
                            bind:value={model}
                            class="select-reset"
                        >
                            <option value="">Default Model</option>
                            {#each CURATED_MODELS as m}
                                <option value={m}>{m}</option>
                            {/each}
                        </select>
                        <Zap size={12} class="icon-muted pointer-events-none" />
                        <div class="select-wrapper pointer-events-none">
                            <span
                                class="select-label"
                                style="max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;"
                            >
                                {model || "default"}
                            </span>
                        </div>
                        <ChevronDown
                            size={10}
                            class="icon-chevron pointer-events-none"
                        />
                    </div>
                {/if}
                {#if branch}
                    <span class="branch-tag">{branch}</span>
                {/if}
            </div>

            <div class="footer-right">
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
                                class="mode-option"
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
                                class="mode-option"
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
        transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    }

    .chat-orb {
        background: var(--color-bg-input);
        /* Or a linear gradient slightly lighter than bg */
        border: 1px solid var(--color-border);
        border-radius: 20px;
        padding: 0;
        position: relative;
        transition: all 0.3s;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
    }

    .chat-container.expanded .chat-orb {
        border-color: var(--color-border-accent);
        box-shadow:
            0 8px 30px rgba(0, 0, 0, 0.4),
            0 0 0 1px rgba(59, 130, 246, 0.1);
    }

    .chat-header {
        padding: 12px 16px 4px;
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .selector-pill {
        display: flex;
        align-items: center;
        gap: 6px;
        background: rgba(255, 255, 255, 0.03);
        border: 1px solid var(--color-border);
        padding: 4px 10px;
        border-radius: 6px;
        font-size: 12px;
        position: relative;
        height: 28px;
        cursor: pointer;
        transition: all 0.2s;
    }

    .selector-pill:hover {
        background: var(--color-bg-hover);
        border-color: var(--color-border-strong);
    }

    .select-wrapper {
        position: relative;
        display: flex;
        align-items: center;
    }

    .select-reset {
        position: absolute;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        opacity: 0;
        cursor: pointer;
        z-index: 10;
        appearance: none;
    }

    :global(.pointer-events-none) {
        pointer-events: none;
    }

    .select-label {
        color: var(--color-text-secondary);
        font-weight: 500;
    }

    :global(.icon-muted) {
        opacity: 0.5;
    }

    :global(.icon-chevron) {
        opacity: 0.4;
    }

    .chat-body {
        padding: 8px 16px 12px;
    }

    .chat-input {
        width: 100%;
        background: transparent;
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
        opacity: 0.7;
    }

    .chat-footer {
        padding: 12px 16px;
        border-top: 1px solid var(--color-border);
        display: flex;
        align-items: center;
        justify-content: space-between;
        opacity: 0.8; /* Dimmed when not focused? Or toggle visibility */
        transition: opacity 0.2s;
    }

    /* Optional: hide footer when collapsed? 
     Jules keeps it visible but minimal. 
     Let's keep it visible. */

    .footer-left {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .branch-tag {
        font-size: 11px;
        padding: 2px 6px;
        background: rgba(59, 130, 246, 0.1);
        color: var(--color-accent);
        border-radius: 4px;
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
        background: transparent;
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
        bottom: 100%;
        right: 0;
        width: 200px;
        margin-bottom: 8px;
        background: var(--color-bg-elevated);
        border-radius: 12px;
        padding: 4px;
        display: flex;
        flex-direction: column;
        gap: 2px;
        z-index: 50;
    }

    .mode-option {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 8px 12px;
        text-align: left;
        background: transparent;
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

    .mode-info {
        flex: 1;
        display: flex;
        flex-direction: column;
    }

    .mode-title {
        font-size: 13px;
        font-weight: 600;
    }

    .mode-desc {
        font-size: 11px;
        opacity: 0.7;
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
        border-color: transparent;
    }

    .submit-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .spinner {
        width: 14px;
        height: 14px;
        border: 2px solid rgba(255, 255, 255, 0.3);
        border-top-color: white;
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
</style>
