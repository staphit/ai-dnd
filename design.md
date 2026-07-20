# Design — D&D Duet AI

A locked design system for this app. Every page redesign reads this file before
emitting code. Do not regenerate per page — extend or amend this file when the
system needs to grow.

## Genre
atmospheric (candlelit tabletop · dark tavern ledger)

## Macrostructure family
- Marketing pages: Letter (if ever added)
- App pages:       Workbench — dense operational table with side-rail objective
- Content pages:   Long Document — journal / campaign memory

## Theme · custom "Candlelit Vellum"
Dark umber paper, warm parchment ink, brass candlelight accent, blood for combat.
Feels like a campaign binder under lantern light — not a SaaS dashboard.

- `--color-paper`       oklch(16% 0.022 55)
- `--color-paper-2`     oklch(20% 0.024 52)
- `--color-paper-3`     oklch(24% 0.026 50)
- `--color-ink`         oklch(91% 0.02 85)
- `--color-ink-2`       oklch(68% 0.02 75)
- `--color-rule`        oklch(34% 0.028 55)
- `--color-rule-soft`   oklch(27% 0.02 55)
- `--color-accent`      oklch(74% 0.13 72)
- `--color-accent-soft` oklch(58% 0.09 68)
- `--color-accent-ink`  oklch(18% 0.03 55)
- `--color-danger`      oklch(55% 0.145 28)
- `--color-focus`       oklch(78% 0.12 72)
- `--color-parchment`   oklch(22% 0.028 70)

## Typography
- Display: Cinzel, weight 500–600, roman only (fantasy titles / scene names)
- Body:    Source Serif 4 Variable, weight 400–500 (story, narration, UI body)
- Mono:    JetBrains Mono Variable, weight 400–500 (stats, dice, rules labels)
- Display tracking: 0.01em–0.04em on small caps / eyebrows
- Type scale anchor: headings use display; story measure ~65–70ch

## Spacing
4-point named scale in `tokens.css`. Prefer named tokens over raw px where new CSS is added.

## Motion
- Easings: `--ease-out: cubic-bezier(0.16, 1, 0.3, 1)`
- Reveal: short opacity + subtle translate; candle pulse on connection only
- Reduced-motion: opacity-only, ≤ 150 ms

## Microinteractions stance
- Silent success for rolls and saves
- Hover delay 800 ms on tooltips · focus delay 0 ms
- Primary CTA: solid brass fill, square corners (character-sheet edge)
- Secondary CTA: 1px brass-soft outline on transparent

## CTA voice
- Primary: filled brass, dark ink label, no pill radius
- Secondary: hairline rule border, warm muted label → brass on hover
- Destructive / combat: danger border wash, never celebrate failure

## Per-page allowances
- App pages: no hero enrichment — function carries the table
- Story feed may use parchment surface + double-rule dividers
- Setup shell may use soft radial candle bloom (subtle, not gradient-slop)

## What pages MUST share
- Scroll brand mark + Cinzel wordmark rhythm on titles
- Brass accent ≤ ~8% of viewport chroma (edges, eyebrows, active nav)
- Display + body + mono stack above
- Square / near-zero radius chrome (no soft SaaS pills)
- Double hairline or brass top-rule on key panels (objective, setup form)

## What pages MAY differ on
- Table vs journal density
- Combat state wash (danger tint) on active combat strip only
