package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/quill/backend/internal/models"
)

type ConsolidationRepo struct {
	pool *pgxpool.Pool
}

func NewConsolidationRepo(pool *pgxpool.Pool) *ConsolidationRepo {
	return &ConsolidationRepo{pool: pool}
}

func (r *ConsolidationRepo) Create(ctx context.Context, cm *models.ConsolidatedMemory) error {
	query := `
		INSERT INTO consolidated_memories (id, entity_id, summary, key_facts, embedding, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (entity_id) DO UPDATE
		SET summary = $3, key_facts = $4, embedding = $5, created_at = $6
	`
	now := time.Now()
	_, err := r.pool.Exec(ctx, query,
		cm.ID, cm.EntityID, cm.Summary, cm.KeyFacts,
		pgvector.NewVector(cm.Embedding), now,
	)
	if err != nil {
		return fmt.Errorf("create consolidated memory: %w", err)
	}
	return nil
}

func (r *ConsolidationRepo) FindByEntityID(ctx context.Context, entityID uuid.UUID) (*models.ConsolidatedMemory, error) {
	query := `
		SELECT id, entity_id, summary, key_facts, embedding, created_at
		FROM consolidated_memories
		WHERE entity_id = $1
	`
	cm := &models.ConsolidatedMemory{}
	var vec pgvector.Vector
	err := r.pool.QueryRow(ctx, query, entityID).Scan(
		&cm.ID, &cm.EntityID, &cm.Summary, &cm.KeyFacts, &vec, &cm.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("consolidated memory not found for entity %s", entityID)
	}
	if err != nil {
		return nil, fmt.Errorf("find consolidated memory: %w", err)
	}
	cm.Embedding = vec.Slice()
	return cm, nil
}

// ConsolidatedHit is a ranked consolidated-memory result, scoped to a
// universe via the entities join.
type ConsolidatedHit struct {
	EntityID uuid.UUID
	Summary  string
	Distance float64
}

// FindSimilarByEmbedding ranks consolidated memories by cosine distance to
// queryEmbedding, joining entities to scope results to the given universe.
func (r *ConsolidationRepo) FindSimilarByEmbedding(ctx context.Context, universeID uuid.UUID, queryEmbedding []float32, k int) ([]ConsolidatedHit, error) {
	query := `
		SELECT cm.entity_id, cm.summary, cm.embedding <=> $1 AS distance
		FROM consolidated_memories cm
		JOIN entities e ON cm.entity_id = e.id
		WHERE e.universe_id = $2
		ORDER BY distance ASC
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, query, pgvector.NewVector(queryEmbedding), universeID, k)
	if err != nil {
		return nil, fmt.Errorf("find similar consolidated memories: %w", err)
	}
	defer rows.Close()

	var hits []ConsolidatedHit
	for rows.Next() {
		var h ConsolidatedHit
		if err := rows.Scan(&h.EntityID, &h.Summary, &h.Distance); err != nil {
			return nil, fmt.Errorf("scan consolidated hit: %w", err)
		}
		hits = append(hits, h)
	}
	return hits, nil
}

// DeleteByEntityID removes the consolidated memory row for a given entity.
// spec: idempotent — no error when no row exists.
func (r *ConsolidationRepo) DeleteByEntityID(ctx context.Context, entityID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM consolidated_memories WHERE entity_id = $1`, entityID)
	if err != nil {
		return fmt.Errorf("delete consolidated memory: %w", err)
	}
	return nil
}
