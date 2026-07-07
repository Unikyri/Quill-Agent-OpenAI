package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EntityRelevanceHistoryRepo persists point-in-time snapshots of an entity's
// relevance_score/status, sampled at the 3 real mutation points (decay tick,
// reactivation, creation) — never on mention/Touch, since that doesn't
// change the score.
type EntityRelevanceHistoryRepo struct {
	pool *pgxpool.Pool
}

func NewEntityRelevanceHistoryRepo(pool *pgxpool.Pool) *EntityRelevanceHistoryRepo {
	return &EntityRelevanceHistoryRepo{pool: pool}
}

// AppendSnapshot inserts one history row per entity currently in the
// universe, capturing its post-decay relevance_score/status in a single
// set-based INSERT...SELECT. Called by RelevanceService.DecayAll right after
// the decay+archive UPDATEs.
func (r *EntityRelevanceHistoryRepo) AppendSnapshot(ctx context.Context, universeID uuid.UUID) error {
	query := `
		INSERT INTO entity_relevance_history (id, entity_id, universe_id, relevance_score, status, recorded_at)
		SELECT gen_random_uuid(), id, universe_id, relevance_score, status, NOW()
		FROM entities
		WHERE universe_id = $1
	`
	if _, err := r.pool.Exec(ctx, query, universeID); err != nil {
		return fmt.Errorf("append relevance snapshot: %w", err)
	}
	return nil
}

// AppendOne inserts a single history row for one entity, reading its current
// relevance_score/status. Used by Reactivate and entity creation.
func (r *EntityRelevanceHistoryRepo) AppendOne(ctx context.Context, entityID uuid.UUID) error {
	query := `
		INSERT INTO entity_relevance_history (id, entity_id, universe_id, relevance_score, status, recorded_at)
		SELECT gen_random_uuid(), id, universe_id, relevance_score, status, NOW()
		FROM entities
		WHERE id = $1
	`
	if _, err := r.pool.Exec(ctx, query, entityID); err != nil {
		return fmt.Errorf("append relevance history: %w", err)
	}
	return nil
}

// RelevanceHistoryPoint is one sampled (score, status) datapoint for an
// entity at a point in time.
type RelevanceHistoryPoint struct {
	EntityID       uuid.UUID
	RelevanceScore float64
	Status         string
	RecordedAt     time.Time
}

// ListRecentByUniverse returns the most recent n history rows per entity in
// the universe, capped via a ROW_NUMBER window function, returned
// oldest-first within each entity (ordered by entity_id then recorded_at).
func (r *EntityRelevanceHistoryRepo) ListRecentByUniverse(ctx context.Context, universeID uuid.UUID, n int) ([]RelevanceHistoryPoint, error) {
	query := `
		SELECT entity_id, relevance_score, status, recorded_at
		FROM (
			SELECT entity_id, relevance_score, status, recorded_at,
			       ROW_NUMBER() OVER (PARTITION BY entity_id ORDER BY recorded_at DESC) AS rn
			FROM entity_relevance_history
			WHERE universe_id = $1
		) ranked
		WHERE rn <= $2
		ORDER BY entity_id, recorded_at ASC
	`
	rows, err := r.pool.Query(ctx, query, universeID, n)
	if err != nil {
		return nil, fmt.Errorf("list recent relevance history: %w", err)
	}
	defer rows.Close()

	points := []RelevanceHistoryPoint{}
	for rows.Next() {
		var p RelevanceHistoryPoint
		if err := rows.Scan(&p.EntityID, &p.RelevanceScore, &p.Status, &p.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan relevance history point: %w", err)
		}
		points = append(points, p)
	}
	return points, nil
}
