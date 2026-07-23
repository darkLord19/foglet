// ── TypeScript interfaces for the Fog local API ──

export interface SessionSummary {
    id: string;
    repo_name: string;
    branch: string;
    worktree_path: string;
    tool: string;
    model?: string;
    autopr: boolean;
    pr_url?: string;
    status: string;
    busy: boolean;
    created_at: string;
    updated_at: string;
    latest_run?: RunSummary;
}

export interface RunSummary {
    id: string;
    session_id: string;
    prompt: string;
    worktree_path: string;
    state: string;
    commit_sha?: string;
    commit_msg?: string;
    error?: string;
    created_at: string;
    updated_at: string;
    completed_at?: string;
}

export interface RunEvent {
    id: number;
    run_id: string;
    ts: string;
    type: string;
    message?: string;
    data?: string;
}

export interface SessionDetail {
    session: SessionSummary;
    runs: RunSummary[];
}

export interface DiffResult {
    base_branch: string;
    branch: string;
    worktree_path: string;
    stat: string;
    patch: string;
}

export interface Settings {
    default_tool?: string;
    default_model?: string;
    default_models?: Record<string, string>;
    default_autopr: boolean;
    default_notify: boolean;
    keep_awake: boolean;
    branch_prefix?: string;
    trash_retention_days: number;
    gh_installed: boolean;
    gh_authenticated: boolean;
    onboarding_required: boolean;
    available_tools: string[];
}

export interface UpdateSettingsPayload {
    default_tool?: string;
    default_model?: string;
    default_models?: Record<string, string>;
    default_autopr?: boolean;
    default_notify?: boolean;
    keep_awake?: boolean;
    branch_prefix?: string;
    trash_retention_days?: number;
}

export interface Repo {
    id: number;
    name: string;
    url: string;
    host?: string;
    owner?: string;
    repo?: string;
    bare_path?: string;
    base_worktree_path: string;
    default_branch?: string;
    created_at?: string;
}

export interface DiscoveredRepo {
    id: string;
    name: string;
    nameWithOwner: string;
    url: string;
    isPrivate: boolean;
    defaultBranchRef: { name: string };
    owner: { login: string };
}

export interface GhStatus {
    installed: boolean;
    authenticated: boolean;
    os: string;
}

export interface Branch {
    name: string;
    is_default: boolean;
}

export interface CreateSessionPayload {
    repo: string;
    prompt: string;
    tool?: string;
    model?: string;
    branch_name?: string;
    autopr?: boolean;
    setup_cmd?: string;
    validate?: boolean;
    validate_cmd?: string;
    base_branch?: string;
    commit_msg?: string;
    async?: boolean;
    pr_title?: string;
}

export interface CreateSessionResponse {
    session_id: string;
    run_id: string;
    status: string;
}

export interface FollowupResponse {
    run_id: string;
    status: string;
    session: string;
}

export interface CancelResponse {
    status: string;
    run_id: string;
}

export interface OpenResponse {
    status: string;
    editor: string;
    worktree_path: string;
}

export interface ImportResponse {
    imported: string[];
}

export const ACTIVE_STATES: Record<string, boolean> = {
    CREATED: true,
    SETUP: true,
    AI_RUNNING: true,
    VALIDATING: true,
    COMMITTED: true,
    PR_CREATED: true,
};

// ── Tasks (board) ──

export type TaskStatus = "todo" | "in_progress" | "in_review" | "done";
export type TaskProvider = "local" | "linear" | "jira";

export interface Task {
    id: string;
    title: string;
    body?: string;
    status: TaskStatus;
    position: number;
    repo_name?: string;
    tool?: string;
    model?: string;
    base_branch?: string;
    session_id?: string;
    provider: TaskProvider;
    external_id?: string;
    external_key?: string;
    external_url?: string;
    external_status?: string;
    synced_at?: string;
    created_at: string;
    updated_at: string;
    /** Set when the task is in trash; recoverable until retention expires. */
    trashed_at?: string;
}

export interface TaskResponse {
    task: Task;
    started: boolean;
    /** Which agent was launched: "implement" or "review". */
    kind?: "implement" | "review";
    session_id?: string;
}

export interface CreateTaskPayload {
    title: string;
    body?: string;
    status?: TaskStatus;
    repo?: string;
    tool?: string;
    model?: string;
    base_branch?: string;
}

export interface UpdateTaskPayload {
    title?: string;
    body?: string;
    repo?: string;
    tool?: string;
    model?: string;
    base_branch?: string;
}

export const TASK_COLUMNS: { id: TaskStatus; label: string }[] = [
    { id: "todo", label: "Todo" },
    { id: "in_progress", label: "In progress" },
    { id: "in_review", label: "In review" },
    { id: "done", label: "Done" },
];

// ── Board timeline filter ──
//
// The board reads as a timeline: live work (todo / in progress / in review) is
// always shown, while finished cards age out of the Done column once they fall
// outside the selected window. `days: null` means "all time" (no cutoff).
export type BoardWindow = "today" | "week" | "month" | "all";

export const BOARD_WINDOWS: { id: BoardWindow; label: string; days: number | null }[] = [
    { id: "today", label: "Today", days: 0 },
    { id: "week", label: "7d", days: 7 },
    { id: "month", label: "30d", days: 30 },
    { id: "all", label: "All", days: null },
];

// ── Tracker sync ──

export interface TrackerStatusMap {
    todo: string[];
    in_progress: string[];
    in_review: string[];
    done: string[];
}

export interface TrackerConfig {
    provider: TaskProvider;
    has_token: boolean;
    status_map: TrackerStatusMap;
    linear_team?: string;
    jira_url?: string;
    jira_email?: string;
    jira_jql?: string;
}

export interface UpdateTrackerPayload {
    provider: TaskProvider;
    /** Blank leaves the stored token untouched. */
    token?: string;
    status_map?: TrackerStatusMap;
    linear_team?: string;
    jira_url?: string;
    jira_email?: string;
    jira_jql?: string;
}

export interface SyncResult {
    Imported: number;
    Updated: number;
    Pushed: number;
    Skipped: number;
    Unmapped: string[] | null;
}
