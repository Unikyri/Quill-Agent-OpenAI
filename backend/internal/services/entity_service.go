package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

type EntityService struct {
	pool                *pgxpool.Pool
	entityRepo          *repositories.EntityRepo
	vectorRepo          *repositories.VectorRepo
	qwenSvc             LLMService
	historyRepo         *repositories.EntityRelevanceHistoryRepo
	confidenceThreshold float64
}

func NewEntityService(pool *pgxpool.Pool, entityRepo *repositories.EntityRepo, vectorRepo *repositories.VectorRepo, qwenSvc LLMService) *EntityService {
	return &EntityService{
		pool: pool, entityRepo: entityRepo, vectorRepo: vectorRepo, qwenSvc: qwenSvc,
		historyRepo: repositories.NewEntityRelevanceHistoryRepo(pool), confidenceThreshold: 0.70,
	}
}

// SetConfidenceThreshold configures the extraction gate without changing the
// long-lived constructor contract used by analysis and ingestion tests.
func (s *EntityService) SetConfidenceThreshold(threshold float64) {
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1 {
		threshold = 1
	}
	s.confidenceThreshold = threshold
}

func (s *EntityService) ConfidenceThreshold() float64 {
	if s.confidenceThreshold <= 0 {
		return 0.70
	}
	return s.confidenceThreshold
}

func (s *EntityService) GetByID(ctx context.Context, id uuid.UUID) (*models.Entity, error) {
	return s.entityRepo.FindByID(ctx, id)
}

func (s *EntityService) lockResolvedEntity(ctx context.Context, tx pgx.Tx, existing *models.Entity) (*models.Entity, error) {
	if existing == nil {
		return nil, nil
	}
	locked, err := s.entityRepo.FindByIDTx(ctx, tx, existing.ID)
	if err != nil {
		return nil, fmt.Errorf("lock resolved entity %s: %w", existing.ID, err)
	}
	return locked, nil
}

func (s *EntityService) ListByUniverse(ctx context.Context, universeID uuid.UUID, filters repositories.EntityFilters) ([]models.Entity, int, error) {
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.Limit < 1 || filters.Limit > 100 {
		filters.Limit = 50
	}
	return s.entityRepo.ListByUniverse(ctx, universeID, filters)
}

func (s *EntityService) CountByType(ctx context.Context, universeID uuid.UUID) (map[string]int, error) {
	return s.entityRepo.CountByType(ctx, universeID)
}

func (s *EntityService) Update(ctx context.Context, id uuid.UUID, input models.UpdateEntityRequest) (*models.Entity, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	e, err := s.entityRepo.FindByIDTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if input.Status != "" && input.Status != e.Status && (isCandidateLifecycleStatus(input.Status) || isCandidateLifecycleStatus(e.Status)) {
		return nil, errors.New("candidate status transitions must use AcceptCandidate, DismissCandidate, or MergeCandidate")
	}

	if input.Name != "" {
		e.Name = input.Name
	}
	if len(input.Aliases) > 0 {
		e.Aliases = input.Aliases
	}
	if input.Description != "" {
		e.Description = input.Description
	}
	if input.Status != "" {
		e.Status = input.Status
	}
	if input.Properties != nil {
		e.Properties = s.entityRepo.MergeProperties(e.Properties, input.Properties)
	}

	if err := s.entityRepo.Update(ctx, tx, e); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return e, nil
}

// ResolveOrCreate finds or creates the entity matching data, merging in new
// mention data. The returned previousStatus is the entity's status as it was
// in the DB *before* this merge — callers doing contradiction checks (e.g.
// deceased/alive) must compare against previousStatus, not the returned
// entity's Status, since mergeEntity already overwrites Status with data.Status.
// previousStatus is "" when a brand-new entity is created (step 4).
func (s *EntityService) ResolveOrCreate(ctx context.Context, universeID uuid.UUID, data repositories.ExtractedEntity) (entity *models.Entity, previousStatus string, isNew bool, err error) {
	data = normalizeExtractedEntity(data, s.ConfidenceThreshold())
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Step 1: Exact name match
	existing, err := s.entityRepo.FindByNaturalKey(ctx, universeID, data.Name, data.Type)
	if err == nil && existing != nil {
		existing, err = s.lockResolvedEntity(ctx, tx, existing)
		if err != nil {
			return nil, "", false, err
		}
		prevStatus := existing.Status
		merged := s.mergeEntity(existing, data)
		if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
			return nil, "", false, err
		}
		if err := s.promoteCandidateArtifacts(ctx, tx, merged, prevStatus); err != nil {
			return nil, "", false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, "", false, err
		}
		return merged, prevStatus, false, nil
	}

	// Step 1.5: Fuzzy name match (substring containment)
	existing, err = s.entityRepo.FindByFuzzyName(ctx, universeID, data.Name, data.Type)
	if err == nil && existing != nil {
		existing, err = s.lockResolvedEntity(ctx, tx, existing)
		if err != nil {
			return nil, "", false, err
		}
		prevStatus := existing.Status
		merged := s.mergeEntity(existing, data)
		if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
			return nil, "", false, err
		}
		if err := s.promoteCandidateArtifacts(ctx, tx, merged, prevStatus); err != nil {
			return nil, "", false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, "", false, err
		}
		return merged, prevStatus, false, nil
	}

	// Step 2: Alias match
	for _, alias := range data.Aliases {
		existing, err = s.entityRepo.FindByAliasAndType(ctx, universeID, alias, data.Type)
		if err == nil && existing != nil {
			existing, err = s.lockResolvedEntity(ctx, tx, existing)
			if err != nil {
				return nil, "", false, err
			}
			prevStatus := existing.Status
			merged := s.mergeEntity(existing, data)
			if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
				return nil, "", false, err
			}
			if err := s.promoteCandidateArtifacts(ctx, tx, merged, prevStatus); err != nil {
				return nil, "", false, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, "", false, err
			}
			return merged, prevStatus, false, nil
		}
	}

	// Step 3: Semantic similarity
	var embedding []float32
	if s.qwenSvc != nil && s.vectorRepo != nil {
		embedding, err = s.qwenSvc.GenerateEmbedding(ctx, data.Name)
	} else {
		err = errors.New("embedding unavailable")
	}
	if err == nil {
		similarID, _, err := s.vectorRepo.FindSimilarEntity(ctx, universeID, embedding, 0.15)
		if err == nil && similarID != nil {
			existing, err = s.entityRepo.FindByID(ctx, *similarID)
			if err == nil {
				existing, err = s.lockResolvedEntity(ctx, tx, existing)
				if err != nil {
					return nil, "", false, err
				}
				prevStatus := existing.Status
				merged := s.mergeEntity(existing, data)
				if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
					return nil, "", false, err
				}
				if err := s.promoteCandidateArtifacts(ctx, tx, merged, prevStatus); err != nil {
					return nil, "", false, err
				}
				if err := tx.Commit(ctx); err != nil {
					return nil, "", false, err
				}
				return merged, prevStatus, false, nil
			}
		}
	}

	// Step 4: Create new entity
	props, _ := json.Marshal(data.Properties)
	newEntity := &models.Entity{
		ID:             uuid.New(),
		UniverseID:     universeID,
		Type:           data.Type,
		Name:           data.Name,
		Aliases:        data.Aliases,
		Description:    data.Description,
		Properties:     props,
		Status:         data.Status,
		Confidence:     data.Confidence,
		RelevanceScore: 0.8,
	}

	if err := s.entityRepo.Create(ctx, tx, newEntity); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return nil, "", false, err
		}
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return nil, "", false, fmt.Errorf("rollback after entity natural-key conflict: %w", rollbackErr)
		}
		winner, fetchErr := s.entityRepo.FindByNaturalKey(ctx, universeID, data.Name, data.Type)
		if fetchErr != nil {
			return nil, "", false, fmt.Errorf("refetch entity after natural-key conflict: %w", fetchErr)
		}
		previousStatus := winner.Status
		merged := s.mergeEntity(winner, data)
		mergeTx, beginErr := s.pool.Begin(ctx)
		if beginErr != nil {
			return nil, "", false, fmt.Errorf("begin entity merge after natural-key conflict: %w", beginErr)
		}
		defer mergeTx.Rollback(ctx)
		winner, lockErr := s.entityRepo.FindByIDTx(ctx, mergeTx, winner.ID)
		if lockErr != nil {
			return nil, "", false, fmt.Errorf("lock entity after natural-key conflict: %w", lockErr)
		}
		previousStatus = winner.Status
		merged = s.mergeEntity(winner, data)
		if updateErr := s.entityRepo.Update(ctx, mergeTx, merged); updateErr != nil {
			return nil, "", false, fmt.Errorf("merge entity after natural-key conflict: %w", updateErr)
		}
		if promoteErr := s.promoteCandidateArtifacts(ctx, mergeTx, merged, previousStatus); promoteErr != nil {
			return nil, "", false, fmt.Errorf("promote entity after natural-key conflict: %w", promoteErr)
		}
		if commitErr := mergeTx.Commit(ctx); commitErr != nil {
			return nil, "", false, fmt.Errorf("commit entity merge after natural-key conflict: %w", commitErr)
		}
		return merged, previousStatus, false, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", false, err
	}
	if newEntity.Status == "candidate" {
		return newEntity, "", true, nil
	}

	// spec: entity creation writes the initial entity_relevance_history row (score 0.8)
	if s.historyRepo != nil {
		if err := s.historyRepo.AppendOne(ctx, newEntity.ID); err != nil {
			log.Printf("[entity] append history for new entity %s: %v", newEntity.Name, err)
		}
	}

	// Create node in AGE graph
	if s.pool != nil {
		graphRepo := repositories.NewGraphRepo(s.pool)
		props := map[string]interface{}{
			"entity_id":       newEntity.ID.String(),
			"name":            newEntity.Name,
			"status":          newEntity.Status,
			"relevance_score": newEntity.RelevanceScore,
		}
		graphName := "universe_" + newEntity.UniverseID.String()
		if err := graphRepo.CreateNode(ctx, graphName, newEntity.Type, props); err != nil {
			log.Printf("[entity] create node in graph for %s: %v", newEntity.Name, err)
		}
	}

	// Save embedding for new entity
	if s.vectorRepo != nil && s.qwenSvc != nil {
		emb, err := s.qwenSvc.GenerateEmbedding(ctx, data.Name)
		if err == nil {
			if saveErr := s.vectorRepo.SaveEntityEmbedding(ctx, newEntity.ID, emb); saveErr != nil {
				log.Printf("[entity] save embedding for %s: %v", newEntity.Name, saveErr)
			}
		}
	}

	return newEntity, "", true, nil
}

func (s *EntityService) mergeEntity(existing *models.Entity, newData repositories.ExtractedEntity) *models.Entity {
	merged := *existing

	// Merge aliases
	if len(newData.Aliases) > 0 {
		aliasSet := make(map[string]bool)
		for _, a := range merged.Aliases {
			aliasSet[a] = true
		}
		for _, a := range newData.Aliases {
			if !aliasSet[a] {
				merged.Aliases = append(merged.Aliases, a)
			}
		}
	}

	// Merge description (longer wins)
	if len(newData.Description) > len(merged.Description) {
		merged.Description = newData.Description
	}

	// Merge properties
	if newData.Properties != nil {
		merged.Properties = s.entityRepo.MergeProperties(merged.Properties, mustMarshal(newData.Properties))
	}

	// A writer decision is terminal for the generic extraction merge path. In
	// particular, a stale ResolveOrCreate lookup must not resurrect a dismissed
	// or merged candidate after the row is re-locked in its transaction.
	if existing.Status != "dismissed" && existing.Status != "merged" && newData.Status != "" {
		merged.Status = newData.Status
	}
	if newData.Confidence > merged.Confidence {
		merged.Confidence = newData.Confidence
	}
	if merged.Status == "candidate" && merged.Confidence >= s.ConfidenceThreshold() {
		merged.Status = "active"
	}

	return &merged
}

func isCandidateLifecycleStatus(status string) bool {
	switch status {
	case "candidate", "dismissed", "merged":
		return true
	default:
		return false
	}
}

func normalizeExtractedEntity(data repositories.ExtractedEntity, threshold float64) repositories.ExtractedEntity {
	if !data.ConfidenceSet {
		// Legacy providers and fixtures predate confidence; omitted values keep
		// their established auto-accept behaviour.
		data.Confidence = 1
	}
	if data.Confidence > 1 {
		data.Confidence = 1
	}
	if data.Status == "" || data.Status == "archived" {
		data.Status = "active"
	}
	if data.Confidence < threshold {
		data.Status = "candidate"
	}
	return data
}

// promoteCandidateArtifacts performs the side effects that make a candidate
// an active memory. It runs on the same transaction as the entity update so a
// failed graph/vector/history write leaves the candidate retryable.
func (s *EntityService) promoteCandidateArtifacts(ctx context.Context, tx pgx.Tx, entity *models.Entity, previousStatus string) error {
	if previousStatus != "candidate" || entity.Status != "active" {
		return nil
	}
	if s.pool == nil || s.vectorRepo == nil || s.qwenSvc == nil || s.historyRepo == nil {
		return errors.New("candidate promotion artifacts are not configured")
	}
	embedding, err := s.qwenSvc.GenerateEmbedding(ctx, entity.Name)
	if err != nil {
		return fmt.Errorf("generate candidate embedding: %w", err)
	}
	graphRepo := repositories.NewGraphRepo(s.pool)
	props := map[string]interface{}{"entity_id": entity.ID.String(), "name": entity.Name, "status": entity.Status, "relevance_score": entity.RelevanceScore}
	if err := graphRepo.CreateNodeTx(ctx, tx, "universe_"+entity.UniverseID.String(), entity.Type, props); err != nil {
		return fmt.Errorf("create candidate graph node: %w", err)
	}
	if err := s.vectorRepo.SaveEntityEmbeddingTx(ctx, tx, entity.ID, embedding); err != nil {
		return fmt.Errorf("save candidate embedding: %w", err)
	}
	if err := s.historyRepo.AppendOneTx(ctx, tx, entity.ID); err != nil {
		return fmt.Errorf("append candidate relevance history: %w", err)
	}
	return nil
}

func (s *EntityService) ListCandidates(ctx context.Context, universeID uuid.UUID) ([]models.EntityCandidate, error) {
	if s.entityRepo == nil {
		return []models.EntityCandidate{}, nil
	}
	return s.entityRepo.ListCandidates(ctx, universeID)
}

// AcceptCandidate promotes a candidate into active memory and creates its
// graph node only after explicit writer confirmation.
func (s *EntityService) AcceptCandidate(ctx context.Context, candidateID uuid.UUID) (*models.Entity, error) {
	return s.changeCandidateStatus(ctx, candidateID, "active", true)
}

// DismissCandidate keeps the row for an auditable decision while excluding it
// from active entity and candidate listings.
func (s *EntityService) DismissCandidate(ctx context.Context, candidateID uuid.UUID) (*models.Entity, error) {
	return s.changeCandidateStatus(ctx, candidateID, "dismissed", false)
}

func (s *EntityService) changeCandidateStatus(ctx context.Context, candidateID uuid.UUID, status string, createGraph bool) (*models.Entity, error) {
	if s.pool == nil || s.entityRepo == nil {
		return nil, errors.New("entity persistence is not configured")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin candidate decision: %w", err)
	}
	defer tx.Rollback(ctx)
	entity, err := s.entityRepo.FindByIDTx(ctx, tx, candidateID)
	if err != nil {
		return nil, err
	}
	if entity.Status != "candidate" {
		return nil, fmt.Errorf("entity is not a candidate")
	}
	var embedding []float32
	if createGraph {
		if s.vectorRepo == nil || s.qwenSvc == nil || s.historyRepo == nil {
			return nil, errors.New("candidate acceptance artifacts are not configured")
		}
		embedding, err = s.qwenSvc.GenerateEmbedding(ctx, entity.Name)
		if err != nil {
			return nil, fmt.Errorf("prepare candidate embedding: %w", err)
		}
	}
	if createGraph {
		graphRepo := repositories.NewGraphRepo(s.pool)
		props := map[string]interface{}{"entity_id": entity.ID.String(), "name": entity.Name, "status": "active", "relevance_score": entity.RelevanceScore}
		if err := graphRepo.CreateNodeTx(ctx, tx, "universe_"+entity.UniverseID.String(), entity.Type, props); err != nil {
			return nil, fmt.Errorf("prepare candidate graph node: %w", err)
		}
		if err := s.vectorRepo.SaveEntityEmbeddingTx(ctx, tx, entity.ID, embedding); err != nil {
			return nil, fmt.Errorf("prepare candidate vector: %w", err)
		}
	}
	entity.Status = status
	updated, err := s.entityRepo.UpdateCandidateStatus(ctx, tx, entity, "candidate")
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, errors.New("entity is no longer a candidate")
	}
	if createGraph {
		if err := s.historyRepo.AppendOneTx(ctx, tx, entity.ID); err != nil {
			return nil, fmt.Errorf("record candidate acceptance history: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit candidate decision: %w", err)
	}
	return entity, nil
}

// MergeCandidate merges descriptive data into an existing active target in
// the same universe, then marks the candidate as merged.
func (s *EntityService) MergeCandidate(ctx context.Context, candidateID, targetID uuid.UUID) (*models.Entity, error) {
	if s.pool == nil || s.entityRepo == nil {
		return nil, errors.New("entity persistence is not configured")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin candidate merge: %w", err)
	}
	defer tx.Rollback(ctx)
	candidate, err := s.entityRepo.FindByIDTx(ctx, tx, candidateID)
	if err != nil {
		return nil, err
	}
	target, err := s.entityRepo.FindByIDTx(ctx, tx, targetID)
	if err != nil {
		return nil, err
	}
	if candidate.Status != "candidate" {
		return nil, fmt.Errorf("entity is not a candidate")
	}
	if candidate.UniverseID != target.UniverseID || target.Status != "active" {
		return nil, fmt.Errorf("candidate and target must belong to the same active universe")
	}
	merged := s.mergeEntity(target, repositories.ExtractedEntity{
		Type: candidate.Type, Name: target.Name, Aliases: candidate.Aliases,
		Description: candidate.Description, Properties: propertiesMap(candidate.Properties),
		Status: target.Status, Confidence: candidate.Confidence,
	})
	merged.Status = "active"
	if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
		return nil, err
	}
	// Repoint every relational reference before marking the source merged. AGE
	// topology is intentionally not retargeted here: its dynamic edge labels
	// cannot be safely recreated generically without risking relationship data.
	if _, err := tx.Exec(ctx, `UPDATE entity_mentions SET entity_id = $1 WHERE entity_id = $2`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("merge entity mentions: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE entity_relevance_history SET entity_id = $1 WHERE entity_id = $2`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("merge relevance history: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE contradictions SET entity_id = $1 WHERE entity_id = $2`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("merge contradictions: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE timeline_events SET event_entity_id = $1 WHERE event_entity_id = $2`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("merge timeline event owner: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE timeline_events te
		SET participants = (
			SELECT ARRAY_AGG(DISTINCT COALESCE(mapping.winner_id, participant_id) ORDER BY COALESCE(mapping.winner_id, participant_id))
			FROM UNNEST(te.participants) AS participant_id
			LEFT JOIN (SELECT $2::uuid AS loser_id, $1::uuid AS winner_id) mapping ON mapping.loser_id = participant_id
		)
		WHERE te.participants @> ARRAY[$2::uuid]
	`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("merge timeline participants: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE plot_holes ph
		SET related_entity_ids = (
			SELECT ARRAY_AGG(DISTINCT COALESCE(mapping.winner_id, related_id) ORDER BY COALESCE(mapping.winner_id, related_id))
			FROM UNNEST(ph.related_entity_ids) AS related_id
			LEFT JOIN (SELECT $2::uuid AS loser_id, $1::uuid AS winner_id) mapping ON mapping.loser_id = related_id
		)
		WHERE ph.related_entity_ids @> ARRAY[$2::uuid]
	`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("merge plot-hole references: %w", err)
	}
	// Preserve one vector/consolidated row for the target when it has none;
	// otherwise discard the source row to satisfy each table's unique key.
	if _, err := tx.Exec(ctx, `
		WITH transferable AS (
			SELECT ee.id
			FROM entity_embeddings ee
			WHERE ee.entity_id = $2
			  AND NOT EXISTS (SELECT 1 FROM entity_embeddings winner WHERE winner.entity_id = $1)
			ORDER BY ee.updated_at DESC NULLS LAST, ee.id ASC
			LIMIT 1
		)
		UPDATE entity_embeddings ee SET entity_id = $1, updated_at = NOW()
		FROM transferable WHERE ee.id = transferable.id
	`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("transfer entity embedding: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM entity_embeddings WHERE entity_id = $1`, candidate.ID); err != nil {
		return nil, fmt.Errorf("delete merged entity embeddings: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		WITH transferable AS (
			SELECT cm.id
			FROM consolidated_memories cm
			WHERE cm.entity_id = $2
			  AND NOT EXISTS (SELECT 1 FROM consolidated_memories winner WHERE winner.entity_id = $1)
			ORDER BY cm.created_at DESC NULLS LAST, cm.id ASC
			LIMIT 1
		)
		UPDATE consolidated_memories cm SET entity_id = $1
		FROM transferable WHERE cm.id = transferable.id
	`, target.ID, candidate.ID); err != nil {
		return nil, fmt.Errorf("transfer consolidated memory: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM consolidated_memories WHERE entity_id = $1`, candidate.ID); err != nil {
		return nil, fmt.Errorf("delete merged consolidated memories: %w", err)
	}
	candidate.Status = "merged"
	updated, err := s.entityRepo.UpdateCandidateStatus(ctx, tx, candidate, "candidate")
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, errors.New("entity is no longer a candidate")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit candidate merge: %w", err)
	}
	return merged, nil
}

func propertiesMap(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var value map[string]interface{}
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return value
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
