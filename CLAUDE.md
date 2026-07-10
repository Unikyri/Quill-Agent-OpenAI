# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Quill — an AI-powered writing IDE for creative writers with persistent memory. As a writer drafts chapters, the backend analyzes each paragraph in the background to extract entities, detect contradictions against established lore, flag plot holes, and validate timeline consistency, pushing results to the editor live over WebSocket.

- **Backend**: Go 1.22 (Fiber v2.52.x) + PostgreSQL 16 (pgvector + Apache AGE graph extension)
- **Frontend**: React 18 + Vite + TypeScript, TipTap (editor), React Flow (graph viz), Zustand (state)
- **AI**: Qwen Cloud API (qwen-max / qwen-turbo for generation, text-embedding-v3 for embeddings), OpenAI-compatible endpoint

## Commands

### Running the stack
```bash
cp .env.example .env        # then set QWEN_API_KEY
docker compose up -d        # postgres + migrations + backend + frontend
```
- Backend only: `cd backend && go run cmd/server/main.go` (needs Postgres reachable via `DATABASE_URL`)
- Frontend only: `cd frontend && npm run dev` (Vite dev server on :3000, proxies `/api` to `localhost:8080`)
- DB only: `docker compose up postgres`
- Migrations are plain numbered `.up.sql`/`.down.sql` pairs in `backend/migrations/`, applied by `backend/scripts/run-migrations.sh` (tracked in a `schema_migrations` table). There is no migration CLI/tool — add a new sequentially-numbered pair to add schema changes.

### Backend (from `backend/`)
- Build: `go build ./...`
- Run all tests: `go test ./...`
- Run a single package: `go test ./internal/services/...`
- Run a single test: `go test ./internal/services/ -run TestName`
- Integration tests (repository/handler tests touching Postgres) require `TEST_DATABASE_URL` to be set; without it they call `t.Skip` (see `internal/testutil/db.go`). Point it at a Postgres instance with `pgvector` + `age` extensions available — `docker compose up postgres` provides one.
- `testutil.RunMigrationsUpTo` tears down and reapplies migrations per test run, and skips migration `014` automatically if the AGE extension isn't loaded on the target DB. Because `go test ./...` runs package binaries in parallel against one shared `TEST_DATABASE_URL`, it holds a Postgres **advisory lock on a dedicated standalone connection for the whole test** (released via `t.Cleanup`) so a concurrent package can't tear the schema down mid-test — do not add a `defer pool.Close()` in a test (it would run before that cleanup); `SetupTestDB` already registers the close and caps the pool at `MaxConns = 8`.
- Note the test harness applies migrations directly and does **not** write `schema_migrations`, so a DB left in that state will make the `migrations` docker service fail ("relation already exists"). After running the suite against the compose Postgres, `docker compose down -v` before bringing the full stack back up.

### Reproduction

- Full memory evaluation harness (needs Postgres + AGE, and `QWEN_API_KEY` for semantic recall/consolidation tests):  
  `TEST_DATABASE_URL=postgres://quill:quill_dev_password@localhost:5432/quill?sslmode=disable QWEN_API_KEY=your_key go test ./backend/eval/ -run TestMemoryEval -v`
- Metrics-only smoke tests (no DB):  
  `go test ./backend/eval/ -run 'TestRecall|TestPrecision|TestMRR|TestNDCG' -v`

### Frontend (from `frontend/`)
- Dev server: `npm run dev`
- Build (typecheck + build): `npm run build`
- Tests: `npm run test` (vitest run), `npm run test:watch` for watch mode
- Run a single test file: `npx vitest run src/path/to/File.test.tsx`

## Architecture

### Request flow / composition root
Everything is wired by hand in `backend/cmd/server/main.go` — no DI framework. Read it first when tracing how a feature connects end to end: repositories → services → handlers → Fiber routes. Note the two-phase init for circular deps: `ws.NewHub` is constructed with a `nil` submitter, `AnalysisService` is built (which needs the hub), then `hub.SetSubmitter(analysisSvc)` wires it back.

### The analysis pipeline (core feature loop)
This is the part that spans the most files and is the most important thing to understand:
1. Frontend submits a paragraph over WebSocket (`paragraph_submit`, see `frontend/src/stores/wsStore.ts` / `useWS.ts`) or via the debounced editor.
2. `ws.Hub` (`backend/internal/ws/hub.go`) receives it and calls into `AnalysisService.SubmitParagraph`.
3. `AnalysisService` (`backend/internal/services/analysis_service.go`) runs **one goroutine per work with a sequential per-work queue** (not a worker pool) so paragraphs from the same work are analyzed in order. It fans out to `EntityService`, `ContradictionService`, `RelevanceService`, `TimelineService`, `PlotHoleService`.
4. Results are pushed back to the client via `hub.SendToUser` using typed WS messages defined in `backend/internal/ws/protocol.go` (`analysis_result`, `contradiction_alert`, `entity_discovered`, `graph_updated`, `ingestion_progress`, etc.).
5. Entities/relationships also get written into an Apache AGE graph, one graph per universe named `universe_<universeUUID>` (see `graph_repo.go`). AGE doesn't support parameterized queries inside `$$ $$` Cypher blocks, so `graph_repo.go` defends every interpolation point explicitly — this is the sharpest edge in the codebase:
   - **Graph names** are UUID-derived, injection-safe by construction.
   - **Property string values** are escaped via `escapeCypherString`.
   - **Identifiers** (node labels, relationship types) come from LLM output (entity/relationship extraction), so they are whitelist-validated by `validCypherIdentifier` (`^[A-Za-z_][A-Za-z0-9_]*$`); a violation returns `ErrInvalidIdentifier` and the caller drops that one node/edge. Never interpolate a label/rel-type without this guard.
   - **`search_path`**: AGE requires `SET search_path = ag_catalog, …` to resolve `cypher()`, but `ag_catalog` shadows `public` (it has its own `entities`/`works`/etc.). `withAgeTx`/`withAgeConn` capture the prior `search_path` and **restore it after the AGE op**, so a pooled connection never leaks a poisoned `search_path` (leaving it poisoned once wrote shadow tables into `ag_catalog` — see git history). Any new AGE-touching code MUST route through these helpers, never run raw `LOAD 'age'`/`SET search_path` on a pooled conn.

### Mini ReAct agent (`QwenService.RunAgentLoop` + `QuillExecutor`)
Contradiction/plot-hole/timeline checks aren't single-shot prompts — they run a small tool-calling agent loop:
- `QwenService` (`backend/internal/services/qwen_service.go`) implements OpenAI-style function calling and a `RunAgentLoop` that lets the model call tools, feeds results back as `role: "tool"` messages, and loops (capped depth) until the model stops calling tools.
- `QuillExecutor` (`backend/internal/services/agent_tools.go`) is the `ToolExecutor` implementation, dispatching by tool name via a plain switch (only two tools, so no registry): `search_vector_memory` (embeds the query, does pgvector similarity search over paragraph embeddings) and `query_entity_graph` (resolves an entity name to ID, then walks the AGE graph for neighbors/relations).
- Read `qwen_service.go` (`RunAgentLoop`) and `agent_tools.go` together before modifying the agent loop or adding new tools.

### Memory subsystem (the "MemoryAgent" story)
This is the second thing to understand after the analysis pipeline. It layers several independent mechanisms, all read through `MemoryService`:

- **Relevance decay** (`RelevanceService`): an event-driven exponential decay model (`DECAY_LAMBDA`, `ARCHIVE_THRESHOLD` config, default 0.15) scores entities by recency of mention, archiving background entities and reactivating them if referenced again — this is what lets `PlotHoleService` distinguish "character quietly written out" from "character abandoned mid-arc". Each decay/reactivation appends a row to `entity_relevance_history` (migration 019), which is what the frontend decay chart plots. Trigger a decay sweep with `POST /universes/:id/decay`.
- **Hybrid recall via RRF fusion** (`MemoryService.Recall`/`RecallWithPipelines`, `fuseRRF`): five independently-ranked pipelines — **vector** (pgvector similarity), **graph** (AGE neighbor walk from vector seeds), **recency** (decay score), **keyword** (`vector_repo.KeywordSearch`, Postgres full-text, migration 018), **consolidated** (LLM summaries) — are combined with Reciprocal Rank Fusion. `RecallWithPipelines` can select a subset. `RecallExplain` (`fuse_rrf_explain.go`) returns the per-pipeline contribution of each fused item — this is what powers the "explain" endpoint.
- **Consolidation** (`ConsolidationService`, `consolidation_repo.go`, migration 017): periodically LLM-summarizes an entity's mentions into a `consolidated_memories` row (summary + key facts + embedding), which then feeds the consolidated recall pipeline. Optional/nil-safe — wired via `MemoryService.SetConsolidationRepo`.
- **Context budget** (`ContextBudgetManager` in `context_budget.go`, `tokenizer.go`): a token-budget knapsack that fits fused recall items into a max-context window (reserving room for the response). Optional/nil-safe — wired via `MemoryService.SetBudgetMgr`. `budgetSurvivors`/`fitToBudget` decide what survives.
- **Status snapshot** (`memory_status.go`): `GET /universes/:id/memory-status` returns per-entity relevance + lifecycle + a capped `History[]` of `{score, recorded_at}` — the data source for the decay chart.

Memory HTTP surface (all under `graphH`, see `main.go`): `POST /universes/:id/recall`, `POST /universes/:id/recall/explain`, `GET /universes/:id/memory-status`, `POST /universes/:id/decay`.

### Memory Theater (frontend)
`/universe/:universeId/memory` (`frontend/src/pages/MemoryInspectorPage.tsx`, "Memory" tab in `UniverseLayout`) makes the memory subsystem visible for demos. Three inline-SVG "acts" in `frontend/src/components/memory/`: `DecayTimeline` (multi-line decay chart from memory-status history, lifecycle-colored, threshold line at `ARCHIVE_THRESHOLD`), `FusionExplorer` (the five RRF pipelines + fused result + per-item contribution chips, consuming `recall/explain`), and `BudgetTheater` (token-budget bars, fitted vs dropped). All inline SVG — no charting library (see `design.md`).

### Data model layering
Each domain (`universe`, `work`, `chapter`, `entity`, `contradiction`, `timeline_event`, `plot_hole`, `ingestion_job`, plus the memory-subsystem tables `consolidated_memory` and `entity_relevance_history`) follows the same three-layer shape: `repositories/*_repo.go` (raw pgx SQL) → `services/*_service.go` (business logic, orchestrates repos + Qwen) → `handlers/*.go` (Fiber HTTP handlers). Cross-domain reads (e.g. graph + vector + entity together) go through `MemoryService` rather than handlers reaching into multiple repos directly. There is also a `DemoService` (`POST /api/v1/demo/clone` · `/reset`) that deep-copies a seeded template universe — including its AGE graph via `GraphRepo.QueryTemplateEdgesTx` — for a one-click demo.

### Document ingestion
`IngestionService` is a separate async pipeline (own goroutine per job) from the live analysis queue: parses uploaded `.md`/`.txt`, splits into chapters by Markdown headers, chunks into paragraphs, runs entity extraction + embeddings + graph population, and streams `ingestion_progress` over the same WS hub.

### Frontend state
Zustand stores in `frontend/src/stores/` mirror backend domains (`authStore`, `universeStore`, `editorStore`, `graphStore`, `wsStore`). `wsStore` owns the single WebSocket connection and dispatches incoming typed messages (matching `ws/protocol.go`'s `Type*` constants) out to the other stores — check `wsStore.ts` when adding a new server→client message type, both the Go constant and the frontend switch need updating together. `lib/api.ts` is a thin fetch wrapper injecting the JWT from `localStorage`.

### Design language
`design.md` documents a strict "manuscript-modern" visual system (teal/gold/tan palette via `:root` tokens in `index.css`, Newsreader + Spline Sans typography, inline Unicode/SVG glyph icons instead of an icon library, and lightweight CSS `@keyframes` — the old GSAP/ScrollTrigger system is retired, no charting library). Consult it before touching UI components or adding colors/animations/data-viz, since it constrains palette, motion, and chart style project-wide.

## Notes
- `.env.example` is used as the template referenced in the README's quick start; when editing it keep placeholder-style values (it is committed to git).
