# Quill Design System

## Core Concept
"Writing a universe as if it were in an ancient manuscript, powered by modern intelligence."

This UI is not a typical application. It is a narrative space — manuscript-modern: the warmth of a
hand-bound journal rendered with a teal/gold accent system and clean, contemporary typography.

---

## Color System

Backgrounds / surfaces:
- App canvas `--bg-app` `#eae1cf` · Sidebar `--bg-sidebar` `#efe7d3` · Header `--bg-header` `#f2ecdd`
- Card `--bg-card` `#f6f1e6` · Input `--bg-input` `#f2ecdd` · Editor rail `--bg-rail` `#f2ecdd`
- Graph canvas `--bg-graph` `#e6ddc9` · Sunken surface (chips, property tiles) `--surface-sunken` `#eae1cf`

Ink / text:
- Primary `--ink` `#2b2620` · Editor body `--ink-body` `#332e26` · Soft `--ink-soft` `#4a4238`
- Secondary `--muted` `#6f6656` · Labels/meta `--muted-2` `#6b6455` · Faint icons `--muted-3` `#b7ad96`

Accent / brand:
- Teal `--teal` `#2f5d54` (primary accent, buttons, active nav)
- Teal deep `--teal-deep` `#234b43` (hero/gradient dark, sidebar hero)
- Gold `--gold` `#dda94a` (highlight, CTA on dark, avatar; nudged from `#d9a441` in the Phase 5 contrast pass to clear WCAG AA on `--teal-deep`)
- Gold-ink `--gold-ink` `#8a6a1f` (gold accent text/borders on light tan surfaces — plain `--gold` fails AA there)
- Text-on-teal `--parchment-hi` `#f4ecdd`, `--teal-tint-on-dark` `#c2d1cb` / `--teal-tint-on-dark-2` `#b6c8c1`

Opacity / border helpers (codified so components stop hardcoding `rgba(43,38,32,.X)`):
- `--line` `rgba(43,38,32,.1)` hairline borders/dividers
- `--line-2` `rgba(43,38,32,.07)` table row divider
- `--line-strong` `rgba(43,38,32,.14)` input borders
- `--hover` `rgba(43,38,32,.05)` nav hover
- `--teal-soft` `rgba(47,93,84,.10)` / `--teal-soft-12` `rgba(47,93,84,.12)` (nav active bg, chips)
- `--gold-soft` `rgba(181,132,47,.12)`

Semantic:
- Danger `--danger` `#954827` (contradiction text/labels; darkened from `#a8542f` in the Phase 5 contrast pass — original only cleared ~4:1 on tan backgrounds), `--danger-deep` `#c0533a` (graph conflict edge, decorative — unchanged)
- `--danger-bg` `#f7ece3`, `--danger-border` `rgba(168,84,47,.25)`
- Warning `--warning` `#7d5a1c` (in-analysis, medium severity, ingestion progress; darkened from `#b5842f` — original only cleared ~2.5-3:1 on tan backgrounds)
- Success `--success` `#356b47` (analyzed/saved text; darkened from `#3f7a4f` — original only cleared ~3.7-4.3:1 on tan backgrounds) / `--success-2` `#5a7a52` (decorative — unchanged)
- Conflict dash `--conflict-dash` `#d98b63` (animated dashed contradiction line)

Knowledge-graph node types (6 backend entity types):
- Character `--node-character` `#2f5d54` · Place `--node-place` `#5a7a52` · Event `--node-event` `#b5842f`
- Faction `--node-faction` `#6f6656` · World Rule `--node-worldrule` `#4e8073` · Plot Arc `--node-plotarc` `#a8542f`

Rules:
- All colors live as `:root` tokens in `index.css`; components consume `var(--token)` only — never
  hardcode hex.
- Opacity helpers (`--line*`, `--hover`, `--*-soft`) replace ad-hoc `rgba()` literals.

### Contrast validation

`--muted-2` is checked against every background it's actually painted on as text color
(`frontend/src/lib/contrast.ts`): `#736c5b` (an earlier candidate, itself darkened from `#8a7f6c`
which only reached ~3.5:1 on `--bg-card`) cleared 4.5:1 on `--bg-card` (~4.63:1) but failed on
`--bg-app` (~4.02:1), `--bg-input`/`--bg-header`/`--bg-rail` (~4.43:1), and `--bg-sidebar` (~4.23:1).
The shipped `#6b6455` clears 4.5:1 against the worst case, `--bg-app` (~4.52:1), and therefore against
every other background too. A full per-screen AA pass across every other color pairing is deferred to
the final migration gate.

---

## Typography

- Titles, editor body, numerals: Newsreader (`--serif`) — serif display face, opsz 6..72, weights 400/500/600 + italic 400/500
- Body / UI: Spline Sans (`--sans`) — weights 400/500/600
- Labels, microcopy, uppercase kickers: Spline Sans Mono (`--mono`) — weights 400/500

Loaded via Google Fonts `<link>` in `index.html` (Newsreader + Spline Sans + Spline Sans Mono).

Cursor:
- Editor caret blinks via the `qblink` keyframe instead of a static line.

---

## Surfaces

- Warm tan/cream cards on a teal-accented ground — manuscript feel without heavy texture.
- Sidebar/header use the lighter `--bg-sidebar` / `--bg-header` surfaces; hero/CTA blocks use
  `--teal-deep` for contrast.

---

## Layout

- Editorial spacing: page padding `26–28px 30–34px`, card padding `18–24px 20–26px`, standard gaps `12–20px`.
- Radius scale: `--r-sm` 7px, `--r-md` 9px, `--r-lg` 12px, `--r-xl` 14px, `--r-2xl` 16px, `--r-pill` 20px, `--r-round` 50%.
- Font sizes in use: 9.5–10px (mono labels), 11.5–13.5px (body/meta), 14–18px (input/editor),
  16–23px (card titles), 28–34px (page/hero H1/H2), 30px (stat numerals).

---

## Components

Cards:
- `--bg-card` fill, `--line` border, `--r-lg` radius — reads as a manuscript page fragment.

Buttons:
- `button.primary`: teal-filled (`--teal`), text `--parchment-hi`, `--r-md` radius, `--teal-deep` on hover.
- Secondary/outline buttons use `--line-strong` borders and `--ink` text.

Inputs:
- Filled, rounded style: `--bg-input` background, `1px solid --line-strong` border, `--r-md` radius,
  `12px 14px` padding — replaces the previous underline-only input style. Focus ring switches the
  border to `--teal`.

Progress:
- Warm-toned bars/dots using `--warning` (in progress) → `--success` (done), not ink strokes.

---

## Icon system

Navigation and chrome glyphs are inline Unicode characters (or inline SVG fallback where a glyph
doesn't render), never a generic icon library dependency. See `Docs/` ADR notes for the full glyph
map; this keeps the bundle dependency-free and matches the manuscript-modern tone.

---

## Animations

Lightweight CSS `@keyframes` in `index.css` — no GSAP/ScrollTrigger dependency:
- `qfloat` — gentle vertical float (hero graph nodes)
- `qdash` — animated dash-offset (conflict/dashed edges)
- `qpulse` — opacity pulse (in-progress indicators)
- `qglow` — soft box-shadow glow (focus/active emphasis)
- `qblink` — editor caret blink
- `qspin` — loading spinner rotation
- `qrise` — fade + rise-in for cards/sections entering view
- `.q-scroll` — thin, muted-color scrollbar utility for rail/panel overflow

Animations stay slow, subtle, and non-distracting — the previous GSAP + ScrollTrigger scroll-driven
system is retired in favor of these CSS keyframes; nothing in the current screens requires
scroll-linked choreography.

---

## Illustrations

- Line-art / engraving-style SVGs where illustration is used (e.g., the login/landing hero
  relationship graph), colored with the teal/gold accent tokens rather than flat modern graphics.

---

## Data visualization (Memory Theater)

The Memory Inspector (`/universe/:id/memory`) renders the memory subsystem as charts. Same rule as icons:
**inline SVG only, no charting library** — hand-drawn `<polyline>`/`<rect>`/`<line>` in the component,
sized by `viewBox`, colored with palette tokens (never hardcoded hex).

- **Decay timeline** (`DecayTimeline`): a multi-line score-over-time chart. Each entity is one
  `<polyline>` colored by lifecycle — `active` = `--success-2`, `dormant`/decaying = warm node tokens
  (`--node-event`/`--node-worldrule`), `archived` = `--muted-3`. The archive threshold is a dashed
  `--muted-3` horizontal line; crossing points get ▼/▲ markers. Axis/labels use `--mono`.
- **Fusion explorer** (`FusionExplorer`): the five RRF pipelines as columns, the fused result list, and
  per-item contribution chips — chips use `--teal`/`--teal-soft` (and `--gold-ink` for emphasis), same
  sunken-surface + `--r-pill` treatment as elsewhere.
- **Budget theater** (`BudgetTheater`): token-budget bars (`--r-md`), warm fill for fitted items, muted
  for dropped — reusing the progress-bar tone (`--success` fitted, `--muted-*` dropped), not new colors.

Charts must read like the rest of the manuscript: warm ground, thin `--line` gridlines/borders, `--mono`
numerals, subtle motion only (no animated axes). If a chart needs a color that isn't already a palette
token, that's a signal to reuse an existing one, not to add a hex.

## Emotional Tone

- Calm, creative control, intimate, thoughtful — now paired with a slightly more "product" teal/gold
  accent so data (entities, contradictions, timeline, memory) reads clearly against the manuscript backdrop.

---

## Consistency Rules

Ask:
1. Does it feel written or generated?
2. Does it feel physical?
3. Does it have space?
4. Could it exist in an ancient book, viewed through a modern lens?

If not, redesign.

---

## Summary

An editorial, manuscript-inspired interface — teal/gold/tan palette, Newsreader + Spline Sans
typography, inline glyph icons, lightweight CSS keyframes — where modern UI disappears behind a
timeless writing experience.
