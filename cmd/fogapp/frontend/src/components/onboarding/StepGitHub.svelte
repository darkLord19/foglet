<script lang="ts">
    import type { GhStatus } from "$lib/types";
    import { RefreshCw } from "@lucide/svelte";

    let {
        ghStatus,
        checking,
        onrefresh,
    }: {
        ghStatus: GhStatus | null;
        checking: boolean;
        onrefresh: () => void;
    } = $props();

    let installCmd = $derived(
        ghStatus?.os === "darwin" ? "brew install gh" : "sudo apt install gh",
    );
</script>

<div class="step">
    <div class="section-head">
        <h2 class="section-head__title">Connect GitHub</h2>
        <p class="section-head__sub">
            Fog uses the <code>gh</code> CLI for authentication and repository access.
        </p>
    </div>

    {#if checking && !ghStatus}
        <p class="step__checking">
            <span class="spinner" aria-hidden="true"></span>
            Checking…
        </p>
    {:else if ghStatus}
        <div class="panel">
            <div class="check">
                <span class="check__label">CLI installed</span>
                <span class="badge badge--{ghStatus.installed ? 'done' : 'failed'}">
                    <span class="badge__dot" aria-hidden="true"></span>
                    {ghStatus.installed ? "Yes" : "Not found"}
                </span>
            </div>

            {#if ghStatus.installed}
                <div class="check">
                    <span class="check__label">Signed in</span>
                    <span
                        class="badge badge--{ghStatus.authenticated
                            ? 'done'
                            : 'warn'}"
                    >
                        <span class="badge__dot" aria-hidden="true"></span>
                        {ghStatus.authenticated ? "Yes" : "Not yet"}
                    </span>
                </div>
            {/if}
        </div>

        {#if !ghStatus.installed}
            <div class="cmd">
                <p class="cmd__label">Install it, then check again</p>
                <code class="cmd__code">{installCmd}</code>
            </div>
        {:else if !ghStatus.authenticated}
            <div class="cmd">
                <p class="cmd__label">Run this in a terminal, then check again</p>
                <code class="cmd__code">gh auth login</code>
            </div>
        {/if}

        <button
            class="btn btn-secondary step__refresh"
            onclick={onrefresh}
            data-state={checking ? "loading" : undefined}
            disabled={checking}
        >
            <RefreshCw size={14} />
            <span>Check again</span>
        </button>
    {:else}
        <div class="empty">
            <p class="empty__title">Status unknown</p>
            <p>Fog couldn&rsquo;t reach the GitHub CLI.</p>
        </div>
        <button class="btn btn-secondary step__refresh" onclick={onrefresh}>
            <RefreshCw size={14} />
            <span>Retry</span>
        </button>
    {/if}
</div>

<style>
    .step {
        display: flex;
        flex-direction: column;
        gap: var(--space-md);
        min-inline-size: 0;
    }

    .step__checking {
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        font-size: var(--text-sm);
        color: var(--color-ink-3);
    }

    .check {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-md);
        padding: var(--space-sm) var(--space-md);
        border-block-end: var(--rule-hair) solid var(--color-rule);
    }

    .check:last-child {
        border-block-end: none;
    }

    .check__label {
        font-size: var(--text-sm);
        color: var(--color-ink);
    }

    /* A real, copyable command — not a fake terminal window. */
    .cmd {
        display: flex;
        flex-direction: column;
        gap: var(--space-2xs);
        padding: var(--space-sm) var(--space-md);
        background: var(--color-paper);
        border-inline-start: var(--rule-active) solid var(--color-accent);
    }

    .cmd__label {
        font-size: var(--text-2xs);
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: var(--tracking-label);
        color: var(--color-ink-3);
    }

    .cmd__code {
        font-family: var(--font-mono);
        font-size: var(--text-sm);
        color: var(--color-accent);
        user-select: all;
        overflow-wrap: anywhere;
    }

    .step__refresh {
        align-self: flex-start;
    }
</style>
