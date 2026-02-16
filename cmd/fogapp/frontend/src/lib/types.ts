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
    branch_prefix?: string;
    has_github_token: boolean;
    onboarding_required: boolean;
    available_tools: string[];
}

export interface UpdateSettingsPayload {
    default_tool?: string;
    default_model?: string;
    default_models?: Record<string, string>;
    default_autopr?: boolean;
    default_notify?: boolean;
    branch_prefix?: string;
    github_pat?: string;
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
    id: number;
    name: string;
    full_name: string;
    clone_url: string;
    private: boolean;
    default_branch: string;
    owner_login?: string;
    html_url?: string;
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
