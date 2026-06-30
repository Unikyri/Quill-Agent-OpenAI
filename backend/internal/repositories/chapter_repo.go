package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

type ChapterRepo struct {
	pool *pgxpool.Pool
}

func NewChapterRepo(pool *pgxpool.Pool) *ChapterRepo {
	return &ChapterRepo{pool: pool}
}

func (r *ChapterRepo) Create(ctx context.Context, tx pgx.Tx, c *models.Chapter) error {
	query := `
		INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	_, err := tx.Exec(ctx, query, c.ID, c.WorkID, c.Title, c.OrderIndex, c.Content, c.RawText, c.WordCount, c.Status)
	if err != nil {
		return fmt.Errorf("create chapter: %w", err)
	}
	return nil
}

func (r *ChapterRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Chapter, error) {
	query := `
		SELECT id, work_id, title, order_index, content, raw_text, word_count, status, analyzed_at, created_at, updated_at
		FROM chapters WHERE id = $1
	`
	c := &models.Chapter{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.WorkID, &c.Title, &c.OrderIndex, &c.Content, &c.RawText,
		&c.WordCount, &c.Status, &c.AnalyzedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("chapter not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find chapter: %w", err)
	}
	return c, nil
}

func (r *ChapterRepo) ListByWork(ctx context.Context, workID uuid.UUID) ([]models.Chapter, error) {
	query := `
		SELECT id, work_id, title, order_index, content, raw_text, word_count, status, analyzed_at, created_at, updated_at
		FROM chapters WHERE work_id = $1
		ORDER BY order_index ASC
	`
	rows, err := r.pool.Query(ctx, query, workID)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	defer rows.Close()

	var chapters []models.Chapter
	for rows.Next() {
		var c models.Chapter
		if err := rows.Scan(
			&c.ID, &c.WorkID, &c.Title, &c.OrderIndex, &c.Content, &c.RawText,
			&c.WordCount, &c.Status, &c.AnalyzedAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chapter: %w", err)
		}
		chapters = append(chapters, c)
	}
	return chapters, nil
}

func (r *ChapterRepo) GetMaxOrderIndex(ctx context.Context, workID uuid.UUID) (int, error) {
	query := `SELECT COALESCE(MAX(order_index), 0) FROM chapters WHERE work_id = $1`
	var maxOrder int
	err := r.pool.QueryRow(ctx, query, workID).Scan(&maxOrder)
	if err != nil {
		return 0, fmt.Errorf("get max order index: %w", err)
	}
	return maxOrder, nil
}

func (r *ChapterRepo) Update(ctx context.Context, tx pgx.Tx, c *models.Chapter) error {
	query := `
		UPDATE chapters SET title=$1, order_index=$2, content=$3, raw_text=$4, word_count=$5, status=$6, updated_at=NOW()
		WHERE id=$7
	`
	_, err := tx.Exec(ctx, query, c.Title, c.OrderIndex, c.Content, c.RawText, c.WordCount, c.Status, c.ID)
	if err != nil {
		return fmt.Errorf("update chapter: %w", err)
	}
	return nil
}

func (r *ChapterRepo) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	query := `DELETE FROM chapters WHERE id = $1`
	_, err := tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete chapter: %w", err)
	}
	return nil
}

func (r *ChapterRepo) CountWords(text string) int {
	words := 0
	inWord := false
	for _, ch := range text {
		if ch == ' ' || ch == '\n' || ch == '\t' || ch == '\r' {
			if inWord {
				words++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		words++
	}
	return words
}
