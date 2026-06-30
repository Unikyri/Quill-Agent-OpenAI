# Quill — Consolidated Analysis & Updated Implementation Plan

> **Session:** Post-interview analysis  
> **Date:** 2026-06-28  
> **Track:** MemoryAgent | **Hackathon:** Global AI Hackathon Series with Qwen Cloud  
> **Deadline:** Jul 9, 2026 (2:00 PM PT) — **11 days remaining**  
> **Approach:** Solo developer, DevOps-first with continuous deployment

---

## 1. Design Decisions (Interview Results)

| # | Decision | Choice |
|---|---|---|
| 1 | Docker Image Strategy | `apache/age:release_PG16_1.6.0` base + pgvector compiled on top (stable version) |
| 2 | Analysis Trigger | Sequential queue per chapter with context cancellation (`context.WithCancel`) — separating Core Transaction (steps 1-4, atomic, not cancelable) and Enrichment Queue (steps 5-10, cancelable) |
| 3 | Frontend Debounce | 5 seconds (up from 2-3s in original SRS) |
| 4 | Decay Visualization | Translucent nodes + dashed gray border + archive icon + "Archive" panel section + glow/pulse reactivation animation |
| 5 | API Optimization | Debounce 5s + sequential queue with core/enrichment stages + Qwen-Max batching (max 3 candidates) + semaphore global rate limiting |
| 6 | Go Fiber Version | **Fiber v2 (v2.52.x)** — stable, well-documented, Go 1.23 |
| 7 | Team Size | Solo developer, working with DevOps-first approach |
| 8 | Scope | **No cuts** — attempt all features, but with explicit Plan B fallbacks for P1 features |
| 9 | Demo Strategy | Pre-loaded fantasy saga (2-3 chapters, ~20 characters, planted contradictions) with automatic session seeds and manual/daily reset endpoint |
| 10 | Day 1 Plan | Docker Compose + custom AGE/pgvector image + golang-migrate setup + agtype parser + transaction rollback test |
| 11 | Qwen Models | `qwen-max-latest` (batched), `qwen-turbo-latest`, `text-embedding-v3` (1024 dims) |
| 12 | License | MIT |
| 13 | AGE Go Driver | Raw SQL via `pgx` with `AfterConnect` hook executing `LOAD 'age'; SET search_path...` |
| 14 | Language | UI + demo content entirely in English |
| 15 | Auth | Skip — hardcoded user `demo@quill.ai` auto-login with JWT bypass |
| 16 | ECS | User configures account/instance; we provide deployment scripts, UptimeRobot, and health endpoint |
| 17 | Paragraph Addressing | TipTap stable node IDs (`paragraph_node_id`) instead of volatile indices |
| 18 | Testing Strategy | **TDD (Test-Driven Development)** for the 3 core algorithms (decay, entity resolution, contradictions) |

---

## 2. Technology Stack — Pinned Versions

> [!IMPORTANT]
> All versions chosen for stability, not bleeding-edge.

### Backend

| Technology | Version | Notes |
|---|---|---|
| **Go** | `1.23.x` | Latest stable LTS. Fiber v2 requires 1.21+ |
| **Go Fiber** | `v2.52.x` | Mature, WebSocket via `gofiber/contrib/websocket` |
| **pgx** | `v5.7.x` | Best Go PostgreSQL driver, native PG protocol |
| **golang-jwt** | `v5.2.x` | JWT handling |
| **uuid** | `github.com/google/uuid` | UUID generation |
| **bcrypt** | `golang.org/x/crypto/bcrypt` | Password hashing |

### Frontend

| Technology | Version | Notes |
|---|---|---|
| **React** | `18.3.x` | Stable, not React 19 (experimental features) |
| **Vite** | `6.x` | Latest stable build tool |
| **TipTap** | `2.11.x` | Rich text editor, well-maintained |
| **React Flow** | `11.11.x` | Knowledge Graph visualization |
| **Zustand** | `5.x` | Lightweight state management |
| **React Router** | `6.28.x` | Client-side routing |

### Database

| Technology | Version | Notes |
|---|---|---|
| **PostgreSQL** | `16` | Via Apache AGE image |
| **Apache AGE** | `1.6.0` | `apache/age:release_PG16_1.6.0` |
| **pgvector** | `0.8.x` | Installed on top of AGE image |

### AI / External

| Technology | Version | Notes |
|---|---|---|
| **Qwen-Max** | `qwen-max-latest` | Contradiction detection, complex reasoning |
| **Qwen-Turbo** | `qwen-turbo-latest` | Entity extraction, relationships, summaries |
| **text-embedding-v3** | latest | Embeddings, **1024 dimensions** (not 1536) |

### Infrastructure

| Technology | Version | Notes |
|---|---|---|
| **Docker** | `27.x` | Container runtime |
| **Docker Compose** | `2.30.x` | Orchestration |
| **Alibaba Cloud ECS** | `ecs.g6.large` | 2 vCPU, 8 GB RAM recommended |

---

## 3. Critical Issues Found in PRD/SRS

### 🔴 Critical Issues

#### Issue 1: Embedding Dimensions Mismatch
- **SRS says:** `vector(1536)` in pgvector tables
- **Decision:** Use `vector(1024)` — saves 33% storage, sufficient quality
- **Files to update:** MIGRATIONS, SRS section 3.1, config vars

#### Issue 2: Apache AGE + PostgreSQL Version
- **SRS says:** PostgreSQL 16 with custom Dockerfile building AGE from source
- **Reality:** AGE Docker Hub has `release_PG16_1.6.0` tag — no need to build from source
- **Decision:** Use official AGE image + compile pgvector on top
- **Files to update:** Dockerfile in SRS Appendix C, docker-compose.yml

#### Issue 3: Debounce Race Condition & Rate Limits
- **SRS says:** 2-3 second debounce, async analysis, no rate limiting
- **Problem:** Multiple concurrent analyses cause data duplication, contradictions, and excessive API consumption
- **Decision:** Sequential queue per chapter with `context.WithCancel`. Debounce raised to 5s. Separated Core Transaction (steps 1-4, atomic, not cancelable once started) from Enrichment (steps 5-10, cancelable). Qwen rate limit via semaphore.
- **Files to update:** SRS sections 5.3, 6.1; PRD sections 5.2, 8.2

#### Issue 4: Fiber v3 in SRS Architecture
- **SRS says:** "Go (Fiber)" without version
- **Context7 shows:** Fiber v3 requires Go 1.25+ (bleeding edge)
- **Decision:** Use Fiber v2 (v2.52.x) with Go 1.23
- **Files to update:** SRS section 1.2, implementation plan

#### Issue 5: Auth Complexity vs. Time & Demo Isolation
- **SRS specifies:** Full JWT auth with bcrypt, register/login/me endpoints
- **Problem:** Jueces sharing same demo user can overwrite each other's data
- **Decision:** Skip auth entirely (hardcoded user `demo@quill.ai` auto-login with JWT bypass) but isolate data by generating an automatic "demo session universe" for each new visitor. 
- **Implementation**: The backend exposes `POST /api/v1/demo/clone` which identifies the visitor via `X-Session-ID` header. It runs a fast SQL transaction cloning the seed template universe, mapping old IDs to new UUIDs (works, chapters, entities, mentions, embeddings) and copying AGE nodes/edges using raw Cypher queries. Daily cron resets the sessions.

### 🟡 Medium Issues

#### Issue 6: AGE Go Driver & agtype Parsing
- **SRS implies:** Direct Cypher queries via some driver
- **Reality:** No stable Go driver for AGE. Must use `pgx` with raw SQL wrapping Cypher. Cypher returns `agtype` (text with metadata suffix `::vertex`).
- **Decision:** Write custom parser package `pkg/agtype` on Day 1. Use `AfterConnect` hook in `pgxpool.Config` to execute `LOAD 'age'; SET search_path` for each connection. Keep all graph identifiers strictly tied to `entity_id` UUID instead of AGE internal IDs. Graph name sanitization: replace hyphens with underscores.

#### Issue 7: Decay Visualization Not Specified Enough
- **PRD says:** "semi-transparent" nodes
- **Decision:** Much more dramatic visualization for demo impact (translucent + dashed border + archive section + reactivation glow)

#### Issue 8: Database Migrations Tool
- **SRS Appendix C shows:** `docker-entrypoint-initdb.d` for migrations
- **Problem:** Cannot iterate on schemas without wiping Docker volumes.
- **Decision:** Use `golang-migrate` tool from Day 1. Reserve `docker-entrypoint-initdb.d` only for `seed.sql` demo content.

### 🟢 Minor Issues

#### Issue 9: Redundant Endpoints
- SRS lists both `/api/v1/` and `/api/` prefixes in different sections. Standardize to `/api/v1/`.

#### Issue 10: Contradiction Fingerprints & WS Auth
- **Problem:** Re-analyzing paragraphs triggers duplicate contradiction alerts. WS query string tokens can leak in server logs.
- **Decision:** Add unique `fingerprint` VARCHAR(64) to `contradictions` table (SHA-256 of entity_id + chapter_a_id + chapter_b_id) to skip duplicate alerts. In WS connections, send token in the first payload message `auth_init` instead of connection URL query parameter.
- **Paragraph Addressing:** Use stable TipTap node IDs (`paragraph_node_id`) instead of volatile index numbers. Persist it in **both** `ENTITY_MENTIONS` and `PARAGRAPH_EMBEDDINGS` tables to avoid misalignment during semantical similarity checks.

---

## 4. Updated Pseudo-Algorithm: Analysis Pipeline with Core vs Enrichment

```python
class AnalysisQueue:
    """
    One queue per chapter. When a new paragraph arrives,
    cancel any in-flight enrichment analysis and process the latest.
    """
    
    def __init__(self):
        self.chapter_contexts = {}  # chapter_id -> cancel_func
        self.chapter_locks = {}     # chapter_id -> mutex
    
    def submit(self, chapter_id, paragraph_data):
        # Cancel any in-flight enrichment analysis for this chapter
        if chapter_id in self.chapter_contexts:
            old_cancel = self.chapter_contexts[chapter_id]
            old_cancel()  # Triggers context cancellation
        
        # Create new cancellable context
        ctx, cancel = context.with_cancel(background_ctx)
        self.chapter_contexts[chapter_id] = cancel
        
        # Process asynchronously
        go self._process(ctx, cancel, chapter_id, paragraph_data)
    
    def _process(self, ctx, cancel, chapter_id, paragraph_data):
        try:
            # ──────────────────────────────────────────────────────────
            # FASE A: TRANSACCIÓN NÚCLEO (No cancelable una vez empezada)
            # ──────────────────────────────────────────────────────────
            if ctx.is_cancelled():
                return
            
            # 1. Extraer entidades (Qwen-Turbo)
            extracted = qwen_turbo.extract_entities(
                paragraph_data.text,
                get_universe_context(paragraph_data.universe_id)
            )
            
            # 2. Transacción de base de datos unificada (Unit-of-Work)
            tx = db.begin_transaction()
            try:
                resolved = []
                for entity_data in flatten(extracted):
                    # Búsqueda relacional + actualización en misma Tx
                    existing = find_existing_entity_tx(tx, paragraph_data.universe_id, entity_data)
                    if existing:
                        merged = merge_entity_info(existing, entity_data)
                        update_entity_tx(tx, merged)
                        update_graph_node_tx(tx, paragraph_data.universe_id, merged)  # AGE en Tx
                        resolved.append(merged)
                    else:
                        new_entity = create_entity_tx(tx, paragraph_data.universe_id, entity_data)
                        create_graph_node_tx(tx, paragraph_data.universe_id, new_entity)  # AGE en Tx
                        embedding = qwen_embedding.generate(entity_data.description)      # pgvector en Tx
                        save_entity_embedding_tx(tx, new_entity.id, embedding)
                        resolved.append(new_entity)
                        ws_send("entity_discovered", new_entity)
                
                # Registrar mención usando el paragraph_node_id de TipTap
                for entity in resolved:
                    create_mention_tx(tx, entity.id, chapter_id, paragraph_data.paragraph_index, paragraph_data.paragraph_node_id, paragraph_data.text)
                    relevance_service.touch_tx(tx, entity.id, chapter_id)
                
                tx.commit()  # Las 3 capas (Postgres + AGE + Vector) se confirman juntas
            except Exception as e:
                tx.rollback()
                raise e
            
            # ──────────────────────────────────────────────────────────
            # FASE B: COLA ENRIQUECEDORA (Cancelable en cada checkpoint)
            # ──────────────────────────────────────────────────────────
            
            if ctx.is_cancelled(): return
            # 5. Relaciones entre entidades (Qwen-Turbo)
            if len(resolved) > 1:
                relationships = qwen_turbo.analyze_relationships(paragraph_data.text, resolved)
                for rel in relationships:
                    create_or_update_graph_edge(paragraph_data.universe_id, rel)
                ws_send("graph_updated", get_graph_changes())
            
            if ctx.is_cancelled(): return
            # 6. Contextual recall
            memories = memory_service.recall_for_context(
                paragraph_data.universe_id, paragraph_data.text, resolved
            )
            ws_send("contextual_recall", memories)
            
            if ctx.is_cancelled(): return
            # 7. Contradiction detection (BATCHED & LIMITED)
            # Lanza chequeos en paralelo y agrupa en 1 sola llamada a Qwen-Max
            contradictions = contradiction_service.check_batch(
                paragraph_data.text, resolved, paragraph_data.universe_id, chapter_id
            )
            for c in contradictions:
                # save_contradiction valida el fingerprint único antes de insertar
                if save_contradiction(c):
                    ws_send("contradiction_alert", c)
            
            if ctx.is_cancelled(): return
            # 8. Timeline
            timeline_issues = timeline_service.validate(paragraph_data.text, resolved, paragraph_data.universe_id)
            for issue in timeline_issues:
                ws_send("timeline_inconsistency", issue)
            
            if ctx.is_cancelled(): return
            # 9. Plot holes
            plot_holes = plothole_service.scan(paragraph_data.universe_id, resolved)
            for ph in plot_holes:
                ws_send("plot_hole_detected", ph)
            
            if ctx.is_cancelled(): return
            # 10. Enviar resultado final
            ws_send("analysis_result", {
                "chapter_id": chapter_id,
                "paragraph_index": paragraph_data.paragraph_index,
                "paragraph_node_id": paragraph_data.paragraph_node_id,
                "entities_found": resolved,
                "analysis_duration_ms": elapsed()
            })
            
        except ContextCancelled:
            pass
        finally:
            if self.chapter_contexts.get(chapter_id) == cancel:
                del self.chapter_contexts[chapter_id]
```

---

## 5. Updated Docker Configuration

### Dockerfile for PostgreSQL (AGE + pgvector)

```dockerfile
# docker/postgres/Dockerfile
FROM apache/age:release_PG16_1.6.0

# Install pgvector on top of the AGE image
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        postgresql-server-dev-16 \
        git \
        ca-certificates && \
    git clone --branch v0.8.2 https://github.com/pgvector/pgvector.git /tmp/pgvector && \
    cd /tmp/pgvector && \
    make && \
    make install && \
    cd / && rm -rf /tmp/pgvector && \
    apt-get remove -y build-essential git && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# AGE already configures shared_preload_libraries
# pgvector doesn't need shared_preload_libraries
```

### docker-compose.yml (Updated)

```yaml
version: "3.9"

services:
  postgres:
    build:
      context: ./docker/postgres
      dockerfile: Dockerfile
    environment:
      POSTGRES_DB: quill
      POSTGRES_USER: quill
      POSTGRES_PASSWORD: ${DB_PASSWORD:-quill_dev_password}
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./docker/postgres/seed.sql:/docker-entrypoint-initdb.d/seed.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U quill"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  backend:
    build:
      context: ./backend
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://quill:${DB_PASSWORD:-quill_dev_password}@postgres:5432/quill?sslmode=disable
      QWEN_API_KEY: ${QWEN_API_KEY}
      QWEN_BASE_URL: https://dashscope-intl.aliyuncs.com/compatible-mode/v1
      QWEN_MAX_MODEL: qwen-max-latest
      QWEN_TURBO_MODEL: qwen-turbo-latest
      QWEN_EMBEDDING_MODEL: text-embedding-v3
      QWEN_EMBEDDING_DIMENSIONS: "1024"
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-in-production}
      PORT: "8080"
      FRONTEND_URL: ${FRONTEND_URL:-http://localhost:3000}
      ALLOWED_ORIGINS: ${FRONTEND_URL:-http://localhost:3000}
      DEBOUNCE_SECONDS: "5"
      QWEN_MAX_CONCURRENCY: "3"
      QWEN_TURBO_CONCURRENCY: "5"
      QWEN_RETRY_MAX_ATTEMPTS: "3"
      MIGRATIONS_DIR: "./migrations"
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - uploads:/app/uploads
      - ./backend/migrations:/app/migrations
    restart: unless-stopped

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
      args:
        VITE_API_URL: ${API_URL:-http://localhost:8080}
        VITE_WS_URL: ${WS_URL:-ws://localhost:8080}
    ports:
      - "3000:80"
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  pgdata:
  uploads:
```

---

## 6. Updated Implementation Schedule (DevOps-First)

> [!IMPORTANT]
> 11 days remaining. Solo developer. Every day counts.

| Day | Date | Focus | Deliverables |
|---|---|---|---|
| **1** | Jun 28 (today) | **Infrastructure & Validation** | Docker Compose + custom Dockerfile (AGE + pgvector) + `golang-migrate` setup. Build custom `pkg/agtype` parser package. Write integration test demonstrating cross-layer transaction rollback (Postgres entities + AGE Graph + Vector embedding). Go skeleton with `/health` and React skeleton. |
| **2** | Jun 29 | **Core Backend & TDD** | Define repos interface + implement pgx connection pool with `AfterConnect` hook. Start TDD for QwenService (mocked and real tests for batch embedding and generation). Go base router, CORS. |
| **3** | Jun 30 | **Unit-of-Work & Entity Engine** | Implement EntityService using Unit-of-Work (`pgx.Tx` shared) and GraphRepo (raw Cypher with underscores). Implement TDD tests for Entity Resolution matching (aliases, fuzzy). CRUD endpoints. |
| **4** | Jul 1 | **WS & Analysis Pipeline** | WebSocket Hub (reconnection support). Build AnalysisService with sequential queue + cancellation separating Core Transaction (no cancel) and Enrichment (cancellable). |
| **5** | Jul 2 | **Memory System (TDD)** | RelevanceService (TDD decay algorithm), ContradictionService (TDD fingerprint check, batched LLM check, candidates limit of 3, global semaphore rate limits). WS events. |
| **6** | Jul 3 | **Frontend Core** | TipTap editor setup with stable node UUIDs. WS client with exponential reconnections and automatic REST re-fetch state. MemoryPanel. |
| **7** | Jul 4 | **Knowledge Graph Visuals** | React Flow layout. Render nodes with opacity decay and dashed borders. Add collapsed Archive view, glow pulse animations on reactivation. |
| **8** | Jul 5 | **Ingestion Worker Pool** | IngestionService using concurrent worker pool (5 goroutines) and batched embeddings. Progress reports via WS. TimelineService and PlotHoles. |
| **9** | Jul 6 | **Demo Seed & Polish** | Automatic demo seed on visitor load. Endpoint `/api/v1/demo/reset` for manual/daily reset. Health endpoint improved + UptimeRobot setup. Fallback banner UI. |
| **10** | Jul 7 | **Deploy & E2E Validation** | ECS Deploy, deploy scripts. Run automated E2E script simulating the exact demo script. |
| **11** | Jul 8 | **Demo Video & Submitting** | Record 3-min demo video, build README with architecture diagrams. Final submission prep. |
| **Buffer** | Jul 9 AM | **Submit** | Final buffer before 2:00 PM PT. |

---

## 7. Hackathon Checklist Alignment

| Requirement | How We Meet It | Status |
|---|---|---|
| ✅ Qwen models on Qwen Cloud | Multi-model: qwen-max-latest + qwen-turbo-latest + text-embedding-v3 | Planned |
| ✅ Track MemoryAgent | Persistent memory (KG + vectors), timely forgetting (decay), recalling (contextual recall) | Core |
| ✅ Public open-source repo | GitHub with MIT license | Day 1 |
| ✅ Demo video ≤ 3 min on YouTube | Pre-loaded saga + live contradiction detection | Day 11 |
| ✅ Proof of Alibaba Cloud Deployment | Docker Compose on ECS + QwenService code link | Day 10 |
| ✅ Architecture Diagram | Mermaid in README + PNG image | Day 11 |
| ✅ Text description | README + PRD | Day 11 |
| ✅ Functioning demo URL | ECS public IP | Day 10 |
| ⭐ Blog post (bonus) | Journey building Quill | Day 11 |

---

## 8. Key Technical Risks

| Risk | Mitigation |
|---|---|
| AGE + pgvector conflict in same PostgreSQL | Test in Docker Day 1 — both are independent extensions, should work |
| Qwen API latency > 5s for analysis | Use Qwen-Turbo for most tasks (faster), Qwen-Max only for contradictions |
| GraphRepo verbosity (raw Cypher SQL) | Custom parser package `pkg/agtype` developed on Day 1 to abstract parsing. |
| 11 days solo is tight | Prioritize features by demo impact; cut timeline/plotholes first if needed. Use predefined demo path. |
| $40 API credits running out | 1024-dim embeddings + sequential queue + debounce 5s + batched contradictions check (max 3 candidates) keeps costs low (~$10 total estimated). |
| Cross-layer Transaction Rollback failure | Implement Unit-of-Work. Test Tx rollback (entities + AGE + pgvector) on Day 1. |
| AGE connection pool initialization | Initialize connection pool using `AfterConnect` hook executing `LOAD 'age'` automatically. |
| Duplicate contradictions alerts | Add unique SHA-256 fingerprint checking before saving contradictions. |
| Ingestion timeout | Implement concurrent worker pool (5 workers) and batched embeddings. |
| WS drop during judging | Implement exponential WS reconnect and REST fallback state fetch. |

---

## Open Questions

> [!IMPORTANT]
> **ECS Instance**: Please set up your Alibaba Cloud account and create an ECS instance (`ecs.g6.large`, Ubuntu 22.04, 2 vCPU, 8 GB RAM) ASAP. Open ports 22, 80, 3000, 8080. Install Docker + Docker Compose. Share the public IP when ready.

> [!NOTE]
> **Qwen API Key**: Make sure you have your `QWEN_API_KEY` from https://home.qwencloud.com/benefits. We'll need it for Day 2 when we start testing the QwenService.

