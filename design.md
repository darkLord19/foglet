# Design — Fog Desktop

A locked design system for the Fog desktop app. Every surface redesign reads
this file before emitting code. Do not regenerate per view — extend or amend
this file when the system needs to grow.

**Scope.** This governs `cmd/fogapp/frontend/` only. The Go TUI
(`internal/tui/`) is a separate surface and is not covered here.

---

## Context

- **Audience** — solo developers, running agent sessions on their own machine.
- **Use case** — queue work on a board, start agents on it, watch them run.
- **Direction** — **Quiet**: the interface recedes so the work is the only thing
  with contrast. Linear / Vercel register.
- **Medium** — Wails desktop binary. Runs **offline**. No CDN may be depended on
  for fonts, icons, or any other asset.

An earlier brutalist system (zero radius, heavy rules, uppercase display type)
was built and rejected. Do not reintroduce its vocabulary.

## Shell

A **persistent sidebar** plus a single main pane.

```
┌──────────────┬────────────────────────────────────┐
│  Fog         │  <main>                            │
│  Board       │                                    │
│  New session │   Board · Session · Settings       │
│              │                                    │
│  Running     │                                    │
│  · session   │                                    │
│  Recent      │                                    │
│  · session   │                                    │
│              │                                    │
│  Settings    │                                    │
└──────────────┴────────────────────────────────────┘
```

Sessions live in the sidebar and are always visible; switching is one click.
The main pane shows the board, a session, or settings. There is no top nav —
`TopNav.svelte` is retained but unused, pending a decision to delete it.

## Surfaces

| Surface | Shape |
| --- | --- |
| **Board** | Four columns of cards. The primary surface, and the default view. |
| **Session** | Header, tab strip (Timeline / Diff / Logs / Stats), follow-up composer. Splits into two panes at ≥1600px. |
| **New session** | Composer only — the lists it used to sit above now live in the sidebar. |
| **Settings** | Sequential sections down one measure-capped column. |

**Enrichment: none, anywhere.** No hero illustration, no decorative SVG,
no abstract background. Function is the page.

## Theme — Quiet

Cool-neutral near-black. The surface ladder spans only 17%→23% lightness, which
is what keeps the chrome quiet; hierarchy comes from text contrast, not from
boxes.

```
--color-paper        oklch(16.8% 0.004 264)   base canvas
--color-paper-2      oklch(19.1% 0.006 271)   panel / sidebar
--color-paper-3      oklch(21.8% 0.008 275)   hover
--color-paper-4      oklch(23.2% 0.010 277)   active / pressed
--color-rule         oklch(26.4% 0.008 264)   decorative separators
--color-rule-2       oklch(31%   0.011 271)   quiet control edges
--color-field        oklch(52%   0.012 273)   text-input boundaries
--color-ink-3        oklch(62%   0.013 275)   tertiary text
--color-ink-2        oklch(71%   0.010 273)   secondary text
--color-ink          oklch(93.7% 0.003 265)   primary text
--color-accent       oklch(86.1% 0.173 92)    Fog yellow — the brand
--color-accent-dim   oklch(79.5% 0.162 86)    accent hover
--color-accent-ink   oklch(19.7% 0.022 104)   text ON accent
```

**Accent budget: about twice per screen.** In practice that is the active
sidebar rail and the one primary button. The drop indicator on the board and
the active tab underline reuse the same 2px accent language.

### The three line tokens

Not light/dark variants of one idea — three different obligations:

- **`--color-rule`** — decorative separators (row and section dividers). Not a
  WCAG target: a row is identified by its text, not the line beneath it.
- **`--color-rule-2`** — quiet edges on controls that carry a visible text
  label (buttons, panels, cards). The label does the identifying.
- **`--color-field`** — the boundary of a text input, held at ≥3:1 against
  `paper-4` per WCAG 1.4.11. You must be able to see where the click target
  starts.

Using `--color-rule` on an input border is a defect, not a style choice.

### Semantic signals — documented deviation

One accent is the rule; an app with diffs, logs and run states needs more.
These four are **functional signals**, not accents:

```
--color-signal-add   oklch(80%   0.182 152)
--color-signal-del   oklch(71.1% 0.166 22)
--color-signal-warn  oklch(83.7% 0.164 84)
--color-signal-info  oklch(82.8% 0.101 230)
```

They appear only in diff gutters, log levels and status badges; never as a
button fill, never as a gradient, and **always paired with a glyph or text
label** so state survives greyscale and colour-blindness.

### Contrast — measured, gated

`npm run contrast` computes OKLCH → sRGB → WCAG ratios for all 25 token pairs
and **exits non-zero on failure**. It is a real gate: it caught four genuine
failures when this palette was first derived from the design mock, including
control borders at 1.32:1.

Two tokens are deliberately *not* the mock's values, because the mock failed:

| Token | Mock | Shipped | Why |
| --- | --- | --- | --- |
| `ink-3` | `#6C6E76` (3.77:1) | `oklch(62%…)` (4.61:1 worst case) | carries timestamps, labels, inactive tabs — all information |
| control border | `#2E3036` (1.32:1) | `--color-field` (3.05:1 worst case) | you must see where an input starts |

If a palette value changes, update `scripts/contrast-check.mjs` to match and
re-run. `--color-ink-3` has the least headroom; if a surface lighter than
`paper-4` is ever introduced, it must be re-solved.

### Bans

- No `#000` / `#fff`. No shadows — on a near-black surface they read as glow.
- No gradients, including on the accent.
- No hover lift. Hover is a background step up the ladder.
- No glassmorphism, no backdrop blur.

## Typography

Two families. **Quiet has no display face** — headings are the UI face at a
heavier weight, which is precisely what keeps it recessive. Self-hosted via
`@fontsource`; the app ships offline and a CDN link would silently fall back to
system sans.

```
Body   Geist Variable            400 / 500 / 600
Mono   JetBrains Mono Variable   400
```

Mono carries exactly one role: **machine text** — code, diffs, logs, commands,
IDs, paths, keys, and any number that lines up in a column. Nothing else.

### Scale — fixed, not fluid

```
--text-2xs  11px    meta, counts, labels
--text-xs   12px    buttons, hints
--text-sm   13px    UI base
--text-md   15px    surface titles
--text-lg   18px    view headings
--text-xl   22px    the largest thing in the app
```

Type is fixed because this is a dense tool at a fixed viewing distance; type
that grows with the window reads as a website. **Spacing stays fluid** via
`--gutter` so the layout still breathes.

Uppercase is reserved for genuine section labels — a structural signal, kept
small and infrequent. Buttons are sentence case.

## Spacing and structure

4-point scale (`--space-3xs` … `--space-2xl`), plus `--gutter` as the single
fluid token. Radius is `6px` (`--radius`), `4px` for small chips, `8px` for
panels. Rows are `34px`, controls `28px`.

## Responsive

Desktop only — the bundle is embedded in the Wails binary and never served to a
browser. **There is no mobile case.** Range is 900px (window minimum) to
ultrawide.

- **Container queries, not media queries**, wherever a component reflows: panes
  resize independently of the window, so viewport width would lie.
- Session detail splits into two panes at ≥1600px.
- `html, body { overflow-x: clip }` — never `hidden`, which breaks `sticky`.
- Heights use `dvh`; widths never use `100vw`.
- Logical properties throughout (`padding-inline`, `border-inline-start`).

## Motion

```
--ease-out     cubic-bezier(0.16, 1,    0.3,  1)
--ease-in      cubic-bezier(0.7,  0,    0.84, 0)
--ease-in-out  cubic-bezier(0.65, 0,    0.35, 1)

--dur-micro    130ms   state change, press
--dur-short    200ms   panel, tooltip
--dur-long     320ms   view transition
```

- Animate `transform` and `opacity` only.
- No hover lift, no scroll-triggered reveals — this is a tool, not a page.
- Dragged cards **dim in place**; they do not lift or rotate. A tilted clone is
  a tell and fights the direction's flatness.
- `prefers-reduced-motion` collapses spatial motion. Functional indicators
  (spinners, the running pulse) keep running — they carry state.

## Microinteraction stance

- **Silent success.** A completed action changes the UI; it does not toast.
- Toasts are for failures and for things that happen **off-screen** — starting
  an agent qualifies, saving a setting does not.
- Focus rings appear instantly, never animated.

## Interaction states

Every interactive element ships all eight: `default` · `hover` ·
`:focus-visible` · `:active` · `disabled` · `loading` · `error` · `success`.
The primitives in `primitives.css` implement all eight once.

## Board conventions

The board is the app's primary surface, so its rules are part of the system:

- Columns are Todo · In progress · In review · Done, in that order.
- **Moving a card into a working column starts an agent** — In progress runs the
  implementation, In review runs a reviewer over the same worktree. This is
  driven by `internal/task`, not by the UI.
- A card that arrived from a tracker shows a **Start** button instead of
  auto-running. Never remove that distinction: see the origin rule below.
- The drop indicator is a 2px accent rule, matching every other active-state
  signal in the app.
- Drag has a keyboard equivalent (`⌥` + `←`/`→`). A board that only works with
  a mouse is not finished.

### The origin rule (do not weaken)

Fog syncs bidirectionally with Linear and Jira, so a teammate dragging a card
upstream produces the same status change as the owner dragging it here. If
origin were ignored, anyone with tracker write access could execute an
autonomous agent on this machine.

`task.AutoStarts` therefore fires **only** for `task.OriginLocal`. HTTP API
calls are local by construction — loopback plus token, and `cloudrelay` polls
outward rather than entering through the API — so the move endpoint hardcodes
the origin rather than reading it from the payload. Sync calls the store
directly with `OriginRemote`.

## File organisation

No component file exceeds **~350 lines**. Two mechanisms hold that:

1. **Shared primitives live in `primitives.css`**, not in per-component
   `<style>` blocks — buttons, inputs, panels, rows, badges and all eight
   interaction states are declared once.
2. **Large surfaces decompose** into a container owning layout and data, plus
   presentational children owning a single concern.

## Exports

`tokens.css` is canonical and is imported by `app.css`, which also maps the
tokens into Tailwind's `@theme inline` namespaces so utilities resolve against
the same values. The `@import "tailwindcss"` directive stays at the top of
`app.css` and must never be removed.

---

*Amend this file rather than overriding it locally. A surface that drifts from
this system is the defect.*
