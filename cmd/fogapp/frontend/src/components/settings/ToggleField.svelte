<script lang="ts">
  // A label's `for` cannot target a <button role="switch"> — the previous
  // build did exactly that, so the text never associated with the control.
  // aria-labelledby / aria-describedby wire it up correctly.
  let {
    checked = $bindable(false),
    label,
    description,
    id,
  }: {
    checked?: boolean;
    label: string;
    description?: string;
    id: string;
  } = $props();
</script>

<div class="tf">
  <div class="tf__text">
    <p class="tf__label" id="{id}-label">{label}</p>
    {#if description}
      <p class="tf__desc" id="{id}-desc">{description}</p>
    {/if}
  </div>

  <button
    {id}
    type="button"
    role="switch"
    class="toggle"
    aria-checked={checked}
    aria-labelledby="{id}-label"
    aria-describedby={description ? `${id}-desc` : undefined}
    onclick={() => (checked = !checked)}
  >
    <span class="toggle__thumb"></span>
  </button>
</div>

<style>
  .tf {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    border-block-end: var(--rule-hair) solid var(--color-rule);
    min-inline-size: 0;
  }

  .tf:last-child {
    border-block-end: none;
  }

  .tf__text {
    min-inline-size: 0;
  }

  .tf__label {
    font-size: var(--text-base);
    color: var(--color-ink);
  }

  .tf__desc {
    margin-block-start: var(--space-3xs);
    font-size: var(--text-xs);
    line-height: var(--leading-body);
    color: var(--color-ink-3);
    max-inline-size: 60ch;
  }
</style>
