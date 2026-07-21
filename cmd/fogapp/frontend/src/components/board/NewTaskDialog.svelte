<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { createTask } from "$lib/api";
    import { toast } from "svelte-sonner";
    import Dropdown from "../Dropdown.svelte";

    let { open = $bindable(false) }: { open?: boolean } = $props();

    let dialog = $state<HTMLDialogElement | null>(null);
    let title = $state("");
    let body = $state("");
    let repo = $state("");
    let saving = $state(false);

    $effect(() => {
        if (!dialog) return;
        if (open && !dialog.open) {
            title = "";
            body = "";
            repo = appState.repos.length === 1 ? appState.repos[0].name : "";
            dialog.showModal();
        } else if (!open && dialog.open) {
            dialog.close();
        }
    });

    async function submit(e: Event) {
        e.preventDefault();
        if (!title.trim() || saving) return;

        saving = true;
        try {
            await createTask({
                title: title.trim(),
                body: body.trim() || undefined,
                repo: repo || undefined,
            });
            await appState.refreshTasks();
            open = false;
        } catch (err) {
            toast.error(
                "Couldn't create the task: " +
                    (err instanceof Error ? err.message : "unknown error"),
            );
        } finally {
            saving = false;
        }
    }
</script>

<dialog bind:this={dialog} class="dlg" onclose={() => (open = false)}>
    <form class="dlg__form" onsubmit={submit}>
        <h2 class="dlg__title">New task</h2>

        <div class="field">
            <label class="label" for="task-title">Title</label>
            <!-- svelte-ignore a11y_autofocus -->
            <input
                id="task-title"
                class="input"
                type="text"
                bind:value={title}
                placeholder="What should the agent do?"
                autofocus
            />
        </div>

        <div class="field">
            <label class="label" for="task-body">Detail</label>
            <textarea
                id="task-body"
                class="textarea"
                bind:value={body}
                rows="3"
                placeholder="Context, constraints, acceptance criteria…"
            ></textarea>
        </div>

        <div class="field">
            <span class="label">Repository</span>
            <Dropdown
                bind:value={repo}
                options={[
                    { value: "", label: "Choose later" },
                    ...appState.repos.map((r) => ({
                        value: r.name,
                        label: r.name,
                    })),
                ]}
                placeholder="Choose later"
                class="dlg__wide"
            />
            <p class="hint">
                A task can&rsquo;t start work until it has a repository.
            </p>
        </div>

        <div class="dlg__actions">
            <button
                type="button"
                class="btn btn-ghost"
                onclick={() => (open = false)}
            >
                Cancel
            </button>
            <button
                type="submit"
                class="btn btn-primary"
                data-state={saving ? "loading" : undefined}
                disabled={!title.trim() || saving}
            >
                Add task
            </button>
        </div>
    </form>
</dialog>

<style>
    .dlg {
        inline-size: min(30rem, calc(100dvw - 2rem));
        padding: 0;
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule-2);
        border-radius: var(--radius-lg);
        color: var(--color-ink);
    }

    .dlg::backdrop {
        background: oklch(10% 0.004 264 / 0.7);
    }

    .dlg__form {
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
        padding: var(--space-lg);
    }

    .dlg__title {
        font-size: var(--text-md);
        font-weight: 600;
        letter-spacing: var(--tracking-tight);
    }

    .dlg__actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--space-xs);
    }

    :global(.dlg__wide) {
        inline-size: 100%;
    }
</style>
