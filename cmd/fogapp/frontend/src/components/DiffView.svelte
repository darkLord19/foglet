<script lang="ts">
    import { appState } from "$lib/stores.svelte";
    import { ArrowRight } from "@lucide/svelte";

    const diff = $derived(appState.detailDiff);
    const diffError = $derived(appState.detailDiffError);

    const lines = $derived(diff?.patch ? diff.patch.split("\n") : []);

    function kindOf(line: string): "add" | "del" | "meta" | "ctx" {
        if (line.startsWith("+++") || line.startsWith("---")) return "meta";
        if (line.startsWith("@@") || line.startsWith("diff ")) return "meta";
        if (line.startsWith("+")) return "add";
        if (line.startsWith("-")) return "del";
        return "ctx";
    }
</script>

<div class="dv">
    {#if diffError}
        <div class="empty">
            <p class="empty__title">Couldn&rsquo;t load the diff</p>
            <p class="dv__err mono">{diffError}</p>
        </div>
    {:else if !diff}
        <div class="empty">
            <p class="empty__title">No changes</p>
            <p>This run didn&rsquo;t modify any files.</p>
        </div>
    {:else}
        <div class="panel dv__panel">
            <div class="panel__head dv__head">
                <span class="panel__title">Changes</span>
                <p class="dv__branches mono">
                    <span class="truncate">{diff.base_branch}</span>
                    <ArrowRight size={12} />
                    <span class="dv__branch-to truncate">{diff.branch}</span>
                </p>
            </div>

            {#if diff.stat}
                <p class="dv__stat mono">{diff.stat}</p>
            {/if}

            {#if lines.length > 0}
                <div class="dv__patch scroll-y">
                    <pre class="dv__pre">{#each lines as line, i}<span
                                class="dv__line dv__line--{kindOf(line)}"
                                ><span class="dv__ln" aria-hidden="true"
                                    >{i + 1}</span
                                ><span class="dv__code">{line || " "}</span
                                ></span
                            >{/each}</pre>
                </div>
            {:else}
                <div class="dv__body">
                    <p class="hint">No text patch available for this change.</p>
                </div>
            {/if}
        </div>
    {/if}
</div>

<style>
    .dv {
        block-size: 100%;
        min-block-size: 0;
        display: flex;
    }

    .dv__panel {
        display: flex;
        flex-direction: column;
        inline-size: 100%;
        min-inline-size: 0;
        min-block-size: 0;
    }

    .dv__head {
        flex-wrap: wrap;
    }

    .dv__branches {
        display: flex;
        align-items: center;
        gap: var(--space-2xs);
        font-size: var(--text-2xs);
        color: var(--color-ink-3);
        min-inline-size: 0;
    }

    .dv__branch-to {
        color: var(--color-accent);
    }

    .dv__stat {
        flex: none;
        padding: var(--space-2xs) var(--space-md);
        border-block-end: var(--rule-hair) solid var(--color-rule);
        font-size: var(--text-2xs);
        color: var(--color-ink-2);
    }

    .dv__body {
        padding: var(--space-md);
    }

    .dv__patch {
        flex: 1;
        min-block-size: 0;
        background: var(--color-paper);
    }

    .dv__pre {
        margin: 0;
        font-family: var(--font-mono);
        font-size: var(--text-sm);
        line-height: 1.6;
        min-inline-size: max-content;
    }

    .dv__line {
        display: flex;
        gap: var(--space-sm);
        /* The gutter rule carries the add/del signal alongside the colour,
           so the diff stays legible without colour perception. */
        border-inline-start: var(--rule-active) solid transparent;
        padding-inline-end: var(--space-md);
    }

    .dv__ln {
        flex: none;
        inline-size: 4ch;
        padding-inline-start: var(--space-xs);
        text-align: end;
        color: var(--color-rule-2);
        user-select: none;
        font-variant-numeric: tabular-nums;
    }

    .dv__code {
        white-space: pre;
        color: var(--color-ink-2);
    }

    .dv__line--add {
        background: var(--color-signal-add-wash);
        border-inline-start-color: var(--color-signal-add);
    }

    .dv__line--add .dv__code {
        color: var(--color-signal-add);
    }

    .dv__line--del {
        background: var(--color-signal-del-wash);
        border-inline-start-color: var(--color-signal-del);
    }

    .dv__line--del .dv__code {
        color: var(--color-signal-del);
    }

    .dv__line--meta .dv__code {
        color: var(--color-ink-3);
        font-weight: 600;
    }

    .dv__err {
        font-size: var(--text-xs);
        color: var(--color-ink-2);
        white-space: pre-wrap;
        overflow-wrap: anywhere;
        padding: var(--space-sm);
        background: var(--color-paper-2);
        border-inline-start: var(--rule-hair) solid
            var(--color-signal-del);
    }
</style>
