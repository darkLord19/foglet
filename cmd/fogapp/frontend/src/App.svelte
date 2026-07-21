<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Toaster } from "svelte-sonner";
  import { appState } from "$lib/stores.svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import BoardView from "./components/BoardView.svelte";
  import HomeView from "./components/HomeView.svelte";
  import SessionDetail from "./components/SessionDetail.svelte";
  import SettingsView from "./components/Settings.svelte";
  import Onboarding from "./components/Onboarding.svelte";

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

<!-- Toasts carry failures and off-screen events only; a successful action
     changes the UI, which is its own feedback. -->
<Toaster
  position="bottom-right"
  theme="dark"
  toastOptions={{
    style:
      "background: var(--color-paper-3); border: 1px solid var(--color-rule-2); border-radius: 6px; color: var(--color-ink); font-family: var(--font-body); font-size: var(--text-sm);",
  }}
/>

{#if initError}
  <div class="stage">
    <div class="fault">
      <p class="fault__label">Can&rsquo;t reach the daemon</p>
      <p class="fault__msg mono">{initError}</p>
      <p class="hint">Fog expects <code>fogd</code> on port 8080.</p>
      <button class="btn btn-primary" onclick={() => location.reload()}>
        Retry
      </button>
    </div>
  </div>
{:else if appState.settings?.onboarding_required}
  <Onboarding />
{:else}
  <div class="shell">
    <Sidebar />
    <main class="shell__main">
      {#if appState.currentView === "board"}
        <BoardView />
      {:else if appState.currentView === "new"}
        <HomeView />
      {:else if appState.currentView === "detail"}
        <SessionDetail />
      {:else if appState.currentView === "settings"}
        <SettingsView />
      {/if}
    </main>
  </div>
{/if}

<style>
  .shell {
    display: flex;
    block-size: 100dvh;
    overflow: hidden;
    background: var(--color-paper);
  }

  .shell__main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-inline-size: 0;
    min-block-size: 0;
    overflow: hidden;
  }

  .stage {
    display: grid;
    place-items: center;
    block-size: 100dvh;
    padding: var(--gutter);
  }

  .fault {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-sm);
    max-inline-size: 30rem;
    padding: var(--space-lg);
    background: var(--color-paper-2);
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius-lg);
  }

  .fault__label {
    font-size: var(--text-md);
    font-weight: 600;
    color: var(--color-ink);
  }

  .fault__msg {
    inline-size: 100%;
    padding: var(--space-xs) var(--space-sm);
    background: var(--color-paper);
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius);
    font-size: var(--text-xs);
    color: var(--color-ink-2);
    overflow-wrap: anywhere;
  }
</style>
