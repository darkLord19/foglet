<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Toaster } from "svelte-sonner";
  import { appState } from "$lib/stores.svelte";
  import TopNav from "./components/TopNav.svelte";
  import HomeView from "./components/HomeView.svelte";
  import SessionDetail from "./components/SessionDetail.svelte";
  import SettingsView from "./components/Settings.svelte";
  import { fly, fade } from "svelte/transition";

  let initError = $state("");

  onMount(async () => {
    try {
      await appState.bootstrap();
    } catch (err) {
      initError = err instanceof Error ? err.message : "Initialization failed";
    }
  });

  onDestroy(() => {
    appState.destroy();
  });
</script>

<Toaster
  position="bottom-right"
  richColors
  theme="dark"
  toastOptions={{
    style:
      "background: var(--color-bg-elevated); border: 1px solid var(--color-border); color: var(--color-text); font-family: var(--font-sans); box-shadow: var(--shadow-md);",
  }}
/>

<div class="app-shell">
  <!-- Top Navigation (always visible) -->
  <TopNav />

  <!-- Main Content Area -->
  <main class="main-content">
    {#if initError}
      <div class="center-stage" in:fly={{ y: 20, duration: 400 }}>
        <div class="error-card card">
          <div class="error-icon">⚠️</div>
          <h2>Connection Failed</h2>
          <p class="error-msg">{initError}</p>
          <p class="hint">Ensure the Fog daemon is running on port 8080</p>
          <button class="btn btn-secondary" onclick={() => location.reload()}>
            Retry
          </button>
        </div>
      </div>
    {:else if appState.currentView === "new"}
      <HomeView />
    {:else if appState.currentView === "detail"}
      <div class="view-container" in:fly={{ y: 10, duration: 300, delay: 100 }}>
        <SessionDetail />
      </div>
    {:else if appState.currentView === "settings"}
      <div class="view-container">
        <SettingsView />
      </div>
    {/if}
  </main>
</div>

<style>
  .app-shell {
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: hidden;
    background: var(--color-bg);
  }

  .main-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    position: relative;
    overflow: hidden; /* Fix scroll constraint */
  }

  /* Adjust view-container to respect fixed header */
  .view-container {
    height: 100%;
    width: 100%;
    overflow-y: auto;
    overflow-x: hidden;
    padding-top: 56px; /* Match header height */
  }

  .center-stage {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    width: 100%;
    padding: 20px;
  }

  .error-card {
    padding: 40px;
    text-align: center;
    max-width: 400px;
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
  }

  .error-icon {
    font-size: 48px;
    margin-bottom: 8px;
  }

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--color-danger);
  }

  .error-msg {
    font-size: 14px;
    color: var(--color-text);
  }

  .hint {
    font-size: 13px;
    color: var(--color-text-muted);
  }
</style>
