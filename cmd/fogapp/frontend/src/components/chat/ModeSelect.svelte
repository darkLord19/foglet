<script lang="ts">
  import { Bot, Hammer, ChevronDown, Check } from "@lucide/svelte";

  let { mode = $bindable<"plan" | "build">("build") } = $props();

  let open = $state(false);
  let root = $state<HTMLDivElement | null>(null);

  const MODES = [
    {
      id: "plan" as const,
      icon: Bot,
      label: "Interactive plan",
      desc: "Agree the goal before any edits land",
    },
    {
      id: "build" as const,
      icon: Hammer,
      label: "Build",
      desc: "Start work immediately",
    },
  ];

  let current = $derived(MODES.find((m) => m.id === mode) ?? MODES[1]);

  function choose(id: "plan" | "build") {
    mode = id;
    open = false;
  }

  // Dismiss on outside click and on Escape — a dropdown that only closes by
  // reselecting is a trap for keyboard users.
  $effect(() => {
    if (!open) return;

    function onPointerDown(e: PointerEvent) {
      if (root && !root.contains(e.target as Node)) open = false;
    }
    function onKeydown(e: KeyboardEvent) {
      if (e.key === "Escape") open = false;
    }

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeydown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeydown);
    };
  });
</script>

<div class="mode" bind:this={root}>
  <button
    type="button"
    class="mode__trigger"
    aria-haspopup="listbox"
    aria-expanded={open}
    onclick={() => (open = !open)}
  >
    <current.icon size={14} />
    <span class="mode__label">{current.label}</span>
    <ChevronDown size={12} class={open ? "mode__chev is-open" : "mode__chev"} />
  </button>

  {#if open}
    <ul class="mode__menu" role="listbox" aria-label="Run mode">
      {#each MODES as m (m.id)}
        <li role="none">
          <button
            type="button"
            role="option"
            aria-selected={mode === m.id}
            class="mode__opt"
            class:is-selected={mode === m.id}
            onclick={() => choose(m.id)}
          >
            <m.icon size={14} />
            <span class="mode__opt-text">
              <span class="mode__opt-label">{m.label}</span>
              <span class="mode__opt-desc">{m.desc}</span>
            </span>
            {#if mode === m.id}<Check size={13} />{/if}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .mode {
    position: relative;
  }

  .mode__trigger {
    display: flex;
    align-items: center;
    gap: var(--space-2xs);
    padding: var(--space-2xs) var(--space-xs);
    background: transparent;
    border: var(--rule-hair) solid var(--color-rule-2);
    border-radius: var(--radius);
    color: var(--color-ink);
    font-size: var(--text-sm);
    white-space: nowrap;
    cursor: pointer;
    transition: border-color var(--dur-micro) var(--ease-out);
  }

  .mode__trigger:hover {
    background: var(--color-paper-3);
  }

  .mode__trigger:focus-visible {
    outline: var(--rule-hair) solid var(--color-focus);
    outline-offset: var(--rule-hair);
  }

  .mode__label {
    font-weight: 500;
  }

  :global(.mode__chev) {
    transition: transform var(--dur-micro) var(--ease-out);
  }

  :global(.mode__chev.is-open) {
    transform: rotate(180deg);
  }

  .mode__menu {
    position: absolute;
    inset-block-end: calc(100% + var(--space-2xs));
    inset-inline-end: 0;
    z-index: var(--z-dropdown);
    inline-size: 15rem;
    margin: 0;
    padding: 0;
    list-style: none;
    background: var(--color-paper-2);
    border: var(--rule-hair) solid var(--color-rule-2);
  }

  .mode__opt {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    inline-size: 100%;
    padding: var(--space-xs) var(--space-sm);
    background: transparent;
    border: none;
    border-inline-start: var(--rule-active) solid transparent;
    color: var(--color-ink-2);
    text-align: start;
    cursor: pointer;
    transition:
      background-color var(--dur-micro) var(--ease-out),
      border-color var(--dur-micro) var(--ease-out);
  }

  /* The options are wrapped in <li>, so the adjacency lives on the list
     item rather than on the button itself. */
  li + li .mode__opt {
    border-block-start: var(--rule-hair) solid var(--color-rule);
  }

  .mode__opt:hover {
    background: var(--color-paper-3);
    color: var(--color-ink);
  }

  .mode__opt:focus-visible {
    outline: var(--rule-hair) solid var(--color-focus);
    outline-offset: calc(var(--rule-hair) * -1);
  }

  .mode__opt.is-selected {
    border-inline-start-color: var(--color-accent);
    color: var(--color-accent);
  }

  .mode__opt-text {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-inline-size: 0;
  }

  .mode__opt-label {
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .mode__opt-desc {
    font-size: var(--text-2xs);
    line-height: var(--leading-tight);
    color: var(--color-ink-3);
  }
</style>
