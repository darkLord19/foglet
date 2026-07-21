<script lang="ts">
  // Shared disclosure header. TaskList and SessionHistory each carried their
  // own near-identical copy of this; it lives once now.
  import { ChevronDown } from "@lucide/svelte";
  import type { Snippet } from "svelte";

  let {
    title,
    count,
    open = $bindable(true),
    children,
  }: {
    title: string;
    count?: number;
    open?: boolean;
    children: Snippet;
  } = $props();

  const id = $props.id();
</script>

<section class="sec">
  <h2 class="sec__heading">
    <button
      type="button"
      class="sec__toggle"
      aria-expanded={open}
      aria-controls={id}
      onclick={() => (open = !open)}
    >
      <ChevronDown
        size={14}
        class={open ? "sec__chev is-open" : "sec__chev"}
      />
      <span class="sec__title">{title}</span>
      {#if count !== undefined}
        <span class="sec__count mono">{count}</span>
      {/if}
    </button>
  </h2>

  <div {id} class="sec__body" hidden={!open}>
    {@render children()}
  </div>
</section>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    min-inline-size: 0;
  }

  .sec__heading {
    margin: 0;
    font: inherit;
  }

  .sec__toggle {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    inline-size: 100%;
    padding: var(--space-xs) 0;
    background: none;
    border: none;
    border-block-end: var(--rule-hair) solid var(--color-rule);
    color: var(--color-ink-2);
    text-align: start;
    cursor: pointer;
    transition: color var(--dur-micro) var(--ease-out);
  }

  .sec__toggle:hover {
    color: var(--color-ink);
  }

  .sec__toggle:focus-visible {
    outline: var(--rule-hair) solid var(--color-focus);
    outline-offset: var(--rule-hair);
  }

  :global(.sec__chev) {
    flex: none;
    color: var(--color-ink-3);
    transition: transform var(--dur-micro) var(--ease-out);
    transform: rotate(-90deg);
  }

  :global(.sec__chev.is-open) {
    transform: rotate(0deg);
  }

  .sec__title {
    flex: 1;
    font-size: var(--text-xs);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: var(--tracking-label);
    line-height: var(--leading-caps);
  }

  .sec__count {
    font-size: var(--text-2xs);
    color: var(--color-ink-3);
    font-variant-numeric: tabular-nums;
  }

  .sec__body {
    padding-block-start: var(--space-sm);
  }

  .sec__body[hidden] {
    display: none;
  }
</style>
