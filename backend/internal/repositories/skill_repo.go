package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

// SkillRepo persists only per-universe activation state. Skill content is an
// immutable, versioned application asset loaded by services.SkillRegistry.
type SkillRepo struct {
	pool *pgxpool.Pool
}

func NewSkillRepo(pool *pgxpool.Pool) *SkillRepo {
	return &SkillRepo{pool: pool}
}

func (r *SkillRepo) ListActive(ctx context.Context, universeID uuid.UUID) ([]models.UniverseSkill, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT universe_id, skill_name, activated_at
		FROM universe_skills
		WHERE universe_id = $1
		ORDER BY skill_name`, universeID)
	if err != nil {
		return nil, fmt.Errorf("list active skills: %w", err)
	}
	defer rows.Close()

	activations := make([]models.UniverseSkill, 0)
	for rows.Next() {
		var activation models.UniverseSkill
		if err := rows.Scan(&activation.UniverseID, &activation.SkillName, &activation.ActivatedAt); err != nil {
			return nil, fmt.Errorf("scan active skill: %w", err)
		}
		activations = append(activations, activation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active skills: %w", err)
	}
	return activations, nil
}

// Replace atomically replaces one universe's activation set. Validation of
// names belongs to SkillRegistry; this repository only handles persistence.
func (r *SkillRepo) Replace(ctx context.Context, universeID uuid.UUID, names []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin skill activation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := r.ReplaceTx(ctx, tx, universeID, names); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit skill activations: %w", err)
	}
	return nil
}

func (r *SkillRepo) ReplaceTx(ctx context.Context, tx pgx.Tx, universeID uuid.UUID, names []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM universe_skills WHERE universe_id = $1`, universeID); err != nil {
		return fmt.Errorf("clear skill activations: %w", err)
	}
	for _, name := range names {
		if _, err := tx.Exec(ctx, `
			INSERT INTO universe_skills (universe_id, skill_name)
			VALUES ($1, $2)
			ON CONFLICT (universe_id, skill_name) DO NOTHING`, universeID, name); err != nil {
			return fmt.Errorf("activate skill %q: %w", name, err)
		}
	}
	return nil
}
