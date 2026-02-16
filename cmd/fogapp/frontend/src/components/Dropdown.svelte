<script lang="ts">
    import { slide } from "svelte/transition";
    import { ChevronDown, Check } from "@lucide/svelte";
    import { clickOutside } from "$lib/actions";
    import type { Snippet } from "svelte";

    // Flexible option type
    type OptionValue = string | number;
    type OptionObj = { value: OptionValue; label: string };
    type Option = OptionValue | OptionObj;

    let {
        value = $bindable(),
        options = [],
        placeholder = "Select...",
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

    function toggle() {
        if (!disabled) open = !open;
    }

    function close() {
        open = false;
    }

    function select(opt: Option) {
        if (disabled) return;
        value = typeof opt === "object" ? opt.value : opt;
        close();
    }

    // Helper to get label to display
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
</script>

<div class="dropdown-container {className}" use:clickOutside={close}>
    <button
        class="dropdown-trigger variant-{variant} {open ? 'open' : ''}"
        onclick={toggle}
        {disabled}
        type="button"
    >
        {#if icon}
            {@render icon()}
        {/if}

        <div class="selected-text-wrapper">
            <span class="selected-text">{selectedLabel}</span>
        </div>

        <ChevronDown size={14} class="chevron {open ? 'rotate' : ''}" />
    </button>

    {#if open}
        <div class="dropdown-menu-v2">
            <div class="dropdown-scroll">
                {#each options as opt}
                    {@const isSelected = getValue(opt) === value}
                    <button
                        class="dropdown-item {isSelected ? 'selected' : ''}"
                        onclick={() => select(opt)}
                        type="button"
                    >
                        {#if labelSnippet}
                            {@render labelSnippet(opt)}
                        {:else}
                            <span class="item-label">{getLabel(opt)}</span>
                        {/if}

                        {#if isSelected}
                            <Check size={14} class="check-icon" />
                        {/if}
                    </button>
                {/each}
            </div>
        </div>
    {/if}
</div>

<style>
    .dropdown-container {
        position: relative;
        display: inline-block;
        min-width: 140px;
    }

    .dropdown-trigger {
        width: 100%;
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 6px 10px;
        background: rgba(255, 255, 255, 0.03);
        border: 1px solid var(--color-border);
        border-radius: 8px;
        color: var(--color-text);
        font-family: inherit;
        font-size: 13px;
        cursor: pointer;
        transition: all 0.2s;
        text-align: left;
    }

    .dropdown-trigger:hover:not(:disabled) {
        background: var(--color-bg-hover);
        border-color: var(--color-border-strong);
    }

    .dropdown-trigger.variant-ghost {
        background: transparent;
        border-color: transparent;
    }

    .dropdown-trigger.variant-ghost:hover:not(:disabled) {
        background: var(--color-bg-hover);
    }

    .dropdown-trigger.open {
        background: var(--color-bg-active);
        border-color: var(--color-text-secondary);
    }

    .dropdown-trigger.variant-ghost.open {
        background: transparent;
        border-color: transparent;
        color: var(--color-text); /* Keep text normal */
    }

    .dropdown-trigger:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .selected-text-wrapper {
        flex: 1;
        overflow: hidden;
    }

    .selected-text {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        display: block;
    }

    .chevron {
        opacity: 0.5;
        transition: transform 0.2s;
        flex-shrink: 0;
    }

    .chevron.rotate {
        transform: rotate(180deg);
    }

    /* Menu */
    .dropdown-menu-v2 {
        position: absolute;
        top: calc(100% + 4px);
        left: 0;
        min-width: 100%; /* Match trigger width */
        width: max-content; /* Or allow it to be wider */
        max-width: 300px;
        z-index: 9999;
        background-color: #09090b !important; /* Force opaque dark */
        opacity: 1 !important;
        backdrop-filter: none !important; /* Ensure no blur/glass */
        border: 1px solid var(--color-border);
        border-radius: 8px;
        padding: 4px;
        box-shadow: var(--shadow-md);
        overflow: hidden;
    }

    .dropdown-scroll {
        max-height: 240px;
        overflow-y: auto;
        /* Scrollbar styling using global keywords if available, else local */
    }

    .dropdown-scroll::-webkit-scrollbar {
        width: 4px;
    }

    .dropdown-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        padding: 8px 12px;
        background: transparent;
        border: none;
        border-radius: 6px;
        color: var(--color-text-secondary);
        font-family: inherit;
        font-size: 13px;
        cursor: pointer;
        transition: all 0.1s;
        text-align: left;
        gap: 8px;
    }

    .dropdown-item:hover {
        background: var(--color-bg-hover);
        color: var(--color-text);
    }

    .dropdown-item.selected {
        background: rgba(250, 204, 21, 0.1);
        color: var(--color-accent);
        border: 1px solid rgba(250, 204, 21, 0.2);
        font-weight: 500;
    }

    .item-label {
        flex: 1;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .check-icon {
        color: var(--color-accent);
    }
</style>
