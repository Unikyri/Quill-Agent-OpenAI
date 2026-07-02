package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

type WorkRepo struct {
	pool *pgxpool.Pool
}

func NewWorkRepo(pool *pgxpool.Pool) *WorkRepo {
	return &WorkRepo{pool: pool}
}

func (r *WorkRepo) Create(ctx context.Context, tx pgx.Tx, w *models.Work) error {
	query := `
		INSERT INTO works (id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`
	_, err := tx.Exec(ctx, query, w.ID, w.UniverseID, w.Title, w.Type, w.OrderIndex, w.Synopsis, w.Status)
	if err != nil {
		return fmt.Errorf("create work: %w", err)
	}
	return nil
}

func (r *WorkRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Work, error) {
	query := `
		SELECT id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at
		FROM works WHERE id = $1
	`
	w := &models.Work{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.UniverseID, &w.Title, &w.Type, &w.OrderIndex,
		&w.Synopsis, &w.Status, &w.CreatedAt, &w.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("work not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find work: %w", err)
	}
	return w, nil
}

func (r *WorkRepo) ListByUniverse(ctx context.Context, universeID uuid.UUID) ([]models.Work, error) {
	query := `
		SELECT id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at
		FROM works WHERE universe_id = $1
		ORDER BY order_index ASC
	`
	rows, err := r.pool.Query(ctx, query, universeID)
	if err != nil {
		return nil, fmt.Errorf("list works: %w", err)
	}
	defer rows.Close()

	works := []models.Work{}
	for rows.Next() {
		var w models.Work
		if err := rows.Scan(
			&w.ID, &w.UniverseID, &w.Title, &w.Type, &w.OrderIndex,
			&w.Synopsis, &w.Status, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan work: %w", err)
		}
		works = append(works, w)
	}
	return works, nil
}

func (r *WorkRepo) GetMaxOrderIndex(ctx context.Context, universeID uuid.UUID) (int, error) {
	query := `SELECT COALESCE(MAX(order_index), 0) FROM works WHERE universe_id = $1`
	var maxOrder int
	err := r.pool.QueryRow(ctx, query, universeID).Scan(&maxOrder)
	if err != nil {
		return 0, fmt.Errorf("get max order index: %w", err)
	}
	return maxOrder, nil
}

func (r *WorkRepo) Update(ctx context.Context, tx pgx.Tx, w *models.Work) error {
	query := `
		UPDATE works SET title=$1, type=$2, order_index=$3, synopsis=$4, status=$5, updated_at=NOW()
		WHERE id=$6
	`
	_, err := tx.Exec(ctx, query, w.Title, w.Type, w.OrderIndex, w.Synopsis, w.Status, w.ID)
	if err != nil {
		return fmt.Errorf("update work: %w", err)
	}
	return nil
}

func (r *WorkRepo) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	query := `DELETE FROM works WHERE id = $1`
	_, err := tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete work: %w", err)
	}
	return nil
}
