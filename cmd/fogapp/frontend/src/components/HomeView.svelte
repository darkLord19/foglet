<script lang="ts">
    import ChatBox from "./ChatBox.svelte";

    function onSessionCreated() {
        // Navigation is handled in ChatBox.
    }
</script>

<!-- The session lists live in the sidebar now, so this surface is just the
     composer: one input, one action. -->
<div class="home scroll-y">
    <div class="home__bar">
        <h1 class="home__title">New session</h1>
    </div>

    <div class="home__inner">
        <ChatBox {onSessionCreated} />
        <p class="hint">
            Starts a session immediately. To queue work instead, add it to the
            board.
        </p>
    </div>
</div>

<style>
    .home {
        flex: 1;
        min-block-size: 0;
        display: flex;
        flex-direction: column;
    }

    .home__bar {
        display: flex;
        align-items: center;
        block-size: var(--bar-h);
        flex: none;
        padding-inline: var(--gutter);
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .home__title {
        font-size: var(--text-md);
        font-weight: 600;
        letter-spacing: var(--tracking-tight);
    }

    .home__inner {
        display: flex;
        flex-direction: column;
        gap: var(--space-sm);
        inline-size: 100%;
        max-inline-size: 46rem;
        padding: var(--space-xl) var(--gutter);
    }
</style>
