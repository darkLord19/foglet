(function () {
  var activeStates = { CREATED: true, SETUP: true, AI_RUNNING: true, VALIDATING: true, COMMITTED: true, PR_CREATED: true };
  var state = {
    apiBaseURL: "http://127.0.0.1:8080",
    settings: null,
    sessions: [],
    repos: [],
    discoveredRepos: []
  };

  function $(id) { return document.getElementById(id); }

  function setStatus(id, message, cls) {
    var el = $(id);
    el.textContent = message || "";
    el.className = "status" + (cls ? (" " + cls) : "");
  }

  function escapeHTML(value) {
    return String(value || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function formatDate(value) {
    if (!value) return "-";
    var dt = new Date(value);
    if (isNaN(dt.getTime())) return "-";
    return dt.toLocaleString();
  }

  async function resolveAPIBaseURL() {
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

  function openExternal(url) {
    var app = window.go && window.go.main && window.go.main.desktopApp;
    if (app && typeof app.OpenExternal === "function") {
      app.OpenExternal(url);
      return;
    }
    window.open(url, "_blank");
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

  function renderSessionSelect() {
    var select = $("followup-session");
    if (!state.sessions.length) {
      select.innerHTML = "<option value=''>No sessions</option>";
      return;
    }
    select.innerHTML = state.sessions.map(function (s) {
      return "<option value='" + escapeHTML(s.id) + "'>" + escapeHTML(s.repo_name + " :: " + s.branch) + "</option>";
    }).join("");
  }

  function renderSessions() {
    var list = $("sessions-list");
    var active = state.sessions.filter(function (s) { return activeStates[s.status] || s.busy; });
    $("kpi-active").textContent = active.length + " active";
    $("kpi-total").textContent = state.sessions.length + " total";

    if (!state.sessions.length) {
      list.innerHTML = "<div class='session-card'>No sessions yet.</div>";
      return;
    }

    list.innerHTML = state.sessions.slice(0, 80).map(function (s) {
      var stateClass = "state";
      if (s.status === "FAILED") stateClass += " state-failed";
      else if (activeStates[s.status]) stateClass += " state-active";

      var prButton = "";
      if (s.pr_url) {
        prButton = "<button class='ghost open-pr-btn' data-url='" + escapeHTML(s.pr_url) + "' type='button'>Open PR</button>";
      }

      return "<article class='session-card'>" +
        "<h4>" + escapeHTML(s.repo_name) + " / " + escapeHTML(s.branch) + "</h4>" +
        "<div class='session-meta'>" +
          "<span class='" + stateClass + "'>" + escapeHTML(s.status) + (s.busy ? "*" : "") + "</span>" +
          "<span>tool: " + escapeHTML(s.tool) + "</span>" +
          "<span>updated: " + escapeHTML(formatDate(s.updated_at)) + "</span>" +
        "</div>" +
        "<div class='actions' style='margin-top:8px'>" + prButton + "</div>" +
      "</article>";
    }).join("");

    Array.prototype.slice.call(document.querySelectorAll(".open-pr-btn")).forEach(function (btn) {
      btn.addEventListener("click", function () {
        openExternal(btn.getAttribute("data-url"));
      });
    });
  }

  function renderRepos() {
    var repoSelect = $("new-repo");
    if (!state.repos.length) {
      repoSelect.innerHTML = "<option value=''>No repos imported</option>";
    } else {
      repoSelect.innerHTML = state.repos.map(function (r) {
        return "<option value='" + escapeHTML(r.name) + "'>" + escapeHTML(r.name) + "</option>";
      }).join("");
    }

    var managed = $("managed-list");
    if (!state.repos.length) {
      managed.innerHTML = "No managed repos.";
      return;
    }
    managed.innerHTML = state.repos.map(function (r) {
      return "<div>" +
        "<strong>" + escapeHTML(r.name) + "</strong>" +
        "<div style='color:#596575;font-size:12px'>" + escapeHTML(r.base_worktree_path || "-") + "</div>" +
      "</div>";
    }).join("");
  }

  function renderDiscoveredRepos() {
    var list = $("discover-list");
    if (!state.discoveredRepos.length) {
      list.innerHTML = "No discovered repos yet.";
      return;
    }
    list.innerHTML = state.discoveredRepos.map(function (r, idx) {
      return "<label class='repo-item'>" +
        "<input type='checkbox' id='repo-" + idx + "' data-repo='" + escapeHTML(r.full_name) + "'>" +
        "<span>" + escapeHTML(r.full_name) + "</span>" +
        "<span style='color:#596575;font-size:12px'>" + escapeHTML(r.default_branch || "-") + "</span>" +
      "</label>";
    }).join("");
  }

  function renderSettings() {
    var s = state.settings || {};
    var tools = s.available_tools || [];
    if (!tools.length && s.default_tool) tools = [s.default_tool];
    var options = tools.map(function (tool) {
      var selected = tool === s.default_tool ? " selected" : "";
      return "<option value='" + escapeHTML(tool) + "'" + selected + ">" + escapeHTML(tool) + "</option>";
    }).join("");
    $("settings-tool").innerHTML = options;

    var newTool = $("new-tool");
    newTool.innerHTML = "<option value=''>default</option>" + options;
    $("settings-prefix").value = s.branch_prefix || "fog";
    $("settings-pat-status").value = s.has_github_token ? "configured" : "missing";
  }

  function renderCloudStatus(cloud) {
    $("cloud-url").value = cloud.cloud_url || "";
    if (cloud.paired) {
      $("cloud-device").value = "paired (" + (cloud.device_id || "-") + ")";
    } else {
      $("cloud-device").value = "unpaired";
    }
  }

  async function loadAll() {
    $("daemon-badge").textContent = "fogd: connected";
    var result = await Promise.all([
      fetchJSON("/api/settings"),
      fetchJSON("/api/sessions"),
      fetchJSON("/api/repos"),
      fetchJSON("/api/cloud")
    ]);
    state.settings = result[0] || {};
    state.sessions = result[1] || [];
    state.repos = result[2] || [];
    renderSettings();
    renderSessions();
    renderSessionSelect();
    renderRepos();
    renderCloudStatus(result[3] || {});
  }

  async function onCreateSession(event) {
    event.preventDefault();
    var btn = $("new-submit");
    setStatus("new-status", "", "");
    btn.disabled = true;
    try {
      var payload = {
        repo: $("new-repo").value,
        prompt: $("new-prompt").value.trim(),
        model: $("new-model").value.trim(),
        branch_name: $("new-branch").value.trim(),
        autopr: $("new-autopr").value === "true",
        async: true
      };
      var tool = $("new-tool").value;
      if (tool) payload.tool = tool;
      if (!payload.repo) throw new Error("Repository is required");
      if (!payload.prompt) throw new Error("Prompt is required");

      var out = await fetchJSON("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      setStatus("new-status", "Queued session " + out.session_id, "ok");
      $("new-prompt").value = "";
      $("new-branch").value = "";
      await refreshSessions();
    } catch (err) {
      setStatus("new-status", "Create failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  async function onFollowup(event) {
    event.preventDefault();
    var btn = $("followup-submit");
    setStatus("followup-status", "", "");
    btn.disabled = true;
    try {
      var sessionID = $("followup-session").value;
      var prompt = $("followup-prompt").value.trim();
      if (!sessionID) throw new Error("Session is required");
      if (!prompt) throw new Error("Prompt is required");
      var out = await fetchJSON("/api/sessions/" + encodeURIComponent(sessionID) + "/runs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt: prompt, async: true })
      });
      setStatus("followup-status", "Queued run " + out.run_id, "ok");
      $("followup-prompt").value = "";
      await refreshSessions();
    } catch (err) {
      setStatus("followup-status", "Follow-up failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  async function onSaveSettings(event) {
    event.preventDefault();
    var btn = $("settings-submit");
    setStatus("settings-status", "", "");
    btn.disabled = true;
    try {
      await fetchJSON("/api/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          default_tool: $("settings-tool").value,
          branch_prefix: $("settings-prefix").value.trim()
        })
      });
      setStatus("settings-status", "Saved", "ok");
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
      setStatus("repos-status", "Imported " + out.imported.length + " repos", "ok");
      state.repos = await fetchJSON("/api/repos");
      renderRepos();
    } catch (err) {
      setStatus("repos-status", "Import failed: " + err.message, "error");
    } finally {
      btn.disabled = false;
    }
  }

  async function onSaveCloudURL() {
    setStatus("cloud-status", "", "");
    try {
      var url = $("cloud-url").value.trim();
      if (!url) throw new Error("Cloud URL is required");
      var cloud = await fetchJSON("/api/cloud", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ cloud_url: url })
      });
      renderCloudStatus(cloud);
      setStatus("cloud-status", "Cloud URL saved", "ok");
    } catch (err) {
      setStatus("cloud-status", "Save failed: " + err.message, "error");
    }
  }

  async function onPairCloud() {
    setStatus("cloud-status", "", "");
    try {
      var code = $("cloud-code").value.trim();
      if (!code) throw new Error("Pair code is required");
      var cloud = await fetchJSON("/api/cloud/pair", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code: code })
      });
      $("cloud-code").value = "";
      renderCloudStatus(cloud);
      setStatus("cloud-status", "Pairing successful", "ok");
    } catch (err) {
      setStatus("cloud-status", "Pairing failed: " + err.message, "error");
    }
  }

  async function onUnpairCloud() {
    setStatus("cloud-status", "", "");
    try {
      var team = $("cloud-team").value.trim();
      var user = $("cloud-user").value.trim();
      if (!team || !user) throw new Error("Team ID and Slack User ID are required");
      var cloud = await fetchJSON("/api/cloud/unpair", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ team_id: team, slack_user_id: user })
      });
      renderCloudStatus(cloud);
      setStatus("cloud-status", "Unpaired", "ok");
    } catch (err) {
      setStatus("cloud-status", "Unpair failed: " + err.message, "error");
    }
  }

  async function refreshSessions() {
    state.sessions = await fetchJSON("/api/sessions");
    renderSessions();
    renderSessionSelect();
  }

  async function bootstrap() {
    $("daemon-badge").textContent = "fogd: connecting";
    state.apiBaseURL = await resolveAPIBaseURL();
    var version = await resolveVersion();
    $("version-badge").textContent = "version: " + version;

    await loadAll();
    setInterval(function () {
      refreshSessions().catch(function (err) {
        setStatus("followup-status", "Refresh failed: " + err.message, "error");
      });
    }, 4000);
  }

  $("new-session-form").addEventListener("submit", onCreateSession);
  $("followup-form").addEventListener("submit", onFollowup);
  $("settings-form").addEventListener("submit", onSaveSettings);
  $("discover-btn").addEventListener("click", onDiscoverRepos);
  $("import-btn").addEventListener("click", onImportRepos);
  $("cloud-save").addEventListener("click", onSaveCloudURL);
  $("cloud-pair").addEventListener("click", onPairCloud);
  $("cloud-unpair").addEventListener("click", onUnpairCloud);

  bootstrap().catch(function (err) {
    $("daemon-badge").textContent = "fogd: unavailable";
    setStatus("new-status", "Initialization failed: " + err.message, "error");
  });
})();
