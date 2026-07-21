// ── API layer: fetch helpers + SSE streaming ──

import type {
    CancelResponse,
    CreateSessionPayload,
    CreateSessionResponse,
    DiffResult,
    DiscoveredRepo,
    FollowupResponse,
    ImportResponse,
    OpenResponse,
    Repo,
    RunEvent,
    SessionDetail,
    SessionSummary,
    Settings,
    UpdateSettingsPayload,
    Branch,
    GhStatus,
    Task,
    TaskStatus,
    TaskResponse,
    CreateTaskPayload,
    UpdateTaskPayload,
    TrackerConfig,
    UpdateTrackerPayload,
    SyncResult,
} from "./types";

let apiBaseURL = "http://127.0.0.1:8080";
let apiToken = "";

export function setAPIBaseURL(url: string): void {
    apiBaseURL = url;
}

export function getAPIBaseURL(): string {
    return apiBaseURL;
}

export function setAPIToken(token: string): void {
    apiToken = token;
}

export async function resolveAPIBaseURL(): Promise<string> {
    if (window.__FOG_API_BASE_URL__) {
        return window.__FOG_API_BASE_URL__;
    }
    try {
        const app = window.go?.main?.desktopApp;
        if (app && typeof app.APIBaseURL === "function") {
            const base = await app.APIBaseURL();
            if (base) return base;
        }
    } catch {
        // ignore
    }
    return apiBaseURL;
}

export async function resolveVersion(): Promise<string> {
    try {
        const app = window.go?.main?.desktopApp;
        if (app && typeof app.Version === "function") {
            const v = await app.Version();
            if (v) return v;
        }
    } catch {
        // ignore
    }
    return "–";
}

export async function resolveAPIToken(): Promise<string> {
    try {
        const app = window.go?.main?.desktopApp;
        if (app && typeof app.APIToken === "function") {
            const t = await app.APIToken();
            if (t) return t;
        }
    } catch {
        // ignore
    }
    return "";
}

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
    const url = apiBaseURL + path;
    const opts = options ?? {};
    const headers = new Headers(opts.headers);
    if (apiToken) {
        headers.set("Authorization", "Bearer " + apiToken);
    }
    opts.headers = headers;
    const res = await fetch(url, opts);
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
    }
    if (res.status === 204) return null as T;
    return res.json();
}

// ── API methods ──

export async function fetchSettings(): Promise<Settings> {
    return fetchJSON<Settings>("/api/settings");
}

export async function fetchGhStatus(): Promise<GhStatus> {
    return fetchJSON<GhStatus>("/api/gh/status");
}

export async function updateSettings(
    payload: UpdateSettingsPayload,
): Promise<Settings> {
    return fetchJSON<Settings>("/api/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
    });
}

export async function fetchRepos(): Promise<Repo[]> {
    return fetchJSON<Repo[]>("/api/repos");
}

export async function discoverRepos(): Promise<DiscoveredRepo[]> {
    return fetchJSON<DiscoveredRepo[]>("/api/repos/discover", {
        method: "POST",
    });
}

export async function importRepos(
    repos: string[],
): Promise<ImportResponse> {
    return fetchJSON<ImportResponse>("/api/repos/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ repos }),
    });
}

export async function fetchSessions(): Promise<SessionSummary[]> {
    return fetchJSON<SessionSummary[]>("/api/sessions");
}

export async function fetchBranches(repoName: string): Promise<Branch[]> {
    return fetchJSON<Branch[]>(
        "/api/repos/branches?name=" + encodeURIComponent(repoName),
    );
}

export async function fetchSessionDetail(
    sessionID: string,
): Promise<SessionDetail> {
    return fetchJSON<SessionDetail>(
        "/api/sessions/" + encodeURIComponent(sessionID),
    );
}

export async function createSession(
    payload: CreateSessionPayload,
): Promise<CreateSessionResponse> {
    return fetchJSON<CreateSessionResponse>("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
    });
}

export async function followUp(
    sessionID: string,
    prompt: string,
): Promise<FollowupResponse> {
    return fetchJSON<FollowupResponse>(
        "/api/sessions/" + encodeURIComponent(sessionID) + "/runs",
        {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ prompt, async: true }),
        },
    );
}

export async function forkSession(
    sessionID: string,
    prompt: string,
): Promise<CreateSessionResponse> {
    return fetchJSON<CreateSessionResponse>(
        "/api/sessions/" + encodeURIComponent(sessionID) + "/fork",
        {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ prompt, async: true }),
        },
    );
}

export async function cancelSession(
    sessionID: string,
): Promise<CancelResponse> {
    return fetchJSON<CancelResponse>(
        "/api/sessions/" + encodeURIComponent(sessionID) + "/cancel",
        { method: "POST" },
    );
}

export async function fetchDiff(sessionID: string): Promise<DiffResult> {
    return fetchJSON<DiffResult>(
        "/api/sessions/" + encodeURIComponent(sessionID) + "/diff",
    );
}

export async function openInEditor(
    sessionID: string,
): Promise<OpenResponse> {
    return fetchJSON<OpenResponse>(
        "/api/sessions/" + encodeURIComponent(sessionID) + "/open",
        { method: "POST" },
    );
}

export async function fetchRunEvents(
    sessionID: string,
    runID: string,
    limit = 200,
): Promise<RunEvent[]> {
    return fetchJSON<RunEvent[]>(
        "/api/sessions/" +
        encodeURIComponent(sessionID) +
        "/runs/" +
        encodeURIComponent(runID) +
        "/events?limit=" +
        limit,
    );
}

// ── SSE streaming ──

export function openRunStream(
    sessionID: string,
    runID: string,
    cursor: number,
    onEvent: (event: RunEvent) => void,
    onDone: () => void,
    onError: () => void,
): EventSource {
    const url =
        apiBaseURL +
        "/api/sessions/" +
        encodeURIComponent(sessionID) +
        "/runs/" +
        encodeURIComponent(runID) +
        "/stream?cursor=" +
        encodeURIComponent(String(cursor));

    const source = new EventSource(url);

    source.addEventListener("run_event", (ev) => {
        try {
            const payload = JSON.parse(ev.data) as RunEvent;
            onEvent(payload);
        } catch {
            // ignore malformed events
        }
    });

    source.addEventListener("done", () => {
        source.close();
        onDone();
    });

    source.onerror = () => {
        source.close();
        onError();
    };

    return source;
}

// ── Tasks ──

export async function fetchTasks(): Promise<Task[]> {
    return fetchJSON<Task[]>("/api/tasks");
}

export async function createTask(
    payload: CreateTaskPayload,
): Promise<TaskResponse> {
    return fetchJSON<TaskResponse>("/api/tasks", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
    });
}

export async function updateTask(
    taskID: string,
    payload: UpdateTaskPayload,
): Promise<TaskResponse> {
    return fetchJSON<TaskResponse>(
        "/api/tasks/" + encodeURIComponent(taskID),
        {
            method: "PATCH",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
        },
    );
}

export async function deleteTask(taskID: string): Promise<void> {
    await fetchJSON<null>("/api/tasks/" + encodeURIComponent(taskID), {
        method: "DELETE",
    });
}

/**
 * Move a card to a column at an index.
 *
 * The server decides whether this starts an agent — it is not a client
 * choice. Dragging into In Progress from Fog's own UI counts as a local
 * move and launches work; the same status arriving via tracker sync does
 * not. See internal/api/tasks.go.
 */
export async function moveTask(
    taskID: string,
    status: TaskStatus,
    index: number,
): Promise<TaskResponse> {
    return fetchJSON<TaskResponse>(
        "/api/tasks/" + encodeURIComponent(taskID) + "/move",
        {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ status, index }),
        },
    );
}

/** Explicitly start work — the affordance offered when auto-start was withheld. */
export async function startTask(taskID: string): Promise<TaskResponse> {
    return fetchJSON<TaskResponse>(
        "/api/tasks/" + encodeURIComponent(taskID) + "/start",
        { method: "POST" },
    );
}

// ── Tracker ──

export async function fetchTrackerConfig(): Promise<TrackerConfig> {
    return fetchJSON<TrackerConfig>("/api/tracker");
}

export async function updateTrackerConfig(
    payload: UpdateTrackerPayload,
): Promise<TrackerConfig> {
    return fetchJSON<TrackerConfig>("/api/tracker", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
    });
}

export async function syncTracker(): Promise<SyncResult> {
    return fetchJSON<SyncResult>("/api/tracker/sync", { method: "POST" });
}
