<script lang="ts">
    import { ChevronDown, Check } from "@lucide/svelte";
    import { clickOutside } from "$lib/actions";
    import type { Snippet } from "svelte";

    type OptionValue = string | number;
    type OptionObj = { value: OptionValue; label: string };
    type Option = OptionValue | OptionObj;

    let {
        value = $bindable(),
        options = [],
        placeholder = "Select…",
        icon,
        disabled = false,
        class: className = "",
        labelSnippet,
        variant = "default",
    }: {
        value: OptionValue;
        options: Option[];
        placeholder?: string;
        icon?: Snippet;
        disabled?: boolean;
        class?: string;
        labelSnippet?: Snippet<[Option]>;
        variant?: "default" | "ghost";
    } = $props();

    let open = $state(false);
    let activeIndex = $state(-1);
    let listEl = $state<HTMLDivElement | null>(null);

    function toggle() {
        if (disabled) return;
        open = !open;
        if (open) activeIndex = options.findIndex((o) => getValue(o) === value);
    }

    function close() {
        open = false;
        activeIndex = -1;
    }

    function select(opt: Option) {
        if (disabled) return;
        value = typeof opt === "object" ? opt.value : opt;
        close();
    }

    function getLabel(opt: Option): string {
        return typeof opt === "object" ? opt.label : String(opt);
    }

    function getValue(opt: Option): OptionValue {
        return typeof opt === "object" ? opt.value : opt;
    }

    let selectedLabel = $derived.by(() => {
        const found = options.find((o) => getValue(o) === value);
        if (found) return getLabel(found);
        if (!value) return placeholder;
        return String(value);
    });

    // Full keyboard path: the previous build was pointer-only.
    function onKeydown(e: KeyboardEvent) {
        if (disabled) return;

        if (!open) {
            if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                toggle();
            }
            return;
        }

        switch (e.key) {
            case "Escape":
                e.preventDefault();
                close();
                break;
            case "ArrowDown":
                e.preventDefault();
                activeIndex = Math.min(activeIndex + 1, options.length - 1);
                scrollActiveIntoView();
                break;
            case "ArrowUp":
                e.preventDefault();
                activeIndex = Math.max(activeIndex - 1, 0);
                scrollActiveIntoView();
                break;
            case "Home":
                e.preventDefault();
                activeIndex = 0;
                scrollActiveIntoView();
                break;
            case "End":
                e.preventDefault();
                activeIndex = options.length - 1;
                scrollActiveIntoView();
                break;
            case "Enter":
            case " ":
                e.preventDefault();
                if (activeIndex >= 0) select(options[activeIndex]);
                break;
            case "Tab":
                close();
                break;
        }
    }

    function scrollActiveIntoView() {
        // preventScroll equivalent: move only the list, never the page.
        queueMicrotask(() => {
            const el = listEl?.children[activeIndex] as HTMLElement | undefined;
            el?.scrollIntoView({ block: "nearest" });
        });
    }
</script>

<div class="dd {className}" use:clickOutside={close}>
    <button
        type="button"
        class="dd__trigger"
        class:is-ghost={variant === "ghost"}
        class:is-open={open}
        aria-haspopup="listbox"
        aria-expanded={open}
        {disabled}
        onclick={toggle}
        onkeydown={onKeydown}
    >
        {#if icon}
            <span class="dd__icon" aria-hidden="true">{@render icon()}</span>
        {/if}

        <span class="dd__value truncate" class:is-placeholder={!value}>
            {selectedLabel}
        </span>

        <ChevronDown size={13} class={open ? "dd__chev is-open" : "dd__chev"} />
    </button>

    {#if open}
        <div class="dd__menu">
            <div class="dd__list" role="listbox" bind:this={listEl}>
                {#each options as opt, i (getValue(opt))}
                    {@const isSelected = getValue(opt) === value}
                    <button
                        type="button"
                        role="option"
                        aria-selected={isSelected}
                        class="dd__opt"
                        class:is-selected={isSelected}
                        class:is-active={i === activeIndex}
                        onclick={() => select(opt)}
                        onmouseenter={() => (activeIndex = i)}
                    >
                        {#if labelSnippet}
                            {@render labelSnippet(opt)}
                        {:else}
                            <span class="truncate">{getLabel(opt)}</span>
                        {/if}

                        {#if isSelected}
                            <Check size={13} />
                        {/if}
                    </button>
                {/each}

                {#if options.length === 0}
                    <p class="dd__empty">Nothing to choose from</p>
                {/if}
            </div>
        </div>
    {/if}
</div>

<style>
    .dd {
        position: relative;
        display: inline-block;
        min-inline-size: 8rem;
        max-inline-size: 100%;
    }

    .dd__trigger {
        inline-size: 100%;
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        padding: var(--space-2xs) var(--space-xs);
        background: var(--color-paper);
        border: var(--rule-hair) solid var(--color-rule-2);
        border-radius: var(--radius);
        color: var(--color-ink);
        font-family: var(--font-body);
        font-size: var(--text-sm);
        text-align: start;
        cursor: pointer;
        min-inline-size: 0;
        transition:
            border-color var(--dur-micro) var(--ease-out),
            background-color var(--dur-micro) var(--ease-out);
    }

    .dd__trigger:hover:not(:disabled) {
        background: var(--color-paper-3);
        border-color: var(--color-rule-2);
    }

    .dd__trigger:focus-visible {
        outline: var(--rule-hair) solid var(--color-focus);
        outline-offset: var(--rule-hair);
    }

    .dd__trigger.is-open {
        border-color: var(--color-accent);
    }

    .dd__trigger.is-ghost {
        background: transparent;
        border-color: transparent;
    }

    .dd__trigger.is-ghost:hover:not(:disabled) {
        background: var(--color-paper-3);
    }

    .dd__trigger:disabled {
        opacity: 0.45;
        cursor: not-allowed;
    }

    .dd__icon {
        display: flex;
        flex: none;
        color: var(--color-ink-3);
    }

    .dd__value {
        flex: 1;
    }

    .dd__value.is-placeholder {
        color: var(--color-ink-3);
    }

    :global(.dd__chev) {
        flex: none;
        color: var(--color-ink-3);
        transition: transform var(--dur-micro) var(--ease-out);
    }

    :global(.dd__chev.is-open) {
        transform: rotate(180deg);
    }

    .dd__menu {
        position: absolute;
        inset-block-start: calc(100% + var(--space-3xs));
        inset-inline-start: 0;
        min-inline-size: 100%;
        inline-size: max-content;
        max-inline-size: min(20rem, 60vw);
        z-index: var(--z-dropdown);
        background: var(--color-paper-2);
        border: var(--rule-hair) solid var(--color-rule-2);
        border-radius: var(--radius);
    }

    .dd__list {
        max-block-size: 15rem;
        overflow-y: auto;
        overscroll-behavior: contain;
    }

    .dd__opt {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-xs);
        inline-size: 100%;
        padding: var(--space-2xs) var(--space-sm);
        background: transparent;
        border: none;
        border-inline-start: var(--rule-active) solid transparent;
        color: var(--color-ink-2);
        font-family: var(--font-body);
        font-size: var(--text-sm);
        text-align: start;
        cursor: pointer;
        min-inline-size: 0;
    }

    .dd__opt + .dd__opt {
        border-block-start: var(--rule-hair) solid var(--color-rule);
    }

    /* Pointer hover and keyboard focus share one visual state, so the
       highlight never desyncs between the two input paths. */
    .dd__opt.is-active {
        background: var(--color-paper-3);
        color: var(--color-ink);
    }

    .dd__opt.is-selected {
        border-inline-start-color: var(--color-accent);
        color: var(--color-accent);
    }

    .dd__empty {
        padding: var(--space-xs) var(--space-sm);
        font-size: var(--text-xs);
        color: var(--color-ink-3);
    }
</style>
