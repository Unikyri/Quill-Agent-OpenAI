# Sprint 7 — Jury-Led Frontend Refactor

**Tier:** Demo-critical · **Focus:** OpenAI Build Week design/impact and Qwen MemoryAgent
presentation/technical proof
**Prerequisite:** Sprints 0–6 complete enough to expose real universe, editor, memory,
review, and demo data.
**Outcome:** Quill becomes a calm, writer-first SPA that makes its real memory intelligence
legible in a three-minute judge demo. It must feel like a coherent consumer product, not a
collection of technical screens.

---

## Why this sprint exists

OpenAI Build Week scores **Technological Implementation, Design, Potential Impact, and
Quality of the Idea** equally; technical implementation wins a tie. The project must be
functional, coherent, and demonstrably built with Codex and GPT-5.6.

Qwen MemoryAgent scores **Innovation & AI Creativity (30%)**, **Technical Depth &
Engineering (30%)**, **Problem Value & Impact (25%)**, and **Presentation & Documentation
(15%)**. The frontend must therefore prove persistent memory, retrieval, forgetting, and
context-budgeted recall instead of merely claiming them.

**Source of truth:** [OpenAI Build Week rules](https://openai.devpost.com/rules),
[OpenAI Build Week overview](https://openai.com/build-week/),
[Qwen Cloud official rules](https://qwencloud-hackathon.devpost.com/rules), and
[Qwen Cloud challenge page](https://www.qwencloud.com/challenge/hackathon). The Qwen
Devpost rules prevail over conflicting landing-page details; the demo target is **under
three minutes**.

---

## Product decision — one writer journey, not eleven peer tabs

Remove the permanent 11-item sidebar. Replace it with a compact top application bar and
five primary destinations:

| Destination | Job to be done | Consolidates |
|---|---|---|
| **Home** | Choose a universe, continue writing, start a demo, or create a new one. | Dashboard, Panorama, universe switching |
| **Write** | Draft, manage chapters, import material, and receive live analysis. | Works, Chapters, Editor, Ingestion |
| **Explore** | Understand the story world and its relationships. | Entities, Graph, Timeline |
| **Memory** | See what Quill recalls, why, and what it deliberately forgets. | Memory Inspector, recall explanation |
| **Review** | Make author decisions on candidates, contradictions, plot holes, and craft notes. | Candidates, Contradictions, Plot Holes, Skills |

Keep existing deep links through redirects. The primary URLs become:

- `/dashboard` — Home / universe library.
- `/universe/:universeId/write/:chapterId?` — chapter picker when no chapter is selected.
- `/universe/:universeId/explore/:view` — `entities`, `map`, or `timeline`.
- `/universe/:universeId/memory`.
- `/universe/:universeId/review/:view` — `issues`, `candidates`, or `craft`.

**Expected result:** a judge immediately sees the product's purpose and can complete a
writer workflow without learning an internal navigation taxonomy.

---

## Task 7.1 — Establish the Quiet Studio design system

**Files:** global frontend tokens, shared UI primitives, page CSS modules, and
`design.md`.

**Decisions:**

1. Keep **Newsreader** for manuscript reading and editorial headings, and **Spline Sans**
   for controls and data. They are already available and preserve reading legibility.
2. Replace the current tan-heavy application chrome with semantic tokens: paper canvas,
   near-black ink, pine/teal for active intelligence, amber for attention, coral for risk,
   and violet reserved for memory. Use whitespace, type hierarchy, and borders for depth;
   do not use ornamental gradients or decorative data effects.
3. Replace Unicode UI glyphs with `lucide-react`; use CSS motion only, honoring
   `prefers-reduced-motion`. Ship a polished light theme only in this sprint rather than
   multiplying accessibility and QA work with a dark mode.
4. Use CSS Modules and semantic CSS tokens. Do not introduce Tailwind or a visual
   component framework. Add focused primitives only: Radix Dialog, Dropdown Menu, Tooltip,
   Popover, and Checkbox.
5. Add a skip link, consistent `:focus-visible`, keyboard behavior, and WCAG AA contrast.

**Expected result:** every route shares one recognizable visual language while the
manuscript remains the most readable thing on screen.

---

## Task 7.2 — Make feedback a product contract

**Files:** new shared feedback provider/components, API hooks/stores, and every async page.

**Steps:**

1. Introduce a client-only `FeedbackEvent` contract:
   `queued → running → completed | failed | offline`, with scope, plain-language message,
   timestamp, and optional retry action.
2. Add a `FeedbackProvider`, a non-blocking `sonner` toast queue, accessible `aria-live`
   announcements, and a small persistent header status for autosave, WebSocket/Qwen
   connectivity, active analysis, and failures.
3. Give every request an honest loading skeleton, empty state, success confirmation, and
   error/retry state. Remove silent catches, browser `alert()`, and `window.confirm()`.
4. Preserve optimistic interactions only when rollback communicates what happened and how
   to recover.

**Expected result:** the user always knows whether Quill is saving, analyzing, recalling,
finished, unavailable, or waiting for a decision.

---

## Task 7.3 — Redesign Home, universe creation, and genres

**Files:** Dashboard/Home, universe shell, and a reusable genre picker.

**Steps:**

1. Make Home a useful universe library: one clear **Continue writing** CTA, compact
   universe cards, visible genre chips, concise progress/attention signals, and a
   guided-demo entry point. Do not show more than two secondary metrics per universe card.
2. Replace the native multi-select control in every create/edit flow with one accessible
   `GenreTagPicker`: search field, checkable genre chips, selected-chip summary, removable
   tags, and full keyboard support.
3. Preserve the existing closed 20-tag vocabulary and `genre_tags: string[]` API. Genres
   are optional and allow **zero or more** selections; remove the forced `fantasy` default.
   This is a frontend-only UX change—no migration or API redesign is required.
4. Add a **Guided Demo** card that uses the existing demo clone/reset APIs and clearly
   reports setup, ready, reset, and failure states.

**Expected result:** a writer can create a multi-genre universe naturally, while a judge
can enter a working demo without setup ambiguity.

---

## Task 7.4 — Consolidate the writing workspace

**Files:** route shell, chapter navigation, editor, and ingestion surfaces.

**Steps:**

1. Combine manuscript selection, chapter navigation, import, and the editor under
   **Write**. The chapter rail is contextual and collapsible, never a global sidebar.
2. Keep Sprint 6 durability visible: save state, local recovery, analysis state, candidate
   actions, and craft review must remain in context without obscuring the manuscript.
3. Make Import a primary contextual action from Write. Show a single current ingestion
   status and ETA, then route naturally into the resulting chapter rather than leaving the
   user in a separate operational screen.
4. Preserve old editor and works URLs via redirects before retiring duplicate components.

**Expected result:** writing is a single uninterrupted flow: choose chapter → write or
import → see live intelligence → decide what to keep.

---

## Task 7.5 — Replace React Flow with a story relationship map

**Files:** graph page/store/components and frontend dependencies.

**Decision:** remove `reactflow`. Quill has a semantic, cyclic, many-to-many knowledge
graph, not an editable architecture flowchart. Use `cytoscape` with `cytoscape-fcose`,
directly integrated through React lifecycle hooks rather than an unmaintained wrapper.
Cytoscape supports graph-specific layouts, including fCoSE.

**Steps:**

1. Preserve the existing focused two-hop-neighborhood API invariant. Start from the selected
   entity, not an unreadable all-universe graph.
2. Render readable entity cards and quiet, unlabeled edges. Remove handles, grid, minimap,
   default flow controls, connection dots, and visible `relationship` labels.
3. Reveal relationship meaning only after selection: the contextual inspector states the
   relation in prose, evidence, source chapter, connected entities, and relevant conflicts.
4. Provide entity search, type filters, reset-focus, keyboard navigation, and an accessible
   relationship list that mirrors the canvas.
5. Lazy-load the map so graph code never delays the initial Home or Write experience.

**Expected result:** the graph reads as a calm relationship map for a novel, not a technical
diagram full of implementation noise.

---

## Task 7.6 — Turn Memory and Review into visible proof

**Files:** Memory, candidates, contradictions, plot holes, and craft-review screens.

**Steps:**

1. Make Memory answer one question at a time: the recalled answer, its evidence, the
   pipelines that contributed, items excluded by the context budget, and a readable
   decay/consolidation history. Hide raw technical detail until requested.
2. Turn Review into a prioritized author inbox. Each item explains *why it surfaced*, its
   evidence, impact, and the single next action. Candidates, contradictions, plot holes,
   and craft notes remain distinct filters, not separate product silos.
3. Add a real-data, six-step demo narrative: clone demo universe → write/import → observe
   analysis → accept a candidate → inspect the relationship map → recall lore with evidence
   and decay/budget proof.
4. Never synthesize fake success data. If Qwen, WebSocket, or an API is unavailable, show
   that honestly with recovery guidance.

**Expected result:** the Qwen MemoryAgent story is understandable in seconds and credible
because the UI shows the actual system reasoning and lifecycle.

---

## Task 7.7 — Verify the product and submission surface

**Tests and acceptance:**

- [ ] Unit-test the genre picker, feedback state machine, redirects, graph-element adapter,
  hidden-by-default edge labels, and retry behavior.
- [ ] Integration-test multi-genre creation/editing, save/analyze/recover, candidate action,
  recall explanation, and failed-request recovery.
- [ ] Add Playwright coverage for the full guided demo at desktop and mobile widths,
  keyboard-only navigation, and no silent error state.
- [ ] Add automated accessibility checks, contrast checks, focus order, and reduced-motion
  checks.
- [ ] Verify the frontend build, test suite, lazy graph chunk, and visual behavior at the
  judge recording width.
- [ ] Update the sprint index and document that the Qwen local rules snapshot is stale: the
  official deadline is July 20, 2026.
- [ ] Keep a separate submission checklist for required evidence: Qwen Cloud/Alibaba
  deployment, public license/repository, architecture diagram, and a demo under three
  minutes; OpenAI Codex/GPT-5.6 usage evidence and session ID cannot be satisfied by UI
  polish alone.

**Definition of done:** a judge can open the SPA, understand Quill's value in one screen,
complete the guided writer-to-memory journey with real data, and receive clear feedback at
every state without encountering a dense or noisy screen.
