# Quill v2 — Sprint Plan

**Companion documents:** [PRD](../PRD.md) · [SRS](../SRS.md) · [SKILLS](../SKILLS.md)

Sprints execute **strictly in order** — each one's verification depends on the previous
one's guarantees. The chain of reasoning: fix the ruler (0) → prove the core (1) → make it
fast (2) → upgrade the engine (3) → remember the writer (4) → apply the craft (5) → polish
and open up (6).

| Sprint | Tier | Theme | SRS sections | Migrations |
|---|---|---|---|---|
| [Sprint 0](./SPRINT-0-entity-standard.md) | P0 | Entity standard, taxonomy migration, entity browser, neighborhood graph, workspace | §2 (TX, ET, FE, GV, WS) | `021` |
| [Sprint 1](./SPRINT-1-live-analysis-e2e.md) | P1 | Live-analysis diagnosis & fix, lifecycle visibility, E2E suite (`make e2e`) | §3 (LA, EE) | — |
| [Sprint 2](./SPRINT-2-ingestion-performance.md) | P1 | MAP/REDUCE ingestion, TPM throttle, model tiering, honest ETA | §4 (IG, TH, MT, PF) | `022` |
| [Sprint 3](./SPRINT-3-dashscope-native.md) | P2 | Native DashScope client, context cache, structured output, rerank | §5 (DS, CC, SO, RR) | — |
| [Sprint 4](./SPRINT-4-writer-memory.md) | P2 | Writer Memory: stylometry, promotion loop, recall conditioning, evidence trail | §6 (WM, RC, EX, CS) | `023` |
| [Sprint 5](./SPRINT-5-skills.md) | P2 | Skill loading/selection machinery, craft review (content already authored) | §7 (SF, SS, CV) | `024` |
| [Sprint 6](./SPRINT-6-editor-mcp.md) | P3 | Autosave durability, import/export, highlighting, candidates tray, MCP | §8–§9 (EH, EC, ID, MC) | — |

## Model & quota strategy

Free quotas: 1M tokens per **exact model code**, expiring 2026-09-23. The coupon pays for
everything after. Two rules govern the plan:

1. **Embeddings are sticky.** Vectors from different embedding models live in different
   spaces — cosine similarity across them is meaningless. Everything in pgvector must come
   from ONE model, forever. The choice is `text-embedding-v4` (`dimensions: 1024`, pinned —
   the pgvector columns are `vector(1024)`), free quota now, same model on the coupon later,
   zero migrations ever. **Wipe any dev DB that still holds v3 vectors** (`docker compose down -v`).
2. **Free quota is granted per exact model code.** Use the codes from the console list, not
   the bare aliases — `qwen-max` may resolve to a snapshot other than the one carrying the
   quota. Verify after the first calls that consumption lands on the free quota.

| Role | Env var | Free-quota phase | Coupon phase |
|---|---|---|---|
| Extraction / MAP | `QWEN_EXTRACTION_MODEL` | `qwen-turbo-latest` | `qwen-turbo` |
| Reasoning (contradictions, plot holes, craft review) | `QWEN_REASONING_MODEL` | `qwen-max-2025-01-25` | `qwen-max` |
| Plumbing tests (E2E wiring, JSON-shape checks — quality irrelevant) | `QWEN_EXTRACTION_MODEL` per run | `qwen3-8b`, then `qwen2.5-7b-instruct`, `qwen2.5-14b-instruct` | — (never; that's what the throwaway quotas are for) |
| Fallback (TH-6, Sprint 2) | `QWEN_FALLBACK_MODEL` (new in Sprint 2) | `qwen-plus-latest` | `qwen-plus` |
| Rerank (Sprint 3) | `QWEN_RERANK_MODEL` (new in Sprint 3) | `qwen3-rerank` | `qwen3-rerank` |
| Embeddings | `QWEN_EMBEDDING_MODEL` | `text-embedding-v4` | `text-embedding-v4` — **never change** |

Sprint 2 uses role-based chat variables: `QWEN_EXTRACTION_MODEL`,
`QWEN_REASONING_MODEL`, and `QWEN_FALLBACK_MODEL`. The legacy
`QWEN_MAX_MODEL` and `QWEN_TURBO_MODEL` aliases remain supported while
deployments migrate, but new configuration should use the role names. Sprint 3
adds `QWEN_RERANK_MODEL` and `LLM_PROTOCOL`. Rotating from free quota to
coupon is always a `.env` value change, never a rebuild (MT-3).

Budget reality check: one full 400-page ingest burns ~300–500K extraction tokens — **two
full runs exhaust qwen-turbo's free million**. Iterate on the 50-page fixture; spend
400-page runs only on the PF-1 timed measurements. Audio/vision/omni quotas are unused —
Quill is text-only.

## Standing rules for every sprint:

- **`make e2e` green is part of every definition of done** from Sprint 1 onward.
- Every migration ships a working `.down.sql` (NF-3).
- All AGE access through `withAgeTx`/`withAgeConn`; all LLM-derived Cypher identifiers
  through `validCypherIdentifier` (NF-1/NF-2).
- No claimed improvement without a recorded number (SRS §11).
- New UI follows `design.md` (palette, inline glyphs, no chart libraries).
