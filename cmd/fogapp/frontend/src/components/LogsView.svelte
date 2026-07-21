<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { Search } from "@lucide/svelte";

    const allEvents = $derived(appState.detailEvents ?? []);
    let filterText = $state("");
    const filterQuery = $derived(filterText.trim().toLowerCase());
    const filteredEvents = $derived(
        filterQuery
            ? allEvents.filter((evt) => {
                  const message = (evt.message ?? "").toLowerCase();
                  const data = (evt.data ?? "").toLowerCase();
                  const type = (evt.type ?? "").toLowerCase();
                  return (
                      message.includes(filterQuery) ||
                      data.includes(filterQuery) ||
                      type.includes(filterQuery)
                  );
              })
            : allEvents,
    );

    function levelOf(type: string | undefined): string {
        const t = (type ?? "").toLowerCase();
        if (t === "error") return "error";
        if (t === "warn" || t === "warning") return "warn";
        return "info";
    }
</script>

<!-- No re-drawn window chrome: the previous build painted fake macOS traffic
     lights, a fake title bar and a fake "UTF-8 / Zsh" status strip around the
     log list. The real window already supplies chrome. -->
<div class="lv panel">
    <div class="panel__head">
        <span class="panel__title">Events</span>

        <div class="lv__filter">
            <Search size={13} aria-hidden="true" />
            <input
                type="text"
                class="lv__filter-input"
                bind:value={filterText}
                placeholder="Filter"
                aria-label="Filter log output"
            />
        </div>
    </div>

    {#if allEvents.length === 0}
        <div class="lv__empty">
            <div class="empty">
                <p class="empty__title">No events</p>
                <p>Output streams here once a run starts.</p>
            </div>
        </div>
    {:else if filteredEvents.length === 0}
        <div class="lv__empty">
            <div class="empty">
                <p class="empty__title">No matches</p>
                <p>Nothing in this session matches “{filterText}”.</p>
            </div>
        </div>
    {:else}
        <!-- No mask-image fade: it hid the first and last lines of the log. -->
        <div class="lv__rows scroll-y">
            {#each filteredEvents as evt (evt.id)}
                <p class="lv__row lv__row--{levelOf(evt.type)}">
                    <time class="lv__ts" datetime={evt.ts}>
                        {new Date(evt.ts).toLocaleTimeString()}
                    </time>
                    <span class="lv__tag">{evt.type}</span>
                    <span class="lv__msg">{evt.message || evt.data}</span>
                </p>
            {/each}
        </div>
    {/if}

    <footer class="lv__foot mono">
        {#if filterQuery}
            <span>{filteredEvents.length} of {allEvents.length} events</span>
        {:else}
            <span>{allEvents.length} events</span>
        {/if}
    </footer>
</div>

<style>
    .lv {
        display: flex;
        flex-direction: column;
        block-size: 100%;
        min-block-size: 0;
        min-inline-size: 0;
    }

    .lv__filter {
        display: flex;
        align-items: center;
        gap: var(--space-2xs);
        padding: var(--space-3xs) var(--space-xs);
        background: var(--color-paper);
        border: var(--rule-hair) solid var(--color-rule-2);
        color: var(--color-ink-3);
        flex: none;
        transition: border-color var(--dur-micro) var(--ease-out);
    }

    .lv__filter:focus-within {
        border-color: var(--color-accent);
    }

    .lv__filter-input {
        inline-size: 9rem;
        max-inline-size: 100%;
        background: none;
        border: none;
        outline: none;
        color: var(--color-ink);
        font-family: var(--font-mono);
        font-size: var(--text-xs);
    }

    .lv__filter-input::placeholder {
        color: var(--color-ink-3);
    }

    .lv__empty {
        flex: 1;
        display: grid;
        place-items: center;
        padding: var(--space-md);
    }

    .lv__rows {
        flex: 1;
        min-block-size: 0;
        padding-block: var(--space-xs);
        background: var(--color-paper);
    }

    .lv__row {
        display: grid;
        grid-template-columns: max-content max-content minmax(0, 1fr);
        gap: var(--space-sm);
        padding: var(--space-3xs) var(--space-md);
        border-inline-start: var(--rule-active) solid transparent;
        font-family: var(--font-mono);
        font-size: var(--text-xs);
        line-height: 1.6;
    }

    .lv__ts {
        color: var(--color-rule-2);
        font-variant-numeric: tabular-nums;
    }

    .lv__tag {
        color: var(--color-ink-3);
        text-transform: uppercase;
    }

    .lv__msg {
        color: var(--color-ink-2);
        white-space: pre-wrap;
        overflow-wrap: anywhere;
    }

    /* Level is carried by the gutter rule and the tag text, not by colour
       on its own. */
    .lv__row--warn {
        border-inline-start-color: var(--color-signal-warn);
        background: var(--color-signal-warn-wash);
    }

    .lv__row--warn .lv__tag {
        color: var(--color-signal-warn);
    }

    .lv__row--error {
        border-inline-start-color: var(--color-signal-del);
        background: var(--color-signal-del-wash);
    }

    .lv__row--error .lv__tag,
    .lv__row--error .lv__msg {
        color: var(--color-signal-del);
    }

    .lv__foot {
        flex: none;
        padding: var(--space-2xs) var(--space-md);
        border-block-start: var(--rule-hair) solid var(--color-rule);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
    }
</style>
