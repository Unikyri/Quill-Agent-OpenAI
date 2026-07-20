# Quill — a MemoryAgent for long-form fiction

**Qwen Cloud Global AI Hackathon Series — Track 1: MemoryAgent**

**🔗 Live demo:** [http://47.253.216.249:3001](http://47.253.216.249:3001) — no account needed, click "Jump into a finished universe" or "Start from scratch" on the landing page. Runs on an Alibaba Cloud Simple Application Server via the `docker-compose.yml` in this repo.

**🎬 Demo video:** [https://youtu.be/iCl3x6ufc7M](https://youtu.be/iCl3x6ufc7M)

Long-form fiction breaks when a writer has to remember every promise made hundreds of pages ago — a hair colour, a death, a vow, a timeline. Quill is a **memory agent** that reads a manuscript as the author writes it, accumulates durable memory of *both the story and the author*, forgets what stops mattering, and recalls only what fits the model's context window — then checks new prose against that memory and gets better at it the longer you write together.

Every model call in Quill goes to **Qwen models on Qwen Cloud (DashScope)**. Quill ships a hand-written **native DashScope client** — not just the OpenAI-compatible shim — so it can use Qwen-specific features (explicit context caching, native reranking, native token accounting) that a portable OpenAI client cannot reach.

---

## Why this is a MemoryAgent (Track 1 mapping)

Track 1 asks for an agent that *"autonomously accumulates experience, remembers user preferences, and makes increasingly accurate decisions across multi-turn, cross-session interactions"*, with *"efficient memory storage and retrieval, timely forgetting of outdated information, and recalling critical memories within limited context windows."* Quill implements each clause as running code:

| Track 1 requirement | Quill implementation | Where |
| --- | --- | --- |
| **Persistent memory** | Paragraphs, entities, and relationships persist in PostgreSQL 16 with **pgvector** embeddings and an **Apache AGE** property graph (one graph per universe). | `repositories/vector_repo.go`, `repositories/graph_repo.go` |
| **Efficient storage & retrieval** | **Hybrid recall**: six independently-ranked pipelines — vector, graph-walk, recency, keyword full-text, consolidated summaries, and writer preferences — fused with **Reciprocal Rank Fusion**, then optionally re-ordered by Qwen's **native reranker**. | `services/memory_service.go`, `services/fuse_rrf.go` |
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

### Four independent agents, not one prompt
Contradiction, plot-hole, and timeline checks are not single-shot prompts — each runs its own **tool-calling agent loop** (`QwenService.RunAgentLoop`) with its own system prompt and persona, calling `search_vector_memory` (pgvector similarity) and `query_entity_graph` (AGE neighbour walk) as needed, receiving results as `role: "tool"` messages, and looping until it stops calling tools. A fourth agent, the **Arbiter**, reviews what the first three found and writes one prioritised note instead of three independent opinions reaching the writer unreconciled. See [The Multi-Agent System](#the-multi-agent-system) below. Those same memory tools are exposed over a real **MCP server** (`internal/mcp/server.go`, JSON-RPC `initialize` / `tools/list` / `tools/call`) so external MCP clients — Claude Desktop, Cursor, any MCP-speaking agent — can query Quill's memory directly.

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

## Architecture

The diagram below is drawn the way a judge actually needs to read it: not "what packages exist," but **where a paragraph goes in, where a memory comes back out, and which of those two directions touches Qwen Cloud**. Everything under *Backend* is Go; everything under *Qwen Cloud* is a hosted DashScope model call; everything under *PostgreSQL* is the durable memory itself.

```mermaid
flowchart TB
    subgraph CLIENT["✍️ Writer-facing client — React 18 + Vite"]
        UI["Editor · Story Graph · Memory Lab"]
        MCPClient["External MCP clients<br/>Claude Desktop · Cursor · any MCP agent"]
    end

    subgraph BACKEND["⚙️ Backend — Go 1.22 + Fiber"]
        WS["WS Hub<br/>real-time push"]
        Analysis["AnalysisService<br/>sequential per-work queue"]
        Sub["EntityService · ContradictionService<br/>RelevanceService · TimelineService · PlotHoleService"]
        Memory["MemoryService<br/>6-pipeline RRF fusion → rerank → token budget"]
        MCP["MCP Server<br/>search_memory · query_entities · recall"]
    end

    subgraph AI["🧠 Qwen Cloud — DashScope"]
        QwenTurbo["qwen-turbo<br/>extraction"]
        QwenMax["qwen-max<br/>reasoning agent loop"]
        Embed["text-embedding-v4<br/>1024-dim"]
        Rerank["qwen3-rerank"]
    end

    subgraph DATA["💾 PostgreSQL 16 — the durable memory"]
        PGVector["pgvector<br/>paragraph + entity embeddings"]
        AGE["Apache AGE<br/>one property graph per universe"]
        Rel["relational tables<br/>entities · contradictions · decay history · writer prefs"]
    end

    UI -- "paragraph_submit" --> WS --> Analysis
    Analysis --> Sub
    Sub -- "extract" --> QwenTurbo
    Sub -- "tool-calling agent loop" --> QwenMax
    Analysis -- "embed paragraph" --> Embed
    Sub --> AGE
    Sub --> Rel
    Analysis -- "persist embedding" --> PGVector

    UI -- "recall / recall_explain" --> Memory
    MCPClient --> MCP --> Memory
    Memory -- "vector pipeline" --> PGVector
    Memory -- "graph-walk pipeline" --> AGE
    Memory -- "recency · keyword · consolidated · preference" --> Rel
    Memory -- "re-order fused results" --> Rerank
    Memory -- "budgeted, cited result" --> WS
    WS -- "analysis_result · contextual_recall · contradiction_alert" --> UI

    classDef client fill:#155e58,color:#fffefa,stroke:#0f4743
    classDef backend fill:#f2c65a,color:#192321,stroke:#8a5d00
    classDef ai fill:#605198,color:#fffefa,stroke:#3e2f6e
    classDef data fill:#a23d33,color:#fffefa,stroke:#7f3029
    class UI,MCPClient client
    class WS,Analysis,Sub,Memory,MCP backend
    class QwenTurbo,QwenMax,Embed,Rerank ai
    class PGVector,AGE,Rel data
```

The two paths worth tracing with your eyes are: **write path** (top-left to bottom, a paragraph becomes entities + graph edges + a vector row) and **recall path** (top-right through `MemoryService`, six pipelines fused and reranked before anything reaches the writer or an external MCP client). Nothing in the recall path is a single vector lookup — that's the whole point of Track 1's "efficient storage and retrieval" clause.

- **Frontend** — React 18 + Vite + TypeScript SPA (TipTap editor, Cytoscape relationship graph, Zustand state). Served on **:3001** in Compose (container listens on 3000, proxies `/api` and the WebSocket to the backend).
- **Backend** — Go 1.22 + Fiber v2. Repositories → services → handlers, wired by hand in `cmd/server/main.go`. Talks to Qwen Cloud over the native DashScope client (or OpenAI-compatible fallback).
- **Database** — PostgreSQL 16 with **pgvector** (embeddings) and **Apache AGE** (per-universe property graph) on **:5432**; 25+ numbered SQL migrations.
- **Model provider** — Qwen Cloud / DashScope (`dashscope-intl.aliyuncs.com`).

---

## The Multi-Agent System

The architecture diagram above deliberately doesn't zoom into the reasoning layer — cramming both "how data flows" and "how the agents reason" into one diagram makes both hard to read. This is that second diagram.

Quill runs **four independent agents**, not one prompt reused four ways. Each has its own system prompt, its own persona, and decides for itself whether it needs to call a tool before answering:

```mermaid
flowchart TB
    subgraph SPECIALISTS["Three specialists — same loop, different persona, never see each other's work"]
        Contra["Continuity Analyst<br/><i>ContradictionService</i><br/>'detect contradictions in this story'<br/>— runs on every paragraph"]
        Plot["Plot-Hole Evaluator<br/><i>PlotHoleService</i><br/>'is this entity's arc a plot hole?'<br/>— runs on every paragraph"]
        Time["Timeline Validator<br/><i>TimelineService</i><br/>'is this event chronologically consistent?'<br/>— runs when a timeline event is created"]
    end

    subgraph TOOLS["Tools — called only if the agent decides it needs them"]
        SearchMem["search_vector_memory<br/>pgvector similarity"]
        QueryGraph["query_entity_graph<br/>AGE neighbour walk"]
    end

    Contra -.-> SearchMem
    Contra -.-> QueryGraph
    Plot -.-> SearchMem
    Plot -.-> QueryGraph
    Time -.-> SearchMem

    Contra --> Arbiter
    Plot --> Arbiter

    Arbiter["Arbiter<br/><i>ArbiterService</i><br/>reads both specialists' raw findings,<br/>synthesises one verdict — no tools needed"]

    Arbiter --> Writer(["One prioritised note<br/>to the writer — not three<br/>unreconciled opinions"])

    Contra -. RunAgentLoop .-> QwenMax["qwen-max"]
    Plot -. RunAgentLoop .-> QwenMax
    Time -. RunAgentLoop .-> QwenMax
    Arbiter -. "RunAgentLoop, tools: none" .-> QwenMax

    classDef agent fill:#605198,color:#fffefa,stroke:#3e2f6e
    classDef tool fill:#f2c65a,color:#192321,stroke:#8a5d00
    classDef model fill:#155e58,color:#fffefa,stroke:#0f4743
    classDef out fill:#a23d33,color:#fffefa,stroke:#7f3029
    class Contra,Plot,Time,Arbiter agent
    class SearchMem,QueryGraph tool
    class QwenMax model
    class Writer out
```

- **Continuity Analyst** and **Plot-Hole Evaluator** run automatically on every submitted paragraph, fanned out from `AnalysisService.processJob` — see [The live analysis pipeline](#the-live-analysis-pipeline) below.
- **Timeline Validator** runs when a writer creates a timeline event whose chronological position is ambiguous relative to the work's latest chapter; an inconsistent verdict is surfaced as a non-blocking `timeline_warning`, not a hard rejection — the writer still makes the final call, same as every other finding in Quill.
- **Arbiter** closes the loop the other three don't: without it, a contradiction and a plot-hole finding for the same paragraph reach the writer as two disconnected alerts, even when they describe the same underlying issue. The Arbiter reads both raw finding sets and writes one short synthesis — which one actually matters, and whether two findings are really one. Its note shows up live in the editor's **Live Analysis** panel, right under the Contradiction section, whenever it actually has something to adjudicate.

None of the four share a tool registry entry that isn't already exposed to the MCP server above, and none run a bespoke reasoning path — they're all the same `RunAgentLoop`, invoked with a different prompt and a different (possibly empty) tool set. That's the whole trick: specialisation through persona and tool access, not four separate codebases.

---

## The live analysis pipeline

As the author drafts, the frontend submits paragraphs over WebSocket. A **sequential per-work queue** (one goroutine per work, not a shared pool) keeps paragraphs from the same manuscript analysed in order, then fans out to entity extraction, contradiction detection, relevance decay, timeline validation, and plot-hole checks. Results stream back as typed WS messages (`analysis_result`, `contradiction_alert`, `entity_discovered`, `graph_updated`). Extracted entities and relationships are also written into the per-universe AGE graph, and the paragraph's own embedding is persisted so the vector recall pipeline has real prose to search — not just what was imported from a file. See `internal/services/analysis_service.go` and `internal/ws/`.

Because AGE forbids parameterised queries inside Cypher blocks, `graph_repo.go` defends every interpolation point: UUID-derived graph names, escaped string values, and whitelist-validated identifiers (`^[A-Za-z_][A-Za-z0-9_]*$`) for LLM-produced labels/relationship types — an injection failure drops that one node/edge rather than executing.

---

## Screenshots

| | |
| --- | --- |
| **Landing** — the pitch, before anyone logs in | **Editor** — live pipeline, entities, contradiction detection, and token budget while writing |
| ![Landing](.github/assets/screenshots/landing.png) | ![Editor](.github/assets/screenshots/editor.png) |
| **Story Graph** — ranked relationship map, tabbed entity detail | **Memory Lab** — a real recall query, fused across pipelines, with the decay chart and token budget always visible |
| ![Story Graph](.github/assets/screenshots/story-graph.png) | ![Memory Lab](.github/assets/screenshots/memory-lab.png) |
| **Review** — contradictions with side-by-side evidence and a suggested resolution | |
| ![Review](.github/assets/screenshots/review.png) | |

---

## Memory Theater (make the memory visible)

`/universe/:universeId/memory` renders the memory subsystem for demos with three always-visible sections (no charting library, inline SVG/hand-built charts):

- **Story Recall** — ask any question about the manuscript; the answer is a real cited passage plus per-pipeline contribution and the pre-rerank → post-rerank position delta, not a paraphrase.
- **Decay, relevance, and consolidation** — per-entity relevance decay over time, lifecycle-coloured, with the archive threshold line, plus a manual decay-sweep trigger so a judge doesn't have to wait for real time to pass.
- **Context budget** — the token-budget knapsack: which recalled items were fitted and which were dropped, and why.

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
# Go build + tests (DB-backed integration tests need TEST_DATABASE_URL).
cd backend && go build ./... && go test ./...

# Frontend typecheck, tests, production build.
cd frontend && npm run test && npm run build

# Model-backed memory evaluation (needs PostgreSQL + AGE + QWEN_API_KEY):
# recall/precision/MRR/nDCG on a small dated corpus, degraded-mode latency
# benchmarks at increasing entity counts, a forgetting-timeline check after N
# decay ticks, and a head-to-head of RRF-only vs. native-DashScope-reranked
# recall.
cd backend
TEST_DATABASE_URL=postgres://quill:quill_dev_password@localhost:5432/quill?sslmode=disable \
  QWEN_API_KEY=your_key go test ./eval/ -run TestMemoryEval -v
```

**Measured results from the eval corpus** — a small, hand-authored corpus (`backend/eval/corpus/saga_gold.json`, 6 gold queries), reproducible with the command above. Six queries is exploratory evidence, not a statistically powered benchmark — the precision below is real but the sample is intentionally small, sized for a fast, deterministic CI check rather than a publishable retrieval study:

| Metric | Result |
| --- | --- |
| Recall@5 — vector only | 0.583 |
| Recall@5 — graph-walk only | 0.333 |
| Recall@5 — recency only | 0.167 |
| Recall@5 — keyword only | 0.000 |
| Recall@5 — **vector + graph** | **0.833** |
| Recall@5 — all pipelines fused | 0.417 |
| Recall latency, p50 / p95 @ 50 entities | 2 ms / 2 ms |
| Recall latency, p50 / p95 @ 5,000 entities | 27 ms / 33 ms |
| Forgetting — active entities before / after 17 decay ticks | 21 → 15 |
| Consolidation fidelity (sample entities) | 0.75, 0.25 |

The ablation is the honest finding here: no single pipeline wins outright, vector+graph beats every pipeline alone (including the full fused set) on this corpus, and keyword-only recall is a real, reported zero rather than a hidden failure — that's what makes hybrid recall a design decision instead of a marketing claim. Production (`MemoryService.RecallWithQuery`, used by both Story Recall and live-analysis contextual recall) fuses all six pipelines unconditionally rather than switching to whichever combination scored best on this six-query sample — a corpus this small isn't a sound basis for hard-coding a narrower pipeline set into the default path, so the current behaviour is the more defensible one until a larger eval says otherwise.

---

## How the Judging Criteria map to this repo

- **Innovation & AI Creativity** — native DashScope client with context caching + native rerank, [four independent agents](#the-multi-agent-system) (three specialists + a consensus-forming Arbiter) over a shared tool-calling loop, an MCP server, and a 15-Skill custom capability layer over Qwen.
- **Technical Depth & Engineering** — provider-neutral `LLMService` with two wire implementations; RRF hybrid recall across six pipelines; event-driven decay with archive/reactivate; token-budget knapsack; injection-hardened AGE graph access; sequential per-work analysis queue. Measured, not asserted — see [Verification](#verification) for real recall/latency/forgetting numbers from the eval harness.
- **Problem Value & Impact** — continuity and voice consistency in long-form fiction, with a memory design (learned author preferences + forgetting + budgeted recall) that generalises to any long-horizon, context-limited agent.
- **Presentation & Documentation** — this README, the architecture diagram above, and the in-app Memory Theater that visualises recall, decay, and budget live.

---

## Three-minute demo route

1. Clone/reset the demo universe via the in-app guided demo ("Try the live demo").
2. **Write** — draft or ingest a passage; watch live analysis, entity discovery, and a contradiction alert.
3. **Explore** — inspect entities and their relationships in the Story Graph.
4. **Memory** — ask a lore question, then show the recall explanation (six pipelines → RRF → rerank), the decay history, and the budgeted result.
5. **Review** — accept/reject a suggestion and show the writer-preference it reinforces.

---

## Submission evidence

- [x] Open-source license: [MIT](LICENSE) (visible in the repository About section).
- [x] Architecture diagram: see [Architecture](#architecture) above.
- [x] Qwen model/API configuration visible without secrets: [`.env.example`](.env.example).
- [x] Memory storage, retrieval, forgetting, budgeting, and preference learning implemented (links above).
- [x] **Proof of Alibaba Cloud deployment** — live at [http://47.253.216.249:3001](http://47.253.216.249:3001), running on an Alibaba Cloud Simple Application Server from the `docker-compose.yml` in this repo (not just Compose in the abstract — this is the actual running instance).
- [x] **Demo video ≤3 min** on YouTube: [https://youtu.be/iCl3x6ufc7M](https://youtu.be/iCl3x6ufc7M)
- [x] Public testing link available to judges through the Judging Period: [http://47.253.216.249:3001](http://47.253.216.249:3001) — no credentials needed, the demo entry points provision a throwaway account automatically.

The official [Devpost rules](https://qwencloud-hackathon.devpost.com/rules) take precedence if anything here differs.

---

## Repository map

```text
frontend/       Judge-facing React/Vite SPA
backend/        Go/Fiber API, services, migrations, Qwen (native DashScope + OpenAI-compatible)
backend/skills/ 15 editorial Skills + genre references
```

## License

Licensed under the [MIT License](LICENSE).
