<script lang="ts">
  import { appState } from "$lib/stores.svelte";
  import { Sparkles, Settings, Activity } from "@lucide/svelte";

  function showHome() {
    appState.setView("new");
    appState.selectedSessionID = "";
  }

  function showSettings() {
    appState.setView("settings");
    appState.selectedSessionID = "";
  }
</script>

<nav class="top-nav glass">
  <div class="nav-left">
    <button class="brand-btn" onclick={showHome}>
      <div class="logo-orb">
        <Sparkles size={16} />
      </div>
      <span class="brand-name">Fog</span>
    </button>
  </div>

  <div class="nav-right">
    <div class="status-pill" title={appState.daemonStatus}>
      <div
        class="status-dot"
        class:connected={appState.daemonStatus === "connected"}
      ></div>
      <span class="status-text">{appState.daemonStatus}</span>
    </div>

    <div class="divider"></div>

    <button
      id="nav-settings"
      class="nav-icon-btn {appState.currentView === 'settings' ? 'active' : ''}"
      onclick={showSettings}
      title="Settings"
    >
      <Settings size={18} />
    </button>
  </div>
</nav>

<style>
  .top-nav {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    height: 56px;
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 24px;
    border-bottom: 1px solid var(--color-border);
    /* Glass effect inherited from global .glass */
  }

  .nav-left {
    display: flex;
    align-items: center;
  }

  .brand-btn {
    display: flex;
    align-items: center;
    gap: 10px;
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    border-radius: 8px;
    transition: background 0.2s;
  }

  .brand-btn:hover {
    background: var(--color-bg-hover);
  }

  .logo-orb {
    width: 28px;
    height: 28px;
    background: var(--color-accent-gradient);
    color: white;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 2px 8px rgba(59, 130, 246, 0.4);
  }

  .brand-name {
    font-size: 16px;
    font-weight: 700;
    letter-spacing: -0.01em;
    color: var(--color-text);
  }

  .nav-right {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .status-pill {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    background: rgba(255, 255, 255, 0.03);
    border-radius: 99px;
    border: 1px solid var(--color-border);
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--color-text-muted);
  }

  .status-dot.connected {
    background: var(--color-success);
    box-shadow: 0 0 6px var(--color-success);
  }

  .status-text {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--color-text-muted);
    letter-spacing: 0.02em;
  }

  .divider {
    width: 1px;
    height: 20px;
    background: var(--color-border);
  }

  .nav-icon-btn {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: all 0.2s;
  }

  .nav-icon-btn:hover {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .nav-icon-btn.active {
    background: var(--color-bg-active);
    color: var(--color-accent);
  }
</style>
