<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { startTask } from "$lib/api";
    import { TASK_COLUMNS } from "$lib/types";
    import type { Task, TaskStatus } from "$lib/types";
    import { toast } from "svelte-sonner";
    import { Plus } from "@lucide/svelte";
    import TaskCard from "./board/TaskCard.svelte";
    import NewTaskDialog from "./board/NewTaskDialog.svelte";

    let dragging = $state<Task | null>(null);
    let overColumn = $state<TaskStatus | null>(null);
    let overIndex = $state(-1);
    let showNew = $state(false);

    const board = $derived(appState.board);

    function onDragStart(task: Task) {
        dragging = task;
    }

    function onDragEnd() {
        dragging = null;
        overColumn = null;
        overIndex = -1;
    }

    /**
     * Work out where the card would land, from the pointer's position relative
     * to the midpoint of each card already in the column.
     */
    function computeIndex(e: DragEvent, status: TaskStatus): number {
        const list = e.currentTarget as HTMLElement;
        const cards = [...list.querySelectorAll<HTMLElement>("[data-card]")];
        const y = e.clientY;

        let index = 0;
        for (const el of cards) {
            if (el.dataset.card === dragging?.id) continue;
            const box = el.getBoundingClientRect();
            if (y > box.top + box.height / 2) index++;
        }
        return index;
    }

    function onDragOver(e: DragEvent, status: TaskStatus) {
        if (!dragging) return;
        e.preventDefault();
        if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
        overColumn = status;
        overIndex = computeIndex(e, status);
    }

    async function onDrop(e: DragEvent, status: TaskStatus) {
        if (!dragging) return;
        e.preventDefault();

        const task = dragging;
        const index = computeIndex(e, status);
        onDragEnd();

        await commitMove(task, status, index);
    }

    async function commitMove(task: Task, status: TaskStatus, index: number) {
        try {
            const res = await appState.moveTaskTo(task.id, status, index);
            if (res.started) {
                // Starting an agent is consequential and happens off-screen,
                // so this one does warrant a toast.
                toast.success(
                    res.kind === "review"
                        ? `Reviewing “${task.title}”`
                        : `Agent started on “${task.title}”`,
                );
            }
        } catch (err) {
            toast.error(
                err instanceof Error ? err.message : "Couldn't move the task",
            );
        }
    }

    /** Keyboard equivalent of a drag: move a focused card between columns. */
    async function onCardKeydown(e: KeyboardEvent, task: Task, index: number) {
        if (!e.altKey) return;
        const order = TASK_COLUMNS.map((c) => c.id);
        const at = order.indexOf(task.status);

        let next: TaskStatus | null = null;
        if (e.key === "ArrowRight" && at < order.length - 1) next = order[at + 1];
        if (e.key === "ArrowLeft" && at > 0) next = order[at - 1];
        if (!next) return;

        e.preventDefault();
        await commitMove(task, next, board[next].length);
    }

    async function start(task: Task) {
        try {
            const res = await startTask(task.id);
            await Promise.all([
                appState.refreshTasks(),
                appState.refreshSessions(),
            ]);
            toast.success(
                res.kind === "review"
                    ? `Reviewing “${task.title}”`
                    : `Agent started on “${task.title}”`,
            );
        } catch (err) {
            toast.error(
                err instanceof Error ? err.message : "Couldn't start the task",
            );
        }
    }

    function open(task: Task) {
        if (task.session_id) {
            appState.selectSession(task.session_id);
        }
    }
</script>

<div class="board">
    <header class="board__bar">
        <h1 class="board__title">Board</h1>
        <button class="btn btn-primary" onclick={() => (showNew = true)}>
            <Plus size={14} />
            <span>New task</span>
        </button>
    </header>

    <div class="board__cols scroll-x">
        {#each TASK_COLUMNS as column (column.id)}
            {@const items = board[column.id]}
            <section
                class="col"
                class:is-over={overColumn === column.id}
                aria-label={column.label}
            >
                <div class="col__head">
                    <span class="col__name">{column.label}</span>
                    <span class="col__count mono">{items.length}</span>
                </div>

                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                    class="col__list scroll-y"
                    ondragover={(e) => onDragOver(e, column.id)}
                    ondragleave={() => (overColumn = null)}
                    ondrop={(e) => onDrop(e, column.id)}
                >
                    {#each items as task, i (task.id)}
                        {#if overColumn === column.id && overIndex === i && dragging}
                            <div class="drop" aria-hidden="true"></div>
                        {/if}
                        <div
                            data-card={task.id}
                            onkeydown={(e) => onCardKeydown(e, task, i)}
                            role="presentation"
                        >
                            <TaskCard
                                {task}
                                dragging={dragging?.id === task.id}
                                onstart={start}
                                onopen={open}
                                ondragstart={onDragStart}
                                ondragend={onDragEnd}
                            />
                        </div>
                    {/each}

                    {#if overColumn === column.id && overIndex >= items.length && dragging}
                        <div class="drop" aria-hidden="true"></div>
                    {/if}

                    {#if items.length === 0 && overColumn !== column.id}
                        <p class="col__empty">
                            {column.id === "todo"
                                ? "Add a task to get started."
                                : "Nothing here."}
                        </p>
                    {/if}
                </div>
            </section>
        {/each}
    </div>

    <p class="board__hint">
        <b>In progress</b> runs the agent · <b>In review</b> runs a reviewer over
        its worktree ·
        <kbd class="kbd">⌥</kbd><kbd class="kbd">←</kbd
        ><kbd class="kbd">→</kbd> moves a focused card
    </p>
</div>

<NewTaskDialog bind:open={showNew} />

<style>
    .board {
        flex: 1;
        display: flex;
        flex-direction: column;
        min-block-size: 0;
        min-inline-size: 0;
    }

    .board__bar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-md);
        block-size: var(--bar-h);
        flex: none;
        padding-inline: var(--gutter);
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .board__title {
        font-size: var(--text-md);
        font-weight: 600;
        letter-spacing: var(--tracking-tight);
    }

    .board__cols {
        flex: 1;
        display: grid;
        grid-auto-flow: column;
        grid-auto-columns: minmax(15rem, 1fr);
        gap: var(--space-md);
        min-block-size: 0;
        padding: var(--space-md) var(--gutter);
        overflow-x: auto;
    }

    .col {
        display: flex;
        flex-direction: column;
        min-block-size: 0;
        min-inline-size: 0;
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule);
        border-radius: var(--radius-lg);
        transition: border-color var(--dur-micro) var(--ease-out);
    }

    .col.is-over {
        border-color: var(--color-accent-line);
    }

    .col__head {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-xs);
        padding: var(--space-xs) var(--space-sm);
        flex: none;
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .col__name {
        font-size: var(--text-2xs);
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        line-height: var(--leading-caps);
        color: var(--color-ink-3);
    }

    .col__count {
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }

    .col__list {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: var(--space-xs);
        min-block-size: 4rem;
        padding: var(--space-xs);
    }

    .col__empty {
        padding: var(--space-sm);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }

    /* Where the card will land. A 2px accent rule, matching the active-state
       language used everywhere else in the app. */
    .drop {
        block-size: var(--rule-active);
        background: var(--color-accent);
        border-radius: 2px;
        flex: none;
    }

    .board__hint {
        flex: none;
        display: flex;
        align-items: center;
        gap: var(--space-3xs);
        padding: var(--space-xs) var(--gutter);
        border-block-start: var(--rule-hair) solid var(--color-rule);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }

    .board__hint b {
        font-weight: 600;
        color: var(--color-ink-2);
    }
</style>
