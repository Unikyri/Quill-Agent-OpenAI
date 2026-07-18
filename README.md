# Quill — a MemoryAgent for long-form fiction

**Qwen Cloud Global AI Hackathon Series — Track 1: MemoryAgent**

Long-form fiction breaks when a writer has to remember every promise made hundreds of pages ago — a hair colour, a death, a vow, a timeline. Quill is a **memory agent** that reads a manuscript as the author writes it, accumulates durable memory of *both the story and the author*, forgets what stops mattering, and recalls only what fits the model's context window — then checks new prose against that memory and gets better at it the longer you write together.

Every model call in Quill goes to **Qwen models on Qwen Cloud (DashScope)**. Quill ships a hand-written **native DashScope client** — not just the OpenAI-compatible shim — so it can use Qwen-specific features (explicit context caching, native reranking, native token accounting) that a portable OpenAI client cannot reach.

---

## Why this is a MemoryAgent (Track 1 mapping)

Track 1 asks for an agent that *"autonomously accumulates experience, remembers user preferences, and makes increasingly accurate decisions across multi-turn, cross-session interactions"*, with *"efficient memory storage and retrieval, timely forgetting of outdated information, and recalling critical memories within limited context windows."* Quill implements each clause as running code:

| Track 1 requirement | Quill implementation | Where |
| --- | --- | --- |
| **Persistent memory** | Paragraphs, entities, and relationships persist in PostgreSQL 16 with **pgvector** embeddings and an **Apache AGE** property graph (one graph per universe). | `repositories/vector_repo.go`, `repositories/graph_repo.go` |
| **Efficient storage & retrieval** | **Hybrid recall**: five independently-ranked pipelines — vector, graph-walk, recency, keyword full-text, and consolidated summaries — fused with **Reciprocal Rank Fusion**, then optionally re-ordered by Qwen's **native reranker**. | `services/memory_service.go`, `services/fuse_rrf.go` |
| **Timely forgetting** | Event-driven **exponential relevance decay**; background entities archive below a threshold and **reactivate** when mentioned again. Every transition is logged for inspection. | `services/relevance_service.go`, migration `019` |
| **Recall within a limited context window** | A **token-budget knapsack** fits fused recall into `QWEN_MAX_CONTEXT_TOKENS` after reserving room for the response; survivors vs. dropped are reported. | `services/context_budget.go`, `services/tokenizer.go` |
| **Remembers user preferences** | A **writer-memory learning loop**: `accept` / `reject` / `behavioural_accept` signals reinforce observations and, past a corroboration threshold, are **promoted into durable writer preferences** via a strict Qwen JSON-schema classification. Passive **stylometry** learns *how* the author writes. | `services/writer_memory_service.go`, `services/stylometry_service.go` |
| **Increasingly accurate decisions across sessions** | Promoted preferences and the author's stylometric profile feed later craft reviews and suggestions, so guidance converges on *this* author's voice over time. | `services/craft_review_service.go` |

The result is a memory that has two subjects: the **manuscript** (lore, entities, timeline, contradictions) and the **author** (preferences, prose style) — and it learns from explicit and behavioural feedback, not just from ingestion.

---

## Sophisticated use of Qwen Cloud

Quill treats Qwen Cloud as a first-class platform, not a generic chat endpoint.

### Native DashScope client
`internal/services/dashscope_service.go` is a from-scratch native client for `dashscope-intl.aliyuncs.com/api/v1`, selected at composition time with `LLM_PROTOCOL=dashscope`. It uses:

- **Native generation** — `/services/aigc/text-generation/generation`
- **Native embeddings** — `/services/embeddings/text-embedding/text-embedding` (`text-embedding-v4`, 1024-dim)
- **Native reranking** — `/services/rerank/text-rerank/text-rerank` (`qwen3-rerank`), used to re-order fused recall
- **Explicit context caching** — `cache_control` content blocks against DashScope's context cache, so stable prompt prefixes are billed once
- **Native token accounting** — input / output / **cached** / cache-creation counters surfaced through `UsageSnapshot`
- **SSE streaming** — `X-DashScope-SSE: enable` for token-streamed agent progress

A wire-neutral `LLMService` interface (`internal/services/llm_service.go`) lets the same domain code run against either the native client or the OpenAI-compatible fallback, so features degrade gracefully but the native path is the intended one.

### Mini ReAct agent + MCP
Contradiction, plot-hole, and timeline checks are not single-shot prompts — they run a small **tool-calling agent loop** (`QwenService.RunAgentLoop`) where Qwen calls `search_vector_memory` (pgvector similarity) and `query_entity_graph` (AGE neighbour walk), receives results as `role: "tool"` messages, and loops until it stops calling tools. Those same memory tools are exposed over a real **MCP server** (`internal/mcp/server.go`, JSON-RPC `initialize` / `tools/list` / `tools/call`) so external MCP clients can query Quill's memory.

### Custom Skills framework
Quill ships **15 editorial Skills** (`backend/skills/`: `developmental-editor`, `line-editor`, `pacing-and-tension`, `sensitivity-reader`, `show-dont-tell`, …) plus genre references. A `SkillRegistry` loads them, they are **activated per universe** over the API (`GET/PUT /universes/:id/skills`), and the craft-review service composes them with recalled memory into Qwen prompts. These are Quill's own domain Skills — an extensible capability layer over the model, not a fixed prompt.

### Qwen models in use

| Role | Model | Config key |
| --- | --- | --- |
| Entity / relationship extraction | `qwen-turbo` | `QWEN_EXTRACTION_MODEL` |
| Reasoning (contradiction / plot-hole / timeline agent) | `qwen-max` | `QWEN_REASONING_MODEL` |
| Craft review & suggestions | `qwen-max` | `QWEN_CRAFT_MODEL` |
| Embeddings (pgvector, 1024-dim) | `text-embedding-v4` | `QWEN_EMBEDDING_MODEL` |
| Reranking fused recall | `qwen3-rerank` | `QWEN_RERANK_MODEL` |
| 429 fallback | `qwen-plus` | `QWEN_FALLBACK_MODEL` |

All model/endpoint configuration is visible without secrets in [`.env.example`](.env.example); the API key is never committed.

---

## The live analysis pipeline

As the author drafts, the frontend submits paragraphs over WebSocket. A **sequential per-work queue** (one goroutine per work, not a shared pool) keeps paragraphs from the same manuscript analysed in order, then fans out to entity extraction, contradiction detection, relevance decay, timeline validation, and plot-hole checks. Results stream back as typed WS messages (`analysis_result`, `contradiction_alert`, `entity_discovered`, `graph_updated`). Extracted entities and relationships are also written into the per-universe AGE graph. See `internal/services/analysis_service.go` and `internal/ws/`.

Because AGE forbids parameterised queries inside Cypher blocks, `graph_repo.go` defends every interpolation point: UUID-derived graph names, escaped string values, and whitelist-validated identifiers (`^[A-Za-z_][A-Za-z0-9_]*$`) for LLM-produced labels/relationship types — an injection failure drops that one node/edge rather than executing.

---

## Architecture

![Quill architecture](Docs/assets/quill-architecture.svg)

- **Frontend** — React 18 + Vite + TypeScript SPA (TipTap editor, React Flow graph, Zustand state). Served on **:3001** in Compose (container listens on 3000, proxies `/api` and the WebSocket to the backend).
- **Backend** — Go 1.22 + Fiber v2. Repositories → services → handlers, wired by hand in `cmd/server/main.go`. Talks to Qwen Cloud over the native DashScope client (or OpenAI-compatible fallback).
- **Database** — PostgreSQL 16 with **pgvector** (embeddings) and **Apache AGE** (per-universe property graph) on **:5432**; 27 numbered SQL migrations.
- **Model provider** — Qwen Cloud / DashScope (`dashscope-intl.aliyuncs.com`).

---

## Memory Theater (make the memory visible)

`/universe/:universeId/memory` renders the memory subsystem for demos with three inline-SVG "acts" (no charting library):

- **DecayTimeline** — per-entity relevance decay over time, lifecycle-coloured, with the archive threshold line.
- **FusionExplorer** — the five recall pipelines, the fused result, and per-item contribution chips (consumes `recall/explain`).
- **BudgetTheater** — the token-budget knapsack: which recalled items were fitted and which were dropped.

Memory HTTP surface: `POST /universes/:id/recall`, `POST /universes/:id/recall/explain`, `GET /universes/:id/memory-status`, `POST /universes/:id/decay`. Writer-memory surface: `GET /users/me/preferences`, `GET /users/me/preferences/:id/evidence`, `POST /users/me/preferences/feedback`.

---

## Run locally

Prerequisites: Docker Compose and a Qwen Cloud API key.

```bash
cp .env.example .env
# Set QWEN_API_KEY. To use the native DashScope client, set LLM_PROTOCOL=dashscope.
docker compose up -d
```

Open [http://localhost:3001](http://localhost:3001). The API is on `http://localhost:8080`; use the frontend for the full editor + WebSocket flow.

Frontend-only: `cd frontend && npm install && npm run dev` (serves `:3000`, proxies `/api` to `:8080`).
Backend-only: `cd backend && go run cmd/server/main.go` with a reachable `DATABASE_URL`.

---

## Verification

```bash
# Go build + tests (65 test files; DB-backed integration tests need TEST_DATABASE_URL).
cd backend && go build ./... && go test ./...

# Frontend typecheck, tests, production build.
cd frontend && npm run test && npm run build

# Model-backed memory evaluation (needs PostgreSQL + AGE + QWEN_API_KEY).
cd backend
TEST_DATABASE_URL=postgres://quill:quill_dev_password@localhost:5432/quill?sslmode=disable \
  QWEN_API_KEY=your_key go test ./eval/ -run TestMemoryEval -v
```

The committed [evaluation report](Docs/eval/results.md) records a small, dated recall/precision corpus and degraded-mode latency — a reproducible project artifact, not a production benchmark.

---

## How the Judging Criteria map to this repo

- **Innovation & AI Creativity** — native DashScope client with context caching + native rerank, a tool-calling agent loop, an MCP server, and a 15-Skill custom capability layer over Qwen.
- **Technical Depth & Engineering** — provider-neutral `LLMService` with two wire implementations; RRF hybrid recall across five pipelines; event-driven decay with archive/reactivate; token-budget knapsack; injection-hardened AGE graph access; sequential per-work analysis queue.
- **Problem Value & Impact** — continuity and voice consistency in long-form fiction, with a memory design (learned author preferences + forgetting + budgeted recall) that generalises to any long-horizon, context-limited agent.
- **Presentation & Documentation** — this README, the architecture diagram, `Docs/` (PRD, SRS, operations), and the in-app Memory Theater that visualises recall, decay, and budget.

---

## Three-minute demo route

1. Clone/reset the demo universe via the in-app guided demo.
2. **Write** — draft or ingest a passage; watch live analysis, entity discovery, and a contradiction alert.
3. **Explore** — inspect entities and their relationships in the graph.
4. **Memory** — ask a lore question, then show the recall explanation (five pipelines → RRF → rerank), the decay history, and the budgeted result.
5. **Review** — accept/reject a suggestion and show the writer-preference it reinforces.

---

## Submission evidence

- [x] Open-source license: [MIT](LICENSE) (visible in the repository About section).
- [x] Architecture diagram committed: [SVG](Docs/assets/quill-architecture.svg).
- [x] Qwen model/API configuration visible without secrets: [`.env.example`](.env.example).
- [x] Memory storage, retrieval, forgetting, budgeting, and preference learning implemented (links above).
- [ ] **Proof of Alibaba Cloud deployment** — link to the code/config running the backend on Alibaba Cloud (Compose alone is not deployment proof). *In progress.*
- [ ] **Demo video ≤3 min** on YouTube / Vimeo / Youku, showing the working flow. *In progress.*
- [ ] Public testing link (or credentials in the testing instructions) available to judges through the Judging Period.

The full working checklist is in [Docs/SUBMISSION-CHECKLIST.md](Docs/SUBMISSION-CHECKLIST.md); the local Track 1 snapshot is [Docs/memoryagent-track-rules.md](Docs/memoryagent-track-rules.md). The official rules and the Devpost form take precedence if they differ.

---

## Repository map

```text
frontend/       Judge-facing React/Vite SPA
backend/        Go/Fiber API, services, migrations, Qwen (native DashScope + OpenAI-compatible)
backend/skills/ 15 editorial Skills + genre references
Docs/           Product, sprint, evaluation, and submission materials
```

## License

Licensed under the [MIT License](LICENSE).
