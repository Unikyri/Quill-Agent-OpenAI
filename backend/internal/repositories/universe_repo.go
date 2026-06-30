package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

type UniverseRepo struct {
	pool *pgxpool.Pool
}

func NewUniverseRepo(pool *pgxpool.Pool) *UniverseRepo {
	return &UniverseRepo{pool: pool}
}

func (r *UniverseRepo) Create(ctx context.Context, tx pgx.Tx, u *models.Universe) error {
	query := `
		INSERT INTO universes (id, user_id, name, description, genre, format, session_id, is_demo_template, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	_, err := tx.Exec(ctx, query, u.ID, u.UserID, u.Name, u.Description, u.Genre, u.Format, u.SessionID, u.IsDemoTemplate)
	if err != nil {
		return fmt.Errorf("create universe: %w", err)
	}
	return nil
}

func (r *UniverseRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Universe, error) {
	query := `
		SELECT id, user_id, name, description, genre, format, session_id, is_demo_template, created_at, updated_at
		FROM universes WHERE id = $1
	`
	u := &models.Universe{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.UserID, &u.Name, &u.Description, &u.Genre, &u.Format,
		&u.SessionID, &u.IsDemoTemplate, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("universe not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find universe: %w", err)
	}
	return u, nil
}

func (r *UniverseRepo) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]models.Universe, int, error) {
	offset := (page - 1) * limit

	countQuery := `SELECT COUNT(*) FROM universes WHERE user_id = $1`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count universes: %w", err)
	}

	query := `
		SELECT id, user_id, name, description, genre, format, session_id, is_demo_template, created_at, updated_at
		FROM universes WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list universes: %w", err)
	}
	defer rows.Close()

	var universes []models.Universe
	for rows.Next() {
		var u models.Universe
		if err := rows.Scan(
			&u.ID, &u.UserID, &u.Name, &u.Description, &u.Genre, &u.Format,
			&u.SessionID, &u.IsDemoTemplate, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan universe: %w", err)
		}
		universes = append(universes, u)
	}

	return universes, total, nil
}

func (r *UniverseRepo) Update(ctx context.Context, tx pgx.Tx, u *models.Universe) error {
	query := `
		UPDATE universes SET name=$1, description=$2, genre=$3, format=$4, updated_at=NOW()
		WHERE id=$5
	`
	_, err := tx.Exec(ctx, query, u.Name, u.Description, u.Genre, u.Format, u.ID)
	if err != nil {
		return fmt.Errorf("update universe: %w", err)
	}
	return nil
}

func (r *UniverseRepo) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	query := `DELETE FROM universes WHERE id = $1`
	_, err := tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete universe: %w", err)
	}
	return nil
}

func (r *UniverseRepo) FindBySessionID(ctx context.Context, sessionID string) (*models.Universe, error) {
	query := `
		SELECT id, user_id, name, description, genre, format, session_id, is_demo_template, created_at, updated_at
		FROM universes WHERE session_id = $1
	`
	u := &models.Universe{}
	err := r.pool.QueryRow(ctx, query, sessionID).Scan(
		&u.ID, &u.UserID, &u.Name, &u.Description, &u.Genre, &u.Format,
		&u.SessionID, &u.IsDemoTemplate, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("universe not found for session")
	}
	if err != nil {
		return nil, fmt.Errorf("find universe by session: %w", err)
	}
	return u, nil
}
