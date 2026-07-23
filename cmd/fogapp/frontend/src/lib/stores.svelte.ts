// ── Global reactive state using Svelte 5 runes ──

import { ACTIVE_STATES, BOARD_WINDOWS } from "./types";
import type {
    BoardWindow,
    DiffResult,
    Repo,
    RunEvent,
    RunSummary,
    SessionSummary,
    Settings,
    Task,
    TaskStatus,
} from "./types";
import {
    fetchDiff,
    fetchRepos,
    fetchRunEvents,
    fetchSessionDetail,
    fetchSessions,
    fetchSettings,
    deleteTask,
    fetchTasks,
    fetchTrashedTasks,
    moveTask,
    purgeTask,
    restoreTask,
    openRunStream,
    resolveAPIBaseURL,
    resolveAPIToken,
    resolveVersion,
    setAPIBaseURL,
    setAPIToken,
} from "./api";

// ── App state ──

export type ViewName = "board" | "new" | "detail" | "settings";

const BOARD_WINDOW_KEY = "fog.boardWindow";

/** Restore the last-picked board window, defaulting to the 7-day view. */
function loadBoardWindow(): BoardWindow {
    try {
        const saved = localStorage.getItem(BOARD_WINDOW_KEY);
        if (saved && BOARD_WINDOWS.some((w) => w.id === saved)) {
            return saved as BoardWindow;
        }
    } catch {
        // ignore unavailable storage
    }
    return "week";
}

class AppState {
    // Connection
    daemonStatus = $state<"connecting" | "connected" | "unavailable">("connecting");
    version = $state("–");

    // Data
    settings = $state<Settings | null>(null);
    repos = $state<Repo[]>([]);
    sessions = $state<SessionSummary[]>([]);
    tasks = $state<Task[]>([]);
    trashedTasks = $state<Task[]>([]);

    // UI state
    currentView = $state<ViewName>("board");
    boardWindow = $state<BoardWindow>(loadBoardWindow());
    showTrash = $state(false);
    selectedSessionID = $state("");
    selectedRunID = $state("");
    selectedTab = $state<"timeline" | "diff" | "logs" | "stats">("timeline");
    autoFollowLatest = $state(true);

    // New UI state
    sessionMode = $state<"plan" | "build">("build");
    selectedBranch = $state("");
    chatExpanded = $state(false);

    // Detail data
    detailSession = $state<SessionSummary | null>(null);
    detailRuns = $state<RunSummary[]>([]);
    detailEvents = $state<RunEvent[]>([]);
    detailDiff = $state<DiffResult | null>(null);
    detailDiffError = $state("");

    // SSE stream
    private streamSource: EventSource | null = null;
    private streamSessionID = "";
    private streamRunID = "";

    // Polling
    private pollInterval: ReturnType<typeof setInterval> | null = null;

    // ── Derived ──

    get runningSessions(): SessionSummary[] {
        return this.sessions.filter((s) => this.isSessionRunning(s));
    }

    get completedSessions(): SessionSummary[] {
        return this.sessions.filter((s) => !this.isSessionRunning(s));
    }

    /** Tasks grouped into board columns, each already in position order. */
    get board(): Record<TaskStatus, Task[]> {
        const columns = {
            todo: [] as Task[],
            in_progress: [] as Task[],
            in_review: [] as Task[],
            done: [] as Task[],
        };
        for (const task of this.tasks) {
            columns[task.status]?.push(task);
        }
        for (const key of Object.keys(columns) as TaskStatus[]) {
            columns[key].sort((a, b) => a.position - b.position);
        }
        return columns;
    }

    /**
     * The board as rendered: identical to {@link board} for live columns, but
     * with finished cards aged out of Done once they fall outside the selected
     * timeline window. Live work is never hidden — only completed work drifts
     * off the board as it gets older. `board` stays the unfiltered source of
     * truth for positioning and keyboard moves.
     */
    get visibleBoard(): Record<TaskStatus, Task[]> {
        const full = this.board;
        const cutoff = this.boardCutoff();
        if (cutoff === null) return full;
        return {
            ...full,
            done: full.done.filter(
                (t) => new Date(t.updated_at).getTime() >= cutoff,
            ),
        };
    }

    /** Done cards hidden by the current window (0 when showing all). */
    get hiddenDoneCount(): number {
        return this.board.done.length - this.visibleBoard.done.length;
    }

    /** Epoch millis a Done card must reach to stay visible, or null for all. */
    private boardCutoff(): number | null {
        const days = BOARD_WINDOWS.find(
            (w) => w.id === this.boardWindow,
        )?.days;
        if (days === null || days === undefined) return null;
        if (days === 0) {
            const now = new Date();
            return new Date(
                now.getFullYear(),
                now.getMonth(),
                now.getDate(),
            ).getTime();
        }
        return Date.now() - days * 86_400_000;
    }

    get selectedRun(): RunSummary | null {
        const found = this.detailRuns.find((r) => r.id === this.selectedRunID);
        return found ?? this.detailRuns[0] ?? null;
    }

    get latestRun(): RunSummary | null {
        return this.detailRuns[0] ?? null;
    }

    get canStop(): boolean {
        const session = this.detailSession;
        const latest = this.latestRun;
        return !!(session?.busy && latest && ACTIVE_STATES[latest.state]);
    }

    // ── Methods ──

    isSessionRunning(session: SessionSummary): boolean {
        if (session.busy) return true;
        const latest = session.latest_run;
        const runState = latest?.state ?? session.status;
        return !!ACTIVE_STATES[runState];
    }

    setView(view: ViewName): void {
        this.currentView = view;
        if (view !== "detail") {
            this.closeStream();
        }
    }

    setBoardWindow(window: BoardWindow): void {
        this.boardWindow = window;
        try {
            localStorage.setItem(BOARD_WINDOW_KEY, window);
        } catch {
            // storage is best-effort; the choice still holds for this session.
        }
    }

    async bootstrap(): Promise<void> {
        this.daemonStatus = "connecting";
        try {
            const baseURL = await resolveAPIBaseURL();
            setAPIBaseURL(baseURL);
            const token = await resolveAPIToken();
            setAPIToken(token);
            this.version = await resolveVersion();

            await this.refreshAll();
            this.daemonStatus = "connected";
            this.startPolling();

            // Handle deep link (session redirect)
            const params = new URLSearchParams(window.location.search);
            const sessionID = params.get("session");
            if (sessionID) {
                // Clear the parameter from the URL without reloading to avoid loops or confusing state
                window.history.replaceState({}, document.title, window.location.pathname);
                await this.selectSession(sessionID);
            }
        } catch {
            this.daemonStatus = "unavailable";
            throw new Error("Initialization failed");
        }
    }

    async refreshAll(): Promise<void> {
        const [settings, repos, sessions, tasks] = await Promise.all([
            fetchSettings(),
            fetchRepos(),
            fetchSessions(),
            fetchTasks(),
        ]);
        this.settings = settings;
        this.repos = repos;
        this.sessions = sessions;
        this.tasks = tasks;

        if (this.selectedSessionID) {
            await this.loadDetail();
        }
    }

    async refreshSessions(): Promise<void> {
        this.sessions = await fetchSessions();
        if (this.selectedSessionID) {
            const found = this.sessions.some(
                (s) => s.id === this.selectedSessionID,
            );
            if (!found) {
                this.closeStream();
                this.selectedSessionID = "";
                this.selectedRunID = "";
            }
        }
    }

    async refreshRepos(): Promise<void> {
        this.repos = await fetchRepos();
    }

    async refreshTasks(): Promise<void> {
        this.tasks = await fetchTasks();
    }

    /**
     * Apply a board move optimistically, then reconcile with the server.
     *
     * The card lands under the cursor immediately — waiting on a round trip
     * makes drag feel broken. On failure the whole board is refetched rather
     * than hand-rolling an inverse move, since the server may also have
     * started a session as part of the same call.
     */
    async moveTaskTo(
        taskID: string,
        status: TaskStatus,
        index: number,
    ): Promise<{ started: boolean; kind?: string; sessionID?: string }> {
        const before = this.tasks;
        const moving = before.find((t) => t.id === taskID);
        if (!moving) return { started: false };

        const column = this.board[status].filter((t) => t.id !== taskID);
        const prev = column[index - 1]?.position;
        const next = column[index]?.position;
        const optimistic =
            prev !== undefined && next !== undefined
                ? (prev + next) / 2
                : next !== undefined
                  ? next - 1
                  : prev !== undefined
                    ? prev + 1
                    : 0;

        this.tasks = before.map((t) =>
            t.id === taskID ? { ...t, status, position: optimistic } : t,
        );

        try {
            const res = await moveTask(taskID, status, index);
            this.tasks = this.tasks.map((t) =>
                t.id === taskID ? res.task : t,
            );
            if (res.started) {
                await this.refreshSessions();
            }
            return {
                started: res.started,
                kind: res.kind,
                sessionID: res.session_id,
            };
        } catch (err) {
            await this.refreshTasks();
            throw err;
        }
    }

    /**
     * Move a task to trash, dropping it from the board immediately and
     * reverting if the server rejects it. Trashing stops any active session on
     * the task but keeps its worktree, so it stays recoverable until retention
     * expires. Sessions are refreshed since one may have just been stopped.
     */
    async trashTaskByID(taskID: string): Promise<void> {
        const before = this.tasks;
        this.tasks = before.filter((t) => t.id !== taskID);
        try {
            await deleteTask(taskID);
            await this.refreshSessions();
        } catch (err) {
            this.tasks = before;
            throw err;
        }
    }

    async refreshTrash(): Promise<void> {
        this.trashedTasks = await fetchTrashedTasks();
    }

    /** Restore a trashed task back onto the board. */
    async restoreTaskByID(taskID: string): Promise<void> {
        const before = this.trashedTasks;
        this.trashedTasks = before.filter((t) => t.id !== taskID);
        try {
            const res = await restoreTask(taskID);
            this.tasks = [...this.tasks, res.task];
        } catch (err) {
            this.trashedTasks = before;
            throw err;
        }
    }

    /** Permanently delete a trashed task, reclaiming its worktree and branch. */
    async purgeTaskByID(taskID: string): Promise<void> {
        const before = this.trashedTasks;
        this.trashedTasks = before.filter((t) => t.id !== taskID);
        try {
            await purgeTask(taskID);
        } catch (err) {
            this.trashedTasks = before;
            throw err;
        }
    }

    async selectSession(
        sessionID: string,
        followLatest = true,
    ): Promise<void> {
        this.selectedSessionID = sessionID;
        if (followLatest) this.autoFollowLatest = true;
        this.setView("detail");
        await this.loadDetail();
    }

    async loadDetail(): Promise<void> {
        if (!this.selectedSessionID) {
            this.closeStream();
            this.detailSession = null;
            this.detailRuns = [];
            this.detailEvents = [];
            this.detailDiff = null;
            this.detailDiffError = "";
            return;
        }

        const detail = await fetchSessionDetail(this.selectedSessionID);
        this.detailSession = detail?.session ?? null;
        this.detailRuns = detail?.runs ?? [];

        if (!this.detailSession) {
            this.closeStream();
            this.detailEvents = [];
            this.detailDiff = null;
            this.detailDiffError = "Session not found.";
            return;
        }

        // pick run
        if (this.autoFollowLatest || !this.selectedRunID) {
            this.selectedRunID = this.detailRuns[0]?.id ?? "";
        } else {
            const exists = this.detailRuns.some(
                (r) => r.id === this.selectedRunID,
            );
            if (!exists) {
                this.selectedRunID = this.detailRuns[0]?.id ?? "";
            }
        }

        // load events
        if (this.selectedRunID) {
            this.detailEvents = await fetchRunEvents(
                this.selectedSessionID,
                this.selectedRunID,
            );
        } else {
            this.detailEvents = [];
        }

        // load diff
        try {
            this.detailDiff = await fetchDiff(this.selectedSessionID);
            this.detailDiffError = "";
        } catch (err) {
            this.detailDiff = null;
            this.detailDiffError =
                err instanceof Error ? err.message : "Failed to load diff";
        }

        this.openStream();
    }

    // ── SSE stream ──

    private closeStream(): void {
        if (this.streamSource) {
            this.streamSource.close();
            this.streamSource = null;
        }
        this.streamSessionID = "";
        this.streamRunID = "";
    }

    private openStream(): void {
        const run = this.selectedRun;
        if (!run || !this.selectedSessionID || !this.selectedRunID) {
            this.closeStream();
            return;
        }
        if (!ACTIVE_STATES[run.state]) {
            this.closeStream();
            return;
        }
        if (
            this.streamSource &&
            this.streamSessionID === this.selectedSessionID &&
            this.streamRunID === this.selectedRunID
        ) {
            return;
        }

        this.closeStream();

        const cursor = this.latestEventID();
        this.streamSource = openRunStream(
            this.selectedSessionID,
            this.selectedRunID,
            cursor,
            (event) => {
                this.appendEvent(event);
            },
            () => {
                this.closeStream();
                this.refreshSessions().catch(() => { });
            },
            () => {
                this.closeStream();
            },
        );
        this.streamSessionID = this.selectedSessionID;
        this.streamRunID = this.selectedRunID;
    }

    private latestEventID(): number {
        if (!this.detailEvents.length) return 0;
        const last = this.detailEvents[this.detailEvents.length - 1];
        return last?.id ?? 0;
    }

    private appendEvent(event: RunEvent): void {
        if (!event?.id) return;
        const exists = this.detailEvents.some((e) => e.id === event.id);
        if (exists) return;
        this.detailEvents = [...this.detailEvents, event].sort(
            (a, b) => a.id - b.id,
        );
    }

    // ── Polling ──

    private startPolling(): void {
        this.pollInterval = setInterval(() => {
            this.refreshSessions()
                .then(() => {
                    if (
                        this.selectedSessionID &&
                        this.currentView === "detail"
                    ) {
                        return this.loadDetail();
                    }
                    return undefined;
                })
                .catch(() => {
                    this.daemonStatus = "unavailable";
                });
        }, 4000);
    }

    destroy(): void {
        if (this.pollInterval) {
            clearInterval(this.pollInterval);
        }
        this.closeStream();
    }
}

export const appState = new AppState();
