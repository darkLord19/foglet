<script lang="ts">
    import { onMount } from "svelte";
    import { toast } from "svelte-sonner";
    import {
        fetchMCPConfig,
        updateMCPConfig,
        connectGitHubMCP,
    } from "$lib/api";
    import { MCP_PRESETS, type MCPPreset } from "$lib/constants";
    import type { MCPUpstream, UpdateMCPUpstream } from "$lib/types";
    import { appState } from "$lib/stores.svelte";
    import { Plus, Trash2, Pencil, X, Plug } from "@lucide/svelte";

    let upstreams = $state<MCPUpstream[]>([]);
    let loading = $state(false);

    // Form state
    let formOpen = $state(false);
    let editingName = $state<string | null>(null);
    let formPreset = $state<MCPPreset | null>(null);
    let formName = $state("");
    let formURL = $state("");
    let formToken = $state("");
    let formNameError = $state("");
    let saving = $state(false);
    let deleting = $state<string | null>(null);
    let connectingPreset = $state<string | null>(null);

    onMount(load);

    async function load() {
        loading = true;
        try {
            const cfg = await fetchMCPConfig();
            upstreams = cfg.upstreams;
        } catch (err) {
            console.error("mcp config", err);
        } finally {
            loading = false;
        }
    }

    // GitHub is the only preset that can auto-connect via gh CLI.
    let ghReady = $derived(
        appState.settings?.gh_installed && appState.settings?.gh_authenticated,
    );

    async function connectGitHub() {
        connectingPreset = "github";
        try {
            const cfg = await connectGitHubMCP();
            upstreams = cfg.upstreams;
            toast.success("GitHub MCP connected");
        } catch (err) {
            toast.error(
                "GitHub connect failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            connectingPreset = null;
        }
    }

    function openAdd(preset?: MCPPreset) {
        // GitHub has its own auto-connect path.
        if (preset?.id === "github") {
            connectGitHub();
            return;
        }
        editingName = null;
        formPreset = preset ?? null;
        formName = preset?.defaultName ?? "";
        formURL = preset?.defaultURL ?? "";
        formToken = "";
        formNameError = "";
        formOpen = true;
    }

    function openEdit(upstream: MCPUpstream) {
        editingName = upstream.name;
        formPreset = null;
        formName = upstream.name;
        formURL = upstream.url;
        formToken = "";
        formNameError = "";
        formOpen = true;
    }

    function cancelForm() {
        formOpen = false;
        editingName = null;
        formPreset = null;
    }

    function validateForm(): boolean {
        formNameError = "";
        const name = formName.trim();
        if (!name) {
            formNameError = "Name is required.";
            return false;
        }
        if (name.includes("__")) {
            formNameError = "Name must not contain __.";
            return false;
        }
        if (
            editingName === null &&
            upstreams.some((u) => u.name === name)
        ) {
            formNameError = "Already configured.";
            return false;
        }
        const lower = formURL.trim().toLowerCase();
        if (!lower.startsWith("http://") && !lower.startsWith("https://")) {
            return false;
        }
        return true;
    }

    async function saveForm() {
        if (!validateForm()) return;
        saving = true;
        try {
            const entry: UpdateMCPUpstream = {
                name: formName.trim(),
                url: formURL.trim(),
                token: formToken.trim() || undefined,
            };
            let newList: UpdateMCPUpstream[];
            if (editingName !== null) {
                newList = upstreams.map((u) =>
                    u.name === editingName
                        ? entry
                        : { name: u.name, url: u.url },
                );
            } else {
                newList = [
                    ...upstreams.map((u) => ({ name: u.name, url: u.url })),
                    entry,
                ];
            }
            const cfg = await updateMCPConfig({ upstreams: newList });
            upstreams = cfg.upstreams;
            formOpen = false;
            editingName = null;
            formPreset = null;
            toast.success(
                editingName !== null ? "Updated" : "MCP server added",
            );
        } catch (err) {
            toast.error(
                "Save failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            saving = false;
        }
    }

    async function removeUpstream(name: string) {
        deleting = name;
        try {
            const newList = upstreams
                .filter((u) => u.name !== name)
                .map((u) => ({ name: u.name, url: u.url }));
            const cfg = await updateMCPConfig({ upstreams: newList });
            upstreams = cfg.upstreams;
            toast.success(`Removed ${name}`);
        } catch (err) {
            toast.error(
                "Remove failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            deleting = null;
        }
    }

    let urlError = $derived(
        formURL.trim() &&
        !formURL.trim().toLowerCase().startsWith("http://") &&
        !formURL.trim().toLowerCase().startsWith("https://")
            ? "URL must be http or https"
            : "",
    );

    function presetAlreadyAdded(preset: MCPPreset): boolean {
        return upstreams.some((u) => u.name === preset.defaultName);
    }

</script>

<div class="mcp">
    <!-- Connected servers list -->
    {#if loading}
        <div class="panel">
            <div class="panel__body mcp__loading">
                <span class="spinner"></span>
            </div>
        </div>
    {:else if upstreams.length > 0}
        <div class="panel">
            <div class="panel__head">
                <span class="panel__title">Connected</span>
                <span class="mcp__head-hint">
                    Injected into every agent session Fog starts
                </span>
            </div>
            <div class="mcp__list">
                {#each upstreams as upstream (upstream.name)}
                    <div class="mcp__row">
                        <div class="mcp__row-info">
                            <span class="mcp__row-name">{upstream.name}</span>
                            <span class="mcp__row-url truncate"
                                >{upstream.url}</span
                            >
                        </div>
                        <div class="mcp__row-actions">
                            {#if upstream.has_token}
                                <span class="badge badge--done">
                                    <span
                                        class="badge__dot"
                                        aria-hidden="true"
                                    ></span>
                                    Auth set
                                </span>
                            {/if}
                            <button
                                class="btn btn-ghost btn-icon btn-sm"
                                onclick={() => openEdit(upstream)}
                                aria-label="Edit {upstream.name}"
                                title="Edit"
                            >
                                <Pencil size={12} />
                            </button>
                            <button
                                class="btn btn-ghost btn-icon btn-sm"
                                data-state={deleting === upstream.name
                                    ? "loading"
                                    : undefined}
                                disabled={!!deleting}
                                onclick={() => removeUpstream(upstream.name)}
                                aria-label="Remove {upstream.name}"
                                title="Remove"
                            >
                                <Trash2 size={12} />
                            </button>
                        </div>
                    </div>
                {/each}
            </div>
        </div>
    {/if}

    <!-- Preset catalog -->
    {#if !formOpen}
        <div class="mcp__catalog">
            <p class="label mcp__catalog-label">Connect a service</p>
            <div class="mcp__presets">
                {#each MCP_PRESETS as preset (preset.id)}
                    {@const added = presetAlreadyAdded(preset)}
                    {@const isGH = preset.id === "github"}
                    {@const connecting = connectingPreset === preset.id}
                    <button
                        class="mcp__preset"
                        class:mcp__preset--added={added}
                        disabled={added || connecting}
                        data-state={connecting ? "loading" : undefined}
                        onclick={() => openAdd(preset)}
                        title={added
                            ? "Already connected"
                            : isGH && ghReady
                              ? "Connects automatically using your gh CLI auth"
                              : preset.description}
                    >
                        <span class="mcp__preset-label">{preset.label}</span>
                        {#if added}
                            <span
                                class="mcp__preset-check"
                                aria-hidden="true">✓</span
                            >
                        {:else if isGH && ghReady}
                            <Plug size={11} />
                        {:else}
                            <Plus size={11} />
                        {/if}
                    </button>
                {/each}
                <button class="mcp__preset" onclick={() => openAdd()}>
                    <span class="mcp__preset-label">Custom</span>
                    <Plus size={11} />
                </button>
            </div>
            {#if upstreams.length === 0}
                <p class="hint mcp__catalog-hint">
                    Connected servers are automatically available in Claude
                    Code, Cursor, and any other agent Fog starts — no tool
                    configuration needed.
                </p>
            {/if}
        </div>
    {/if}

    <!-- Add / edit form -->
    {#if formOpen}
        <div class="panel mcp__form-panel">
            <div class="panel__head">
                <span class="panel__title">
                    {editingName !== null
                        ? "Edit server"
                        : formPreset
                          ? "Connect " + formPreset.label
                          : "Custom server"}
                </span>
                <button
                    class="btn btn-ghost btn-icon btn-sm"
                    onclick={cancelForm}
                    aria-label="Cancel"
                >
                    <X size={12} />
                </button>
            </div>
            <div class="panel__body mcp__form">
                <div class="field">
                    <label class="label" for="mcp-name">Name</label>
                    <input
                        id="mcp-name"
                        class="input input-mono"
                        class:mcp__input-err={!!formNameError}
                        bind:value={formName}
                        placeholder="my-service"
                        disabled={editingName !== null}
                        autocomplete="off"
                    />
                    {#if formNameError}
                        <span class="field-error">{formNameError}</span>
                    {:else}
                        <p class="hint">
                            Prefix for tool names: <code
                                >{formName || "name"}__toolname</code
                            >
                        </p>
                    {/if}
                </div>

                <div class="field">
                    <label class="label" for="mcp-url">Server URL</label>
                    <input
                        id="mcp-url"
                        class="input input-mono"
                        class:mcp__input-err={!!urlError}
                        bind:value={formURL}
                        placeholder="https://mcp.example.com/"
                        autocomplete="off"
                    />
                    {#if urlError}
                        <span class="field-error">{urlError}</span>
                    {:else if formPreset?.docsHint}
                        <p class="hint">{formPreset.docsHint}</p>
                    {/if}
                </div>

                <div class="field">
                    <label class="label" for="mcp-token">
                        {formPreset?.tokenLabel ?? "Bearer token"}
                    </label>
                    <input
                        id="mcp-token"
                        class="input input-mono"
                        type="password"
                        bind:value={formToken}
                        placeholder={editingName !== null
                            ? "Leave blank to keep existing"
                            : "Paste your API token (optional)"}
                        autocomplete="off"
                    />
                    <p class="hint">
                        Stored encrypted locally. Only sent to the configured
                        URL.
                    </p>
                </div>

                <div class="mcp__form-actions">
                    <button
                        class="btn btn-secondary"
                        data-state={saving ? "loading" : undefined}
                        disabled={saving}
                        onclick={saveForm}
                    >
                        {editingName !== null ? "Update" : "Connect"}
                    </button>
                    <button class="btn btn-ghost" onclick={cancelForm}>
                        Cancel
                    </button>
                </div>
            </div>
        </div>
    {/if}
</div>

<style>
    .mcp {
        display: flex;
        flex-direction: column;
        gap: var(--space-sm);
    }

    .mcp__loading {
        display: flex;
        justify-content: center;
        padding-block: var(--space-md);
    }

    .mcp__head-hint {
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
        font-weight: 400;
        text-transform: none;
        letter-spacing: 0;
    }

    .mcp__list {
        border-radius: inherit;
    }

    .mcp__row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-sm);
        padding: var(--space-xs) var(--space-sm);
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .mcp__row:last-child {
        border-block-end: none;
    }

    .mcp__row-info {
        display: flex;
        flex-direction: column;
        gap: 2px;
        min-inline-size: 0;
        flex: 1;
    }

    .mcp__row-name {
        font-family: var(--font-mono);
        font-size: var(--text-xs);
        font-weight: 600;
        color: var(--color-ink);
    }

    .mcp__row-url {
        font-family: var(--font-mono);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
        max-inline-size: 28rem;
    }

    .mcp__row-actions {
        display: flex;
        align-items: center;
        gap: var(--space-2xs);
        flex: none;
    }

    .mcp__form-panel {
        background: var(--color-paper-3);
    }

    .mcp__form {
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
    }

    .mcp__form-actions {
        display: flex;
        gap: var(--space-xs);
    }

    .mcp__input-err {
        border-color: var(--color-signal-del) !important;
    }

    /* Catalog */
    .mcp__catalog {
        display: flex;
        flex-direction: column;
        gap: var(--space-xs);
    }

    .mcp__catalog-label {
        padding-block-start: var(--space-2xs);
    }

    .mcp__catalog-hint {
        max-inline-size: 42rem;
        line-height: var(--leading-body);
    }

    .mcp__presets {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2xs);
    }

    .mcp__preset {
        position: relative;
        display: inline-flex;
        align-items: center;
        gap: var(--space-2xs);
        height: 1.75rem;
        padding-inline: var(--space-sm);
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule-2);
        border-radius: var(--radius);
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--color-ink);
        cursor: pointer;
        transition:
            background-color var(--dur-micro) var(--ease-out),
            border-color var(--dur-micro) var(--ease-out);
    }

    .mcp__preset:hover:not(:disabled):not([data-state="loading"]) {
        background: var(--color-paper-3);
        border-color: var(--color-field);
    }

    .mcp__preset--added {
        opacity: 0.5;
        cursor: default;
    }

    .mcp__preset[data-state="loading"] {
        cursor: progress;
        color: transparent;
    }

    .mcp__preset[data-state="loading"]::after {
        content: "";
        position: absolute;
        inset-block-start: 50%;
        inset-inline-start: 50%;
        inline-size: 0.75rem;
        block-size: 0.75rem;
        margin: -0.375rem 0 0 -0.375rem;
        border: 2px solid var(--color-ink-3);
        border-block-start-color: transparent;
        border-radius: 50%;
        animation: spin 0.65s linear infinite;
    }

    .mcp__preset-check {
        color: var(--color-signal-add);
        font-size: var(--text-2xs);
    }

    code {
        font-family: var(--font-mono);
        font-size: 0.9em;
        color: var(--color-ink-2);
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
</style>
