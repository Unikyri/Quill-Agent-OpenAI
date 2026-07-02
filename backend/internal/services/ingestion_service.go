package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// ── Ingestion service ──

// ingestionChunk represents a parsed section of the uploaded document.
type ingestionChunk struct {
	title   string
	content string
}

// IngestionQwen is the minimal Qwen interface used by IngestionService.
// QwenService satisfies this interface.
type IngestionQwen interface {
	ExtractEntities(ctx context.Context, text, universeContext string) (*ExtractedEntities, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
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
	qwenSvc    IngestionQwen
	hub        AnalysisHub
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

// Start creates an ingestion job and kicks off the async pipeline.
// Returns the job ID immediately. The caller should return 202 Accepted.
func (s *IngestionService) Start(ctx context.Context, universeID, workID uuid.UUID, reader io.Reader, filename string) (uuid.UUID, error) {
	jobID := uuid.New()

	if s.pool != nil {
		repo := repositories.NewIngestionRepo(s.pool)
		if err := repo.Create(ctx, jobID, universeID, workID, "pending", filename); err != nil {
			return uuid.Nil, fmt.Errorf("create ingestion job: %w", err)
		}
	}

	go s.runWorker(jobID, universeID, workID, reader, filename)

	return jobID, nil
}

// runWorker processes the document in a background goroutine.
//
// ponytail: synchronous per-chunk — no parallel chunk extraction to avoid
// overwhelming the Qwen API rate limit.
func (s *IngestionService) runWorker(jobID, universeID, workID uuid.UUID, reader io.Reader, filename string) {
	ctx := context.Background()

	// Read entire document content
	content, err := io.ReadAll(reader)
	if err != nil {
		s.updateJobStatus(ctx, jobID, "failed", fmt.Sprintf("read file: %v", err))
		return
	}

	s.updateJobStatus(ctx, jobID, "running", "")

	// Split content by markdown headers
	chunks := s.splitChunks(string(content))
	if len(chunks) == 0 {
		s.updateJobStatus(ctx, jobID, "completed", "")
		return
	}

	s.emitProgress(jobID, "running", 0, len(chunks))

	for i, ch := range chunks {
		select {
		case <-ctx.Done():
			s.updateJobStatus(ctx, jobID, "failed", "cancelled")
			return
		default:
		}

		// Extract entities from chunk
		if s.qwenSvc != nil && s.entitySvc != nil && s.pool != nil {
			extracted, err := s.qwenSvc.ExtractEntities(ctx, ch.content, "")
			if err != nil {
				log.Printf("[ingestion] extract entities chunk %d: %v", i, err)
				s.emitProgress(jobID, "running", i+1, len(chunks))
				continue
			}
			s.resolveAndBuildGraph(ctx, universeID, extracted)
		}

		// ponytail: split chunk content into paragraphs and embed each one.
		// chapterID uses uuid.Nil as placeholder — should map to real chapter IDs
		// once chapter creation is implemented in the ingestion pipeline.
		if s.qwenSvc != nil && s.vectorRepo != nil {
			paragraphs := strings.Split(ch.content, "\n\n")
			for pIdx, p := range paragraphs {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				embedding, err := s.qwenSvc.GenerateEmbedding(ctx, p)
				if err != nil {
					log.Printf("[ingestion] embed paragraph chunk %d para %d: %v", i, pIdx, err)
					continue
				}
				if err := s.vectorRepo.SaveParagraphEmbedding(ctx, uuid.Nil, i*1000+pIdx, ch.title, p, embedding); err != nil {
					log.Printf("[ingestion] save paragraph embedding chunk %d para %d: %v", i, pIdx, err)
				}
			}
		}

		s.emitProgress(jobID, "running", i+1, len(chunks))
	}

	s.updateJobStatus(ctx, jobID, "completed", "")
}

// splitChunks splits document content by markdown headers (# Chapter N).
//
// ponytail: simple regex split — no AST parser needed for markdown chapters.
func (s *IngestionService) splitChunks(content string) []ingestionChunk {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	headerRe := regexp.MustCompile(`(?m)^# (.+)$`)
	locs := headerRe.FindAllStringSubmatchIndex(content, -1)

	if len(locs) == 0 {
		// No headers found — treat whole document as one chunk
		return []ingestionChunk{{title: "Untitled", content: content}}
	}

	chunks := make([]ingestionChunk, 0, len(locs))
	for i, loc := range locs {
		title := content[loc[2]:loc[3]]
		bodyStart := loc[1] + 1 // after the newline
		var bodyEnd int
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		} else {
			bodyEnd = len(content)
		}
		body := strings.TrimSpace(content[bodyStart:bodyEnd])
		if body != "" {
			chunks = append(chunks, ingestionChunk{title: title, content: body})
		}
	}
	return chunks
}

// resolveAndBuildGraph resolves or creates entities and builds graph nodes.
//
// ponytail: reuses EntityService.ResolveOrCreate — same dedup/merge logic.
func (s *IngestionService) resolveAndBuildGraph(ctx context.Context, universeID uuid.UUID, extracted *ExtractedEntities) {
	if extracted == nil {
		return
	}

	allEntities := make([]repositories.ExtractedEntity, 0)
	for _, e := range extracted.Characters {
		allEntities = append(allEntities, repositories.ExtractedEntity{
			Type: e.Type, Name: e.Name, Aliases: e.Aliases,
			Description: e.Description, Status: e.Status, Properties: e.Properties,
		})
	}
	for _, e := range extracted.Places {
		allEntities = append(allEntities, repositories.ExtractedEntity{
			Type: e.Type, Name: e.Name, Aliases: e.Aliases,
			Description: e.Description, Status: e.Status, Properties: e.Properties,
		})
	}
	for _, e := range extracted.Events {
		allEntities = append(allEntities, repositories.ExtractedEntity{
			Type: e.Type, Name: e.Name, Aliases: e.Aliases,
			Description: e.Description, Status: e.Status, Properties: e.Properties,
		})
	}
	for _, e := range extracted.Factions {
		allEntities = append(allEntities, repositories.ExtractedEntity{
			Type: e.Type, Name: e.Name, Aliases: e.Aliases,
			Description: e.Description, Status: e.Status, Properties: e.Properties,
		})
	}
	for _, e := range extracted.WorldRules {
		allEntities = append(allEntities, repositories.ExtractedEntity{
			Type: e.Type, Name: e.Name, Aliases: e.Aliases,
			Description: e.Description, Status: e.Status, Properties: e.Properties,
		})
	}
	for _, e := range extracted.PlotDevelopments {
		allEntities = append(allEntities, repositories.ExtractedEntity{
			Type: e.Type, Name: e.Name, Aliases: e.Aliases,
			Description: e.Description, Status: e.Status, Properties: e.Properties,
		})
	}

	for _, ee := range allEntities {
		if _, _, err := s.entitySvc.ResolveOrCreate(ctx, universeID, ee); err != nil {
			log.Printf("[ingestion] resolve entity %s: %v", ee.Name, err)
		}
	}
}

// emitProgress sends an ingestion_progress WebSocket event.
func (s *IngestionService) emitProgress(jobID uuid.UUID, status string, processed, total int) {
	if s.hub == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"job_id":             jobID.String(),
		"status":             status,
		"chapters_processed": processed,
		"total_chapters":     total,
	})
	msg := models.WSMessage{
		Type:    "ingestion_progress",
		Payload: payload,
	}
	// ponytail: hub.SendToUser requires userID. Ingestion is system-initiated,
	// so userID is empty (uuid.Nil). The hub stores conns by userID.
	// Progress broadcasts are best-effort — we log when no connection found.
	_ = s.hub.SendToUser(uuid.Nil, msg)
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
