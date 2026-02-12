package api

const webUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Fog</title>
  <style>
    :root {
      --bg: #f5f4ef;
      --panel: #fffdf6;
      --ink: #181a1b;
      --muted: #5f6b6d;
      --accent: #006b5f;
      --accent-soft: #d5f0ea;
      --warn: #8b2e24;
      --line: #d9d6cd;
      --mono: "JetBrains Mono", "SFMono-Regular", Menlo, monospace;
      --sans: "Avenir Next", "Segoe UI", sans-serif;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: var(--sans);
      color: var(--ink);
      background:
        radial-gradient(circle at 10% -20%, #d6ece8 0%, transparent 40%),
        radial-gradient(circle at 95% 0%, #efe2c6 0%, transparent 30%),
        var(--bg);
    }
    .wrap {
      max-width: 1080px;
      margin: 0 auto;
      padding: 24px 16px 48px;
    }
    h1 {
      margin: 0 0 6px;
      font-size: 28px;
      letter-spacing: 0.2px;
    }
    .sub {
      margin: 0;
      color: var(--muted);
    }
    .grid {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(12, minmax(0, 1fr));
      margin-top: 18px;
    }
    .card {
      grid-column: span 12;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 14px;
      padding: 16px;
      box-shadow: 0 6px 18px rgba(0,0,0,0.04);
    }
    .card h2 {
      margin: 0 0 10px;
      font-size: 17px;
    }
    .kpi {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      margin-top: 6px;
    }
    .pill {
      font-family: var(--mono);
      font-size: 12px;
      padding: 6px 10px;
      border-radius: 999px;
      border: 1px solid var(--line);
      background: #fff;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
    }
    th, td {
      text-align: left;
      border-bottom: 1px solid var(--line);
      padding: 8px 6px;
    }
    th {
      color: var(--muted);
      font-weight: 600;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.35px;
    }
    .mono { font-family: var(--mono); }
    .state {
      font-family: var(--mono);
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 10px;
      background: #eef2ee;
      display: inline-block;
    }
    .state.active { background: var(--accent-soft); color: #004b43; }
    .state.failed { background: #f6d5cf; color: #732017; }
    .row {
      display: grid;
      grid-template-columns: 160px 1fr;
      gap: 10px;
      margin-bottom: 10px;
      align-items: center;
    }
    label { font-size: 13px; color: var(--muted); }
    input, select {
      width: 100%;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 9px 10px;
      background: #fff;
      color: var(--ink);
    }
    button {
      border: 0;
      border-radius: 10px;
      padding: 10px 14px;
      background: var(--accent);
      color: #fff;
      font-weight: 600;
      cursor: pointer;
    }
    button:disabled { opacity: 0.6; cursor: not-allowed; }
    .status {
      margin-left: 8px;
      font-size: 12px;
      color: var(--muted);
    }
    @media (min-width: 880px) {
      .tasks { grid-column: span 8; }
      .settings { grid-column: span 4; }
    }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Fog Control Plane</h1>
    <p class="sub">Track running agents and adjust Fog defaults.</p>

    <div class="grid">
      <section class="card tasks">
        <h2>Task Activity</h2>
        <div class="kpi">
          <div class="pill">Active: <span id="kpi-active">0</span></div>
          <div class="pill">Total: <span id="kpi-total">0</span></div>
          <div class="pill">Updated: <span id="kpi-updated">-</span></div>
        </div>
        <div style="overflow:auto; margin-top:12px;">
          <table>
            <thead>
              <tr>
                <th>State</th>
                <th>Repo</th>
                <th>Branch</th>
                <th>Tool</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody id="task-body">
              <tr><td colspan="5">Loading tasks...</td></tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card settings">
        <h2>Settings</h2>
        <form id="settings-form">
          <div class="row">
            <label for="default-tool">Default Tool</label>
            <select id="default-tool" name="default_tool"></select>
          </div>
          <div class="row">
            <label for="branch-prefix">Branch Prefix</label>
            <input id="branch-prefix" name="branch_prefix" placeholder="fog">
          </div>
          <div class="row">
            <label>GitHub PAT</label>
            <div class="mono" id="pat-status">-</div>
          </div>
          <button id="save-btn" type="submit">Save Settings</button>
          <span class="status" id="save-status"></span>
        </form>
      </section>
    </div>
  </div>

  <script>
    var activeStates = { CREATED: true, SETUP: true, AI_RUNNING: true, VALIDATING: true, COMMITTED: true, PR_CREATED: true };

    function escapeHTML(value) {
      return String(value || "")
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
    }

    function stateClass(state) {
      if (state === "FAILED") return "state failed";
      if (activeStates[state]) return "state active";
      return "state";
    }

    function formatDate(value) {
      if (!value) return "-";
      var dt = new Date(value);
      if (isNaN(dt.getTime())) return "-";
      return dt.toLocaleString();
    }

    async function fetchJSON(url, options) {
      var res = await fetch(url, options || {});
      if (!res.ok) {
        var txt = await res.text();
        throw new Error(txt || ("HTTP " + res.status));
      }
      return res.json();
    }

    async function refreshTasks() {
      var body = document.getElementById("task-body");
      try {
        var tasks = await fetchJSON("/api/tasks");
        var active = tasks.filter(function (t) { return activeStates[t.state]; });

        document.getElementById("kpi-active").textContent = String(active.length);
        document.getElementById("kpi-total").textContent = String(tasks.length);
        document.getElementById("kpi-updated").textContent = new Date().toLocaleTimeString();

        if (!tasks.length) {
          body.innerHTML = "<tr><td colspan='5'>No tasks yet</td></tr>";
          return;
        }

        body.innerHTML = tasks.slice(0, 40).map(function (t) {
          var repo = (t.metadata && t.metadata.repo) ? t.metadata.repo : "-";
          return "<tr>" +
            "<td><span class='" + stateClass(t.state) + "'>" + escapeHTML(t.state) + "</span></td>" +
            "<td class='mono'>" + escapeHTML(repo) + "</td>" +
            "<td class='mono'>" + escapeHTML(t.branch) + "</td>" +
            "<td>" + escapeHTML(t.ai_tool) + "</td>" +
            "<td>" + escapeHTML(formatDate(t.created_at)) + "</td>" +
            "</tr>";
        }).join("");
      } catch (err) {
        body.innerHTML = "<tr><td colspan='5'>Failed to load tasks: " + escapeHTML(err.message) + "</td></tr>";
      }
    }

    async function loadSettings() {
      var settings = await fetchJSON("/api/settings");
      var select = document.getElementById("default-tool");
      var options = settings.available_tools || [];
      if (!options.length && settings.default_tool) {
        options = [settings.default_tool];
      }
      select.innerHTML = options.map(function (tool) {
        var selected = tool === settings.default_tool ? " selected" : "";
        return "<option value='" + escapeHTML(tool) + "'" + selected + ">" + escapeHTML(tool) + "</option>";
      }).join("");
      document.getElementById("branch-prefix").value = settings.branch_prefix || "fog";
      document.getElementById("pat-status").textContent = settings.has_github_token ? "configured" : "missing";
    }

    async function onSaveSettings(event) {
      event.preventDefault();
      var saveBtn = document.getElementById("save-btn");
      var status = document.getElementById("save-status");
      status.textContent = "";
      saveBtn.disabled = true;

      try {
        await fetchJSON("/api/settings", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            default_tool: document.getElementById("default-tool").value,
            branch_prefix: document.getElementById("branch-prefix").value
          })
        });
        status.textContent = "Saved";
        await loadSettings();
      } catch (err) {
        status.textContent = "Save failed: " + err.message;
      } finally {
        saveBtn.disabled = false;
      }
    }

    document.getElementById("settings-form").addEventListener("submit", onSaveSettings);
    refreshTasks();
    loadSettings().catch(function (err) {
      document.getElementById("save-status").textContent = "Settings load failed: " + err.message;
    });
    setInterval(refreshTasks, 4000);
  </script>
</body>
</html>
`
