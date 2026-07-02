package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

type ContradictionRepo struct {
	pool *pgxpool.Pool
}

func NewContradictionRepo(pool *pgxpool.Pool) *ContradictionRepo {
	return &ContradictionRepo{pool: pool}
}

func (r *ContradictionRepo) Create(ctx context.Context, c *models.Contradiction) error {
	query := `
		INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
			evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id, fingerprint, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
	`
	_, err := r.pool.Exec(ctx, query,
		c.ID, c.UniverseID, c.EntityID, c.Severity, c.Description, c.Suggestion,
		c.EvidenceA, c.EvidenceAChapterID, c.EvidenceB, c.EvidenceBChapterID,
		c.Fingerprint, c.Status,
	)
	if err != nil {
		return fmt.Errorf("create contradiction: %w", err)
	}
	return nil
}

func (r *ContradictionRepo) FindByFingerprint(ctx context.Context, fingerprint string) (*models.Contradiction, error) {
	query := `
		SELECT id, universe_id, entity_id, severity, description, suggestion,
		       evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
		       fingerprint, status, resolved_at, created_at
		FROM contradictions WHERE fingerprint = $1
	`
	c := &models.Contradiction{}
	err := r.pool.QueryRow(ctx, query, fingerprint).Scan(
		&c.ID, &c.UniverseID, &c.EntityID, &c.Severity, &c.Description, &c.Suggestion,
		&c.EvidenceA, &c.EvidenceAChapterID, &c.EvidenceB, &c.EvidenceBChapterID,
		&c.Fingerprint, &c.Status, &c.ResolvedAt, &c.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil // ponytail: nil,nil means "not found but not an error"
	}
	if err != nil {
		return nil, fmt.Errorf("find contradiction by fingerprint: %w", err)
	}
	return c, nil
}

func (r *ContradictionRepo) ListByUniverse(ctx context.Context, universeID uuid.UUID) ([]models.Contradiction, error) {
	query := `
		SELECT id, universe_id, entity_id, severity, description, suggestion,
		       evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
		       fingerprint, status, resolved_at, created_at
		FROM contradictions WHERE universe_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, universeID)
	if err != nil {
		return nil, fmt.Errorf("list contradictions: %w", err)
	}
	defer rows.Close()

	result := []models.Contradiction{}
	for rows.Next() {
		var c models.Contradiction
		if err := rows.Scan(
			&c.ID, &c.UniverseID, &c.EntityID, &c.Severity, &c.Description, &c.Suggestion,
			&c.EvidenceA, &c.EvidenceAChapterID, &c.EvidenceB, &c.EvidenceBChapterID,
			&c.Fingerprint, &c.Status, &c.ResolvedAt, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contradiction: %w", err)
		}
		result = append(result, c)
	}
	return result, nil
}

func (r *ContradictionRepo) Resolve(ctx context.Context, id uuid.UUID, resolvedAt *time.Time) error {
	query := `UPDATE contradictions SET status = 'resolved', resolved_at = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, resolvedAt)
	if err != nil {
		return fmt.Errorf("resolve contradiction: %w", err)
	}
	return nil
}

// Dismiss marks a contradiction as dismissed without resolving.
func (r *ContradictionRepo) Dismiss(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE contradictions SET status = 'dismissed', resolved_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("dismiss contradiction: %w", err)
	}
	return nil
}
