<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import ChatBox from "./ChatBox.svelte";
    import TaskList from "./TaskList.svelte";
    import SessionHistory from "./SessionHistory.svelte";
    import { fade } from "svelte/transition";

    function onSessionCreated() {
        // maybe show a toast or auto-nav (handled in ChatBox)
    }
</script>

<div class="home-view" in:fade={{ duration: 300 }}>
    <div class="content-wrapper centered">
        <!-- Hero / Chat Section -->
        <div class="hero-section">
            <ChatBox {onSessionCreated} />
        </div>

        <!-- Active Tasks -->
        <TaskList />

        <!-- History -->
        <SessionHistory />
    </div>
</div>

<style>
    .home-view {
        width: 100%;
        height: 100%;
        overflow-y: auto;
        padding-top: 80px; /* Space for TopNav + spacing */
        padding-bottom: 40px;
    }

    .content-wrapper {
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 0 24px;
        max-width: 900px; /* Wider container for centering */
        margin: 0 auto;
    }

    .hero-section {
        width: 100%;
        display: flex;
        justify-content: center;
        margin-bottom: 20px;
    }

    /* Custom scrollbar for webkit */
    .home-view::-webkit-scrollbar {
        width: 6px;
    }
    .home-view::-webkit-scrollbar-track {
        background: transparent;
    }
    .home-view::-webkit-scrollbar-thumb {
        background: var(--color-border);
        border-radius: 99px;
    }
</style>
