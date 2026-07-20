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

func scanEntity(row pgx.Row) (*models.Entity, error) {
	e := &models.Entity{}
	err := row.Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
		&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	return e, err
}

func (r *EntityRepo) Create(ctx context.Context, tx pgx.Tx, e *models.Entity) error {
	query := `
		INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, confidence, relevance_score, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
	`
	_, err := tx.Exec(ctx, query, e.ID, e.UniverseID, e.Type, e.Name, e.Aliases, e.Description, e.Properties, e.Status, e.Confidence, e.RelevanceScore)
	if err != nil {
		return fmt.Errorf("create entity: %w", err)
	}
	return nil
}

func (r *EntityRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, COALESCE(description, ''), properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE id = $1
	`
	e, err := scanEntity(r.pool.QueryRow(ctx, query, id))
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity: %w", err)
	}
	return e, nil
}

// FindByIDTx returns an entity while holding a row lock until tx commits or
// rolls back. Resolution first performs the cheap pool lookup, then uses this
// method immediately before mutating the row so candidate decisions and
// mention merges cannot overwrite each other with stale data.
func (r *EntityRepo) FindByIDTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, COALESCE(description, ''), properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE id = $1
		FOR UPDATE
	`
	e, err := scanEntity(tx.QueryRow(ctx, query, id))
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity by id in transaction: %w", err)
	}
	return e, nil
}

func (r *EntityRepo) FindByName(ctx context.Context, universeID uuid.UUID, name string) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, COALESCE(description, ''), properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE universe_id = $1 AND LOWER(name) = LOWER($2)
		ORDER BY type, id
		LIMIT 1
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, universeID, name).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
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

// FindByNaturalKey resolves the type-aware entity identity enforced by the
// entities_universe_name_type_key unique index. Callers that resolve mentions
// must use this method so same-named entities of different types stay distinct.
func (r *EntityRepo) FindByNaturalKey(ctx context.Context, universeID uuid.UUID, name, entityType string) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, COALESCE(description, ''), properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities
		WHERE universe_id = $1 AND LOWER(name) = LOWER($2) AND type = $3
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, universeID, name, entityType).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
		&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity by natural key: %w", err)
	}
	return e, nil
}

func (r *EntityRepo) FindByAlias(ctx context.Context, universeID uuid.UUID, alias string) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, description, properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities WHERE universe_id = $1 AND LOWER($2) = ANY(SELECT LOWER(unnest(aliases)))
		ORDER BY type, id
		LIMIT 1
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, universeID, alias).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
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

// FindByAliasAndType keeps alias resolution consistent with the entity natural
// key when the same alias is used by different entity types.
func (r *EntityRepo) FindByAliasAndType(ctx context.Context, universeID uuid.UUID, alias, entityType string) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, COALESCE(description, ''), properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities
		WHERE universe_id = $1
		  AND type = $3
		  AND LOWER($2) = ANY(SELECT LOWER(unnest(aliases)))
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, universeID, alias, entityType).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
		&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found by alias")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity by typed alias: %w", err)
	}
	return e, nil
}

func (r *EntityRepo) FindByFuzzyName(ctx context.Context, universeID uuid.UUID, name string, entityType string) (*models.Entity, error) {
	query := `
		SELECT id, universe_id, type, name, aliases, description, properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities
		WHERE universe_id = $1
		  AND type = $3
		  AND (LOWER(name) LIKE '%' || LOWER($2) || '%' OR LOWER($2) LIKE '%' || LOWER(name) || '%')
		ORDER BY LENGTH(name) DESC
		LIMIT 1
	`
	e := &models.Entity{}
	err := r.pool.QueryRow(ctx, query, universeID, name, entityType).Scan(
		&e.ID, &e.UniverseID, &e.Type, &e.Name, &e.Aliases, &e.Description,
		&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
		&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("entity not found by fuzzy name")
	}
	if err != nil {
		return nil, fmt.Errorf("find entity by fuzzy name: %w", err)
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
	Confidence  float64                `json:"confidence,omitempty"`
	// ConfidenceSet is propagated from the Qwen extraction decoder so an
	// explicit confidence of zero is not mistaken for a legacy omitted value.
	ConfidenceSet bool `json:"-"`
}

// ListCandidates returns the review-tray projection. Evidence comes from the
// latest mention, keeping the candidate entity row as the single source of
// truth while still giving the writer a useful quote and chapter context.
func (r *EntityRepo) ListCandidates(ctx context.Context, universeID uuid.UUID) ([]models.EntityCandidate, error) {
	const query = `
		SELECT e.id, e.universe_id, e.name, e.type, e.aliases,
		       COALESCE(e.description, ''), e.confidence, e.status,
		       COALESCE(m.context_snippet, ''), COALESCE(m.chapter_id, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM entities e
		LEFT JOIN LATERAL (
			SELECT context_snippet, chapter_id
			FROM entity_mentions
			WHERE entity_id = e.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) m ON TRUE
		WHERE e.universe_id = $1 AND e.status = 'candidate'
		ORDER BY e.confidence ASC, e.updated_at ASC, e.id ASC
	`
	rows, err := r.pool.Query(ctx, query, universeID)
	if err != nil {
		return nil, fmt.Errorf("list entity candidates: %w", err)
	}
	defer rows.Close()
	result := make([]models.EntityCandidate, 0)
	for rows.Next() {
		var item models.EntityCandidate
		if err := rows.Scan(&item.EntityID, &item.UniverseID, &item.Name, &item.Type, &item.Aliases,
			&item.Description, &item.Confidence, &item.Status, &item.EvidenceQuote, &item.ChapterID); err != nil {
			return nil, fmt.Errorf("scan entity candidate: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity candidates: %w", err)
	}
	return result, nil
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
		where = append(where, fmt.Sprintf("(LOWER(name) LIKE $%d OR EXISTS (SELECT 1 FROM unnest(COALESCE(aliases, ARRAY[]::text[])) AS alias WHERE LOWER(alias) LIKE $%d))", argIdx, argIdx))
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
		SELECT id, universe_id, type, name, aliases, description, properties, status, confidence, relevance_score,
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
			&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
			&e.LastMentionedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}

	return entities, total, nil
}

// ListGraphInventory returns every entity known to the relational source of
// truth. AGE node writes are deliberately best-effort during ingestion, so
// graph views use this inventory to retain isolated entities without
// inventing a relationship for them.
func (r *EntityRepo) ListGraphInventory(ctx context.Context, universeID uuid.UUID) ([]models.Entity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, universe_id, type, name, aliases, description, properties, status, confidence, relevance_score,
		       last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at
		FROM entities
		WHERE universe_id = $1
		ORDER BY relevance_score DESC, name ASC, id ASC
	`, universeID)
	if err != nil {
		return nil, fmt.Errorf("list graph inventory: %w", err)
	}
	defer rows.Close()

	entities := make([]models.Entity, 0)
	for rows.Next() {
		var entity models.Entity
		if err := rows.Scan(
			&entity.ID, &entity.UniverseID, &entity.Type, &entity.Name, &entity.Aliases, &entity.Description,
			&entity.Properties, &entity.Status, &entity.Confidence, &entity.RelevanceScore, &entity.LastMentionedChapterID,
			&entity.LastMentionedAt, &entity.CreatedAt, &entity.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan graph inventory entity: %w", err)
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate graph inventory: %w", err)
	}
	return entities, nil
}

// CountByType returns the number of entities stored for each type in a universe.
// It intentionally includes archived entities: the browser's type chips describe
// the universe's complete entity inventory, while status remains a list filter.
func (r *EntityRepo) CountByType(ctx context.Context, universeID uuid.UUID) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT type, COUNT(*)
		FROM entities
		WHERE universe_id = $1
		GROUP BY type
	`, universeID)
	if err != nil {
		return nil, fmt.Errorf("count entities by type: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var entityType string
		var count int
		if err := rows.Scan(&entityType, &count); err != nil {
			return nil, fmt.Errorf("scan entity type count: %w", err)
		}
		counts[entityType] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity type counts: %w", err)
	}

	return counts, nil
}

func (r *EntityRepo) Update(ctx context.Context, tx pgx.Tx, e *models.Entity) error {
	query := `
		UPDATE entities SET type=$1, name=$2, aliases=$3, description=$4, properties=$5,
		       status=$6, confidence=$7, relevance_score=$8, last_mentioned_chapter_id=$9, last_mentioned_at=$10, updated_at=NOW()
		WHERE id=$11
	`
	_, err := tx.Exec(ctx, query, e.Type, e.Name, e.Aliases, e.Description, e.Properties,
		e.Status, e.Confidence, e.RelevanceScore, e.LastMentionedChapterID, e.LastMentionedAt, e.ID)
	if err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	return nil
}

// UpdateCandidateStatus changes a candidate only if it is still a candidate.
// The affected-row check closes the race between two simultaneous decisions.
func (r *EntityRepo) UpdateCandidateStatus(ctx context.Context, tx pgx.Tx, e *models.Entity, fromStatus string) (bool, error) {
	query := `
		UPDATE entities SET type=$1, name=$2, aliases=$3, description=$4, properties=$5,
		       status=$6, confidence=$7, relevance_score=$8, last_mentioned_chapter_id=$9, last_mentioned_at=$10, updated_at=NOW()
		WHERE id=$11 AND status=$12
	`
	tag, err := tx.Exec(ctx, query, e.Type, e.Name, e.Aliases, e.Description, e.Properties,
		e.Status, e.Confidence, e.RelevanceScore, e.LastMentionedChapterID, e.LastMentionedAt, e.ID, fromStatus)
	if err != nil {
		return false, fmt.Errorf("update candidate status: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *EntityRepo) CreateMention(ctx context.Context, tx pgx.Tx, m *models.EntityMention) error {
	query := `
		INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, character_offset, paragraph_node_id, context_snippet, mention_type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	`
	_, err := tx.Exec(ctx, query, m.ID, m.EntityID, m.ChapterID, m.ParagraphIndex, m.CharacterOffset, m.ParagraphNodeID, m.ContextSnippet, m.MentionType)
	if err != nil {
		return fmt.Errorf("create mention: %w", err)
	}
	return nil
}

func (r *EntityRepo) GetMentionsByEntity(ctx context.Context, entityID uuid.UUID, limit int) ([]models.EntityMention, error) {
	query := `
		SELECT id, entity_id, chapter_id, paragraph_index, character_offset, COALESCE(paragraph_node_id, ''), context_snippet, mention_type, created_at
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
			&m.ID, &m.EntityID, &m.ChapterID, &m.ParagraphIndex, &m.CharacterOffset, &m.ParagraphNodeID,
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

// ListMentionedEntityIDs returns the distinct active-memory entities that
// were mentioned in one completed chapter. It is deliberately based on the
// persisted mention rows rather than transient LLM output so retries and
// imported chapters follow the same relevance rule.
func (r *EntityRepo) ListMentionedEntityIDs(ctx context.Context, chapterID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT entity_id
		FROM entity_mentions
		WHERE chapter_id = $1
	`, chapterID)
	if err != nil {
		return nil, fmt.Errorf("list chapter mentioned entities: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan chapter mentioned entity: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chapter mentioned entities: %w", err)
	}
	return ids, nil
}

// ListRelevanceStates returns the canonical SQL state used to synchronize AGE
// after a relevance mutation. SQL remains authoritative if a graph is absent
// or an AGE operation fails.
func (r *EntityRepo) ListRelevanceStates(ctx context.Context, universeID uuid.UUID) ([]models.Entity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, universe_id, type, name, aliases, COALESCE(description, ''), properties,
		       status, confidence, relevance_score, last_mentioned_chapter_id,
		       last_mentioned_at, created_at, updated_at
		FROM entities
		WHERE universe_id = $1
	`, universeID)
	if err != nil {
		return nil, fmt.Errorf("list relevance states: %w", err)
	}
	defer rows.Close()

	entities := make([]models.Entity, 0)
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan relevance state: %w", err)
		}
		entities = append(entities, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relevance states: %w", err)
	}
	return entities, nil
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
// It is retained for explicit maintenance sweeps. Chapter-aware callers must
// prefer DecayExcept so entities mentioned in the completed chapter retain
// the relevance earned from that chapter.
func (r *EntityRepo) DecayAll(ctx context.Context, universeID uuid.UUID, lambda float64) error {
	return r.DecayExcept(ctx, universeID, nil, lambda)
}

// DecayExcept applies one event-driven decay tick to active entities other
// than the supplied entity IDs. A completed chapter passes its mentioned
// entities here so the same chapter cannot both reinforce and decay them.
func (r *EntityRepo) DecayExcept(ctx context.Context, universeID uuid.UUID, excludedIDs []uuid.UUID, lambda float64) error {
	// ponytail: per-chapter decay, multiply by e^(-lambda) each chapter advance.
	// DecayAll calls this with excludedIDs == nil (manual sweep, nothing to
	// exclude); pgx sends a nil slice as SQL NULL, and `cardinality(NULL) = 0`
	// and `id <> ALL(NULL)` both evaluate to NULL (not TRUE), making the whole
	// WHERE clause NULL for every row — the UPDATE silently affected zero
	// rows. COALESCE to an empty array before the cardinality/ALL checks so a
	// nil exclusion list means "exclude nothing" instead of "match nothing".
	query := `
		UPDATE entities SET relevance_score = relevance_score * EXP($2), updated_at = NOW()
		WHERE universe_id = $1
		  AND status = 'active'
		  AND NOT (id = ANY(COALESCE($3::uuid[], ARRAY[]::uuid[])))
	`
	_, err := r.pool.Exec(ctx, query, universeID, -lambda, excludedIDs)
	if err != nil {
		return fmt.Errorf("decay entities: %w", err)
	}
	return nil
}

// ReinforceMention records a real mention as a relevance mutation. New
// entities start conservatively; every active mention earns a small bounded
// bump, while an archived entity is restored to its reactivation baseline.
// Candidate/dismissed/merged rows intentionally remain outside the active
// memory graph and are not mutated by this method.
func (r *EntityRepo) ReinforceMention(ctx context.Context, entityID, chapterID uuid.UUID, bump float64) (*models.Entity, error) {
	query := `
		UPDATE entities
		SET relevance_score = CASE
				WHEN status = 'archived' THEN 0.8
				ELSE LEAST(1.0, relevance_score + $3)
			END,
			status = CASE WHEN status = 'archived' THEN 'active' ELSE status END,
			last_mentioned_chapter_id = $2,
			last_mentioned_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND status IN ('active', 'archived')
		RETURNING id, universe_id, type, name, aliases, COALESCE(description, ''), properties,
		          status, confidence, relevance_score, last_mentioned_chapter_id,
		          last_mentioned_at, created_at, updated_at
	`
	e, err := scanEntity(r.pool.QueryRow(ctx, query, entityID, chapterID, bump))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reinforce entity mention: %w", err)
	}
	return e, nil
}

// TouchBatch remains for callers that only need to update mention metadata.
// RelevanceService.Touch is the application boundary for a user-visible
// mention and uses ReinforceMention instead.
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
		SELECT id, universe_id, type, name, aliases, description, properties, status, confidence, relevance_score,
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
			&e.Properties, &e.Status, &e.Confidence, &e.RelevanceScore, &e.LastMentionedChapterID,
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
