<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { fetchBranches, createSession, importRepos } from "$lib/api";
    import { getModelsForTool } from "$lib/constants";
    import type { CreateSessionPayload, Branch } from "$lib/types";
    import { toast } from "svelte-sonner";
    import { LayoutGrid, GitBranch, ArrowRight } from "@lucide/svelte";

    import Dropdown from "./Dropdown.svelte";
    import ModeSelect from "./chat/ModeSelect.svelte";
    import ComposerOptions from "./chat/ComposerOptions.svelte";
    import PRConfigDialog from "./chat/PRConfigDialog.svelte";

    let { onSessionCreated }: { onSessionCreated?: () => void } = $props();

    let prompt = $state("");
    let repo = $state("");
    let branch = $state("");
    let branches = $state<Branch[]>([]);
    let submitting = $state(false);
    let expanded = $state(false);

    let tool = $state("");
    let model = $state("");
    let mode = $state<"plan" | "build">("build");

    let createPR = $state(false);
    let showPRConfig = $state(false);
    let prBranch = $state("");
    let prTitle = $state("");

    let availableModels = $derived(getModelsForTool(tool));
    let canSubmit = $derived(!!prompt.trim() && !!repo && !submitting);

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

            if (mode === "plan") {
                payload.prompt = "[PLAN MODE] " + payload.prompt;
            }

            const out = await createSession(payload);
            prompt = "";

            if (onSessionCreated) onSessionCreated();

            // Silent success: navigating to the new session IS the feedback.
            await appState.refreshSessions();
            await appState.selectSession(out.session_id, true);
        } catch (err) {
            toast.error(
                "Couldn't start the session: " +
                    (err instanceof Error ? err.message : "unknown error"),
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

{#snippet repoIcon()}<LayoutGrid size={12} />{/snippet}
{#snippet branchIcon()}<GitBranch size={12} />{/snippet}

<section class="composer" class:is-expanded={expanded} aria-label="New session">
    <div class="composer__targets">
        <Dropdown
            bind:value={repo}
            options={appState.repos.map((r) => r.name)}
            placeholder="Repository"
            icon={repoIcon}
        />

        {#if repo}
            <Dropdown
                bind:value={branch}
                options={branches.map((b) => b.name)}
                placeholder="default branch"
                icon={branchIcon}
            />
        {/if}
    </div>

    <label class="composer__label" for="chat-prompt">Task</label>
    <textarea
        id="chat-prompt"
        class="composer__input"
        bind:value={prompt}
        onfocus={handleFocus}
        onkeydown={handleKeydown}
        placeholder="Describe what Fog should work on."
        spellcheck="false"
    ></textarea>

    <div class="composer__controls">
        <ComposerOptions
            bind:tool
            bind:model
            bind:createPR
            {availableModels}
            onConfigurePR={() => (showPRConfig = true)}
        />

        <div class="composer__submit">
            <ModeSelect bind:mode />

            <button
                id="chat-submit"
                type="button"
                class="btn btn-primary"
                data-state={submitting ? "loading" : undefined}
                disabled={!canSubmit}
                onclick={handleSubmit}
            >
                <span>Start run</span>
                <ArrowRight size={15} />
            </button>
        </div>
    </div>

    <p class="composer__hint">
        <kbd class="kbd">⌘</kbd><kbd class="kbd">↵</kbd> to start
    </p>
</section>

<PRConfigDialog
    bind:open={showPRConfig}
    bind:branchName={prBranch}
    bind:title={prTitle}
/>

<style>
    .composer {
        inline-size: 100%;
        display: flex;
        flex-direction: column;
        gap: var(--space-sm);
        padding: var(--space-md);
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule-2);
        /* Focus promotes the leading edge to accent — the composer is the
           one primary action on this surface. */
        border-inline-start: var(--rule-active) solid var(--color-rule-2);
        transition: border-color var(--dur-short) var(--ease-out);
        container-type: inline-size;
    }

    .composer.is-expanded {
        border-inline-start-color: var(--color-accent);
    }

    .composer__targets {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--space-xs);
    }

    .composer__label {
        font-size: var(--text-2xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        line-height: var(--leading-caps);
        color: var(--color-ink-3);
    }

    .composer__input {
        inline-size: 100%;
        min-block-size: 3.5rem;
        padding: 0;
        background: transparent;
        border: none;
        outline: none;
        resize: none;
        color: var(--color-ink);
        font-family: var(--font-body);
        font-size: var(--text-md);
        line-height: var(--leading-body);
        transition: min-block-size var(--dur-short) var(--ease-out);
    }

    .composer.is-expanded .composer__input {
        min-block-size: 7rem;
    }

    .composer__input::placeholder {
        color: var(--color-ink-3);
    }

    .composer__controls {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-sm);
        padding-block-start: var(--space-sm);
        border-block-start: var(--rule-hair) solid var(--color-rule);
    }

    .composer__submit {
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        flex-wrap: wrap;
        min-inline-size: 0;
    }

    /* Below ~30rem the control row stacks so nothing is squeezed — a
       container query, since the composer's width is set by its pane, not
       by the window. */
    @container (max-width: 30rem) {
        .composer__controls {
            flex-direction: column;
            align-items: stretch;
        }

        .composer__submit {
            justify-content: space-between;
        }
    }

    .composer__hint {
        display: flex;
        align-items: center;
        gap: var(--space-3xs);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }
</style>
