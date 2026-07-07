package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/ws"
)

// analysisJob represents a single paragraph to analyze.
type analysisJob struct {
	WorkID     uuid.UUID
	ChapterID  uuid.UUID
	UniverseID uuid.UUID
	Text       string
	UserID     uuid.UUID
}

// AnalysisResult holds the output of a complete analysis pass.
type AnalysisResult struct {
	WorkID         uuid.UUID
	ChapterID      uuid.UUID
	Entities       []models.EntityBrief
	Contradictions []models.Contradiction
	PlotHoles      []models.PlotHole
}

// AnalysisHub is the minimal WebSocket hub interface used by AnalysisService.
// ws.Hub satisfies this interface via its SendToUser method.
type AnalysisHub interface {
	SendToUser(userID uuid.UUID, msg models.WSMessage) error
}

// Reactivatr is the minimal relevance interface used by AnalysisService
// to reactivate archived entities and touch entity relevance.
// RelevanceService satisfies this interface.
type Reactivatr interface {
	Touch(ctx context.Context, entityID, chapterID uuid.UUID) error
	Reactivate(ctx context.Context, entityID uuid.UUID) error
}

// EntityResolvr resolves or creates an entity from extracted data.
// EntityService satisfies this interface.
type EntityResolvr interface {
	ResolveOrCreate(ctx context.Context, universeID uuid.UUID, data repositories.ExtractedEntity) (*models.Entity, string, bool, error)
}

// AnalysisService runs a per-work sequential analysis queue.
//
// ponytail: one goroutine per work, sequential queue. No worker pool needed
// for hackathon scale. Cancel/Shutdown stop the goroutine.
type AnalysisService struct {
	pool        *pgxpool.Pool
	entitySvc   EntityResolvr
	contraSvc   *ContradictionService
	relevSvc    Reactivatr
	timelineSvc *TimelineService
	plotHoleSvc *PlotHoleService
	qwenSvc     *QwenService
	hub         AnalysisHub
	memorySvc   *MemoryService

	queues  map[uuid.UUID]chan analysisJob
	cancels map[uuid.UUID]context.CancelFunc
	mu      sync.Mutex
}

// NewAnalysisService creates an analysis service. All parameters may be nil
// for testing; Submit will only enqueue. Workers start via runWorker.
func NewAnalysisService(
	pool *pgxpool.Pool,
	entitySvc EntityResolvr,
	contraSvc *ContradictionService,
	relevSvc Reactivatr,
	timelineSvc *TimelineService,
	plotHoleSvc *PlotHoleService,
	qwenSvc *QwenService,
	hub AnalysisHub,
	memorySvc *MemoryService,
) *AnalysisService {
	return &AnalysisService{
		pool:        pool,
		entitySvc:   entitySvc,
		contraSvc:   contraSvc,
		relevSvc:    relevSvc,
		timelineSvc: timelineSvc,
		plotHoleSvc: plotHoleSvc,
		qwenSvc:     qwenSvc,
		hub:         hub,
		memorySvc:   memorySvc,
		queues:      make(map[uuid.UUID]chan analysisJob),
		cancels:     make(map[uuid.UUID]context.CancelFunc),
	}
}

// SubmitParagraph is a convenience wrapper that satisfies the ws.ParagraphSubmitter
// interface. It creates an analysisJob and enqueues it via Submit.
// Starts a worker goroutine for first-time work submissions.
func (s *AnalysisService) SubmitParagraph(ctx context.Context, workID, chapterID, universeID, userID uuid.UUID, text string) error {
	s.mu.Lock()
	_, exists := s.queues[workID]
	s.mu.Unlock()

	job := analysisJob{
		WorkID:     workID,
		ChapterID:  chapterID,
		UniverseID: universeID,
		Text:       text,
		UserID:     userID,
	}

	if err := s.Submit(ctx, job); err != nil {
		return err
	}

	// Start a worker if this is a new work ID
	if !exists {
		go s.runWorker(workID)
	}

	return nil
}

// Submit enqueues an analysis job into the per-work channel.
func (s *AnalysisService) Submit(ctx context.Context, job analysisJob) error {
	s.mu.Lock()
	q, exists := s.queues[job.WorkID]
	if !exists {
		q = make(chan analysisJob, 100)
		s.queues[job.WorkID] = q
	}
	s.mu.Unlock()

	select {
	case q <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cancel stops the worker goroutine for the given workID.
func (s *AnalysisService) Cancel(workID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, exists := s.cancels[workID]; exists {
		cancel()
		delete(s.cancels, workID)
	}
	delete(s.queues, workID)
}

// Shutdown cancels all running workers and removes all queues.
func (s *AnalysisService) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for workID, cancel := range s.cancels {
		cancel()
		delete(s.cancels, workID)
	}
	for workID := range s.queues {
		delete(s.queues, workID)
	}
}

// runWorker starts a goroutine that drains the per-work queue sequentially.
func (s *AnalysisService) runWorker(workID uuid.UUID) {
	s.mu.Lock()
	if _, exists := s.cancels[workID]; exists {
		s.mu.Unlock()
		return
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	s.cancels[workID] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.cancels, workID)
		s.mu.Unlock()
	}()

	s.mu.Lock()
	q, exists := s.queues[workID]
	s.mu.Unlock()
	if !exists {
		return
	}

	for {
		select {
		case job, ok := <-q:
			if !ok {
				return
			}
			result, err := s.processJob(workerCtx, job)
			if err != nil {
				log.Printf("[analysis] work %s job failed: %v", workID, err)
				continue
			}
			if s.hub != nil && result != nil {
				s.broadcastResult(job.UserID, *result)
			}
		case <-workerCtx.Done():
			return
		}
	}
}

// processJob runs the full analysis pipeline for a single paragraph.
//
// ponytail: sequential pipeline — core pass then enrichment pass.
func (s *AnalysisService) processJob(ctx context.Context, job analysisJob) (*AnalysisResult, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("analysis context cancelled: %w", ctx.Err())
	}

	result := &AnalysisResult{
		WorkID:    job.WorkID,
		ChapterID: job.ChapterID,
	}

	// ── Core Pass (deterministic, fast) ──

	// 1. Extract entities from paragraph text
	var resolvedEntities []ResolvedEntity
	if s.entitySvc != nil && s.pool != nil {
		entities, err := s.extractEntities(ctx, job.UniverseID, job.Text, job.ChapterID)
		if err != nil {
			log.Printf("[analysis] extract entities: %v", err)
		} else {
			resolvedEntities = entities
			for _, re := range resolvedEntities {
				result.Entities = append(result.Entities, models.EntityBrief{
					ID:   re.Entity.ID,
					Name: re.Entity.Name,
					Type: re.Entity.Type,
				})
			}
		}
	}

	entityCount := len(result.Entities)
	s.sendProgress(job.UserID, job.ChapterID, "entities_extracted", func(p *models.AnalysisProgressPayload) {
		p.EntityCount = &entityCount
	})

	// 2. Deterministic contradiction checks (deceased/alive rules)
	if s.contraSvc != nil && len(resolvedEntities) > 0 {
		deterministic, err := s.contraSvc.CheckDeterministic(ctx, job.UniverseID, job.ChapterID, resolvedEntities)
		if err != nil {
			log.Printf("[analysis] deterministic check: %v", err)
		} else {
			result.Contradictions = append(result.Contradictions, deterministic...)
		}
	}

	// 3. Touch relevance for each mentioned entity
	if s.relevSvc != nil {
		for _, re := range resolvedEntities {
			if err := s.relevSvc.Touch(ctx, re.Entity.ID, job.ChapterID); err != nil {
				log.Printf("[analysis] touch entity %s: %v", re.Entity.ID, err)
			}
		}
	}

	// 3b. Analyze relationships and create graph edges
	if s.qwenSvc != nil && len(resolvedEntities) > 0 && s.pool != nil {
		entityNames := make([]string, len(resolvedEntities))
		for i, re := range resolvedEntities {
			entityNames[i] = re.Entity.Name
		}
		relationships, err := s.qwenSvc.AnalyzeRelationships(ctx, job.Text, entityNames)
		if err != nil {
			log.Printf("[analysis] analyze relationships: %v", err)
		} else if s.hub != nil {
			graphName := "universe_" + job.UniverseID.String()
			graphRepo := repositories.NewGraphRepo(s.pool)
			for _, rel := range relationships {
				source, _ := rel["source"].(string)
				target, _ := rel["target"].(string)
				relType, _ := rel["type"].(string)
				if source == "" || target == "" || relType == "" {
					continue
				}
				var sourceID, targetID *models.Entity
				for _, re := range resolvedEntities {
					if re.Entity.Name == source {
						sourceID = &re.Entity
					}
					if re.Entity.Name == target {
						targetID = &re.Entity
					}
				}
				if sourceID != nil && targetID != nil {
					if err := graphRepo.CreateEdge(ctx, graphName, sourceID.ID.String(), targetID.ID.String(), relType, nil); err != nil {
						log.Printf("[analysis] create edge %s->%s: %v", source, target, err)
					}
				}
			}
		}
	}

	// 3c. Emit entity_discovered for each NEW entity
	if s.hub != nil {
		for _, re := range resolvedEntities {
			if !re.IsNew {
				continue
			}
			payloadBytes, _ := json.Marshal(models.EntityDiscoveredPayload{
				Entity: re.Entity,
				IsNew:  true,
			})
			wsMsg := models.WSMessage{
				Type:    "entity_discovered",
				Payload: payloadBytes,
			}
			if err := s.hub.SendToUser(job.UserID, wsMsg); err != nil {
				log.Printf("[analysis] send entity_discovered: %v", err)
			}
		}
	}

	// 3d. Emit graph_updated
	if s.hub != nil {
		graphPayload, _ := json.Marshal(models.GraphUpdatedPayload{
			UniverseID: job.UniverseID,
			Action:     "relationships_added",
		})
		graphMsg := models.WSMessage{
			Type:    "graph_updated",
			Payload: graphPayload,
		}
		if err := s.hub.SendToUser(job.UserID, graphMsg); err != nil {
			log.Printf("[analysis] send graph_updated: %v", err)
		}
	}

	// ── Enrichment Pass (Qwen-Max) ──

	// 4. Semantic contradiction checks via Qwen-Max
	s.sendProgress(job.UserID, job.ChapterID, "checking_contradictions", nil)
	if s.contraSvc != nil && len(resolvedEntities) > 0 {
		semantic, err := s.contraSvc.CheckSemantic(ctx, job.UniverseID, job.ChapterID, job.Text, resolvedEntities, func(stage string, tc *QwenToolCall) {
			// ponytail: forward streamed tool-call progress under the same
			// checking_contradictions stage — no new WS stage invented for
			// per-tool-call granularity, matching the design's 5-stage list.
			s.sendProgress(job.UserID, job.ChapterID, "checking_contradictions", nil)
		})
		if err != nil {
			log.Printf("[analysis] semantic check: %v", err)
		} else {
			result.Contradictions = append(result.Contradictions, semantic...)
		}
	}
	contradictionCount := len(result.Contradictions)
	s.sendProgress(job.UserID, job.ChapterID, "contradictions_checked", func(p *models.AnalysisProgressPayload) {
		p.ContradictionCount = &contradictionCount
	})

	// 5. Scan for plot holes
	if s.plotHoleSvc != nil {
		holes, err := s.plotHoleSvc.Scan(ctx, job.UniverseID, job.ChapterID)
		if err != nil {
			log.Printf("[analysis] plot hole scan: %v", err)
		} else {
			result.PlotHoles = holes
		}
	}
	plotHoleCount := len(result.PlotHoles)
	s.sendProgress(job.UserID, job.ChapterID, "plot_holes_scanned", func(p *models.AnalysisProgressPayload) {
		p.PlotHoleCount = &plotHoleCount
	})

	// 6. Contextual recall after analysis
	if s.memorySvc != nil && len(resolvedEntities) > 0 {
		items, err := s.memorySvc.Recall(ctx, job.UniverseID, nil, 5)
		if err != nil {
			log.Printf("[analysis] contextual recall: %v", err)
		} else if s.hub != nil && len(items) > 0 {
			recallPayload, _ := json.Marshal(map[string]interface{}{
				"universe_id": job.UniverseID,
				"items":       items,
			})
			recallMsg := models.WSMessage{
				Type:    "contextual_recall",
				Payload: recallPayload,
			}
			if err := s.hub.SendToUser(job.UserID, recallMsg); err != nil {
				log.Printf("[analysis] send contextual_recall: %v", err)
			}
		}
	}

	// 7. Report context budget usage, if a budget manager is configured.
	// ponytail: input tokens coarse-estimated from job.Text alone (no access
	// to the exact system/user prompts CheckSemantic built) — good enough for
	// a progress indicator, apply refines if a precise figure is needed.
	if s.qwenSvc != nil && s.qwenSvc.budgetMgr != nil {
		mgr := s.qwenSvc.budgetMgr
		alloc := mgr.ComputeBudget(0, mgr.tok.CountTokens(job.Text))
		report := alloc.Report(mgr.maxContextTokens)
		s.sendProgress(job.UserID, job.ChapterID, "context_budget", func(p *models.AnalysisProgressPayload) {
			p.Budget = report
		})
	} else {
		s.sendProgress(job.UserID, job.ChapterID, "context_budget", nil)
	}

	log.Printf("[analysis] work=%s chapter=%s: %d entities, %d contradictions, %d plot holes",
		job.WorkID, job.ChapterID, len(result.Entities), len(result.Contradictions), len(result.PlotHoles))

	return result, nil
}

// extractEntities resolves or creates entities from paragraph text.
//
// ponytail: best-effort; if QwenService is nil or extraction fails, returns
// empty slice rather than failing the whole job.
func (s *AnalysisService) extractEntities(ctx context.Context, universeID uuid.UUID, text string, chapterID uuid.UUID) ([]ResolvedEntity, error) {
	if s.qwenSvc == nil || s.entitySvc == nil {
		return nil, nil
	}

	extracted, err := s.qwenSvc.ExtractEntities(ctx, text, "")
	if err != nil {
		return nil, fmt.Errorf("qwen extract: %w", err)
	}

	// Collect all extracted entities from all categories
	allEntities := make([]ExtractedEntity, 0)
	if extracted != nil {
		allEntities = append(allEntities, extracted.Characters...)
		allEntities = append(allEntities, extracted.Places...)
		allEntities = append(allEntities, extracted.Events...)
		allEntities = append(allEntities, extracted.Factions...)
		allEntities = append(allEntities, extracted.WorldRules...)
		allEntities = append(allEntities, extracted.PlotDevelopments...)
	}

	// ponytail: use first 120 chars of text as mention context
	mentionText := text
	if len(mentionText) > 120 {
		mentionText = mentionText[:120]
	}

	var resolved []ResolvedEntity
	for _, ee := range allEntities {
		entityData := repositories.ExtractedEntity{
			Type:        ee.Type,
			Name:        ee.Name,
			Aliases:     ee.Aliases,
			Description: ee.Description,
			Status:      ee.Status,
			Properties:  ee.Properties,
		}
		entity, previousStatus, isNew, err := s.entitySvc.ResolveOrCreate(ctx, universeID, entityData)
		if err != nil {
			log.Printf("[analysis] resolve entity %s: %v", ee.Name, err)
			continue
		}
		resolved = append(resolved, ResolvedEntity{
				Entity:         *entity,
				MentionText:    mentionText,
				IsNew:          isNew,
				PreviousStatus: previousStatus,
			})

			// spec: when an archived entity is re-mentioned, reactivate it
			if previousStatus == "archived" && s.relevSvc != nil {
				if err := s.relevSvc.Reactivate(ctx, entity.ID); err != nil {
					log.Printf("[analysis] reactivate entity %s: %v", entity.ID, err)
				}
			}
		}

	return resolved, nil
}

// sendProgress emits an analysis_progress WS message for a single processJob
// pipeline stage. mut may be nil; when provided it sets stage-specific
// fields (counts, budget) on the payload before it's sent.
//
// ponytail: fires unconditionally at each of the 5 documented stages,
// regardless of whether that stage's underlying step actually ran (e.g. nil
// entitySvc) — the stage marks "pipeline reached here", counts just stay
// zero/omitted when there's nothing to report. Matches existing
// hub.SendToUser error handling elsewhere in processJob: log and continue,
// never abort the pipeline.
func (s *AnalysisService) sendProgress(userID, chapterID uuid.UUID, stage string, mut func(*models.AnalysisProgressPayload)) {
	if s.hub == nil {
		return
	}

	payload := models.AnalysisProgressPayload{Stage: stage, ChapterID: chapterID}
	if mut != nil {
		mut(&payload)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[analysis] marshal progress %s: %v", stage, err)
		return
	}

	msg := models.WSMessage{Type: ws.TypeAnalysisProgress, Payload: payloadBytes}
	if err := s.hub.SendToUser(userID, msg); err != nil {
		log.Printf("[analysis] send progress %s: %v", stage, err)
	}
}

// broadcastResult pushes the analysis result to the user's WebSocket connection.
func (s *AnalysisService) broadcastResult(userID uuid.UUID, result AnalysisResult) {
	payloadBytes, err := json.Marshal(map[string]interface{}{
		"work_id":        result.WorkID,
		"chapter_id":     result.ChapterID,
		"entities":       result.Entities,
		"contradictions": result.Contradictions,
		"plot_holes":     result.PlotHoles,
	})
	if err != nil {
		log.Printf("[analysis] marshal result: %v", err)
		return
	}

	msg := models.WSMessage{
		Type:    "analysis_result",
		Payload: payloadBytes,
	}

	if err := s.hub.SendToUser(userID, msg); err != nil {
		log.Printf("[analysis] send result to user %s: %v", userID, err)
	}
}
