<script lang="ts">
    import { toast } from "svelte-sonner";
    import { appState } from "$lib/stores.svelte";
    import { createSession, discoverRepos, importRepos } from "$lib/api";
    import type { CreateSessionPayload, DiscoveredRepo } from "$lib/types";
    import { slide, fade } from "svelte/transition";
    import {
        Sparkles,
        ChevronDown,
        SlidersHorizontal,
        LayoutGrid,
        Cpu,
        Layers,
        Search,
    } from "@lucide/svelte";

    let prompt = $state("");
    let repo = $state("");
    let tool = $state("");
    let model = $state("");
    let submitting = $state(false);
    let showAdvanced = $state(false);
    let discoverLoading = $state(false);
    let discovered = $state<DiscoveredRepo[]>([]);

    const curatedModels = [
        "claude-3-5-sonnet-latest",
        "claude-3-5-sonnet-20241022",
        "claude-3-opus-latest",
        "gemini-1.5-pro-latest",
        "gemini-1.5-flash-latest",
        "gpt-4o",
    ];

    // Auto-select repo if only one exists
    $effect(() => {
        if (appState.repos.length === 1 && !repo) {
            repo = appState.repos[0].name;
        }
    });

    // Set defaults from settings when available
    $effect(() => {
        const settings = appState.settings;
        if (!settings) return;

        const defaultTool = settings.default_tool ?? "";
        const defaultModel = settings.default_model ?? "";
        if (!tool && defaultTool) tool = defaultTool;
        if (!model && defaultModel) model = defaultModel;
    });

    async function handleDiscover() {
        discoverLoading = true;
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
            discoverLoading = false;
        }
    }

    async function handleSubmit() {
        if (!prompt.trim()) return;
        if (!repo) {
            toast.error("Please select a repository");
            return;
        }

        submitting = true;
        try {
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
            if (tool) payload.tool = tool;
            if (model) payload.model = model;

            const out = await createSession(payload);
            toast.success(`Queued session ${out.session_id}`);
            prompt = "";

            // Refresh and switch view (handled by SSE/Sidebar usually, but good ensuring)
            await appState.refreshSessions();
            if (out.session_id) {
                await appState.selectSession(out.session_id, true);
            }
        } catch (err) {
            toast.error(
                "Failed to start session: " +
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

<div id="new-session-form" class="new-session-page" in:fade={{ duration: 400 }}>
    <div class="glow-orb top-left"></div>
    <div class="glow-orb bottom-right"></div>

    <div class="hero-section">
        <h1
            class="hero-title animate-slide-up text-shimmer"
            style="animation-delay: 100ms"
        >
            Create something new
        </h1>
        <p
            class="hero-subtitle animate-slide-up"
            style="animation-delay: 200ms"
        >
            Describe your vision and let Fog handle the rest.
        </p>
    </div>

    <div class="form-container animate-scale-in" style="animation-delay: 300ms">
        <div class="chatbox-orb-aura">
            <div class="chatbox-orb glass">
                <!-- Header: Repo Picker -->
                <div class="chatbox-header">
                    <div class="repo-picker-pill">
                        <LayoutGrid size={12} class="picker-icon" />
                        <div class="select-wrapper">
                            <select
                                id="new-repo"
                                bind:value={repo}
                                class="select-minimal"
                            >
                                <option value="" disabled>Target Repository...</option>
                                {#if appState.repos.length > 0}
                                    <optgroup label="Imported">
                                        {#each appState.repos as r}
                                            <option value={r.name}>{r.name}</option>
                                        {/each}
                                    </optgroup>
                                {/if}
                                {#if discovered.length > 0}
                                    <optgroup label="GitHub">
                                        {#each discovered.filter(
                                            (d) =>
                                                !appState.repos.some(
                                                    (r) =>
                                                        r.name === d.full_name,
                                                ),
                                        ) as d}
                                            <option value={d.full_name}
                                                >{d.full_name}</option
                                            >
                                        {/each}
                                    </optgroup>
                                {/if}
                            </select>
                            <ChevronDown size={10} class="select-chevron" />
                        </div>
                        <button
                            id="new-discover"
                            type="button"
                            class="discover-btn"
                            disabled={discoverLoading ||
                                !appState.settings?.has_github_token}
                            onclick={handleDiscover}
                            title={appState.settings?.has_github_token
                                ? "Discover GitHub repositories"
                                : "Configure a GitHub PAT in Settings to discover repositories"}
                            aria-label="Discover GitHub repositories"
                        >
                            {#if discoverLoading}
                                <div class="discover-loader"></div>
                            {:else}
                                <Search size={14} />
                            {/if}
                        </button>
                    </div>
                </div>

                <!-- Body: Instructions -->
                <div class="chatbox-body">
                    <textarea
                        id="new-prompt"
                        bind:value={prompt}
                        onkeydown={handleKeydown}
                        class="chat-textarea"
                        placeholder="Describe what you want to build or refactor..."
                    ></textarea>
                </div>

                <!-- Footer: Configs & Submit -->
                <div class="chatbox-footer">
                    <div class="footer-left">
                        <button
                            type="button"
                            class="config-toggle {showAdvanced ? 'active' : ''}"
                            onclick={() => (showAdvanced = !showAdvanced)}
                            title="Advanced Tuning"
                        >
                            <SlidersHorizontal size={14} />
                        </button>

                        {#if showAdvanced}
                            <div
                                class="compact-advanced"
                                transition:slide={{ axis: "x", duration: 200 }}
                            >
                                <div class="compact-field">
                                    <Cpu size={12} />
                                    <div class="select-wrapper">
                                        <select
                                            bind:value={tool}
                                            class="select-minimal-v2"
                                        >
                                            {#each appState.settings?.available_tools || [] as t}
                                                <option value={t}>{t}</option>
                                            {/each}
                                        </select>
                                        <ChevronDown
                                            size={10}
                                            class="select-chevron"
                                        />
                                    </div>
                                </div>
                                <div class="compact-field">
                                    <Layers size={12} />
                                    <div class="select-wrapper">
                                        <select
                                            bind:value={model}
                                            class="select-minimal-v2"
                                        >
                                            <option value="">default</option>
                                            {#each curatedModels as m}
                                                <option value={m}>{m}</option>
                                            {/each}
                                        </select>
                                        <ChevronDown
                                            size={10}
                                            class="select-chevron"
                                        />
                                    </div>
                                </div>
                            </div>
                        {/if}
                    </div>

                    <div class="footer-right">
                        <div class="shortcut-hint">
                            <span>⌘Enter</span>
                        </div>
                        <button
                            id="new-submit"
                            class="dispatch-btn-premium"
                            disabled={submitting || !prompt.trim() || !repo}
                            onclick={handleSubmit}
                        >
                            {#if submitting}
                                <div class="btn-loader"></div>
                            {:else}
                                <Sparkles size={16} />
                                <span>Dispatch Intelligence</span>
                            {/if}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</div>

<style>
    .new-session-page {
        position: relative;
        max-width: 900px;
        margin: 0;
        padding: 100px 60px;
        display: flex;
        flex-direction: column;
        gap: 64px;
        min-height: 100vh;
    }

    .glow-orb {
        position: absolute;
        width: 400px;
        height: 400px;
        border-radius: 50%;
        filter: blur(100px);
        opacity: 0.1;
        z-index: -1;
        pointer-events: none;
    }

    .top-left {
        top: -150px;
        left: -100px;
        background: radial-gradient(circle, #3b82f6, transparent 70%);
    }

    .bottom-right {
        bottom: 50px;
        right: -50px;
        background: radial-gradient(circle, #8b5cf6, transparent 70%);
    }

    .hero-section {
        text-align: left;
    }

    .hero-title {
        font-size: 56px;
        font-weight: 800;
        margin-bottom: 16px;
        letter-spacing: -0.04em;
        line-height: 1;
    }

    .hero-subtitle {
        font-size: 20px;
        color: var(--color-text-secondary);
        max-width: 520px;
        line-height: 1.5;
        opacity: 0.8;
    }

    .form-container {
        width: 100%;
        max-width: 760px;
    }

    /* Aura around the chatbox */
    .chatbox-orb-aura {
        position: relative;
        padding: 1px;
        border-radius: 28px;
        background: linear-gradient(
            135deg,
            rgba(59, 130, 246, 0.2),
            rgba(139, 92, 246, 0.2) 50%,
            rgba(59, 130, 246, 0)
        );
        box-shadow:
            0 20px 80px rgba(0, 0, 0, 0.5),
            0 0 40px rgba(59, 130, 246, 0.05);
    }

    .chatbox-orb {
        padding: 0;
        border-radius: 27px;
        display: flex;
        flex-direction: column;
        border: 1px solid rgba(255, 255, 255, 0.08);
        background: linear-gradient(
            160deg,
            rgba(10, 12, 18, 0.95) 0%,
            rgba(5, 7, 10, 0.98) 100%
        );
        backdrop-filter: blur(20px);
        -webkit-backdrop-filter: blur(20px);
        overflow: hidden;
    }

    .chatbox-header {
        padding: 20px 24px 12px;
        display: flex;
        align-items: center;
    }

    .repo-picker-pill {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 6px 14px;
        background: rgba(255, 255, 255, 0.03);
        border: 1px solid rgba(255, 255, 255, 0.06);
        border-radius: 100px;
        color: var(--color-accent);
        transition: all 0.2s;
    }

    .repo-picker-pill:hover {
        background: rgba(255, 255, 255, 0.05);
        border-color: rgba(59, 130, 246, 0.3);
    }

    .discover-btn {
        width: 30px;
        height: 30px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        border-radius: 10px;
        border: 1px solid rgba(255, 255, 255, 0.08);
        background: rgba(255, 255, 255, 0.02);
        color: var(--color-text-muted);
        cursor: pointer;
        transition: all 0.2s;
    }

    .discover-btn:hover:not(:disabled) {
        background: rgba(255, 255, 255, 0.05);
        color: var(--color-text);
        border-color: rgba(255, 255, 255, 0.2);
    }

    .discover-btn:disabled {
        opacity: 0.35;
        cursor: not-allowed;
    }

    .discover-loader {
        width: 14px;
        height: 14px;
        border: 2px solid rgba(255, 255, 255, 0.25);
        border-top-color: rgba(255, 255, 255, 0.8);
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
    }

    .select-wrapper {
        position: relative;
        display: flex;
        align-items: center;
    }

    .select-minimal {
        background: transparent;
        border: none;
        color: var(--color-text);
        font-size: 13px;
        font-weight: 700;
        cursor: pointer;
        outline: none;
        padding-right: 18px;
        appearance: none;
    }

    :global(.select-chevron) {
        position: absolute;
        right: 0;
        top: 50%;
        transform: translateY(-50%);
        pointer-events: none;
        opacity: 0.5;
        color: var(--color-text-muted);
    }

    .chatbox-body {
        padding: 4px 28px 12px;
    }

    .chat-textarea {
        width: 100%;
        min-height: 200px;
        background: transparent;
        border: none;
        resize: none;
        color: var(--color-text);
        font-size: 17px;
        line-height: 1.6;
        outline: none;
        padding: 12px 0;
    }

    .chat-textarea::placeholder {
        color: rgba(255, 255, 255, 0.2);
    }

    .chatbox-footer {
        padding: 16px 16px 16px 28px;
        background: rgba(255, 255, 255, 0.02);
        display: flex;
        align-items: center;
        justify-content: space-between;
        border-top: 1px solid rgba(255, 255, 255, 0.05);
    }

    .footer-left {
        display: flex;
        align-items: center;
        gap: 16px;
    }

    .config-toggle {
        width: 36px;
        height: 36px;
        border-radius: 10px;
        display: flex;
        align-items: center;
        justify-content: center;
        background: rgba(255, 255, 255, 0.03);
        border: 1px solid rgba(255, 255, 255, 0.08);
        color: var(--color-text-muted);
        cursor: pointer;
        transition: all 0.2s;
    }

    .config-toggle:hover {
        background: rgba(255, 255, 255, 0.05);
        color: var(--color-text);
        border-color: rgba(255, 255, 255, 0.2);
    }

    .config-toggle.active {
        background: rgba(59, 130, 246, 0.1);
        color: var(--color-accent);
        border-color: rgba(59, 130, 246, 0.3);
    }

    .compact-advanced {
        display: flex;
        align-items: center;
        gap: 14px;
        padding-left: 14px;
        border-left: 1px solid rgba(255, 255, 255, 0.1);
    }

    .compact-field {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 12px;
        color: var(--color-text-muted);
    }

    .select-minimal-v2 {
        background: transparent;
        border: none;
        color: var(--color-text-secondary);
        font-size: 11px;
        font-weight: 700;
        cursor: pointer;
        outline: none;
        appearance: none;
        padding-right: 18px;
        transition: all 0.2s;
    }

    .select-minimal-v2:hover {
        color: var(--color-text);
    }

    .footer-right {
        display: flex;
        align-items: center;
        gap: 24px;
    }

    .shortcut-hint {
        font-size: 11px;
        font-weight: 600;
        color: var(--color-text-muted);
        opacity: 0.5;
        letter-spacing: 0.05em;
        text-transform: uppercase;
    }

    .dispatch-btn-premium {
        height: 52px;
        padding: 0 28px;
        border-radius: 16px;
        background: var(--color-accent-gradient);
        color: white;
        border: none;
        display: flex;
        align-items: center;
        gap: 12px;
        font-size: 15px;
        font-weight: 700;
        cursor: pointer;
        transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
        box-shadow:
            0 4px 20px rgba(59, 130, 246, 0.3),
            inset 0 1px 1px rgba(255, 255, 255, 0.2);
    }

    .dispatch-btn-premium:hover:not(:disabled) {
        transform: translateY(-2px) scale(1.02);
        box-shadow: 0 12px 30px rgba(59, 130, 246, 0.5);
        filter: brightness(1.1);
    }

    .dispatch-btn-premium:active {
        transform: translateY(0) scale(0.98);
    }

    .dispatch-btn-premium:disabled {
        opacity: 0.3;
        filter: grayscale(1);
        cursor: not-allowed;
        box-shadow: none;
    }

    .btn-loader {
        width: 20px;
        height: 20px;
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
