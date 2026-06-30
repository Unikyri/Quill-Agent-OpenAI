package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VectorRepo struct {
	pool *pgxpool.Pool
}

func NewVectorRepo(pool *pgxpool.Pool) *VectorRepo {
	return &VectorRepo{pool: pool}
}

func (r *VectorRepo) SaveEntityEmbedding(ctx context.Context, entityID uuid.UUID, embedding []float32) error {
	query := `
		INSERT INTO entity_embeddings (id, entity_id, description_embedding, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (entity_id) DO UPDATE SET description_embedding = $3, updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, uuid.New(), entityID, embedding)
	if err != nil {
		return fmt.Errorf("save entity embedding: %w", err)
	}
	return nil
}

func (r *VectorRepo) FindSimilarEntity(ctx context.Context, universeID uuid.UUID, embedding []float32, threshold float64) (*uuid.UUID, float64, error) {
	query := `
		SELECT e.id, ee.description_embedding <=> $1 AS distance
		FROM entities e
		JOIN entity_embeddings ee ON e.id = ee.entity_id
		WHERE e.universe_id = $2
		ORDER BY distance ASC
		LIMIT 1
	`
	var entityID uuid.UUID
	var distance float64
	err := r.pool.QueryRow(ctx, query, embedding, universeID).Scan(&entityID, &distance)
	if err != nil {
		return nil, 0, fmt.Errorf("find similar entity: %w", err)
	}

	if distance > threshold {
		return nil, distance, nil
	}

	return &entityID, distance, nil
}

func (r *VectorRepo) SaveParagraphEmbedding(ctx context.Context, chapterID uuid.UUID, paragraphIndex int, nodeID, content string, embedding []float32) error {
	query := `
		INSERT INTO paragraph_embeddings (id, chapter_id, paragraph_index, paragraph_node_id, content, embedding, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`
	_, err := r.pool.Exec(ctx, query, uuid.New(), chapterID, paragraphIndex, nodeID, content, embedding)
	if err != nil {
		return fmt.Errorf("save paragraph embedding: %w", err)
	}
	return nil
}

func (r *VectorRepo) FindSimilarParagraphs(ctx context.Context, universeID uuid.UUID, embedding []float32, excludeChapterID uuid.UUID, limit int) ([]SimilarParagraph, error) {
	query := `
		SELECT pe.content, pe.chapter_id, c.title, pe.embedding <=> $1 AS distance
		FROM paragraph_embeddings pe
		JOIN chapters c ON pe.chapter_id = c.id
		JOIN works w ON c.work_id = w.id
		WHERE w.universe_id = $2 AND pe.chapter_id != $3
		ORDER BY distance ASC
		LIMIT $4
	`
	rows, err := r.pool.Query(ctx, query, embedding, universeID, excludeChapterID, limit)
	if err != nil {
		return nil, fmt.Errorf("find similar paragraphs: %w", err)
	}
	defer rows.Close()

	var results []SimilarParagraph
	for rows.Next() {
		var sp SimilarParagraph
		if err := rows.Scan(&sp.Content, &sp.ChapterID, &sp.ChapterTitle, &sp.Distance); err != nil {
			return nil, fmt.Errorf("scan similar paragraph: %w", err)
		}
		results = append(results, sp)
	}
	return results, nil
}

type SimilarParagraph struct {
	Content      string
	ChapterID    uuid.UUID
	ChapterTitle string
	Distance     float64
}
