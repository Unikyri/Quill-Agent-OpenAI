# Quill

> An AI writing IDE that remembers a novelist's fictional world — and learns how that novelist wants to write.

Quill helps prose-fiction writers keep continuity across novels, novellas, and short stories in the same universe. It ingests existing manuscripts, extracts story knowledge into a graph, recalls relevant context while the writer works, and surfaces continuity risks. The v2 direction adds **Writer Memory**: evidence-backed preferences that improve editorial feedback across sessions without treating the writer's habits as unquestionable intent.

## The Problem

Long-form fiction has two kinds of memory problems:

- **Story memory:** details of characters, places, objects, rules, events, timelines, and open plot threads spread across hundreds of pages or multiple books.
- **Writer memory:** the editorial guidance that helps one author may be wrong for another. A writer may deliberately prefer long sentences, sparse dialogue, or a genre-specific register.

Existing writing tools rarely connect these problems. Quill is designed to retain both the story's canon and evidence-backed knowledge of the writer's editorial preferences.

## What Quill Does

- Organizes a fictional **universe** into works and chapters.
- Ingests `.pdf`, `.docx`, `.md`, and `.txt` manuscripts.
- Extracts entities, embeddings, and relationships into PostgreSQL/pgvector and an Apache AGE graph.
- Retrieves context through hybrid recall: vector, graph, recency, keyword, and consolidated-memory pipelines fused with RRF.
- Tracks entity relevance over time so background details can decay and reactivate when mentioned again.
- Detects contradictions, timeline issues, and potential plot holes through the analysis pipeline.
- Visualizes recall, decay, and context-budget decisions in the Memory Theater.

## v2: Writer Memory and Editorial Skills

The v2 release narrows Quill to prose fiction: novels, novellas, and short stories. Its central addition is **Writer Memory**:

1. Stylometry records observable facts such as sentence length, dialogue ratio, and adverb density without an LLM call.
2. Craft-review feedback supplies intent signals: accept, reject, or a lower-confidence behavioral acceptance.
3. Only repeated, corroborated evidence promotes an observation into a writer preference.
4. Preferences are scoped as universal or genre-bound, decay over time, and participate in context retrieval.
5. The writer can inspect and override the evidence behind a preference.

Editorial reviews use curated, read-only skills. The agent selects only the skills relevant to the writer's requested passage, then loads only their full instructions. Skills provide margin observations, not ghostwritten prose.

The v2 requirements and skill catalogue are documented in [Docs/PRD.md](Docs/PRD.md), [Docs/SRS.md](Docs/SRS.md), and [Docs/SKILLS.md](Docs/SKILLS.md).

## Architecture

```mermaid
flowchart LR
    Writer["Writer"] --> UI["React + TipTap editor"]
    UI <--> WS["WebSocket hub"]
    UI <--> API["Go + Fiber API"]
    WS --> Analysis["Per-work analysis queue"]
    Analysis --> Qwen["Qwen Cloud"]
    Analysis --> Memory["Memory service"]
    Memory --> PG["PostgreSQL + pgvector"]
    Memory --> AGE["Apache AGE graph"]
    Ingest["Document ingestion"] --> Analysis
    Ingest --> PG
    Ingest --> AGE
    MCP["Quill MCP server — v2"] --> Memory
```

| Layer | Technology |
| --- | --- |
| Backend | Go 1.22+, Fiber v2.52.x |
| Frontend | React 18, Vite, TypeScript, TipTap, Zustand |
| Database | PostgreSQL 16, pgvector, Apache AGE |
| Graph visualization | React Flow |
| AI | Qwen Cloud (`qwen-max`, `qwen-turbo`, `text-embedding-v4`) |

## Quick Start

### Prerequisites

- Docker and Docker Compose
- A Qwen Cloud API key

### Run the stack

```bash
cp .env.example .env
# Set QWEN_API_KEY in .env
docker compose up -d
```

Open [http://localhost:3000](http://localhost:3000).

### Run locally

```bash
# Backend (PostgreSQL must be reachable through DATABASE_URL)
cd backend && go run cmd/server/main.go

# Frontend
cd frontend && npm run dev

# Database only
docker compose up postgres
```

### Verification

```bash
# Backend tests
cd backend && go test ./...

# Memory evaluation harness: requires PostgreSQL + AGE and QWEN_API_KEY
TEST_DATABASE_URL=postgres://quill:quill_dev_password@localhost:5432/quill?sslmode=disable \
  QWEN_API_KEY=your_key go test ./eval/ -run TestMemoryEval -v

# Frontend tests and build
cd frontend && npm run test
cd frontend && npm run build

# Assembled, model-backed browser verification (requires QWEN_API_KEY).
# This boots an isolated `quill-e2e` Compose project on 15432/18080/13001,
# waits (at most two minutes) for health, runs Playwright, and removes only
# that project's disposable database volume when it finishes.
cd .. && QWEN_API_KEY=your_key make e2e
```

## Demo Path

1. Create a universe and a work.
2. Import a manuscript or open a chapter.
3. Inspect extracted entities, relationships, and timeline information.
4. Use the Memory Theater to inspect recall sources, relevance decay, and context-budget decisions.
5. For v2, select a passage for a craft review, inspect the selected skills and their observations, then accept or reject feedback to demonstrate Writer Memory learning.

## Hackathon Evidence

### Global AI Hackathon Series with Qwen Cloud — Track 1: MemoryAgent

Quill is submitted to **Track 1: MemoryAgent**. Its story-memory foundation already combines persistent storage, hybrid retrieval, decay, consolidation, and context budgeting. The v2 Writer Memory extension applies the same discipline to user preferences: it accumulates evidence across sessions, forgets stale assumptions, conditions recall by genre, and produces auditable explanations.

Before submitting, complete every item below with real links and measurements:

- [ ] **Alibaba Cloud deployment:** link to the deployment/configuration code and a screenshot proving the backend runs on Alibaba Cloud.
- [ ] **Architecture diagram:** export the diagram above as a PNG or SVG and upload it in the Devpost submission.
- [ ] **Performance evidence:** record ingestion timing and memory-evaluation metrics; do not claim targets that were not measured.
- [ ] **Demo video:** show a working end-to-end flow, including the Writer Memory learning loop.
- [ ] **Open-source repository:** keep this repository public and retain the [MIT License](LICENSE).

### OpenAI Build Week — Apps for Your Life

Quill fits **Apps for Your Life** as a creative-writing companion. The OpenAI Build Week submission focuses on the v2 extension delivered during its submission period: Writer Memory, agent-selected editorial skills, and the demonstrated learning loop.

OpenAI requires a clear account of how **Codex and GPT-5.6** were used. This section must be completed from actual development evidence before submission; do not replace it with generic claims.

| Evidence required | What to record |
| --- | --- |
| Codex contribution | The concrete components Codex accelerated: diagnosis, implementation, tests, documentation, or UI work. |
| GPT-5.6 contribution | The model used in Codex for each meaningful task and the resulting implementation decision. |
| Human decisions | Product and engineering choices made by the team, including trade-offs and review of Codex output. |
| Session evidence | The `/feedback` Codex Session ID required by the Devpost submission form. |
| Demo | A public video under three minutes showing the working v2 flow and explaining the Codex/GPT-5.6 contribution. |

> Do not submit this section with placeholders. Replace the table with dated, verifiable evidence after the OpenAI Build Week work is complete.

## Repository Guide

```text
backend/        Go API, services, repositories, migrations, and Qwen integration
frontend/       React editor and visualizations
backend/skills/ Curated editorial-skill instruction files
Docs/           Product, software, and skill specifications
```

## Submission Strategy

Use **one repository**, not two divergent copies. Submit separate Devpost project pages, demo videos, and descriptions for each hackathon, while keeping the shared code and its history in this repository.

- **Qwen Cloud:** emphasize Qwen-powered MemoryAgent behavior, Alibaba Cloud deployment, architecture, and measured memory performance.
- **OpenAI Build Week:** emphasize the substantial v2 extension and the verified Codex/GPT-5.6 development evidence.

If a hackathon-specific artifact becomes large — for example a deployment proof, video script, or submission narrative — place it under `Docs/submissions/` and link it from this README. Do not fork the codebase merely to change the README.

## License

[MIT](LICENSE)
