package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// ── Ingestion service ──

// ingestionChunk represents a parsed section of the uploaded document.
type ingestionChunk struct {
	title   string
	content string
}

const (
	ingestionRelationshipTimeout     = 15 * time.Second
	ingestionDecayTimeout            = 30 * time.Second
	ingestionRelationshipCorpusLimit = 12_000
	ingestionRelationshipEntityLimit = 40
	ingestionFallbackEdgeLimit       = 100
)

// IngestionQwen is the minimal Qwen interface used by IngestionService.
// QwenService satisfies this interface.
type IngestionQwen interface {
	ExtractEntities(ctx context.Context, text, universeContext string) (*ExtractedEntities, error)
	AnalyzeRelationships(ctx context.Context, text string, entityNames []string) ([]map[string]interface{}, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// relationshipEdgeWriter keeps relationship persistence testable without
// bypassing GraphRepo in production. GraphRepo remains the default writer and
// therefore retains AGE search-path and Cypher identifier protections.
type relationshipEdgeWriter interface {
	CreateEdge(ctx context.Context, graphName, sourceEntityID, targetEntityID, relType string, properties map[string]interface{}) error
}

// ingestionRelevance records a real imported mention and advances the
// chapter-aware decay clock only after that chapter's mentions are durable.
// RelevanceService satisfies this narrow interface; it remains optional for
// older test seams and degraded deployments.
type ingestionRelevance interface {
	Touch(context.Context, uuid.UUID, uuid.UUID) error
	DecayExcept(context.Context, uuid.UUID, []uuid.UUID) error
}

// IngestionService processes document uploads asynchronously:
// file → chunk by headers → extract entities → embed → graph.
//
// ponytail: one goroutine per job, sequential chunk processing. No worker pool
// needed for hackathon scale. Cancel via ctx.Done().
type IngestionService struct {
	pool       *pgxpool.Pool
	entitySvc  *EntityService
	vectorRepo *repositories.VectorRepo
	graphRepo  *repositories.GraphRepo
	// relationshipEdges is a test seam; nil uses graphRepo.
	relationshipEdges relationshipEdgeWriter
	qwenSvc           IngestionQwen
	hub               AnalysisHub
	relevance         ingestionRelevance

	// Post-ingest bounded analysis (D4) — all nil-safe, wired via
	// SetPostIngestAnalysis. Unset means analysis is silently skipped.
	contraSvc           postIngestContradictionAnalyzer
	plotHoleSvc         postIngestPlotHoleAnalyzer
	analysisBudgetMgr   *ContextBudgetManager
	analysisMaxChapters int
	progressNow         func() time.Time
	newProgressTicker   func(time.Duration) ingestionTicker
	stylometrySvc       WriterCorpusObservationSink
	timelineRepo        *repositories.TimelineRepo
}

// SetTimelineRepo wires timeline event creation for imported manuscripts.
// Optional — nil-safe; createTimelineEventsForChapter silently skips without
// it, same as this service's other optional dependencies.
func (s *IngestionService) SetTimelineRepo(timelineRepo *repositories.TimelineRepo) {
	s.timelineRepo = timelineRepo
}

// SetStylometry wires the corpus-wide cold-start pass. It is optional so
// existing ingestion tests and deployments can remain unchanged.
func (s *IngestionService) SetStylometry(stylometry WriterCorpusObservationSink) {
	s.stylometrySvc = stylometry
}

// SetRelevance wires canonical relevance accounting into imported mentions.
// It is setter-based to keep the constructor stable for callers and tests.
func (s *IngestionService) SetRelevance(relevance ingestionRelevance) {
	s.relevance = relevance
}

// NewIngestionService creates an IngestionService. All parameters may be nil
// for testing; Start will create a job ID but the worker will be a no-op.
func NewIngestionService(
	pool *pgxpool.Pool,
	entitySvc *EntityService,
	vectorRepo *repositories.VectorRepo,
	graphRepo *repositories.GraphRepo,
	qwenSvc IngestionQwen,
	hub AnalysisHub,
) *IngestionService {
	return &IngestionService{
		pool:       pool,
		entitySvc:  entitySvc,
		vectorRepo: vectorRepo,
		graphRepo:  graphRepo,
		qwenSvc:    qwenSvc,
		hub:        hub,
	}
}

// supportedFileTypes are the extensions parseDocument can handle. Checked
// synchronously in Start (before any I/O) so unsupported uploads (legacy
// .doc, unknown formats) get an immediate 400 instead of a garbage job row.
var supportedFileTypes = map[string]bool{"md": true, "txt": true, "docx": true, "pdf": true}

func provisionalWorkTitle(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

// ErrUnsupportedFileType is returned by Start when filename's extension isn't
// one of supportedFileTypes.
var ErrUnsupportedFileType = errors.New("unsupported file type — only .md, .txt, .docx, and .pdf are supported (legacy .doc? Save as .docx)")

// Start creates an ingestion job and kicks off the async pipeline.
// Returns the job ID immediately; duplicate is true when the same content
// was already ingested into this universe (the existing job's ID is
// returned and no worker is started). The caller should return 202 Accepted
// for new jobs and 200 for duplicates.
func (s *IngestionService) Start(ctx context.Context, universeID uuid.UUID, reader io.Reader, filename string) (uuid.UUID, bool, error) {
	return s.StartForWork(ctx, universeID, uuid.Nil, reader, filename)
}

// StartForWork starts an import for a specific Work when targetWorkID is set.
// A missing target deliberately creates a new Work: an uploaded manuscript is
// a distinct artifact, not extra chapters for whichever Work happens to sort
// first in a universe.
func (s *IngestionService) StartForWork(ctx context.Context, universeID, targetWorkID uuid.UUID, reader io.Reader, filename string) (uuid.UUID, bool, error) {
	fileType := fileTypeOf(filename)
	if !supportedFileTypes[fileType] {
		return uuid.Nil, false, ErrUnsupportedFileType
	}

	jobID := uuid.New()

	// ponytail: read the full content synchronously before spawning the
	// goroutine. The handler's file.Close() runs as soon as Start returns, so
	// passing the io.Reader to a goroutine would read from a closed handle.
	content, err := io.ReadAll(reader)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("read uploaded file: %w", err)
	}

	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	workTitle := provisionalWorkTitle(filename)
	// Title extraction is intentionally best-effort here. Invalid documents
	// still become a durable failed ingestion job in runWorker, while valid
	// metadata makes the newly-created Work readable immediately.
	if parsed, parseErr := parseDocumentDetails(filename, content); parseErr == nil && parsed.title != "" {
		workTitle = parsed.title
	}

	var workID uuid.UUID
	createdWork := false
	if s.pool != nil {
		repo := repositories.NewIngestionRepo(s.pool)
		existing, err := repo.FindByContentHash(ctx, universeID, hash)
		if err != nil {
			return uuid.Nil, false, fmt.Errorf("check duplicate ingestion: %w", err)
		}
		if existing != nil {
			return existing.ID, true, nil
		}

		workRepo := repositories.NewWorkRepo(s.pool)
		if targetWorkID != uuid.Nil {
			if _, err := workRepo.FindByIDInUniverse(ctx, targetWorkID, universeID); err != nil {
				return uuid.Nil, false, fmt.Errorf("resolve target work: %w", err)
			}
			workID = targetWorkID
		} else {
			tx, err := s.pool.Begin(ctx)
			if err != nil {
				return uuid.Nil, false, fmt.Errorf("begin transaction: %w", err)
			}
			orderIdx, err := workRepo.GetMaxOrderIndex(ctx, universeID)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.Nil, false, fmt.Errorf("get max order index: %w", err)
			}
			work := models.Work{
				ID:         uuid.New(),
				UniverseID: universeID,
				Title:      workTitle,
				Type:       "novel",
				Status:     "in_progress",
				OrderIndex: orderIdx + 1,
			}
			if err := workRepo.Create(ctx, tx, &work); err != nil {
				_ = tx.Rollback(ctx)
				return uuid.Nil, false, fmt.Errorf("create default work: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return uuid.Nil, false, fmt.Errorf("commit transaction: %w", err)
			}
			workID = work.ID
			createdWork = true
		}

		if err := repo.Create(ctx, jobID, universeID, workID, "pending", filename, fileType, hash); err != nil {
			// Unique violation: another upload of the same content won the
			// race between our FindByContentHash and this insert.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				if existing, ferr := repo.FindByContentHash(ctx, universeID, hash); ferr == nil && existing != nil {
					return existing.ID, true, nil
				}
			}
			return uuid.Nil, false, fmt.Errorf("create ingestion job: %w", err)
		}
	}

	go s.runWorker(jobID, universeID, workID, createdWork, content, filename)

	return jobID, false, nil
}

// ListJobs returns the recent ingestion jobs for a universe.
func (s *IngestionService) ListJobs(ctx context.Context, universeID uuid.UUID) ([]models.IngestionJob, error) {
	if s.pool == nil {
		return []models.IngestionJob{}, nil
	}
	return repositories.NewIngestionRepo(s.pool).ListByUniverse(ctx, universeID)
}

// ingestedChapter tracks a persisted chapter and the entities resolved from
// it, collected during runWorker's chunk loop for the post-ingest analysis
// pass (SetPostIngestAnalysis).
type ingestedChapter struct {
	ID       uuid.UUID
	Content  string
	Entities []ResolvedEntity
}

// extractedMention is the write-free output of MAP. It retains document
// coordinates so REDUCE can persist a deterministic entity_mentions row
// without re-parsing LLM output or consulting neighbouring chunks.
type extractedMention struct {
	Entity         repositories.ExtractedEntity
	ParagraphIndex int
	Offset         int
	Snippet        string
}

type mappedParagraph struct {
	Index  int
	Offset int
	Text   string
	NodeID string
}

// ingestionMapResult is intentionally free of database identifiers. MAP can
// therefore run in parallel safely; REDUCE assigns chapter/entity IDs in
// document order after all model calls finish.
type ingestionMapResult struct {
	Index         int
	Chunk         ingestionChunk
	Paragraphs    []mappedParagraph
	Embeddings    [][]float32
	Mentions      []extractedMention
	ExtractionErr error
	EmbeddingErr  error
}

type ingestionConcurrencyProvider interface {
	IngestionConcurrency() int
}

func (s *IngestionService) mapConcurrency() int {
	if provider, ok := s.qwenSvc.(ingestionConcurrencyProvider); ok {
		if limit := provider.IngestionConcurrency(); limit > 0 {
			return limit
		}
	}
	// The Qwen throttle starts at two concurrent calls. Keep the same safe
	// bound for alternate test/providers that do not expose its ramp state.
	return 2
}

// mapChunks performs only stateless Qwen work. It must never resolve an
// entity, open a transaction, or write a chapter/vector/graph row.
func (s *IngestionService) mapChunks(ctx context.Context, chunks []ingestionChunk, onComplete func(int, int)) []ingestionMapResult {
	results := make([]ingestionMapResult, len(chunks))
	if len(chunks) == 0 {
		return results
	}

	var completed atomic.Int32
	group, mapCtx := errgroup.WithContext(ctx)
	group.SetLimit(s.mapConcurrency())
	type completion struct{ remaining atomic.Int32 }
	completions := make([]completion, len(chunks))
	for index, chunk := range chunks {
		paragraphs := mapParagraphs(index, chunk.content)
		results[index] = ingestionMapResult{Index: index, Chunk: chunk, Paragraphs: paragraphs}
		if s.qwenSvc == nil {
			if onComplete != nil {
				onComplete(int(completed.Add(1)), index)
			}
			continue
		}

		taskCount := int32(1) // extraction is always a MAP task.
		if len(paragraphs) > 0 {
			taskCount++
		}
		completions[index].remaining.Store(taskCount)
		finish := func() {
			if completions[index].remaining.Add(-1) == 0 && onComplete != nil {
				onComplete(int(completed.Add(1)), index)
			}
		}
		index, chunk, paragraphs := index, chunk, paragraphs
		group.Go(func() error {
			extracted, err := s.qwenSvc.ExtractEntities(mapCtx, chunk.content, "")
			results[index].ExtractionErr = err
			if err == nil {
				results[index].Mentions = mapExtractedMentions(extracted, paragraphs)
			}
			finish()
			return nil
		})
		if len(paragraphs) > 0 {
			group.Go(func() error {
				results[index].Embeddings, results[index].EmbeddingErr = s.embedParagraphBatches(mapCtx, paragraphs)
				finish()
				return nil
			})
		}
	}
	_ = group.Wait()
	return results
}

// embedParagraphBatches keeps DashScope's maximum batch size while retaining
// a result slot for every original paragraph index. A failed batch is logged
// by REDUCE but does not discard embeddings produced by other batches.
func (s *IngestionService) embedParagraphBatches(ctx context.Context, paragraphs []mappedParagraph) ([][]float32, error) {
	const maxEmbeddingBatchSize = 10
	embeddings := make([][]float32, len(paragraphs))
	var failures []error
	for start := 0; start < len(paragraphs); start += maxEmbeddingBatchSize {
		end := minInt(start+maxEmbeddingBatchSize, len(paragraphs))
		texts := make([]string, end-start)
		for index, paragraph := range paragraphs[start:end] {
			texts[index] = paragraph.Text
		}
		batch, err := s.qwenSvc.GenerateEmbeddingBatch(ctx, texts)
		if err != nil {
			failures = append(failures, fmt.Errorf("paragraphs %d-%d: %w", start, end-1, err))
			continue
		}
		for index, embedding := range batch {
			if start+index >= len(embeddings) {
				break
			}
			embeddings[start+index] = embedding
		}
	}
	return embeddings, errors.Join(failures...)
}

func mapParagraphs(chunkIndex int, content string) []mappedParagraph {
	parts := strings.Split(content, "\n\n")
	paragraphs := make([]mappedParagraph, 0, len(parts))
	cursor := 0
	for index, part := range parts {
		offset := strings.Index(content[cursor:], part)
		if offset < 0 {
			offset = 0
		}
		offset += cursor
		cursor = offset + len(part)
		leading := len(part) - len(strings.TrimLeftFunc(part, unicode.IsSpace))
		text := strings.TrimSpace(part)
		if text == "" || len(text) > 30_000 {
			continue
		}
		paragraphs = append(paragraphs, mappedParagraph{Index: index, Offset: offset + leading, Text: text, NodeID: fmt.Sprintf("chunk:%d:paragraph:%d", chunkIndex, index)})
	}
	return paragraphs
}

func mapExtractedMentions(extracted *ExtractedEntities, paragraphs []mappedParagraph) []extractedMention {
	if extracted == nil {
		return nil
	}
	mentions := make([]extractedMention, 0, len(extracted.All()))
	for _, item := range extracted.All() {
		entity := repositories.ExtractedEntity{Type: item.Type, Name: item.Name, Aliases: item.Aliases, Description: item.Description, Status: item.Status, Properties: item.Properties, Confidence: item.Confidence, ConfidenceSet: item.ConfidenceSet}
		paragraphIndex, offset, snippet := 0, 0, ""
		needle := strings.ToLower(strings.TrimSpace(item.Name))
		for _, paragraph := range paragraphs {
			if at := strings.Index(strings.ToLower(paragraph.Text), needle); at >= 0 {
				paragraphIndex, offset = paragraph.Index, paragraph.Offset+at
				snippet = paragraphSnippet(paragraph.Text, at)
				break
			}
		}
		if snippet == "" && len(paragraphs) > 0 {
			paragraphIndex, offset = paragraphs[0].Index, paragraphs[0].Offset
			snippet = paragraphSnippet(paragraphs[0].Text, 0)
		}
		mentions = append(mentions, extractedMention{Entity: entity, ParagraphIndex: paragraphIndex, Offset: offset, Snippet: snippet})
	}
	return mentions
}

func paragraphSnippet(text string, at int) string {
	const maxSnippet = 240
	if at < 0 {
		at = 0
	}
	start := at - maxSnippet/3
	if start < 0 {
		start = 0
	}
	end := start + maxSnippet
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}

// runWorker processes the document in a background goroutine.
//
// ponytail: synchronous per-chunk — no parallel chunk extraction to avoid
// overwhelming the Qwen API rate limit.
func (s *IngestionService) runWorker(jobID, universeID, workID uuid.UUID, createdWork bool, content []byte, filename string) {
	ctx := WithQwenRequestClass(context.Background(), QwenIngestionRequest)

	s.updateJobStatus(ctx, jobID, "running", "")

	// Resolve the universe owner once per job — this never changes during a
	// job's lifetime, so N+1 identical lookups per emit would be wasteful.
	// Failure (deleted universe, or pool==nil in unit tests) degrades to
	// best-effort: ownerID stays uuid.Nil and progress simply won't be routed.
	var ownerID uuid.UUID
	if s.pool != nil {
		if u, err := repositories.NewUniverseRepo(s.pool).FindByID(ctx, universeID); err != nil {
			log.Printf("[ingestion] resolve universe owner %s: %v (progress events will not be delivered)", universeID, err)
		} else {
			ownerID = u.UserID
		}
	}

	// Parse the raw upload into plain text. Raw binary must never reach
	// splitChunks/chapters.content — a parse failure or empty/whitespace-only
	// extraction fails the job cleanly instead.
	parsed, err := parseDocumentDetails(filename, content)
	if err != nil || strings.TrimSpace(parsed.text) == "" {
		msg := "document contains no text"
		if err != nil {
			msg = err.Error()
		}
		s.updateJobStatus(ctx, jobID, "failed", msg)
		s.emitProgress(jobID, ownerID, universeID, "failed", 0, 0)
		// The failed job row (with its error_message) is the durable record of
		// this attempt — it must survive so a reload shows "upload failed: …"
		// instead of nothing. We deliberately do NOT delete the Work here:
		// ingestion_jobs.work_id is `NOT NULL REFERENCES works(id) ON DELETE
		// CASCADE` (migration 012), so deleting the Work would cascade-delete
		// this job row and its error_message. The Work has a meaningful title
		// (the filename stem) and is user-removable via the delete-work button.
		return
	}
	if createdWork && parsed.title != "" && s.pool != nil {
		// The work is visible while import runs. Only replace the provisional
		// filename title if it is still intact, so a manual rename wins this
		// race instead of being silently overwritten by metadata parsing.
		updated, err := repositories.NewWorkRepo(s.pool).UpdateImportedTitle(ctx, workID, universeID, provisionalWorkTitle(filename), parsed.title)
		if err != nil {
			// The imported content remains valid even if this non-critical rename
			log.Printf("[ingestion] update imported work title: %v", err)
		} else if !updated {
			log.Printf("[ingestion] preserve manually renamed work %s during import", workID)
		}
	}

	// Split parsed text into chapters (markdown/EN/ES/roman/ALL-CAPS heading
	// cascade, falling back to paragraph-boundary chunks).
	chunks := s.splitChunks(parsed.text)
	if len(chunks) == 0 {
		s.updateJobStatus(ctx, jobID, "completed", "")
		return
	}

	// One real chapters row per chunk, under the imported work, so paragraph
	// embeddings get a valid chapter FK and the document survives reloads.
	var chRepo *repositories.ChapterRepo
	baseOrder := 0
	if s.pool != nil {
		chRepo = repositories.NewChapterRepo(s.pool)
		if bo, err := chRepo.GetMaxOrderIndex(ctx, workID); err != nil {
			log.Printf("[ingestion] get max chapter order for work %s: %v", workID, err)
		} else {
			baseOrder = bo
		}
	}

	entitiesTotal := 0
	progress := newIngestionProgressTracker(s, jobID, ownerID, universeID, len(chunks))
	progress.start()
	defer progress.stop()
	mapResults := s.mapChunks(ctx, chunks, func(completed, chunkIndex int) {
		progress.markMap(completed, fmt.Sprintf("Extracting entities from %s…", chunks[chunkIndex].title))
	})

	anySucceeded := false
	var lastErr error
	var ingestedChapters []ingestedChapter
	var ingestedCorpus []string
	var relationshipCorpus strings.Builder
	var relationshipEntities []ResolvedEntity
	var relationshipChunks [][]ResolvedEntity

	for i, mapped := range mapResults {
		ch := mapped.Chunk
		progress.markReduce(i, entitiesTotal, fmt.Sprintf("Saving chapter %s…", ch.title))
		select {
		case <-ctx.Done():
			s.updateJobStatus(ctx, jobID, "failed", "cancelled")
			return
		default:
		}

		chapterID := uuid.Nil
		if chRepo != nil {
			editorContent := MarkdownToEditorHTML(ch.content)
			if editorContent == "" {
				editorContent = ch.content
			}
			chapter := models.Chapter{
				ID:         uuid.New(),
				WorkID:     workID,
				Title:      ch.title,
				OrderIndex: baseOrder + i + 1,
				Content:    editorContent,
				RawText:    ch.content,
				WordCount:  chRepo.CountWords(stripEditorMarkup(editorContent)),
				Status:     "draft",
			}
			if err := s.createChapter(ctx, chRepo, &chapter); err != nil {
				// Without a valid chapter FK there is nothing to persist for
				// this chunk — skip it entirely.
				log.Printf("[ingestion] create chapter chunk %d: %v", i, err)
				progress.markReduce(i+1, entitiesTotal, fmt.Sprintf("Saving chapter %s…", ch.title))
				continue
			}
			chapterID = chapter.ID
			ingestedCorpus = append(ingestedCorpus, ch.content)
		}

		// MAP has already produced embeddings; REDUCE only attaches the now-known
		// chapter ID and persists them in paragraph order.
		if mapped.EmbeddingErr != nil {
			log.Printf("[ingestion] embed chunk %d: %v", i, mapped.EmbeddingErr)
		}
		s.persistMappedEmbeddings(ctx, chapterID, mapped.Paragraphs, mapped.Embeddings)

		if mapped.ExtractionErr != nil {
			log.Printf("[ingestion] extract entities chunk %d: %v", i, mapped.ExtractionErr)
			lastErr = mapped.ExtractionErr
			progress.markReduce(i+1, entitiesTotal, fmt.Sprintf("Saving chapter %s…", ch.title))
			continue
		}
		if s.qwenSvc != nil && s.entitySvc != nil && s.pool != nil {
			anySucceeded = true
			resolved := s.reduceMentions(ctx, universeID, chapterID, mapped.Mentions)
			s.createTimelineEventsForChapter(ctx, universeID, chapterID, baseOrder+i+1, resolved)
			entitiesTotal += len(resolved)
			relationshipCorpus.WriteString(truncateIngestionRelationshipCorpus(ch.content, relationshipCorpus.Len()))
			relationshipEntities = append(relationshipEntities, resolved...)
			if len(resolved) > 0 {
				relationshipChunks = append(relationshipChunks, resolved)
			}
			if chapterID != uuid.Nil {
				ingestedChapters = append(ingestedChapters, ingestedChapter{ID: chapterID, Content: ch.content, Entities: resolved})
			}
		}

		progress.markReduce(i+1, entitiesTotal, fmt.Sprintf("Saving chapter %s…", ch.title))
	}

	if !anySucceeded && lastErr != nil {
		s.updateJobStatus(ctx, jobID, "failed", fmt.Sprintf("entity extraction failed for all %d chunks", len(chunks)))
		progress.finish("failed", len(chunks), entitiesTotal, "Ingestion failed.")
		return
	}

	// An import is one relevance event, not one event per parsed chunk. Decay
	// once after REDUCE has finished and preserve the existing chapter-aware
	// contract by excluding the entities from the final durable chapter.
	s.decayCompletedIngestion(ctx, universeID, ingestedChapters)

	// Relationship extraction is deliberately one bounded, best-effort pass for
	// the whole manuscript. Calling the model per chunk made ingestion serially
	// slow and exceeded the E2E budget on long documents.
	s.enrichRelationships(ctx, universeID, relationshipCorpus.String(), relationshipEntities, relationshipChunks)

	// Bounded post-ingest analysis (contradiction + plot-hole checks) runs
	// before the job is marked completed, so the job honestly reports
	// "running" until analysis ends. Best-effort/enrichment: never flips a
	// completed job to failed. No-ops when SetPostIngestAnalysis wasn't
	// called (nil deps).
	s.runPostIngestAnalysis(ctx, universeID, ingestedChapters, ownerID)
	if s.stylometrySvc != nil && ownerID != uuid.Nil && len(ingestedCorpus) > 0 {
		if _, err := s.stylometrySvc.ObserveCorpus(ctx, ownerID, universeID, ingestedCorpus); err != nil {
			log.Printf("[ingestion] writer stylometry corpus: %v", err)
		}
	}

	s.updateJobStatus(ctx, jobID, "completed", "")
	progress.finish("completed", len(chunks), entitiesTotal, "Ingestion complete.")
}

func (s *IngestionService) persistMappedEmbeddings(ctx context.Context, chapterID uuid.UUID, paragraphs []mappedParagraph, embeddings [][]float32) {
	if s.vectorRepo == nil || chapterID == uuid.Nil {
		return
	}
	for index, embedding := range embeddings {
		if index >= len(paragraphs) || embedding == nil {
			continue
		}
		paragraph := paragraphs[index]
		if err := s.vectorRepo.SaveParagraphEmbedding(ctx, chapterID, paragraph.Index, paragraph.NodeID, paragraph.Text, embedding); err != nil {
			log.Printf("[ingestion] save paragraph embedding para %d: %v", paragraph.Index, err)
		}
	}
}

func (s *IngestionService) enrichRelationships(ctx context.Context, universeID uuid.UUID, corpus string, entities []ResolvedEntity, chunks [][]ResolvedEntity) {
	relationshipCtx, cancelRelationships := context.WithTimeout(ctx, ingestionRelationshipTimeout)
	persisted, err := s.persistRelationships(relationshipCtx, universeID, corpus, entities)
	cancelRelationships()
	if err != nil {
		log.Printf("[ingestion] analyze relationships: %v", err)
	}
	if persisted > 0 {
		return
	}

	// Model enrichment is optional. When it produces no usable edge (including
	// timeout/error), preserve basic graph connectivity from deterministic
	// canonical co-occurrence within each imported chunk.
	if fallbackEdges, fallbackErr := s.persistCooccurrenceEdges(ctx, universeID, chunks); fallbackErr != nil {
		log.Printf("[ingestion] create co-occurrence fallback edges: %v", fallbackErr)
	} else if fallbackEdges > 0 {
		log.Printf("[ingestion] created %d CO_OCCURS_WITH fallback edges", fallbackEdges)
	}
}

func truncateIngestionRelationshipCorpus(chunk string, used int) string {
	if used >= ingestionRelationshipCorpusLimit {
		return ""
	}
	remaining := ingestionRelationshipCorpusLimit - used
	prefix := ""
	if used > 0 && chunk != "" {
		prefix = "\n\n"
		remaining -= len(prefix)
		if remaining <= 0 {
			return ""
		}
	}
	if len(chunk) > remaining {
		chunk = chunk[:remaining]
	}
	return prefix + chunk
}

// persistRelationships enriches the universe graph from relationships found
// in one ingested chunk. GraphRepo owns identifier validation and AGE session
// safety; an invalid model-supplied relationship is therefore logged and
// skipped without failing the ingestion job.
func (s *IngestionService) persistRelationships(ctx context.Context, universeID uuid.UUID, text string, resolved []ResolvedEntity) (int, error) {
	edgeWriter := s.relationshipEdges
	if edgeWriter == nil {
		edgeWriter = s.graphRepo
	}
	if s.qwenSvc == nil || edgeWriter == nil || len(resolved) == 0 {
		return 0, nil
	}

	entities := make([]models.Entity, 0, min(len(resolved), ingestionRelationshipEntityLimit))
	entityNames := make([]string, 0, min(len(resolved), ingestionRelationshipEntityLimit))
	seenEntityIDs := make(map[uuid.UUID]struct{}, len(resolved))
	for _, item := range resolved {
		if item.Entity.Name == "" || len(entityNames) >= ingestionRelationshipEntityLimit {
			continue
		}
		if _, seen := seenEntityIDs[item.Entity.ID]; seen {
			continue
		}
		seenEntityIDs[item.Entity.ID] = struct{}{}
		entities = append(entities, item.Entity)
		entityNames = append(entityNames, item.Entity.Name)
	}
	if len(entityNames) == 0 {
		return 0, nil
	}

	relationships, err := s.qwenSvc.AnalyzeRelationships(ctx, text, entityNames)
	if err != nil {
		return 0, err
	}

	graphName := "universe_" + universeID.String()
	persisted := 0
	for _, relationship := range relationships {
		sourceName, _ := relationship["source"].(string)
		targetName, _ := relationship["target"].(string)
		relationType, _ := relationship["type"].(string)
		if sourceName == "" || targetName == "" || relationType == "" {
			log.Printf("[ingestion] skip malformed relationship: source=%q target=%q type=%q", sourceName, targetName, relationType)
			continue
		}
		source, sourceFound, sourceDiagnostic := resolveIngestionRelationshipEntity(sourceName, entities)
		if !sourceFound {
			log.Printf("[ingestion] skip relationship source %q: %s", sourceName, sourceDiagnostic)
			continue
		}
		target, targetFound, targetDiagnostic := resolveIngestionRelationshipEntity(targetName, entities)
		if !targetFound {
			log.Printf("[ingestion] skip relationship target %q: %s", targetName, targetDiagnostic)
			continue
		}
		if err := edgeWriter.CreateEdge(ctx, graphName, source.ID.String(), target.ID.String(), relationType, nil); err != nil {
			log.Printf("[ingestion] create graph edge %q->%q (%q): %v", sourceName, targetName, relationType, err)
			continue
		}
		persisted++
	}
	return persisted, nil
}

type ingestionEntityPair struct {
	source models.Entity
	target models.Entity
}

func (s *IngestionService) persistCooccurrenceEdges(ctx context.Context, universeID uuid.UUID, chunks [][]ResolvedEntity) (int, error) {
	edgeWriter := s.relationshipEdges
	if edgeWriter == nil {
		edgeWriter = s.graphRepo
	}
	if edgeWriter == nil {
		return 0, nil
	}

	pairs := make(map[string]ingestionEntityPair)
	for _, chunk := range chunks {
		if len(pairs) >= ingestionFallbackEdgeLimit {
			break
		}
		unique := make(map[uuid.UUID]models.Entity, len(chunk))
		for _, item := range chunk {
			if item.Entity.ID != uuid.Nil {
				unique[item.Entity.ID] = item.Entity
			}
		}
		entities := make([]models.Entity, 0, len(unique))
		for _, entity := range unique {
			entities = append(entities, entity)
		}
		sort.Slice(entities, func(i, j int) bool { return entities[i].ID.String() < entities[j].ID.String() })
		for i := 0; i < len(entities); i++ {
			for j := i + 1; j < len(entities); j++ {
				if len(pairs) >= ingestionFallbackEdgeLimit {
					break
				}
				key := entities[i].ID.String() + ":" + entities[j].ID.String()
				if _, exists := pairs[key]; exists {
					continue
				}
				pairs[key] = ingestionEntityPair{source: entities[i], target: entities[j]}
			}
			if len(pairs) >= ingestionFallbackEdgeLimit {
				break
			}
		}
	}

	keys := make([]string, 0, len(pairs))
	for key := range pairs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	graphName := "universe_" + universeID.String()
	persisted := 0
	for _, key := range keys {
		pair := pairs[key]
		if err := edgeWriter.CreateEdge(ctx, graphName, pair.source.ID.String(), pair.target.ID.String(), "CO_OCCURS_WITH", nil); err != nil {
			log.Printf("[ingestion] create CO_OCCURS_WITH edge %q->%q: %v", pair.source.Name, pair.target.Name, err)
			continue
		}
		persisted++
	}
	return persisted, nil
}

// resolveIngestionRelationshipEntity maps a model-emitted name to exactly one
// persisted entity. Exact canonical/alias matches win; otherwise a shortened
// name may match a canonical name or alias only when its word prefix is unique.
// Ambiguity is an intentional no-edge outcome rather than a guess.
func resolveIngestionRelationshipEntity(name string, entities []models.Entity) (models.Entity, bool, string) {
	query := strings.TrimSpace(name)
	if query == "" {
		return models.Entity{}, false, "empty entity name"
	}

	exact := matchingRelationshipEntities(query, entities, false)
	if len(exact) == 1 {
		return exact[0], true, ""
	}
	if len(exact) > 1 {
		return models.Entity{}, false, fmt.Sprintf("ambiguous canonical or alias match (%d entities)", len(exact))
	}

	prefix := matchingRelationshipEntities(query, entities, true)
	if len(prefix) == 1 {
		return prefix[0], true, ""
	}
	if len(prefix) > 1 {
		return models.Entity{}, false, fmt.Sprintf("ambiguous prefix match (%d entities)", len(prefix))
	}
	return models.Entity{}, false, "no canonical, alias, or unique prefix match"
}

func matchingRelationshipEntities(query string, entities []models.Entity, allowPrefix bool) []models.Entity {
	matches := make([]models.Entity, 0, 1)
	for _, entity := range entities {
		candidateNames := append([]string{entity.Name}, entity.Aliases...)
		for _, candidate := range candidateNames {
			candidate = strings.TrimSpace(candidate)
			exact := strings.EqualFold(query, candidate)
			prefix := allowPrefix && hasRelationshipWordPrefix(query, candidate)
			if exact || prefix {
				matches = append(matches, entity)
				break
			}
		}
	}
	return matches
}

func hasRelationshipWordPrefix(query, candidate string) bool {
	queryWords := strings.Fields(query)
	candidateWords := strings.Fields(candidate)
	if len(queryWords) == 0 || len(queryWords) >= len(candidateWords) {
		return false
	}
	return strings.EqualFold(strings.Join(queryWords, " "), strings.Join(candidateWords[:len(queryWords)], " "))
}

// createChapter wraps ChapterRepo.Create (which requires a transaction) in a
// short single-statement transaction.
func (s *IngestionService) createChapter(ctx context.Context, chRepo *repositories.ChapterRepo, ch *models.Chapter) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := chRepo.Create(ctx, tx, ch); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// maxSaneHeadingMatches guards against a heading pattern matching almost
// every line (a false positive, e.g. a doc that happens to contain many bare
// roman-numeral-looking lines) — treated the same as "no match", falling
// through to the next pattern in the cascade.
const maxSaneHeadingMatches = 500

// headingMatch is a single detected chapter heading: title is the extracted
// heading text, start/end are the byte offsets of the whole matched heading
// line in the source content (used to slice out chapter bodies).
type headingMatch struct {
	start, end int
	title      string
}

// headingPatterns is the priority cascade of heading patterns tried in
// splitChunks, in order — the first pattern class with >= 2 matches (and
// <= maxSaneHeadingMatches) wins. Each has exactly one capture group holding
// the extracted title text.
var headingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^#{1,3} (.+)$`), // markdown (also styled DOCX via D1 bonus)
	// ponytail: spelled-out numbers (one, two, three…) added because books like The Expanse use "Chapter One" not "Chapter 1"
	regexp.MustCompile(`(?mi)^[ \t]*(chapter[ \t]+(?:\d+|[ivxlc]+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety|hundred|thousand)\b.*)$`),      // English
	regexp.MustCompile(`(?mi)^[ \t]*(cap[ií]tulo[ \t]+(?:\d+|[ivxlc]+|uno|dos|tres|cuatro|cinco|seis|siete|ocho|nueve|diez|once|doce|trece|catorce|quince|dieciséis|diecisiete|dieciocho|diecinueve|veinte|treinta|cuarenta|cincuenta|sesenta|setenta|ochenta|noventa|cien)\b.*)$`), // Spanish
	regexp.MustCompile(`(?m)^[ \t]*([IVXLC]{1,7}\.?)[ \t]*$`),                // bare roman numeral
	regexp.MustCompile(`(?m)^[ \t]*([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)[ \t]*$`), // title case heading ("Holden", "The Rocinante")
}

// regexHeadingMatches runs re against content and returns one headingMatch
// per match, using the first capture group as the title.
func regexHeadingMatches(content string, re *regexp.Regexp) []headingMatch {
	locs := re.FindAllStringSubmatchIndex(content, -1)
	matches := make([]headingMatch, 0, len(locs))
	for _, loc := range locs {
		matches = append(matches, headingMatch{
			start: loc[0],
			end:   loc[1],
			title: strings.TrimSpace(content[loc[2]:loc[3]]),
		})
	}
	return matches
}

// isAllCapsHeadingLine reports whether a trimmed line looks like an
// ALL-CAPS chapter heading: short (<= 60 chars), no lowercase letters, and
// at least 3 letters (so pure punctuation/numeral lines don't qualify).
//
// Short names are allowed only when their surrounding whitespace proves they
// are standalone headings; sentence-ending punctuation still rejects prose.
func isAllCapsHeadingLine(line string) bool {
	if len(line) < 3 || len(line) > 60 {
		return false
	}
	last := line[len(line)-1]
	if last == '.' || last == '!' || last == '?' || last == '"' || last == '»' || last == ',' || last == ';' || last == ':' {
		return false
	}
	letters := 0
	for _, r := range line {
		if unicode.IsLower(r) {
			return false
		}
		if unicode.IsUpper(r) {
			letters++
		}
	}
	return letters >= 3
}

// allCapsHeadingMatches scans content line-by-line for ALL-CAPS heading
// candidates — this shape (short lines, no lowercase) isn't expressible as a
// single regex the way the other patterns are.
func allCapsHeadingMatches(content string) []headingMatch {
	var matches []headingMatch
	lines := strings.Split(content, "\n")
	offset := 0
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Short uppercase character names (for example, HOLDEN) are common
		// manuscript chapter titles. Requiring whitespace around short lines
		// avoids treating inline PDF fragments as chapters.
		separated := (index == 0 || strings.TrimSpace(lines[index-1]) == "") && (index == len(lines)-1 || strings.TrimSpace(lines[index+1]) == "")
		// A short standalone response is indistinguishable from an ALL-CAPS
		// heading in extracted PDF text. Treat short candidates as character
		// headings only when their Title Case form reappears in body prose.
		if isAllCapsHeadingLine(trimmed) && (len([]rune(trimmed)) >= 10 || hasTitleCaseNameEvidence(lines, trimmed)) && (len(trimmed) >= 10 || separated) {
			leading := len(line) - len(strings.TrimLeft(line, " \t"))
			start := offset + leading
			matches = append(matches, headingMatch{start: start, end: start + len(trimmed), title: trimmed})
		}
		offset += len(line) + 1 // +1 for the '\n' consumed by Split
	}
	return matches
}

// hasTitleCaseNameEvidence accepts a short ALL-CAPS candidate only when the
// same name occurs after another word in a non-heading sentence. This protects
// dialogue such as YES and NO at sentence starts without a brittle word list.
func hasTitleCaseNameEvidence(lines []string, candidate string) bool {
	candidateRunes := []rune(strings.ToLower(candidate))
	if len(candidateRunes) == 0 || strings.ContainsRune(candidate, ' ') {
		return false
	}
	candidateRunes[0] = unicode.ToUpper(candidateRunes[0])
	name := string(candidateRunes)
	for _, line := range lines {
		if isAllCapsHeadingLine(strings.TrimSpace(line)) {
			continue
		}
		for _, sentence := range strings.FieldsFunc(line, func(r rune) bool { return r == '.' || r == '!' || r == '?' }) {
			words := strings.FieldsFunc(sentence, func(r rune) bool { return !unicode.IsLetter(r) })
			for _, word := range words[1:] {
				if word == name {
					return true
				}
			}
		}
	}
	return false
}

// splitByHeadings turns a set of detected heading matches into chapter
// chunks: the body of each chunk runs from just after its heading line to
// just before the next heading (or end of content).
func splitByHeadings(content string, matches []headingMatch) []ingestionChunk {
	chunks := make([]ingestionChunk, 0, len(matches)+1)
	if len(matches) > 0 {
		if preface := strings.TrimSpace(content[:matches[0].start]); preface != "" {
			// A parser cannot always distinguish a title page or foreword from
			// prose. Preserve it rather than silently dropping writer content.
			chunks = append(chunks, ingestionChunk{title: "Front Matter", content: preface})
		}
	}
	for i, m := range matches {
		bodyStart := m.end + 1 // after the newline
		var bodyEnd int
		if i+1 < len(matches) {
			bodyEnd = matches[i+1].start
		} else {
			bodyEnd = len(content)
		}
		if bodyStart > len(content) {
			bodyStart = len(content)
		}
		if bodyEnd > bodyStart {
			body := strings.TrimSpace(content[bodyStart:bodyEnd])
			if body != "" {
				chunks = append(chunks, ingestionChunk{title: m.title, content: body})
			}
		}
	}
	return chunks
}

var narrativePDFHeading = regexp.MustCompile(`(?i)^(pr[oó]logo|prologue|ep[ií]logo|epilogue)(?:\s*:\s*.+)?$`)
var bareArabicChapterMarker = regexp.MustCompile(`^\d{1,4}\.?$`)

// pdfNarrativeHeadingMatches recognises front-matter narrative sections such
// as "Prólogo: Julie" and the common PDF layout where a bare chapter number
// is followed by a character-named section heading. The latter is represented
// by one match spanning both lines, so the number never becomes chapter body.
func pdfNarrativeHeadingMatches(content string) []headingMatch {
	type sourceLine struct {
		start, end int
		text       string
	}

	rawLines := strings.Split(content, "\n")
	lines := make([]sourceLine, len(rawLines))
	offset := 0
	for i, raw := range rawLines {
		trimmed := strings.TrimSpace(raw)
		leading := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
		start := offset + leading
		lines[i] = sourceLine{start: start, end: start + len(trimmed), text: trimmed}
		offset += len(raw) + 1
	}

	matches := make([]headingMatch, 0)
	for i, line := range lines {
		if narrativePDFHeading.MatchString(line.text) {
			matches = append(matches, headingMatch{start: line.start, end: line.end, title: line.text})
			continue
		}
		if !bareArabicChapterMarker.MatchString(line.text) {
			continue
		}
		next := i + 1
		for next < len(lines) && lines[next].text == "" {
			next++
		}
		if next < len(lines) && looksLikeTitleCaseHeading(lines[next].text) {
			matches = append(matches, headingMatch{start: line.start, end: lines[next].end, title: lines[next].text})
		}
	}
	return matches
}

// splitChunks splits document content into chapters. It tries a priority
// cascade of heading patterns (markdown, English "Chapter N", Spanish
// "Capítulo N", bare roman numerals, then short ALL-CAPS lines) — the first
// pattern class with >= 2 matches wins. No pattern matching falls back to
// splitByParagraphs.
//
// ponytail: simple regex cascade — no AST parser needed for chapter detection.
func (s *IngestionService) splitChunks(content string) []ingestionChunk {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	// Explicit markdown and conventional Chapter/Capítulo markers remain the
	// highest-confidence document structures.
	for _, re := range headingPatterns[:4] {
		matches := regexHeadingMatches(content, re)
		if len(matches) >= 2 && len(matches) <= maxSaneHeadingMatches {
			return splitByHeadings(content, matches)
		}
	}
	if matches := pdfNarrativeHeadingMatches(content); len(matches) >= 1 && len(matches) <= maxSaneHeadingMatches {
		return splitByHeadings(content, matches)
	}
	for _, re := range headingPatterns[4:] {
		matches := regexHeadingMatches(content, re)
		if len(matches) >= 2 && len(matches) <= maxSaneHeadingMatches {
			return splitByHeadings(content, matches)
		}
	}

	if matches := allCapsHeadingMatches(content); len(matches) >= 2 && len(matches) <= maxSaneHeadingMatches {
		return splitByHeadings(content, matches)
	}

	return splitByParagraphs(content)
}

// splitByParagraphs splits content at paragraph boundaries when there are no
// markdown headers. Each chunk targets ~50K chars so entity extraction gets
// manageable text and progress is granular.
//
// ponytail: greedy paragraph fill — splits at \n\n boundaries, no tokenizer.
func splitByParagraphs(content string) []ingestionChunk {
	const maxChunkSize = 50_000
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if len(content) <= maxChunkSize {
		return []ingestionChunk{{title: "Untitled", content: content}}
	}

	paragraphs := strings.Split(content, "\n\n")
	chunks := make([]ingestionChunk, 0, len(paragraphs)/3+1)
	var buf strings.Builder
	part := 1
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if buf.Len() > 0 && buf.Len()+len(p) > maxChunkSize {
			chunks = append(chunks, ingestionChunk{
				title:   fmt.Sprintf("Part %d", part),
				content: buf.String(),
			})
			buf.Reset()
			part++
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(p)
	}
	if buf.Len() > 0 {
		chunks = append(chunks, ingestionChunk{
			title:   fmt.Sprintf("Part %d", part),
			content: buf.String(),
		})
	}
	return chunks
}

// resolveAndBuildGraph resolves or creates entities and builds graph nodes,
// returning the number of entities successfully resolved and the resolved
// entities themselves (needed by the post-ingest analysis pass, D4).
//
// ponytail: reuses EntityService.ResolveOrCreate — same dedup/merge logic.
func (s *IngestionService) resolveAndBuildGraph(ctx context.Context, universeID uuid.UUID, extracted *ExtractedEntities, mentionText string) (int, []ResolvedEntity) {
	if extracted == nil {
		return 0, nil
	}

	allEntities := make([]repositories.ExtractedEntity, 0, len(extracted.All()))
	for _, e := range extracted.All() {
		allEntities = append(allEntities, repositories.ExtractedEntity{
			Type: e.Type, Name: e.Name, Aliases: e.Aliases,
			Description: e.Description, Status: e.Status, Properties: e.Properties, Confidence: e.Confidence, ConfidenceSet: e.ConfidenceSet,
		})
	}

	var resolved []ResolvedEntity
	for _, ee := range allEntities {
		entity, previousStatus, isNew, err := s.entitySvc.ResolveOrCreate(ctx, universeID, ee)
		if err != nil {
			log.Printf("[ingestion] resolve entity %s: %v", ee.Name, err)
			continue
		}
		resolved = append(resolved, ResolvedEntity{
			Entity:         *entity,
			MentionText:    mentionText,
			IsNew:          isNew,
			PreviousStatus: previousStatus,
		})
	}
	return len(resolved), resolved
}

// reduceMentions is deliberately serial: each ResolveOrCreate observes every
// entity created/merged by earlier mentions in document order. This is the
// ingestion-side guard against duplicate natural keys.
func (s *IngestionService) reduceMentions(ctx context.Context, universeID, chapterID uuid.UUID, mentions []extractedMention) []ResolvedEntity {
	if s.entitySvc == nil || s.pool == nil {
		return nil
	}
	ordered := sortMentionsForReduce(mentions)
	resolved := make([]ResolvedEntity, 0, len(ordered))
	for _, mention := range ordered {
		entity, previousStatus, isNew, err := s.entitySvc.ResolveOrCreate(ctx, universeID, mention.Entity)
		if err != nil {
			log.Printf("[ingestion] resolve entity %s: %v", mention.Entity.Name, err)
			continue
		}
		item := ResolvedEntity{Entity: *entity, MentionText: mention.Snippet, IsNew: isNew, PreviousStatus: previousStatus}
		resolved = append(resolved, item)
		if chapterID == uuid.Nil || s.entitySvc.entityRepo == nil {
			continue
		}
		if err := s.persistMention(ctx, entity.ID, chapterID, mention); err != nil {
			log.Printf("[ingestion] persist mention for %s: %v", entity.Name, err)
			continue
		}
		if s.relevance != nil {
			if err := s.relevance.Touch(ctx, entity.ID, chapterID); err != nil {
				log.Printf("[ingestion] reinforce relevance for %s: %v", entity.Name, err)
			}
		}
	}
	return resolved
}

// createTimelineEventsForChapter seeds one timeline_events row per resolved
// "event"-type entity in this chapter, using the chapter's position among
// this import's chunks as a first-pass chronological ordering. Without this,
// an imported manuscript's Timeline stays permanently empty — only the
// hand-seeded demo template ever populated it. The writer can still reorder
// or edit these later; this just seeds something instead of nothing.
// Nil-safe: skips entirely without a timelineRepo (existing tests/deployments
// that never call SetTimelineRepo are unaffected).
func (s *IngestionService) createTimelineEventsForChapter(ctx context.Context, universeID, chapterID uuid.UUID, position int, resolved []ResolvedEntity) {
	if s.timelineRepo == nil || chapterID == uuid.Nil {
		return
	}

	var participantIDs []uuid.UUID
	for _, re := range resolved {
		if re.Entity.Type == "character" {
			participantIDs = append(participantIDs, re.Entity.ID)
		}
	}

	for _, re := range resolved {
		if re.Entity.Type != "event" {
			continue
		}
		pos := float64(position)
		entityID := re.Entity.ID
		event := models.TimelineEvent{
			ID:               uuid.New(),
			UniverseID:       universeID,
			EventEntityID:    &entityID,
			Title:            re.Entity.Name,
			Description:      re.Entity.Description,
			TimelinePosition: &pos,
			ChapterID:        &chapterID,
			Participants:     participantIDs,
		}
		if err := s.timelineRepo.Create(ctx, &event); err != nil {
			log.Printf("[ingestion] create timeline event for %s: %v", re.Entity.Name, err)
		}
	}
}

// decayCompletedIngestion applies one chapter-aware decay tick for a completed
// import. Only the final durable chapter is excluded, preserving the semantics
// of a completed chapter without issuing one full-universe update per chunk.
func (s *IngestionService) decayCompletedIngestion(ctx context.Context, universeID uuid.UUID, chapters []ingestedChapter) {
	if s.relevance == nil || len(chapters) == 0 {
		return
	}
	resolved := chapters[len(chapters)-1].Entities
	mentioned := make([]uuid.UUID, 0, len(resolved))
	seen := make(map[uuid.UUID]struct{}, len(resolved))
	for _, item := range resolved {
		if _, exists := seen[item.Entity.ID]; exists {
			continue
		}
		seen[item.Entity.ID] = struct{}{}
		mentioned = append(mentioned, item.Entity.ID)
	}
	decayCtx, cancel := context.WithTimeout(ctx, ingestionDecayTimeout)
	defer cancel()
	if err := s.relevance.DecayExcept(decayCtx, universeID, mentioned); err != nil {
		log.Printf("[ingestion] decay after completed import: %v", err)
	}
}

func sortMentionsForReduce(mentions []extractedMention) []extractedMention {
	ordered := append([]extractedMention(nil), mentions...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].ParagraphIndex != ordered[j].ParagraphIndex {
			return ordered[i].ParagraphIndex < ordered[j].ParagraphIndex
		}
		if ordered[i].Offset != ordered[j].Offset {
			return ordered[i].Offset < ordered[j].Offset
		}
		return ordered[i].Entity.Name < ordered[j].Entity.Name
	})
	return ordered
}

func (s *IngestionService) persistMention(ctx context.Context, entityID, chapterID uuid.UUID, mention extractedMention) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mention transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	paragraphNodeID := fmt.Sprintf("chapter:%s:paragraph:%d", chapterID, mention.ParagraphIndex)
	row := &models.EntityMention{
		ID: uuid.New(), EntityID: entityID, ChapterID: chapterID,
		ParagraphIndex: mention.ParagraphIndex, CharacterOffset: mention.Offset, ParagraphNodeID: paragraphNodeID,
		ContextSnippet: mention.Snippet, MentionType: mention.Entity.Type,
	}
	if err := s.entitySvc.entityRepo.CreateMention(ctx, tx, row); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit mention transaction: %w", err)
	}
	return nil
}

// emitProgress sends an ingestion_progress WebSocket event to the resolved
// universe owner.
func (s *IngestionService) emitProgress(jobID, userID, universeID uuid.UUID, status string, processed, total int) {
	s.emitProgressDetails(jobID, userID, universeID, status, processed, total, "", nil)
}

func (s *IngestionService) emitProgressDetails(jobID, userID, universeID uuid.UUID, status string, processed, total int, action string, etaSeconds *int) {
	if s.hub == nil {
		return
	}
	payload, _ := json.Marshal(models.IngestionProgressPayload{
		JobID:             jobID,
		UniverseID:        universeID,
		Status:            status,
		ChaptersProcessed: processed,
		TotalChapters:     total,
		Action:            action,
		ETASeconds:        etaSeconds,
	})
	msg := models.WSMessage{
		Type:    "ingestion_progress",
		Payload: payload,
	}
	// ponytail: userID is the universe owner resolved once in runWorker.
	// Delivery remains best-effort — SendToUser's error is discarded because
	// an offline/missing WS connection is expected and non-fatal.
	_ = s.hub.SendToUser(userID, msg)
}

// updateProgress persists the chapter/entity counters, mirroring what
// emitProgress reports over WS. Best-effort like updateJobStatus.
func (s *IngestionService) updateProgress(ctx context.Context, jobID uuid.UUID, totalDetected, processed, entities int) {
	if s.pool == nil {
		return
	}
	repo := repositories.NewIngestionRepo(s.pool)
	if err := repo.UpdateProgress(ctx, jobID, totalDetected, processed, entities); err != nil {
		log.Printf("[ingestion] update progress job %s: %v", jobID, err)
	}
}

// updateJobStatus persists a status change to the ingestion_jobs table.
func (s *IngestionService) updateJobStatus(ctx context.Context, jobID uuid.UUID, status, errMsg string) {
	if s.pool == nil {
		return
	}
	repo := repositories.NewIngestionRepo(s.pool)
	if err := repo.UpdateStatus(ctx, jobID, status, errMsg); err != nil {
		log.Printf("[ingestion] update status %s: %v", status, err)
	}
}
