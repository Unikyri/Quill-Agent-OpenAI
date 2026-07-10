# implementation_plan_3 — Economía de tokens, dedup, persistencia, datos reales y CRUD

> **Entregable**: al aprobar este plan, el primer paso es guardar este documento como `Docs/implementation_plan_3.md` (secuela de `implementation_plan_2.md`, misma convención en español).

## 0. Contexto

Cinco problemas reportados: (1) ingesta de un documento de 43 páginas consume ~1.5M tokens y queda incompleta, (2) re-subir el mismo documento re-analiza todo, (3) al recargar la página desaparece el registro del documento y no se guardan entidades/embeddings, (4) la pestaña Memory muestra valores que parecen estáticos (Entities 9,000 / Vector 11,200 / Tools 7,000 / 6.6667%), (5) faltan operaciones Delete en Mundos/Capítulos.

La exploración del código real (sin CodeGraph) confirmó los síntomas pero **corrigió varias hipótesis** — el diagnóstico exacto está abajo. Cada tarea indica archivo, función, línea y verificación.

---

## 1. Diagnóstico técnico (evidencia file:line)

### 1.1 La fuga de tokens NO está en la ingesta — está en el análisis por párrafo

- El worker de ingesta (`backend/internal/services/ingestion_service.go:135-223`) hace por chunk: 1× `ExtractEntities` (qwen-turbo, chunk completo ≤50K chars, `:201`) + 1 embedding HTTP **por párrafo** (`:188`, sin batching) + embeddings de nombres en `entity_service.go:131,196`. Total ≈ **~70K tokens** para 43 páginas. No es la explosión.
- La explosión es `AnalysisService.processJob` (`analysis_service.go`), disparado **por párrafo** vía WebSocket (`ws/hub.go:283` → `SubmitParagraph`). Por párrafo:
  - 2× qwen-turbo: `ExtractEntities` (`analysis_service.go:435`) + `AnalyzeRelationships` (`:283`)
  - `CheckSemantic` → `RunAgentLoop(Stream)` **qwen-max, profundidad 5 hardcodeada** (`contradiction_service.go:216,218`); el historial de mensajes crece en cada iteración (`qwen_service.go:604-621`)
  - `PlotHoleService.Scan` (`analysis_service.go:372`) — **sin gate**: corre aunque el párrafo no tenga entidades, y por cada entidad "stale" ejecuta un agent loop qwen-max profundidad 3 (`plot_hole_service.go:160`)
  - Timeline: loop profundidad 2 solo con eventos explícitos (`timeline_service.go:101`)
- Matemática: ~250 párrafos × (~5 turnos qwen-max × ~12K tokens + plot-holes S×3×~2K + 2 turbo) ≈ **1.5M tokens** — coincide con el número observado.
- **No hay dedup de párrafos**: si el editor (debounce de TipTap) o una reconexión re-envía el mismo párrafo, se repite el fan-out completo (~60K tokens) por texto idéntico.
- Nota: la lista de entidades en el prompt de contradicciones **ya está capeada** por presupuesto (`buildEntityLines`, `contradiction_service.go:284-312`, con `budgetMgr` cableado en `main.go:120`) — no hay que tocarla.
- Embeddings: `GenerateEmbeddingBatch` existe (`qwen_service.go:339-361`) pero **nadie lo usa**; la ingesta llama `GenerateEmbedding` de a uno.

### 1.2 Sin dedup de documentos

`ingestion_jobs` (migración 012) no tiene columna de hash. `IngestionService.Start` (`ingestion_service.go:70-129`) ya tiene los bytes completos en memoria (`io.ReadAll`, `:76`) pero crea un job nuevo siempre. `crypto/sha256` ya se usa en el repo (`contradiction_service.go:5`).

### 1.3 Persistencia rota — el bug silencioso central

- `SaveParagraphEmbedding(ctx, uuid.Nil, ...)` en `ingestion_service.go:193` **viola siempre el FK** `paragraph_embeddings.chapter_id NOT NULL REFERENCES chapters(id)` (migración 008). El error se loguea y se traga (`:194-195`): **ningún embedding de ingesta se guarda jamás**.
- La ingesta **nunca crea filas `chapters`** (solo un Work "Imported Manuscript", `:102-118`) → el contenido del documento no se persiste → "el documento desaparece al recargar".
- Los contadores `total_chapters_detected` / `chapters_processed` / `entities_extracted` existen en schema y modelo pero **nunca se escriben** (`ingestion_repo.go` `UpdateStatus` `:75-91` los omite).
- **No existe GET de jobs**: única ruta `POST /universes/:id/ingest` (`main.go:221`). `IngestionRepo.FindByID` no tiene callers. El frontend guarda jobs en `useState` (`IngestPage.tsx:36`) hidratado solo por WS (`wsStore.ts:155-161`) → se pierde al recargar.
- Corrección de hipótesis: el evento WS terminal `completed` **SÍ se emite** (`ingestion_service.go:222`); el comentario en `IngestPage.tsx:116-122` está desactualizado — la página no lee `progress.status`.

### 1.4 La pestaña Memory NO está mockeada — datos reales con entrada degenerada

- `BudgetTheater` renderiza `recall.budget` real de `POST /universes/:id/recall/explain`. Los números "estáticos" son `ComputeBudget(0, 0)` en `memory_service.go:546` (`budgetSurvivors`): available = 30000 − 2000 = 28000 → split 35/40/25 (`context_budget.go:75-87`) = Entities 9,800 / Vector 11,200 / Tools 7,000; UsedPercent = 2000/30000 = 6.6667%. **Mismos números para cualquier query** porque nunca se pasan los tokens reales.
- `FusionExplorer` llama `api.recallExplain` de verdad (`FusionExplorer.tsx:33`). El pipeline vector se ve vacío **porque los embeddings de ingesta nunca se guardaron** (§1.3) — arreglar la persistencia arregla este síntoma.
- Quirk adicional: `POST /recall` (handler `graph.go:187`) pasa embedding `nil` — ignora el texto de la query. Sin caller en el frontend hoy; queda anotado.

### 1.5 CRUD — el diagnóstico real

- **Universos SÍ se pueden eliminar** end-to-end (`UniverseLayout.tsx:128-140` → `DELETE /universes/:id`, `main.go:187`; cascadas FK verificadas en todas las tablas, migraciones 003-019). **PERO** `UniverseService.Delete` (`universe_service.go:177-189`) nunca borra el grafo AGE: `GraphRepo.DropGraph` (`graph_repo.go:392-400`) tiene **cero callers** y además solo hace `MATCH (n) DETACH DELETE n` (vacía nodos, no dropea el grafo ni sus tablas label en `ag_catalog`) → **cada universo eliminado filtra un grafo AGE para siempre**.
- Lo que el usuario llama "Mundos con título/sinopsis/portada" son **Works** (`UniverseWorksTab.tsx`). Backend `DELETE /works/:id` y `DELETE /chapters/:id` **ya existen** (`main.go:193-194,201`); lo que falta es `deleteWork`/`deleteChapter` en `frontend/src/lib/api.ts` y los botones de UI.
- Hallazgo extra: la imagen de portada del Work es `useState` local, **nunca se persiste** (no hay columna `cover_url`; confirmado en `WorkPage.tsx:49-50`). Fuera de alcance de este plan — anotado como follow-up.

---

## 2. Matemática de tokens — antes / después

Documento de 43 páginas ≈ 100K chars ≈ 26K tokens, ~250 párrafos.

| Concepto | Antes | Después |
|---|---|---|
| Ingesta (extract + embeddings) | ~70K tokens, ~250 llamadas HTTP de embedding | ~70K tokens, **~25 llamadas** (batch de 10) |
| Análisis por párrafo (editor) | ~62K tok/párrafo (loop qwen-max ×5 + plot-holes sin gate) → **~1.5M** por pasada completa | ~36K tok/párrafo (loop ×3, plot-holes con gate) → **~0.9M**; párrafos sin entidades ≈ ~1.3K |
| Párrafo re-enviado idéntico (debounce/reconexión) | fan-out completo repetido (~62K c/u) | **0** (dedup por hash en memoria) |
| Re-subir el mismo documento | todo de nuevo (~70K + análisis) | **0** (dedup por sha256, HTTP 200 "duplicate") |

La palanca restante más grande (rediseñar el análisis por párrafo a por-capítulo con debounce) es un cambio de comportamiento del producto — queda como follow-up explícito si 0.9M por pasada completa sigue siendo mucho.

---

## 3. Decisiones de diseño

- **D1 Capítulos en ingesta**: una fila `chapters` por chunk del split, bajo el Work importado; `order_index = GetMaxOrderIndex(workID) + i + 1`, `content = raw_text = chunk`, `status = "draft"`. Los embeddings usan el ID real del capítulo (se elimina el hack `i*1000+pIdx`). La ingesta NO dispara `AnalysisService` (el análisis pesado queda para el editor).
- **D2 Batching**: `GenerateEmbeddingBatch` en slices de 10 (límite de batch de DashScope text-embedding-v3). Error de batch → log + skip del slice (misma semántica best-effort de hoy).
- **D3 Knobs**: `CONTRADICTION_AGENT_DEPTH` (default 3) y `PLOT_HOLE_AGENT_DEPTH` (default 2) como env vars en `config.go`. Reversible por configuración si baja la calidad del análisis.
- **D4 Dedup de documento**: sha256 hex de los bytes; índice único parcial `(universe_id, content_hash) WHERE status <> 'failed'` (constraint en DB, no en app; jobs fallidos se pueden reintentar). Respuesta HTTP 200 `{"job_id": ..., "status": "duplicate"}` con el job existente. Sin mensaje WS nuevo.
- **D5 Dedup de párrafo (análisis)**: set en memoria `map[string]struct{}` en `AnalysisService` con mutex, clave `sha256(chapterID|text)`; si ya se analizó texto idéntico del mismo capítulo, se salta el job completo. Cap simple: al superar 4096 entradas se vacía el map (ponytail). Tradeoff aceptado: un párrafo idéntico no se re-analiza aunque el lore haya cambiado — cualquier edición del texto cambia el hash y re-analiza.
- **D6 Presupuesto real**: pasar `tok.CountTokens(query)` a `ComputeBudget` y exponer `tokensUsed` de `FitToBudget` como `vector_tokens_used` en `BudgetReport` → los números del BudgetTheater se mueven por query.
- **D7 Drop AGE**: `DropGraph` pasa a `SELECT ag_catalog.drop_graph('<name>', true)` vía `withAgeConn` (respetando la regla del CLAUDE.md: todo AGE por los helpers), ignorando "does not exist"; `UniverseService.Delete` lo llama best-effort post-commit.
- **D8 Migración**: la última es 019 → nueva pareja `020_add_ingestion_content_hash.{up,down}.sql`.

---

## 4. Tareas

### Fase 0 — Entregable
**T0.** Guardar este documento como `Docs/implementation_plan_3.md`.

### Fase 1 — Economía de tokens (sin schema, wins inmediatos)

**T1. Knobs de profundidad** — `backend/internal/config/config.go`
- Agregar campos `ContradictionAgentDepth int` y `PlotHoleAgentDepth int` al struct `Config`.
- En `Load()`: `ContradictionAgentDepth: getEnvInt("CONTRADICTION_AGENT_DEPTH", 3)`, `PlotHoleAgentDepth: getEnvInt("PLOT_HOLE_AGENT_DEPTH", 2)` (reusar el helper `getEnvInt` existente).
- Documentar ambas en `.env.example` (valores placeholder).
- Verificar: `cd backend && go build ./...`

**T2. Profundidad en contradicciones** (depende T1) — `backend/internal/services/contradiction_service.go`
- Agregar campo `agentDepth int` al struct + parámetro en `NewContradictionService` (`:36`).
- Reemplazar el literal `5` en `:216` y `:218` por `s.agentDepth`.
- `backend/cmd/server/main.go:120`: pasar `cfg.ContradictionAgentDepth`.
- Actualizar constructores en tests (`rg -n 'NewContradictionService' backend/`).
- Verificar: `go test ./internal/services/ -run Contradiction`

**T3. Profundidad en plot holes** (depende T1) — `backend/internal/services/plot_hole_service.go`
- Ídem T2: campo + parámetro en `NewPlotHoleService` (`:25`); reemplazar el literal `3` en `:160`.
- `main.go:118`: pasar `cfg.PlotHoleAgentDepth`.
- Verificar: `go test ./internal/services/ -run PlotHole`

**T4. Gate de plot holes por entidades** — `backend/internal/services/analysis_service.go:372`
- Cambiar `if s.plotHoleSvc != nil {` por `if s.plotHoleSvc != nil && len(resolvedEntities) > 0 {`.
- Justificación: un párrafo sin entidades no toca relevancia (`Touch`, `:269-275`), así que el scan devolvería lo mismo que el anterior — es puro gasto qwen-max (S entidades stale × 3 turnos por CADA párrafo). Mismo patrón de gate que ya usan los pasos 2 y 4 (`:259`, `:353`).
- Verificar: `go test ./internal/services/ -run Analysis`

**T5. Dedup de párrafos idénticos en análisis** — `backend/internal/services/analysis_service.go`
- Agregar al struct `AnalysisService`: `seenMu sync.Mutex` y `seen map[string]struct{}` (inicializar en el constructor).
- Al inicio de `processJob`: computar `key := sha256(job.ChapterID.String() + "|" + job.Text)` (hex); con el mutex, si existe → log `[analysis] skip duplicate paragraph` y devolver resultado vacío sin llamar a ningún servicio; si no, registrar la clave. Si `len(seen) > 4096`, vaciar el map antes de insertar.
- Esto elimina el fan-out repetido (~60K tokens) cuando el debounce del editor o una reconexión WS re-envía texto sin cambios — la causa más probable del "bucle que reenvía todo" que se sospechaba.
- Verificar: test unitario nuevo: dos `processJob` con el mismo texto/capítulo → el fake de Qwen registra UNA sola llamada a `ExtractEntities`.

**T6. Batching de embeddings en ingesta** — `backend/internal/services/ingestion_service.go`
- Extender la interfaz `IngestionQwen` (`:29-32`) con `GenerateEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error)` (QwenService ya lo implementa, `qwen_service.go:339`). Actualizar fakes de tests.
- En `runWorker`, bloque de embeddings (`:176-197`): juntar los párrafos elegibles (trim no vacío, ≤ `maxEmbedChars`) en un slice y llamar `GenerateEmbeddingBatch` en tandas de 10; guardar cada resultado con su párrafo. (El chapter ID correcto llega con T8.)
- Verificar: `go test ./internal/services/ -run Ingestion`

### Fase 2 — Dedup de documentos (idempotencia)

**T7. Migración + repo + short-circuit**
- Nuevos archivos `backend/migrations/020_add_ingestion_content_hash.up.sql`:
  ```sql
  ALTER TABLE ingestion_jobs ADD COLUMN content_hash CHAR(64);
  CREATE UNIQUE INDEX uq_ingestion_jobs_universe_hash
      ON ingestion_jobs(universe_id, content_hash)
      WHERE status <> 'failed' AND content_hash IS NOT NULL;
  ```
  y su `.down.sql` (`DROP INDEX IF EXISTS ...; ALTER TABLE ... DROP COLUMN IF EXISTS content_hash;`).
- `backend/internal/repositories/ingestion_repo.go`:
  - `Create` (`:26-36`): parámetro `contentHash string`, columna `content_hash` en el INSERT con `NULLIF($n,'')`.
  - Nuevo `FindByContentHash(ctx, universeID, hash) (*models.IngestionJob, error)`: mismo SELECT que `FindByID` con `WHERE universe_id=$1 AND content_hash=$2 AND status <> 'failed' ORDER BY created_at DESC LIMIT 1`; `nil, nil` en `pgx.ErrNoRows`.
  - Agregar `ContentHash` al modelo (`models.go:127-141`).
- `backend/internal/services/ingestion_service.go` `Start` (`:70-129`):
  - Tras `io.ReadAll` (`:76`): `sum := sha256.Sum256(content); hash := hex.EncodeToString(sum[:])` (imports `crypto/sha256`, `encoding/hex`).
  - Cambiar firma a `Start(...) (jobID uuid.UUID, duplicate bool, err error)`.
  - Antes de crear work/job: `FindByContentHash`; si existe → `return existing.ID, true, nil`.
  - Pasar `hash` a `repo.Create` (`:121`); si falla con unique violation (código `23505` vía `errors.As` + `*pgconn.PgError` — carrera entre dos uploads), re-consultar `FindByContentHash` y devolver duplicate.
- `backend/internal/handlers/ingestion_handler.go`: actualizar la interfaz `IngestionStarter` (`:12-14`) y `Ingest`: con `duplicate == true` responder `200 {"job_id": ..., "status": "duplicate"}`; si no, `202` como hoy. Actualizar tests de handler y servicio.
- Verificar: `go test ./internal/...`; manual: subir el mismo `.md` dos veces → la segunda devuelve `"status":"duplicate"` con el mismo `job_id` y no dispara worker (cero llamadas Qwen en logs).

### Fase 3 — Persistencia + recarga

**T8. Capítulos reales + contadores** — `backend/internal/services/ingestion_service.go` + `ingestion_repo.go`
- `runWorker` (`:135-223`):
  1. Tras `splitChunks` (`:154`): `chRepo := repositories.NewChapterRepo(s.pool)` (guard `s.pool != nil`); `baseOrder := chRepo.GetMaxOrderIndex(ctx, workID)`; persistir `total_chapters_detected = len(chunks)` con el nuevo `UpdateProgress`.
  2. Por chunk `i`, ANTES del bloque de embeddings: crear `models.Chapter{ID: uuid.New(), WorkID: workID, Title: ch.title, OrderIndex: baseOrder + i + 1, Content: ch.content, RawText: ch.content, WordCount: <helper existente de conteo>, Status: "draft"}` vía `chRepo.Create`. Si falla: log + `continue` (sin FK válido no hay embeddings para ese chunk).
  3. Bloque de embeddings (ya batcheado por T6): `SaveParagraphEmbedding(ctx, chapter.ID, pIdx, ch.title, p, emb)` — chapter ID real y `pIdx` simple; **eliminar `uuid.Nil` y el hack `i*1000+pIdx`** (`:193`) y el comentario ponytail `:173-175`.
  4. Junto a cada `emitProgress` (`:160,:205,:212,:217,:222`): llamar `UpdateProgress` para que la DB refleje lo mismo que el WS. Contar entidades sumando lo devuelto por `resolveAndBuildGraph` (hacer que devuelva el count).
- `ingestion_repo.go`: nuevo `UpdateProgress(ctx, jobID uuid.UUID, totalDetected, processed, entities int) error` → `UPDATE ingestion_jobs SET total_chapters_detected=$2, chapters_processed=$3, entities_extracted=$4 WHERE id=$1`.
- Verificar: subir un `.md` con headers `# Capítulo` → `psql`: `SELECT title, order_index FROM chapters ORDER BY order_index;` y `SELECT count(*) FROM paragraph_embeddings;` no vacíos; contadores de `ingestion_jobs` poblados; recargar la página del Work → capítulos visibles con contenido.

**T9. Endpoint GET de jobs** (depende T8 para contadores; no bloqueante)
- `ingestion_repo.go`: `ListByUniverse(ctx, universeID) ([]models.IngestionJob, error)` — columnas de `FindByID`, `WHERE universe_id=$1 ORDER BY created_at DESC LIMIT 50`.
- `ingestion_service.go`: `ListJobs(ctx, universeID)` → delega al repo (guard nil-pool → slice vacío).
- `ingestion_handler.go`: extender la interfaz con `ListJobs`; método `Jobs(c *fiber.Ctx)` → `{"jobs": [...]}` (mismo estilo de errores que `:33-37`).
- `cmd/server/main.go`, después de `:221`: `api.Get("/universes/:id/ingestions", ingestionH.Jobs)`.
- Verificar: `curl -H "Authorization: Bearer $T" localhost:8080/api/v1/universes/$U/ingestions`.

**T10. Hidratación del frontend** (depende T9)
- `frontend/src/lib/api.ts`, tras `ingestDocument` (`:173`): `listIngestionJobs: (universeId: string) => request<{ jobs: IngestionJobDTO[] }>(...)` (tipar en `types.ts`, sin `any`).
- `frontend/src/pages/IngestPage.tsx`:
  - `useEffect` al montar: `api.listIngestionJobs(universeId)` → sembrar el estado `jobs` (`:36`) con `{jobId, filename, status, processed: chapters_processed, total: total_chapters_detected}`.
  - Estado terminal: leer `progress.status === 'completed'` del WS (SÍ se emite) o el `status` hidratado; eliminar la inferencia por conteo (`:122`) y los comentarios desactualizados (`:48-55`, `:116-121`).
  - `handleCheckStatus` (`:53`): re-fetch del GET en vez de reconectar el WS.
  - Manejar `status: "duplicate"` del upload (T7): mostrar el job existente en lugar de agregar tarjeta nueva.
- Verificar: subir, recargar a mitad de proceso → la tarjeta reaparece con progreso real y termina en `completed`. `npm run build` (typecheck).

### Fase 4 — Números reales en BudgetTheater

**T11. Presupuesto con tokens reales** — backend + frontend
- `backend/internal/services/memory_service.go`:
  - `budgetSurvivors` (`:545-553`): nueva firma `(ranked []RankedItem, queryTokens int) (map[string]bool, BudgetAllocation, int)`; `ComputeBudget(0, queryTokens)`; devolver el `tokensUsed` de `FitToBudget`.
  - `fitToBudget` (`:558`) y su caller (`:180`): pasar `s.budgetMgr.tok.CountTokens(queryText)`.
  - `RecallExplain` (`:400`): ídem, y setear `report.VectorTokensUsed = tokensUsed`.
- `backend/internal/services/context_budget.go` `BudgetReport` (`:24-31`): agregar `VectorTokensUsed int \`json:"vector_tokens_used"\``.
- `frontend/src/lib/types.ts`: agregar `vector_tokens_used` al tipo del budget.
- `frontend/src/components/memory/BudgetTheater.tsx`: en la fila Vector mostrar `used / alloc` (`${budget.vector_tokens_used.toLocaleString()} / ${value.toLocaleString()} tok`).
- Actualizar `context_budget_test.go` y tests del memory service.
- Verificar: `go test ./internal/services/`; en UI, dos queries de distinta longitud en Fusion Explorer muestran números distintos; con T8 aplicado, el pipeline vector deja de estar vacío.

### Fase 5 — Deletes + limpieza del grafo AGE

**T12. Drop real del grafo AGE al eliminar universo**
- `backend/internal/repositories/graph_repo.go` `DropGraph` (`:392-400`): reemplazar el cuerpo por `SELECT ag_catalog.drop_graph('<graphName>', true)` ejecutado vía `withAgeConn` (nombre derivado de UUID = seguro por construcción, mismo criterio del resto del repo); si el error contiene `does not exist` → `return nil`.
- `backend/internal/services/universe_service.go` `Delete` (`:177-189`): tras el commit, best-effort `if err := s.graphRepo.DropGraph(ctx, "universe_"+id.String()); err != nil { log.Printf(...) }` (`graphRepo` ya está en el struct, `:67`).
- Verificar: crear universo con entidades → eliminarlo → `psql -c "SELECT name FROM ag_catalog.ag_graph;"` sin el grafo; eliminar un universo sin grafo → sin error.

**T13. Delete de Works y Chapters en el frontend** (backend ya existe: `main.go:193-194,201`)
- `frontend/src/lib/api.ts`: `deleteWork: (id) => request<void>(\`/works/\${id}\`, { method: 'DELETE' })` tras `updateWork` (`:75`); ídem `deleteChapter` tras `updateChapter` (`:84`) — copiar el estilo de `deleteUniverse` (`:65`).
- `frontend/src/pages/UniverseWorksTab.tsx`: botón 🗑️ por work (junto al listado `:353-364`) y por capítulo (junto al botón de rename `:250-256`), copiando el patrón `handleDeleteUniverseFor` (`UniverseLayout.tsx:128-140`): `window.confirm` → `api.deleteX` → refetch. Sin librería de toasts (no existe en el proyecto).
- Verificar: eliminar un work → desaparece y sus capítulos cascadean (FK migración 004); eliminar un capítulo → la lista se actualiza. `npm run build`.

---

## 5. Orden de ejecución y dependencias

```
T0 → T1 → {T2, T3} · T4 · T5 · T6   (Fase 1, paralelizables tras T1)
T7                                   (Fase 2)
T8 → T9 → T10                        (Fase 3; T8 aprovecha T6)
T11                                  (Fase 4, independiente)
T12 · T13                            (Fase 5, independientes)
```

Verificación final end-to-end: `docker compose down -v && docker compose up -d` (regla del CLAUDE.md tras correr la suite contra el Postgres de compose), subir un documento de prueba con varios `# headers`, confirmar: capítulos + embeddings + contadores en DB, tarjeta de job sobrevive a recarga, segundo upload = duplicate, Fusion Explorer con pipeline vector poblado y budget que varía por query, delete de work/chapter/universo (grafo AGE incluido). Suite completa: `cd backend && go test ./...` y `cd frontend && npm run build && npm run test`.

## 6. Omitido deliberadamente (follow-ups anotados)

- **Persistencia de la portada del Work** (hoy es estado local que se pierde al recargar — el usuario cree que funciona): requiere columna `cover_url` + manejo de upload. Añadir si se pide.
- **Rediseño análisis por-capítulo con debounce**: la palanca de tokens más grande que queda (~0.9M → ~0.1M por pasada), pero cambia el comportamiento del producto (feedback por párrafo en vivo). Decidir después de medir con T1-T5.
- Knob de profundidad del timeline (ya es 2), knobs de chunk-size, librería de toasts.
