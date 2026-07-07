package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

type EntityRepo struct {
	pool *pgxpool.Pool
}

func NewEntityRepo(pool *pgxpool.Pool) *EntityRepo {
	return &EntityRepo{pool: pool}
}

func (r *EntityRepo) Create(ctx context.Context, tx pgx.Tx, e *models.Entity) error {
	query := `
		INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`
	_, err := tx.Exec(ctx, query, e.ID, e.UniverseID, e.Type, e.Name, e.Aliases, e.Description, e.Properties, e.Status, e.RelevanceScore)
	if err != nil {
		return fmt.Errorf("create entity: %w", err)
	}
	return nil
}

func (r *EntityRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, description, properties, status, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE id = $1
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.RelevanceScore, &e.LastMentionedChapterID,
		&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity: %w", err)
	}
	return e, nil
}

func (r *EntityRepo) FindByName(ctx context.Context, universeID uuid.UUID, name string) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, description, properties, status, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE universe_id = $1 AND LOWER(name) = LOWER($2)
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, universeID, name).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.RelevanceScore, &e.LastMentionedChapterID,
		&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity by name: %w", err)
	}
	return e, nil
}

func (r *EntityRepo) FindByAlias(ctx context.Context, universeID uuid.UUID, alias string) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, description, properties, status, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE universe_id = $1 AND LOWER($2) = ANY(SELECT LOWER(unnest(aliases)))
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, universeID, alias).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.RelevanceScore, &e.LastMentionedChapterID,
		&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found by alias")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity by alias: %w", err)
	}
	return e, nil
}

type EntityFilters struct {
	Type         string
	Status       string
	MinRelevance float64
	Search       string
	Page         int
	Limit        int
}

type ExtractedEntity struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Aliases     []string               `json:"aliases,omitempty"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Status      string                 `json:"status,omitempty"`
}

func (r *EntityRepo) ListByUniverse(ctx context.Context, universeID uuid.UUID, filters EntityFilters) ([]models.Entity, int, error) {
	where := []string{"universe_id = $1"}
	args := []interface{}{universeID}
	argIdx := 2

	if filters.Type != "" {
		where = append(where, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, filters.Type)
		argIdx++
	}
	if filters.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filters.Status)
		argIdx++
	}
	if filters.MinRelevance > 0 {
		where = append(where, fmt.Sprintf("relevance_score >= $%d", argIdx))
		args = append(args, filters.MinRelevance)
		argIdx++
	}
	if filters.Search != "" {
		where = append(where, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER($%d) = ANY(SELECT LOWER(unnest(aliases)))", argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(filters.Search)+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM entities WHERE %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count entities: %w", err)
	}

	offset := (filters.Page - 1) * filters.Limit
	query := fmt.Sprintf(`
		SELECT id, universe_id, type, name, aliases, description, properties, status, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE %s
		ORDER BY relevance_score DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, filters.Limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	entities := []models.Entity{}
	for rows.Next() {
		var e models.Entity
		if err := rows.Scan(
			&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
			&e.Properties, &e.Status, &e.RelevanceScore, &e.LastMentionedChapterID,
			&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}

	return entities, total, nil
}

func (r *EntityRepo) Update(ctx context.Context, tx pgx.Tx, e *models.Entity) error {
	query := `
		UPDATE entities SET type=$1, name=$2, aliases=$3, description=$4, properties=$5,
		       status=$6, relevance_score=$7, last_mentioned_chapter_id=$8, last_mentioned_at=$9, updated_at=NOW()
		WHERE id=$10
	`
	_, err := tx.Exec(ctx, query, e.Type, e.Name, e.Aliases, e.Description, e.Properties,
		e.Status, e.RelevanceScore, e.LastMentionedChapterID, e.LastMentionedAt, e.ID)
	if err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	return nil
}

func (r *EntityRepo) CreateMention(ctx context.Context, tx pgx.Tx, m *models.EntityMention) error {
	query := `
		INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, paragraph_node_id, context_snippet, mention_type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`
	_, err := tx.Exec(ctx, query, m.ID, m.EntityID, m.ChapterID, m.ParagraphIndex, m.ParagraphNodeID, m.ContextSnippet, m.MentionType)
	if err != nil {
		return fmt.Errorf("create mention: %w", err)
	}
	return nil
}

func (r *EntityRepo) GetMentionsByEntity(ctx context.Context, entityID uuid.UUID, limit int) ([]models.EntityMention, error) {
	query := `
		SELECT id, entity_id, chapter_id, paragraph_index, paragraph_node_id, context_snippet, mention_type, created_at
		FROM entity_mentions WHERE entity_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("get mentions: %w", err)
	}
	defer rows.Close()

	mentions := []models.EntityMention{}
	for rows.Next() {
		var m models.EntityMention
		if err := rows.Scan(
			&m.ID, &m.EntityID, &m.ChapterID, &m.ParagraphIndex, &m.ParagraphNodeID,
			&m.ContextSnippet, &m.MentionType, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan mention: %w", err)
		}
		mentions = append(mentions, m)
	}
	return mentions, nil
}

func (r *EntityRepo) CountMentions(ctx context.Context, entityID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM entity_mentions WHERE entity_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, query, entityID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count mentions: %w", err)
	}
	return count, nil
}

func (r *EntityRepo) GetMaxMentionsInUniverse(ctx context.Context, universeID uuid.UUID) (int, error) {
	query := `
		SELECT COALESCE(MAX(mention_count), 0) FROM (
			SELECT COUNT(*) as mention_count
			FROM entity_mentions em
			JOIN entities e ON em.entity_id = e.id
			WHERE e.universe_id = $1
			GROUP BY em.entity_id
		) sub
	`
	var max int
	err := r.pool.QueryRow(ctx, query, universeID).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("get max mentions: %w", err)
	}
	return max, nil
}

// FindNewlyArchivable returns IDs of active entities whose relevance score is
// at or below the given threshold. Called before the archive UPDATE so the
// caller can launch consolidation goroutines for entities that will be archived.
func (r *EntityRepo) FindNewlyArchivable(ctx context.Context, universeID uuid.UUID, threshold float64) ([]uuid.UUID, error) {
	query := `SELECT id FROM entities WHERE universe_id = $1 AND status = 'active' AND relevance_score <= $2`
	rows, err := r.pool.Query(ctx, query, universeID, threshold)
	if err != nil {
		return nil, fmt.Errorf("find newly archivable: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan archivable id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// DecayAll applies exponential decay to all active entities in the universe.
// score = score * e^(-lambda). Archived entities are skipped.
func (r *EntityRepo) DecayAll(ctx context.Context, universeID uuid.UUID, lambda float64) error {
	// ponytail: per-chapter decay, multiply by e^(-lambda) each chapter advance
	query := `
		UPDATE entities SET relevance_score = relevance_score * EXP($2), updated_at = NOW()
		WHERE universe_id = $1 AND status = 'active'
	`
	_, err := r.pool.Exec(ctx, query, universeID, -lambda)
	if err != nil {
		return fmt.Errorf("decay all: %w", err)
	}
	return nil
}

// TouchBatch resets the idle counter for multiple entities by updating
// last_mentioned_chapter_id and last_mentioned_at for each entity ID.
func (r *EntityRepo) TouchBatch(ctx context.Context, entityIDs []uuid.UUID, chapterID uuid.UUID) error {
	if len(entityIDs) == 0 {
		return nil
	}

	query := `
		UPDATE entities SET last_mentioned_chapter_id = $2, last_mentioned_at = NOW(), updated_at = NOW()
		WHERE id = ANY($1)
	`
	_, err := r.pool.Exec(ctx, query, entityIDs, chapterID)
	if err != nil {
		return fmt.Errorf("touch batch: %w", err)
	}
	return nil
}

// ListByUniverseActive returns all active entities for a universe ordered by relevance.
func (r *EntityRepo) ListByUniverseActive(ctx context.Context, universeID uuid.UUID) ([]models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, description, properties, status, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE universe_id = $1 AND status = 'active'
		ORDER BY relevance_score DESC
	`
	rows, err := r.pool.Query(ctx, query, universeID)
	if err != nil {
		return nil, fmt.Errorf("list active entities: %w", err)
	}
	defer rows.Close()

	entities := []models.Entity{}
	for rows.Next() {
		var e models.Entity
		if err := rows.Scan(
			&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
			&e.Properties, &e.Status, &e.RelevanceScore, &e.LastMentionedChapterID,
			&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, nil
}

// ParagraphKey identifies a paragraph by its owning chapter + index within
// that chapter, matching the key paragraph_embeddings and entity_mentions
// share.
type ParagraphKey struct {
	ChapterID      uuid.UUID
	ParagraphIndex int
}

// EntityIDsForParagraphs resolves entity_mentions for a batch of paragraph
// keys in a single query, so vector hits can be joined to their mentioned
// entities without one query per paragraph. Empty input short-circuits
// without hitting the database.
func (r *EntityRepo) EntityIDsForParagraphs(ctx context.Context, keys []ParagraphKey) (map[ParagraphKey][]uuid.UUID, error) {
	result := make(map[ParagraphKey][]uuid.UUID)
	if len(keys) == 0 {
		return result, nil
	}

	chapterIDs := make([]uuid.UUID, len(keys))
	paragraphIndexes := make([]int, len(keys))
	for i, k := range keys {
		chapterIDs[i] = k.ChapterID
		paragraphIndexes[i] = k.ParagraphIndex
	}

	query := `
		SELECT chapter_id, paragraph_index, entity_id
		FROM entity_mentions
		WHERE (chapter_id, paragraph_index) IN (
			SELECT * FROM UNNEST($1::uuid[], $2::int[])
		)
	`
	rows, err := r.pool.Query(ctx, query, chapterIDs, paragraphIndexes)
	if err != nil {
		return nil, fmt.Errorf("entity ids for paragraphs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key ParagraphKey
		var entityID uuid.UUID
		if err := rows.Scan(&key.ChapterID, &key.ParagraphIndex, &entityID); err != nil {
			return nil, fmt.Errorf("scan entity id for paragraph: %w", err)
		}
		result[key] = append(result[key], entityID)
	}
	return result, nil
}

func (r *EntityRepo) MergeProperties(existing json.RawMessage, newData json.RawMessage) json.RawMessage {
	if existing == nil {
		return newData
	}
	if newData == nil {
		return existing
	}

	var existingMap, newMap map[string]interface{}
	json.Unmarshal(existing, &existingMap)
	json.Unmarshal(newData, &newMap)

	for k, v := range newMap {
		if v != nil && (existingMap[k] == nil || existingMap[k] == "") {
			existingMap[k] = v
		}
	}

	merged, _ := json.Marshal(existingMap)
	return merged
}
