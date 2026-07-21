<script lang="ts">
  // Native <dialog> rather than a hand-rolled backdrop div: it gives focus
  // trapping, Escape-to-close, and inert background for free. The previous
  // implementation used a click-only <div> backdrop with no keyboard path.
  let {
    open = $bindable(false),
    branchName = $bindable(""),
    title = $bindable(""),
  }: {
    open?: boolean;
    branchName?: string;
    title?: string;
  } = $props();

  let dialog = $state<HTMLDialogElement | null>(null);
  let draftBranch = $state("");
  let draftTitle = $state("");

  $effect(() => {
    if (!dialog) return;
    if (open && !dialog.open) {
      draftBranch = branchName;
      draftTitle = title;
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  });

  function commit(event: Event) {
    event.preventDefault();
    branchName = draftBranch;
    title = draftTitle;
    open = false;
  }
</script>

<dialog bind:this={dialog} class="dlg" onclose={() => (open = false)}>
  <form class="dlg__form" onsubmit={commit}>
    <h2 class="dlg__title">Pull request</h2>

    <div class="field">
      <label class="label" for="pr-branch">Branch name</label>
      <input
        id="pr-branch"
        class="input input-mono"
        type="text"
        bind:value={draftBranch}
        placeholder="fog/feature-name"
      />
      <p class="hint">Left empty, Fog generates one from the prompt.</p>
    </div>

    <div class="field">
      <label class="label" for="pr-title">Title</label>
      <input
        id="pr-title"
        class="input"
        type="text"
        bind:value={draftTitle}
        placeholder="feat: describe the change"
      />
    </div>

    <div class="dlg__actions">
      <button
        type="button"
        class="btn btn-secondary"
        onclick={() => (open = false)}
      >
        Cancel
      </button>
      <button type="submit" class="btn btn-primary">Save</button>
    </div>
  </form>
</dialog>

<style>
  .dlg {
    inline-size: min(28rem, calc(100dvw - var(--space-xl)));
    padding: 0;
    background: var(--color-paper-2);
    border: var(--rule-hair) solid var(--color-rule-2);
    border-radius: var(--radius);
    color: var(--color-ink);
  }

  .dlg::backdrop {
    background: oklch(8% 0.004 92 / 0.72);
  }

  .dlg__form {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    padding: var(--space-lg);
  }

  .dlg__title {
    font-size: var(--text-md);
    padding-block-end: var(--space-sm);
    border-block-end: var(--rule-hair) solid var(--color-rule);
  }

  .dlg__actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-xs);
    margin-block-start: var(--space-2xs);
  }
</style>
