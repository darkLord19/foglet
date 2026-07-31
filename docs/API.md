# Local HTTP API

`fogd` serves a local HTTP API (default `http://127.0.0.1:8080`).

This API is designed for local clients (desktop app, scripts). It is not an internet-facing service.

## Health

`GET /health`

## Settings

`GET /api/settings`

Response:
- `default_tool` (string)
- `default_model` (string)
- `default_models` (object: `{ "<tool>": "<model>" }` per-tool model defaults)
- `default_autopr` (bool)
- `default_notify` (bool; when true, Fog sends macOS desktop notifications on run completion/failure when available)
- `branch_prefix` (string)
- `gh_installed` (bool)
- `gh_authenticated` (bool)
- `onboarding_required` (bool, true when `gh_authenticated` is false or `default_tool` is empty)
- `available_tools` ([]string)

`PUT /api/settings`

Body:
- `default_tool` (string, optional)
- `default_model` (string, optional)
- `default_models` (object, optional)
- `default_autopr` (bool, optional)
- `default_notify` (bool, optional)
- `branch_prefix` (string, optional)

## GitHub CLI Status

`GET /api/gh/status`

Response:
- `installed` (bool)
- `authenticated` (bool)
- `os` (string)

## Repos

`GET /api/repos`

`POST /api/repos/discover`

Uses the authenticated GitHub CLI (`gh`) to list accessible repos.

`POST /api/repos/import`

Body:

```json
{"repos":["owner/repo","owner/another"]}
```

## Sessions (Desktop)

`GET /api/sessions`

Returns session summaries with `latest_run` when present.

`POST /api/sessions`

Body:
- `repo` (required, managed repo alias `owner/repo`)
- `prompt` (required)
- `tool` (optional if `default_tool` is configured)
- `model` (optional)
- `branch_name` (optional; generated from prompt when omitted, with `-N` suffix on collisions)
- `autopr` (optional; when true, creates a draft PR via the authenticated GitHub CLI `gh`)
- `pr_title` (optional; when `autopr` is true and a PR is created, uses this title)
- `setup_cmd`, `validate`, `validate_cmd`, `base_branch`, `commit_msg` (optional)
- `async` (optional, default true)

Follow-ups:

- `POST /api/sessions/{id}/runs` (body: `{ "prompt": "...", "async": true }`)
- `GET /api/sessions/{id}/runs`
- `GET /api/sessions/{id}/runs/{run_id}/events`

Fork:

- `POST /api/sessions/{id}/fork`
  - Body supports: `prompt` (required), `branch_name`, `tool`, `model`, `autopr`, `pr_title`, `setup_cmd`, `validate`, `validate_cmd`, `base_branch`, `commit_msg`, `async` (all optional unless noted)

Streaming:

- `GET /api/sessions/{id}/runs/{run_id}/stream`

Other actions:

- `POST /api/sessions/{id}/cancel` (cancels only the latest active run)
- `POST /api/sessions/{id}/fork` (creates a new session from the source session head)
- `GET /api/sessions/{id}/diff` (diff is base-branch vs session branch)
- `POST /api/sessions/{id}/open` (open session worktree in editor)

## Tasks (Desktop board)

`GET /api/tasks`

`POST /api/tasks`

Body:

```json
{
  "title":"Do thing",
  "body":"Context and acceptance criteria",
  "repo":"owner/repo",
  "status":"todo"
}
```

`GET /api/tasks/{id}`

`PATCH /api/tasks/{id}` updates task text, repository, tool, model, or base branch.

`POST /api/tasks/{id}/move` moves a card and locally starts the matching
implementer, reviewer, or QA run when the target is a working column.

`POST /api/tasks/{id}/start` explicitly starts work for tracker-originated
cards. Remote tracker moves never launch an agent automatically.

Canonical statuses are `todo`, `in_progress`, `in_review`, `in_qa`, and `done`.

## Notes

Some cloud/slack endpoints exist in the codebase for experiments, but they are not part of the current desktop-first docs.
