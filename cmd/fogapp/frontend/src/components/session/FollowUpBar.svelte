<script lang="ts">
    import { ArrowUp } from "@lucide/svelte";

    let {
        value = $bindable(""),
        submitting = false,
        onsubmit,
    }: {
        value?: string;
        submitting?: boolean;
        onsubmit: () => void;
    } = $props();

    let canSend = $derived(!!value.trim() && !submitting);

    function handleKeydown(e: KeyboardEvent) {
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            if (canSend) onsubmit();
        }
    }
</script>

<div class="followup">
    <div class="followup__box">
        <label class="sr-only" for="followup-prompt">Follow-up instruction</label>
        <textarea
            id="followup-prompt"
            class="followup__input"
            bind:value
            onkeydown={handleKeydown}
            disabled={submitting}
            placeholder="Add a follow-up instruction…"
            rows="1"
        ></textarea>

        <button
            id="followup-submit"
            class="btn btn-primary btn-icon"
            data-state={submitting ? "loading" : undefined}
            disabled={!canSend}
            onclick={onsubmit}
            aria-label="Send follow-up"
        >
            <ArrowUp size={16} />
        </button>
    </div>

    <p class="followup__hint">
        <kbd class="kbd">↵</kbd> send
        <span aria-hidden="true">·</span>
        <kbd class="kbd">⇧</kbd><kbd class="kbd">↵</kbd> new line
    </p>
</div>

<style>
    .followup {
        display: flex;
        flex-direction: column;
        gap: var(--space-2xs);
        padding: var(--space-sm) var(--gutter) var(--space-md);
        background: var(--color-paper);
        border-block-start: var(--rule-hair) solid var(--color-rule);
    }

    .followup__box {
        display: flex;
        align-items: flex-end;
        gap: var(--space-xs);
        inline-size: 100%;
        max-inline-size: 70rem;
        padding: var(--space-2xs) var(--space-2xs) var(--space-2xs) var(--space-sm);
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule-2);
        transition: border-color var(--dur-micro) var(--ease-out);
    }

    .followup__box:focus-within {
        border-color: var(--color-accent);
    }

    .followup__input {
        flex: 1;
        min-inline-size: 0;
        max-block-size: 9rem;
        padding-block: var(--space-xs);
        background: transparent;
        border: none;
        outline: none;
        resize: none;
        color: var(--color-ink);
        font-family: var(--font-body);
        font-size: var(--text-base);
        line-height: var(--leading-body);
    }

    .followup__input::placeholder {
        color: var(--color-ink-3);
    }

    .followup__hint {
        display: flex;
        align-items: center;
        gap: var(--space-3xs);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }

    .sr-only {
        position: absolute;
        inline-size: 1px;
        block-size: 1px;
        overflow: hidden;
        clip-path: inset(50%);
        white-space: nowrap;
    }
</style>
