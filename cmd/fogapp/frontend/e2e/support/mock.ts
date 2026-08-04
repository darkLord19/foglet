import type { Page, Route } from '@playwright/test';

// ── Shared mock data ──

export const mockSettings = {
  default_tool: 'claude',
  default_model: '',
  default_models: {},
  default_autopr: false,
  default_notify: false,
  keep_awake: false,
  branch_prefix: 'fog/',
  trash_retention_days: 30,
  gh_installed: true,
  gh_authenticated: true,
  onboarding_required: false,
  available_tools: ['claude', 'cursor'],
};

export const mockRepo = {
  id: 1,
  name: 'owner/repo',
  url: 'https://github.com/owner/repo',
  default_branch: 'main',
  base_worktree_path: '/tmp/fog-test/owner/repo',
};

export const mockRun = {
  id: 'run-1',
  session_id: 'session-1',
  prompt: 'Initial prompt',
  worktree_path: '/tmp/fog-test/owner/repo',
  state: 'COMPLETED',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:01:00Z',
  completed_at: '2024-01-01T00:01:00Z',
};

export const mockSession = {
  id: 'session-1',
  repo_name: 'owner/repo',
  branch: 'fog/test-session',
  worktree_path: '/tmp/fog-test/owner/repo',
  tool: 'claude',
  status: 'COMPLETED',
  busy: false,
  autopr: false,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:01:00Z',
  latest_run: mockRun,
};

export const mockTask = {
  id: 'task-1',
  title: 'Sample task',
  status: 'todo',
  position: 1,
  provider: 'local',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

// ── Route handler ──

export async function setupApiMocks(page: Page): Promise<void> {
  await page.route('http://127.0.0.1:8080/api/**', async (route: Route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const method = route.request().method().toUpperCase();

    // Settings
    if (path === '/api/settings') {
      return route.fulfill({ json: mockSettings });
    }

    // Repos
    if (path === '/api/repos' && method === 'GET') {
      return route.fulfill({ json: [mockRepo] });
    }
    if (path === '/api/repos/branches') {
      return route.fulfill({ json: [{ name: 'main', is_default: true }] });
    }
    if (path === '/api/repos/discover' && method === 'POST') {
      return route.fulfill({ json: [] });
    }
    if (path === '/api/repos/import' && method === 'POST') {
      return route.fulfill({ json: { imported: [] } });
    }

    // Sessions list
    if (path === '/api/sessions' && method === 'GET') {
      return route.fulfill({ json: [mockSession] });
    }
    // Session create
    if (path === '/api/sessions' && method === 'POST') {
      return route.fulfill({
        json: { session_id: 'session-1', run_id: 'run-1', status: 'CREATED' },
      });
    }

    // Session detail: GET /api/sessions/{id}
    const sessionDetailMatch = path.match(/^\/api\/sessions\/([^/]+)$/);
    if (sessionDetailMatch && method === 'GET') {
      if (sessionDetailMatch[1] === 'session-1') {
        return route.fulfill({ json: { session: mockSession, runs: [mockRun] } });
      }
      return route.fulfill({ status: 404, json: { error: 'Session not found' } });
    }

    // Session cancel
    if (path.match(/^\/api\/sessions\/[^/]+\/cancel$/) && method === 'POST') {
      return route.fulfill({ json: { status: 'ok', run_id: 'run-1' } });
    }
    // Session open in editor
    if (path.match(/^\/api\/sessions\/[^/]+\/open$/) && method === 'POST') {
      return route.fulfill({
        json: { status: 'ok', editor: 'cursor', worktree_path: '/tmp/fog-test/owner/repo' },
      });
    }
    // Session fork
    if (path.match(/^\/api\/sessions\/[^/]+\/fork$/) && method === 'POST') {
      return route.fulfill({
        json: { session_id: 'session-1', run_id: 'run-1', status: 'CREATED' },
      });
    }
    // Follow-up run
    if (path.match(/^\/api\/sessions\/[^/]+\/runs$/) && method === 'POST') {
      return route.fulfill({
        json: { run_id: 'run-1', status: 'CREATED', session: 'session-1' },
      });
    }

    // Run events
    if (path.match(/^\/api\/sessions\/[^/]+\/runs\/[^/]+\/events$/)) {
      return route.fulfill({
        json: [
          { id: 1, run_id: 'run-1', ts: '2024-01-01T00:00:00Z', type: 'output', message: 'Agent started' },
          { id: 2, run_id: 'run-1', ts: '2024-01-01T00:01:00Z', type: 'output', message: 'Done.' },
        ],
      });
    }

    // SSE stream: return a complete body with a "done" event so EventSource closes cleanly.
    if (path.match(/^\/api\/sessions\/[^/]+\/runs\/[^/]+\/stream$/)) {
      return route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
        body: 'event: done\ndata: "COMPLETED"\n\n',
      });
    }

    // Diff
    if (path.match(/^\/api\/sessions\/[^/]+\/diff$/) && method === 'GET') {
      return route.fulfill({
        json: {
          base_branch: 'main',
          branch: 'fog/test-session',
          worktree_path: '/tmp/fog-test/owner/repo',
          stat: '1 file changed, 1 insertion(+)',
          patch: 'diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -0,0 +1 @@\n+hello',
        },
      });
    }

    // Tasks list
    if (path === '/api/tasks' && method === 'GET') {
      return route.fulfill({ json: [mockTask] });
    }
    // Task create
    if (path === '/api/tasks' && method === 'POST') {
      const body = JSON.parse((await route.request().postData()) || '{}');
      return route.fulfill({
        json: {
          task: {
            id: 'task-new',
            title: body.title,
            body: body.body || '',
            status: 'todo',
            position: 2,
            provider: 'local',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
          started: false,
        },
      });
    }
    // Trash list
    if (path === '/api/tasks/trash' && method === 'GET') {
      return route.fulfill({ json: [] });
    }
    // Task delete (trash)
    if (path.match(/^\/api\/tasks\/[^/]+$/) && method === 'DELETE') {
      return route.fulfill({ status: 204 });
    }
    // Task move
    if (path.match(/^\/api\/tasks\/[^/]+\/move$/) && method === 'POST') {
      return route.fulfill({ json: { task: { ...mockTask }, started: false } });
    }
    // Task restore
    if (path.match(/^\/api\/tasks\/[^/]+\/restore$/) && method === 'POST') {
      return route.fulfill({ json: { task: mockTask, started: false } });
    }
    // Task purge
    if (path.match(/^\/api\/tasks\/[^/]+\/purge$/) && method === 'POST') {
      return route.fulfill({ status: 204 });
    }
    // Task start
    if (path.match(/^\/api\/tasks\/[^/]+\/start$/) && method === 'POST') {
      return route.fulfill({ json: { task: mockTask, started: true } });
    }

    // MCP
    if (path === '/api/mcp') {
      return route.fulfill({ json: { upstreams: [] } });
    }

    // Tracker
    if (path === '/api/tracker' && method !== 'POST') {
      return route.fulfill({
        json: {
          provider: 'local',
          has_token: false,
          status_map: { todo: [], in_progress: [], in_review: [], in_qa: [], done: [] },
        },
      });
    }
    if (path === '/api/tracker/sync' && method === 'POST') {
      return route.fulfill({ json: { Imported: 0, Updated: 0, Pushed: 0, Skipped: 0, Unmapped: null } });
    }

    // Unhandled: log and 404 so tests fail visibly rather than hanging.
    console.warn(`[mock] Unhandled: ${method} ${path}`);
    await route.fulfill({ status: 404, json: { error: `mock: unhandled ${method} ${path}` } });
  });
}
