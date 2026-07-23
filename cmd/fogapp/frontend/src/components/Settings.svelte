<script lang="ts">
    import { toast } from "svelte-sonner";
    import { appState } from "$lib/stores.svelte";
    import { updateSettings } from "$lib/api";
    import { getModelsForTool } from "$lib/constants";
    import type { UpdateSettingsPayload } from "$lib/types";
    import { Cpu, Zap, ArrowLeft } from "@lucide/svelte";
    import Dropdown from "./Dropdown.svelte";
    import ToggleField from "./settings/ToggleField.svelte";
    import RepoManager from "./settings/RepoManager.svelte";
    import TrackerSettings from "./settings/TrackerSettings.svelte";

    let loading = $state(false);

    // Local state for settings form
    let defaultTool = $state(appState.settings?.default_tool || "");
    let defaultModels = $state<Record<string, string>>({
        ...(appState.settings?.default_models ?? {}),
    });
    let defaultModel = $state("");
    let defaultAutoPR = $state(appState.settings?.default_autopr || false);
    let defaultNotify = $state(appState.settings?.default_notify || false);
    let keepAwake = $state(appState.settings?.keep_awake || false);
    let branchPrefix = $state(appState.settings?.branch_prefix || "fog/");

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
                keep_awake: keepAwake,
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
                "Couldn't save settings: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            loading = false;
        }
    }
</script>

{#snippet toolIcon()}<Cpu size={13} />{/snippet}
{#snippet modelIcon()}<Zap size={13} />{/snippet}

<!-- Long Document: sequential sections down one measure-capped column. -->
<div class="st scroll-y">
    <div class="st__inner shell-width">
        <header class="section-head reveal" style="--i: 0">
            <button
                class="btn btn-ghost btn-sm st__back"
                onclick={() => appState.setView("board")}
            >
                <ArrowLeft size={14} />
                <span>Board</span>
            </button>
            <h1 class="section-head__title">Settings</h1>
            <p class="section-head__sub">
                Defaults for new sessions, and the repositories Fog can work in.
            </p>
        </header>

        <section class="st__sec reveal" style="--i: 1">
            <h2 class="st__sec-title">Agent</h2>
            <div class="panel">
                <div class="panel__body st__fields">
                    <div class="field">
                        <span class="label">Default tool</span>
                        <Dropdown
                            bind:value={defaultTool}
                            options={appState.settings?.available_tools || []}
                            placeholder="Select a tool"
                            icon={toolIcon}
                            class="st__wide"
                        />
                    </div>

                    <div class="field">
                        <span class="label">Default model</span>
                        <Dropdown
                            bind:value={defaultModel}
                            options={[
                                { value: "", label: "Tool default" },
                                ...availableModels.map((m) => ({
                                    value: m,
                                    label: m,
                                })),
                            ]}
                            placeholder="Tool default"
                            icon={modelIcon}
                            class="st__wide"
                        />
                        <p class="hint">
                            Remembered per tool. Leave unset to use whatever the
                            tool picks.
                        </p>
                    </div>
                </div>
            </div>
        </section>

        <section class="st__sec reveal" style="--i: 2">
            <h2 class="st__sec-title">Workflow</h2>
            <div class="panel">
                <ToggleField
                    id="auto-pr"
                    bind:checked={defaultAutoPR}
                    label="Open a pull request automatically"
                    description="Every successful session opens a draft PR when it finishes."
                />
                <ToggleField
                    id="notify"
                    bind:checked={defaultNotify}
                    label="Desktop notifications"
                    description="Get a system alert when a long-running task completes."
                />
                <ToggleField
                    id="keep-awake"
                    bind:checked={keepAwake}
                    label="Keep this Mac awake"
                    description="Stops idle sleep while agents run. A closed lid with no external display still sleeps — that's Apple Silicon hardware behaviour no app can override."
                />
            </div>
        </section>

        <section class="st__sec reveal" style="--i: 3">
            <h2 class="st__sec-title">Git</h2>
            <div class="panel">
                <div class="panel__body st__fields">
                    <div class="field">
                        <label class="label" for="settings-branch-prefix">
                            Branch prefix
                        </label>
                        <input
                            id="settings-branch-prefix"
                            bind:value={branchPrefix}
                            class="input input-mono st__wide"
                            placeholder="fog/"
                        />
                    </div>

                    <div class="field">
                        <span class="label">GitHub CLI</span>
                        <div class="st__status">
                            <span
                                class="badge badge--{appState.settings
                                    ?.gh_installed
                                    ? 'done'
                                    : 'failed'}"
                            >
                                <span class="badge__dot" aria-hidden="true"
                                ></span>
                                {appState.settings?.gh_installed
                                    ? "Installed"
                                    : "Not installed"}
                            </span>
                            <span
                                class="badge badge--{appState.settings
                                    ?.gh_authenticated
                                    ? 'done'
                                    : 'warn'}"
                            >
                                <span class="badge__dot" aria-hidden="true"
                                ></span>
                                {appState.settings?.gh_authenticated
                                    ? "Signed in"
                                    : "Not signed in"}
                            </span>
                        </div>
                        <p class="hint">
                            Managed with the <code>gh</code> CLI, not by Fog.
                        </p>
                    </div>
                </div>
            </div>
        </section>

        <section class="st__sec reveal" style="--i: 4">
            <h2 class="st__sec-title">Task tracker</h2>
            <TrackerSettings />
        </section>

        <section class="st__sec reveal" style="--i: 5">
            <h2 class="st__sec-title">Repositories</h2>
            <RepoManager />
        </section>
    </div>

    <!-- Sticky footer: the save action stays reachable however long the page
         gets, rather than sitting below 160px of padding. -->
    <div class="st__save">
        <div class="st__save-inner shell-width">
            <button
                id="settings-save"
                class="btn btn-primary"
                data-state={loading ? "loading" : undefined}
                disabled={loading}
                onclick={saveAll}
            >
                Save settings
            </button>
        </div>
    </div>
</div>

<style>
    .st {
        flex: 1;
        min-block-size: 0;
        display: flex;
        flex-direction: column;
    }

    .st__inner {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: var(--space-xl);
        padding-block: var(--space-xl) var(--space-2xl);
        max-inline-size: 56rem;
    }

    .st__back {
        align-self: flex-start;
        margin-block-end: var(--space-xs);
    }

    .st__sec {
        display: flex;
        flex-direction: column;
        gap: var(--space-sm);
        min-inline-size: 0;
    }

    .st__sec-title {
        font-size: var(--text-md);
        padding-block-end: var(--space-2xs);
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .st__fields {
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
    }

    :global(.st__wide) {
        inline-size: 100%;
        max-inline-size: 24rem;
    }

    .st__status {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-xs);
    }

    .st__save {
        position: sticky;
        inset-block-end: 0;
        background: var(--color-paper);
        border-block-start: var(--rule-hair) solid var(--color-rule);
    }

    .st__save-inner {
        display: flex;
        justify-content: flex-end;
        padding-block: var(--space-sm);
        max-inline-size: 56rem;
    }
</style>
