(function () {
  var activeStates = {
    CREATED: true,
    SETUP: true,
    AI_RUNNING: true,
    VALIDATING: true,
    COMMITTED: true,
    PR_CREATED: true
  };

  var state = {
    apiBaseURL: "http://127.0.0.1:8080",
    settings: null,
    repos: [],
    sessions: [],
    discoveredRepos: [],
    selectedSessionID: "",
    selectedRunID: "",
    selectedTab: "timeline",
    view: "new",
    autoFollowLatest: true,
    detail: {
      session: null,
      runs: [],
      events: [],
      diff: null,
      diffError: ""
    }
  };

  function $(id) { return document.getElementById(id); }

  function setStatus(id, message, cls) {
    var el = $(id);
    if (!el) return;
    el.textContent = message || "";
    el.className = "status" + (cls ? (" " + cls) : "");
  }

  function escapeHTML(value) {
    return String(value || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/\"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function formatDate(value) {
    if (!value) return "-";
    var dt = new Date(value);
    if (isNaN(dt.getTime())) return "-";
    return dt.toLocaleString();
  }

  function firstPromptLine(prompt) {
    var text = String(prompt || "").trim();
    if (!text) return "Untitled session";
    var first = text.split(/\r?\n/).find(function (line) { return String(line || "").trim() !== ""; }) || text;
    first = first.trim();
    if (first.length > 110) first = first.slice(0, 110) + "...";
    return first;
  }

  function latestRunFromSessionSummary(session) {
    return session && session.latest_run ? session.latest_run : null;
  }

  function isSessionRunning(session) {
    if (!session) return false;
    if (session.busy) return true;
    var latest = latestRunFromSessionSummary(session);
    var runState = latest && latest.state ? latest.state : session.status;
    return !!activeStates[runState];
  }

  function findSessionSummary(sessionID) {
    return (state.sessions || []).find(function (s) {
      return String(s.id || "") === String(sessionID || "");
    }) || null;
  }

  function selectedRun() {
    var runs = state.detail.runs || [];
    var found = runs.find(function (r) { return r.id === state.selectedRunID; });
    return found || (runs.length ? runs[0] : null);
  }

  function latestRun() {
    var runs = state.detail.runs || [];
    return runs.length ? runs[0] : null;
  }

  async function resolveAPIBaseURL() {
    if (window.__FOG_API_BASE_URL__) {
      return String(window.__FOG_API_BASE_URL__);
    }
    try {
      var app = window.go && window.go.main && window.go.main.desktopApp;
      if (app && typeof app.APIBaseURL === "function") {
        var base = await app.APIBaseURL();
        if (base) return String(base);
      }
    } catch (_) {}
    return state.apiBaseURL;
  }

  async function resolveVersion() {
    try {
      var app = window.go && window.go.main && window.go.main.desktopApp;
      if (app && typeof app.Version === "function") {
        var v = await app.Version();
        if (v) return String(v);
      }
    } catch (_) {}
    return "-";
  }

  async function fetchJSON(path, options) {
    var url = state.apiBaseURL + path;
    var res = await fetch(url, options || {});
    if (!res.ok) {
      var text = await res.text();
      throw new Error(text || ("HTTP " + res.status));
    }
    if (res.status === 204) return null;
    return res.json();
  }

  function setView(viewName) {
    state.view = viewName;
    ["new", "detail", "settings"].forEach(function (name) {
      var panel = $("view-" + name);
      if (!panel) return;
      if (name === viewName) panel.classList.remove("hidden");
      else panel.classList.add("hidden");
    });
  }

  function renderSettings() {
    var s = state.settings || {};
    var tools = s.available_tools || [];
    if (!tools.length && s.default_tool) tools = [s.default_tool];

    var options = tools.map(function (tool) {
      var selected = tool === s.default_tool ? " selected" : "";
      return "<option value='" + escapeHTML(tool) + "'" + selected + ">" + escapeHTML(tool) + "</option>";
    }).join("");

    $("settings-tool").innerHTML = options || "<option value=''>No tools detected</option>";
    $("new-tool").innerHTML = "<option value=''>Auto</option>" + options;

    $("settings-prefix").value = s.branch_prefix || "fog";
    $("settings-pat-status").value = s.has_github_token ? "configured" : "missing";
    $("settings-onboarding").value = s.onboarding_required ? "required" : "complete";
  }

  function renderRepoSelectors() {
    var repoSelect = $("new-repo");
    if (!state.repos.length) {
      repoSelect.innerHTML = "<option value=''>No repos imported</option>";
    } else {
      repoSelect.innerHTML = state.repos.map(function (repo) {
        return "<option value='" + escapeHTML(repo.name) + "'>" + escapeHTML(repo.name) + "</option>";
      }).join("");
    }
    $("repo-count").textContent = "repos: " + state.repos.length;

    var managed = $("managed-list");
    if (!state.repos.length) {
      managed.innerHTML = "No managed repos.";
      return;
    }
    managed.innerHTML = state.repos.map(function (repo) {
      return "<div>" +
        "<strong>" + escapeHTML(repo.name) + "</strong>" +
        "<div class='repo-meta'>" + escapeHTML(repo.base_worktree_path || "-") + "</div>" +
      "</div>";
    }).join("");
  }

  function renderDiscoveredRepos() {
    var list = $("discover-list");
    if (!state.discoveredRepos.length) {
      list.innerHTML = "No discovered repos yet.";
      return;
    }
    list.innerHTML = state.discoveredRepos.map(function (repo, idx) {
      return "<label class='repo-item'>" +
        "<input type='checkbox' id='repo-" + idx + "' data-repo='" + escapeHTML(repo.full_name) + "'>" +
        "<span>" + escapeHTML(repo.full_name) + "</span>" +
        "<span class='repo-meta'>" + escapeHTML(repo.default_branch || "-") + "</span>" +
      "</label>";
    }).join("");
  }

  function renderSidebar() {
    var running = [];
    var completed = [];

    (state.sessions || []).forEach(function (session) {
      if (isSessionRunning(session)) running.push(session);
      else completed.push(session);
    });

    function renderSessionList(targetID, sessions, emptyText) {
      var root = $(targetID);
      if (!sessions.length) {
        root.innerHTML = emptyText;
        return;
      }
      root.innerHTML = sessions.slice(0, 100).map(function (session) {
        var latest = latestRunFromSessionSummary(session);
        var title = latest ? firstPromptLine(latest.prompt) : (session.branch || "Untitled session");
        var status = latest && latest.state ? latest.state : (session.status || "-");
        var activeClass = session.id === state.selectedSessionID ? " active" : "";
        return "<article class='session-item" + activeClass + "' data-session-id='" + escapeHTML(session.id) + "'>" +
          "<div class='session-title'>" + escapeHTML(title) + "</div>" +
          "<div class='session-meta'>" +
            "<span>" + escapeHTML(session.tool || "-") + "</span>" +
            "<span>" + escapeHTML(status + (session.busy ? "*" : "")) + "</span>" +
          "</div>" +
        "</article>";
      }).join("");

      Array.prototype.slice.call(root.querySelectorAll(".session-item[data-session-id]")).forEach(function (node) {
        node.addEventListener("click", function () {
          var id = node.getAttribute("data-session-id");
          selectSession(id, true).catch(function (err) {
            setStatus("followup-status", "Failed to load session: " + err.message, "error");
          });
        });
      });
    }

    renderSessionList("running-sessions", running, "No running sessions.");
    renderSessionList("completed-sessions", completed, "No completed sessions.");
  }

  function renderDetail() {
    var session = state.detail.session;
    if (!session) {
      $("detail-title").textContent = "Task";
      $("detail-meta").textContent = "Select a task from the sidebar.";
      $("tab-timeline").innerHTML = "No session selected.";
      $("tab-diff").innerHTML = "No diff available.";
      $("tab-logs").innerHTML = "No logs available.";
      $("tab-stats").innerHTML = "No stats available.";
      $("detail-stop").disabled = true;
      $("detail-rerun").disabled = true;
      $("detail-open").disabled = true;
      return;
    }

    var current = selectedRun();
    var latest = latestRun();
    var title = current ? firstPromptLine(current.prompt) : (session.branch || "Untitled session");
    var worktreePath = current && current.worktree_path ? current.worktree_path : (session.worktree_path || "-");
    var runState = latest && latest.state ? latest.state : (session.status || "-");

    $("detail-title").textContent = title;
    $("detail-meta").textContent = session.repo_name + " | " + session.tool + " | " + runState + " | " + worktreePath;

    var canStop = !!(session.busy && latest && activeStates[latest.state]);
    $("detail-stop").disabled = !canStop;
    $("detail-rerun").disabled = !latest;
    $("detail-open").disabled = false;

    renderTimelineTab();
    renderDiffTab();
    renderLogsTab();
    renderStatsTab();
    renderTabs();
  }

  function renderTimelineTab() {
    var runs = state.detail.runs || [];
    var events = state.detail.events || [];

    var runsHTML = "<div class='timeline-list'>" + (runs.length ? runs.map(function (run) {
      var isLatest = run.id === (latestRun() && latestRun().id);
      var marker = isLatest ? "latest" : "run";
      return "<article class='timeline-item'>" +
        "<strong>" + escapeHTML(marker.toUpperCase()) + "</strong>" +
        "<span>" + escapeHTML(run.state || "-") + "</span>" +
        "<div class='repo-meta'>" + escapeHTML(formatDate(run.updated_at || run.created_at)) + "</div>" +
        "<div>" + escapeHTML(firstPromptLine(run.prompt)) + "</div>" +
      "</article>";
    }).join("") : "No runs yet.") + "</div>";

    var eventsHTML = "<div class='timeline-list'>" + (events.length ? events.map(function (ev) {
      return "<article class='timeline-item'>" +
        "<strong>" + escapeHTML(ev.type || "event") + "</strong>" +
        "<span>" + escapeHTML(formatDate(ev.ts)) + "</span>" +
        "<div>" + escapeHTML(ev.message || ev.data || "-") + "</div>" +
      "</article>";
    }).join("") : "No events for selected run.") + "</div>";

    $("tab-timeline").innerHTML =
      "<h3>Runs</h3>" + runsHTML +
      "<h3>Events</h3>" + eventsHTML;
  }

  function renderDiffTab() {
    if (state.detail.diffError) {
      $("tab-diff").innerHTML = "<div class='status error'>" + escapeHTML(state.detail.diffError) + "</div>";
      return;
    }
    var diff = state.detail.diff;
    if (!diff) {
      $("tab-diff").innerHTML = "No diff available.";
      return;
    }
    var stat = diff.stat || "No file changes.";
    var patch = diff.patch || "";
    $("tab-diff").innerHTML =
      "<h3>" + escapeHTML(diff.base_branch + "..." + diff.branch) + "</h3>" +
      "<div class='repo-meta'>" + escapeHTML(diff.worktree_path || "") + "</div>" +
      "<pre class='code'>" + escapeHTML(stat) + "</pre>" +
      "<pre class='code'>" + escapeHTML(patch || "No patch output.") + "</pre>";
  }

  function renderLogsTab() {
    var events = state.detail.events || [];
    if (!events.length) {
      $("tab-logs").innerHTML = "No logs for selected run.";
      return;
    }
    var lines = events.map(function (ev) {
      var ts = formatDate(ev.ts);
      var kind = String(ev.type || "log").toUpperCase();
      var msg = ev.message || ev.data || "-";
      return "[" + ts + "] [" + kind + "] " + msg;
    }).join("\n");
    $("tab-logs").innerHTML = "<pre class='code'>" + escapeHTML(lines) + "</pre>";
  }

  function renderStatsTab() {
    var runs = state.detail.runs || [];
    var latest = latestRun();
    var completed = runs.filter(function (r) { return r.state === "COMPLETED"; }).length;
    var failed = runs.filter(function (r) { return r.state === "FAILED"; }).length;
    var canceled = runs.filter(function (r) { return r.state === "CANCELLED"; }).length;
    $("tab-stats").innerHTML =
      "<div class='timeline-list'>" +
        "<article class='timeline-item'><strong>Total runs</strong> " + runs.length + "</article>" +
        "<article class='timeline-item'><strong>Completed</strong> " + completed + "</article>" +
        "<article class='timeline-item'><strong>Failed</strong> " + failed + "</article>" +
        "<article class='timeline-item'><strong>Cancelled</strong> " + canceled + "</article>" +
        "<article class='timeline-item'><strong>Latest commit</strong> " + escapeHTML((latest && latest.commit_sha) || "-") + "</article>" +
      "</div>";
  }

  function renderTabs() {
    Array.prototype.slice.call(document.querySelectorAll(".tab")).forEach(function (tab) {
      var target = tab.getAttribute("data-tab");
      if (target === state.selectedTab) tab.classList.add("active");
      else tab.classList.remove("active");
    });
    ["timeline", "diff", "logs", "stats"].forEach(function (name) {
      var panel = $("tab-" + name);
      if (!panel) return;
      if (name === state.selectedTab) panel.classList.remove("hidden");
      else panel.classList.add("hidden");
    });
  }

  async function loadDetailSession() {
    if (!state.selectedSessionID) {
      state.detail = { session: null, runs: [], events: [], diff: null, diffError: "" };
      renderDetail();
      return;
    }

    var detail = await fetchJSON("/api/sessions/" + encodeURIComponent(state.selectedSessionID));
    state.detail.session = detail && detail.session ? detail.session : null;
    state.detail.runs = detail && detail.runs ? detail.runs : [];

    if (!state.detail.session) {
      state.detail.events = [];
      state.detail.diff = null;
      state.detail.diffError = "Session not found.";
      renderDetail();
      return;
    }

    if (state.autoFollowLatest || !state.selectedRunID) {
      state.selectedRunID = state.detail.runs.length ? state.detail.runs[0].id : "";
    } else {
      var exists = state.detail.runs.some(function (run) { return run.id === state.selectedRunID; });
      if (!exists) state.selectedRunID = state.detail.runs.length ? state.detail.runs[0].id : "";
    }

    if (state.selectedRunID) {
      state.detail.events = await fetchJSON(
        "/api/sessions/" + encodeURIComponent(state.selectedSessionID) +
        "/runs/" + encodeURIComponent(state.selectedRunID) + "/events?limit=200"
      ) || [];
    } else {
      state.detail.events = [];
    }

    try {
      state.detail.diff = await fetchJSON("/api/sessions/" + encodeURIComponent(state.selectedSessionID) + "/diff");
      state.detail.diffError = "";
    } catch (err) {
      state.detail.diff = null;
      state.detail.diffError = err.message;
    }

    renderDetail();
  }

  async function selectSession(sessionID, followLatest) {
    state.selectedSessionID = String(sessionID || "").trim();
    if (followLatest) state.autoFollowLatest = true;
    setView("detail");
    await loadDetailSession();
    renderSidebar();
  }

  async function refreshSessions() {
    state.sessions = await fetchJSON("/api/sessions");

    if (state.selectedSessionID) {
      var found = state.sessions.some(function (session) { return session.id === state.selectedSessionID; });
      if (!found) {
        state.selectedSessionID = "";
        state.selectedRunID = "";
      }
    }

    renderSidebar();
  }

  async function refreshAll() {
    var data = await Promise.all([
      fetchJSON("/api/settings"),
      fetchJSON("/api/repos"),
      fetchJSON("/api/sessions")
    ]);

    state.settings = data[0] || {};
    state.repos = data[1] || [];
    state.sessions = data[2] || [];

    renderSettings();
    renderRepoSelectors();
    renderSidebar();

    if (!state.selectedSessionID && state.sessions.length) {
      var preferred = state.sessions.find(function (session) { return isSessionRunning(session); }) || state.sessions[0];
      state.selectedSessionID = preferred.id;
      state.autoFollowLatest = true;
      setView("detail");
      await loadDetailSession();
    } else if (state.selectedSessionID) {
      await loadDetailSession();
    }
  }

  async function onCreateSession(event) {
    event.preventDefault();
    setStatus("new-status", "", "");
    var btn = $("new-submit");
    btn.disabled = true;

    try {
      var payload = {
        repo: $("new-repo").value,
        prompt: $("new-prompt").value.trim(),
        model: $("new-model").value.trim(),
        autopr: $("new-autopr").value === "true",
        async: true
      };

      var tool = $("new-tool").value;
      if (tool) payload.tool = tool;

      var branch = $("new-branch").value.trim();
      if (branch) payload.branch_name = branch;

      var commitMsg = $("new-commit-msg").value.trim();
      if (commitMsg) payload.commit_msg = commitMsg;

      var validate = $("new-validate").value === "true";
      payload.validate = validate;

      var validateCmd = $("new-validate-cmd").value.trim();
      if (validateCmd) payload.validate_cmd = validateCmd;

      if (!payload.repo) throw new Error("Repo is required");
      if (!payload.prompt) throw new Error("Prompt is required");

      var out = await fetchJSON("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      setStatus("new-status", "Queued session " + out.session_id, "ok");
      $("new-prompt").value = "";
      $("new-branch").value = "";
      $("new-commit-msg").value = "";

      await refreshSessions();
      if (out.session_id) {
        await selectSession(out.session_id, true);
      }
    } catch (err) {
      setStatus("new-status", "Create failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  async function onFollowup(event) {
    event.preventDefault();
    setStatus("followup-status", "", "");
    var btn = $("followup-submit");
    btn.disabled = true;

    try {
      if (!state.selectedSessionID) throw new Error("Select a session first");
      var prompt = $("followup-prompt").value.trim();
      if (!prompt) throw new Error("Prompt is required");

      var out = await fetchJSON("/api/sessions/" + encodeURIComponent(state.selectedSessionID) + "/runs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt: prompt, async: true })
      });

      setStatus("followup-status", "Queued run " + out.run_id, "ok");
      $("followup-prompt").value = "";
      state.autoFollowLatest = true;
      await refreshSessions();
      await loadDetailSession();
    } catch (err) {
      setStatus("followup-status", "Follow-up failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  async function onStopRun() {
    setStatus("followup-status", "", "");
    try {
      if (!state.selectedSessionID) throw new Error("Select a session first");
      var out = await fetchJSON("/api/sessions/" + encodeURIComponent(state.selectedSessionID) + "/cancel", {
        method: "POST"
      });
      setStatus("followup-status", "Cancel requested for " + out.run_id, "ok");
      await refreshSessions();
      await loadDetailSession();
    } catch (err) {
      setStatus("followup-status", "Cancel failed: " + err.message, "error");
    }
  }

  async function onRerun() {
    setStatus("followup-status", "", "");
    try {
      if (!state.selectedSessionID) throw new Error("Select a session first");
      var run = latestRun();
      if (!run || !String(run.prompt || "").trim()) throw new Error("No prior prompt available");

      var out = await fetchJSON("/api/sessions/" + encodeURIComponent(state.selectedSessionID) + "/runs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt: run.prompt, async: true })
      });
      setStatus("followup-status", "Queued re-run " + out.run_id, "ok");
      state.autoFollowLatest = true;
      await refreshSessions();
      await loadDetailSession();
    } catch (err) {
      setStatus("followup-status", "Re-run failed: " + err.message, "error");
    }
  }

  async function onOpenInEditor() {
    setStatus("followup-status", "", "");
    try {
      if (!state.selectedSessionID) throw new Error("Select a session first");
      var out = await fetchJSON("/api/sessions/" + encodeURIComponent(state.selectedSessionID) + "/open", {
        method: "POST"
      });
      setStatus("followup-status", "Opened in " + (out.editor || "editor"), "ok");
    } catch (err) {
      setStatus("followup-status", "Open failed: " + err.message, "error");
    }
  }

  async function onSaveSettings(event) {
    event.preventDefault();
    setStatus("settings-status", "", "");
    var btn = $("settings-submit");
    btn.disabled = true;

    try {
      var payload = {
        default_tool: $("settings-tool").value,
        branch_prefix: $("settings-prefix").value.trim()
      };
      var pat = $("settings-pat").value.trim();
      if (pat) payload.github_pat = pat;

      await fetchJSON("/api/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      setStatus("settings-status", "Saved", "ok");
      $("settings-pat").value = "";
      state.settings = await fetchJSON("/api/settings");
      renderSettings();
    } catch (err) {
      setStatus("settings-status", "Save failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  async function onDiscoverRepos() {
    setStatus("repos-status", "", "");
    var btn = $("discover-btn");
    btn.disabled = true;
    try {
      state.discoveredRepos = await fetchJSON("/api/repos/discover", { method: "POST" }) || [];
      renderDiscoveredRepos();
      setStatus("repos-status", "Discovered " + state.discoveredRepos.length + " repos", "ok");
    } catch (err) {
      setStatus("repos-status", "Discover failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  async function onImportRepos() {
    setStatus("repos-status", "", "");
    var btn = $("import-btn");
    btn.disabled = true;
    try {
      var checked = Array.prototype.slice.call(document.querySelectorAll("#discover-list input[type='checkbox']:checked"));
      var repos = checked.map(function (el) { return el.getAttribute("data-repo"); });
      if (!repos.length) throw new Error("Select at least one repo");

      var out = await fetchJSON("/api/repos/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ repos: repos })
      });

      setStatus("repos-status", "Imported " + ((out.imported || []).length) + " repos", "ok");
      state.repos = await fetchJSON("/api/repos");
      renderRepoSelectors();
    } catch (err) {
      setStatus("repos-status", "Import failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  function bindTabs() {
    Array.prototype.slice.call(document.querySelectorAll("#detail-tabs .tab")).forEach(function (tab) {
      tab.addEventListener("click", function () {
        state.selectedTab = tab.getAttribute("data-tab") || "timeline";
        renderTabs();
      });
    });
  }

  function bindActions() {
    $("new-session-form").addEventListener("submit", onCreateSession);
    $("followup-form").addEventListener("submit", onFollowup);
    $("settings-form").addEventListener("submit", onSaveSettings);

    $("detail-stop").addEventListener("click", function () {
      onStopRun().catch(function (err) {
        setStatus("followup-status", "Cancel failed: " + err.message, "error");
      });
    });

    $("detail-rerun").addEventListener("click", function () {
      onRerun().catch(function (err) {
        setStatus("followup-status", "Re-run failed: " + err.message, "error");
      });
    });

    $("detail-open").addEventListener("click", function () {
      onOpenInEditor().catch(function (err) {
        setStatus("followup-status", "Open failed: " + err.message, "error");
      });
    });

    $("show-new").addEventListener("click", function () {
      setView("new");
      setStatus("new-status", "", "");
    });

    $("show-settings").addEventListener("click", function () {
      setView("settings");
    });

    $("discover-btn").addEventListener("click", onDiscoverRepos);
    $("import-btn").addEventListener("click", onImportRepos);

    bindTabs();
  }

  async function bootstrap() {
    $("daemon-badge").textContent = "fogd: connecting";
    state.apiBaseURL = await resolveAPIBaseURL();
    var version = await resolveVersion();
    $("version-badge").textContent = "version: " + version;

    bindActions();
    await refreshAll();

    $("daemon-badge").textContent = "fogd: connected";

    setInterval(function () {
      refreshSessions()
        .then(function () {
          if (state.selectedSessionID && state.view === "detail") {
            return loadDetailSession();
          }
          return null;
        })
        .catch(function (err) {
          $("daemon-badge").textContent = "fogd: unavailable";
          setStatus("followup-status", "Refresh failed: " + err.message, "error");
        });
    }, 4000);
  }

  bootstrap().catch(function (err) {
    $("daemon-badge").textContent = "fogd: unavailable";
    setStatus("new-status", "Initialization failed: " + err.message, "error");
  });
})();
