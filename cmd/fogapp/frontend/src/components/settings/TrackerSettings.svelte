<script lang="ts">
    import { onMount } from "svelte";
    import { toast } from "svelte-sonner";
    import { fetchTrackerConfig, updateTrackerConfig, syncTracker } from "$lib/api";
    import type { TrackerConfig, TaskProvider } from "$lib/types";
    import { appState } from "$lib/stores.svelte";
    import Dropdown from "../Dropdown.svelte";

    let config = $state<TrackerConfig | null>(null);
    let provider = $state<TaskProvider>("local");
    let token = $state("");
    let linearTeam = $state("");
    let jiraURL = $state("");
    let jiraEmail = $state("");
    let jiraJQL = $state("");
    let saving = $state(false);
    let syncing = $state(false);

    onMount(load);

    async function load() {
        try {
            const cfg = await fetchTrackerConfig();
            config = cfg;
            provider = cfg.provider;
            linearTeam = cfg.linear_team ?? "";
            jiraURL = cfg.jira_url ?? "";
            jiraEmail = cfg.jira_email ?? "";
            jiraJQL = cfg.jira_jql ?? "";
        } catch (err) {
            console.error("tracker config", err);
        }
    }

    async function save() {
        saving = true;
        try {
            config = await updateTrackerConfig({
                provider,
                token: token.trim() || undefined,
                linear_team: linearTeam.trim(),
                jira_url: jiraURL.trim(),
                jira_email: jiraEmail.trim(),
                jira_jql: jiraJQL.trim(),
            });
            token = "";
            toast.success("Tracker settings saved");
        } catch (err) {
            toast.error(
                "Couldn't save: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            saving = false;
        }
    }

    async function runSync() {
        syncing = true;
        try {
            const res = await syncTracker();
            await appState.refreshTasks();

            const parts = [];
            if (res.Imported) parts.push(`${res.Imported} imported`);
            if (res.Updated) parts.push(`${res.Updated} updated`);
            if (res.Pushed) parts.push(`${res.Pushed} pushed`);
            toast.success(parts.length ? parts.join(" · ") : "Already up to date");

            // Unmapped statuses are a config problem the user has to fix, so
            // they get named rather than silently counted.
            if (res.Unmapped?.length) {
                const unique = [...new Set(res.Unmapped)];
                toast.warning(
                    `Skipped unmapped ${unique.length === 1 ? "status" : "statuses"}: ${unique.join(", ")}`,
                );
            }
        } catch (err) {
            toast.error(
                "Sync failed: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            syncing = false;
        }
    }
</script>

<div class="panel">
    <div class="panel__body tr">
        <div class="field">
            <span class="label">Provider</span>
            <Dropdown
                bind:value={provider}
                options={[
                    { value: "local", label: "Local only" },
                    { value: "linear", label: "Linear" },
                    { value: "jira", label: "Jira" },
                ]}
                class="tr__wide"
            />
        </div>

        {#if provider !== "local"}
            <div class="field">
                <label class="label" for="tracker-token">
                    {provider === "linear" ? "API key" : "API token"}
                </label>
                <input
                    id="tracker-token"
                    class="input input-mono"
                    type="password"
                    bind:value={token}
                    placeholder={config?.has_token
                        ? "Stored — leave blank to keep"
                        : "Paste your token"}
                    autocomplete="off"
                />
                <p class="hint">
                    Stored encrypted on this machine. Never sent anywhere except
                    {provider === "linear" ? "Linear" : "Jira"}.
                </p>
            </div>
        {/if}

        {#if provider === "linear"}
            <div class="field">
                <label class="label" for="linear-team">Team key</label>
                <input
                    id="linear-team"
                    class="input input-mono"
                    bind:value={linearTeam}
                    placeholder="ENG"
                />
                <p class="hint">Leave blank to sync every team the key can see.</p>
            </div>
        {/if}

        {#if provider === "jira"}
            <div class="field">
                <label class="label" for="jira-url">Site URL</label>
                <input
                    id="jira-url"
                    class="input input-mono"
                    bind:value={jiraURL}
                    placeholder="https://yourteam.atlassian.net"
                />
            </div>
            <div class="field">
                <label class="label" for="jira-email">Account email</label>
                <input
                    id="jira-email"
                    class="input"
                    bind:value={jiraEmail}
                    placeholder="you@company.com"
                />
            </div>
            <div class="field">
                <label class="label" for="jira-jql">JQL filter</label>
                <input
                    id="jira-jql"
                    class="input input-mono"
                    bind:value={jiraJQL}
                    placeholder="assignee = currentUser() AND statusCategory != Done"
                />
                <p class="hint">
                    Jira workflow states are per-project, so if yours are named
                    unusually the sync will skip them and say which.
                </p>
            </div>
        {/if}

        <div class="tr__actions">
            <button
                class="btn btn-secondary"
                data-state={saving ? "loading" : undefined}
                disabled={saving}
                onclick={save}
            >
                Save
            </button>
            {#if provider !== "local"}
                <button
                    class="btn btn-secondary"
                    data-state={syncing ? "loading" : undefined}
                    disabled={syncing || !config?.has_token}
                    onclick={runSync}
                >
                    Sync now
                </button>
            {/if}
        </div>
    </div>
</div>

<style>
    .tr {
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
    }

    .tr__actions {
        display: flex;
        gap: var(--space-xs);
    }

    :global(.tr__wide) {
        inline-size: 100%;
        max-inline-size: 18rem;
    }
</style>
