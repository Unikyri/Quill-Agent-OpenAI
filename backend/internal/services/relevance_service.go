package services

import (
	"context"
	"log"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/repositories"
)

// RelevanceService manages entity relevance scoring with exponential decay.
//
// ponytail: per-chapter event-driven decay; no background worker needed.
// DecayAll runs on chapter advance; Touch on entity mention; Reactivate on
// manual override or story relevance.
type RelevanceService struct {
	pool             *pgxpool.Pool
	entityRepo       *repositories.EntityRepo
	lambda           float64
	archiveThreshold float64
	consolidationSvc Consolidator
}

// NewRelevanceService creates a relevance service with the given decay lambda
// and archive threshold. lambda controls the decay rate per chapter advance;
// archiveThreshold is the score below which entities get archived.
// consolidationSvc may be nil — deconsolidation on reactivate will be skipped.
func NewRelevanceService(pool *pgxpool.Pool, entityRepo *repositories.EntityRepo, lambda, archiveThreshold float64, consolidationSvc Consolidator) *RelevanceService {
	return &RelevanceService{
		pool:             pool,
		entityRepo:       entityRepo,
		lambda:           lambda,
		archiveThreshold: archiveThreshold,
		consolidationSvc: consolidationSvc,
	}
}

// Touch resets the idle counter for a mentioned entity, updating
// last_mentioned_chapter_id and last_mentioned_at.
func (s *RelevanceService) Touch(ctx context.Context, entityID, chapterID uuid.UUID) error {
	return s.entityRepo.TouchBatch(ctx, []uuid.UUID{entityID}, chapterID)
}

// Reactivate sets the entity's score to 0.8 and status to "active".
// Used when a previously archived entity becomes relevant again.
// Also triggers deconsolidation: removes the consolidated memory row
// since the entity is no longer fully archived.
func (s *RelevanceService) Reactivate(ctx context.Context, entityID uuid.UUID) error {
	e, err := s.entityRepo.FindByID(ctx, entityID)
	if err != nil {
		return err
	}

	e.RelevanceScore = 0.8
	e.Status = "active"

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.entityRepo.Update(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// spec: after reactivation, deconsolidate (nil-safe)
	if s.consolidationSvc != nil {
		if err := s.consolidationSvc.DeconsolidateEntity(ctx, entityID); err != nil {
			log.Printf("[relevance] deconsolidate entity %s: %v", entityID, err)
		}
	}

	return nil
}

// DecayAll applies exponential decay (score *= e^-lambda) to all active entities
// in the universe. Entities whose score drops below archiveThreshold are set to
// status "archived". After archiving, newly-archived entities are consolidated
// asynchronously (fire-and-forget, errors logged).
//
// ponytail: single-pass strategy — decay and archive in one call; no separate
// archive sweep needed.
func (s *RelevanceService) DecayAll(ctx context.Context, universeID uuid.UUID) error {
	if err := s.entityRepo.DecayAll(ctx, universeID, s.lambda); err != nil {
		return err
	}

	// Identify entities about to be archived BEFORE the UPDATE
	newlyArchivedIDs, err := s.entityRepo.FindNewlyArchivable(ctx, universeID, s.archiveThreshold)
	if err != nil {
		log.Printf("[relevance] find newly archivable: %v", err)
	}

	// Archive entities that fell below threshold
	_, err = s.pool.Exec(ctx, `
		UPDATE entities SET status = 'archived', updated_at = NOW()
		WHERE universe_id = $1 AND status = 'active' AND relevance_score <= $2
	`, universeID, s.archiveThreshold)
	if err != nil {
		return err
	}

	// Fire-and-forget consolidation goroutines for newly-archived entities
	if s.consolidationSvc != nil && len(newlyArchivedIDs) > 0 {
		for _, entityID := range newlyArchivedIDs {
			go func(eid uuid.UUID) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[relevance] panic consolidating entity %s: %v", eid, r)
					}
				}()
				if err := s.consolidationSvc.ConsolidateEntity(context.Background(), eid, universeID); err != nil {
					log.Printf("[relevance] consolidate entity %s: %v", eid, err)
				}
			}(entityID)
		}
	}

	return nil
}

// applyDecay computes the decayed relevance score after idle chapters.
// score *= e^(-lambda * idle). Exported for testing.
func applyDecay(score float64, idleChapters float64, lambda float64) float64 {
	return score * math.Exp(-lambda*idleChapters)
}
