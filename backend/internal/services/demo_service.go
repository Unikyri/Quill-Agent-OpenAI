package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/repositories"
)

type DemoService struct {
	pool         *pgxpool.Pool
	universeRepo *repositories.UniverseRepo
	graphRepo    *repositories.GraphRepo
}

func NewDemoService(pool *pgxpool.Pool, universeRepo *repositories.UniverseRepo, graphRepo *repositories.GraphRepo) *DemoService {
	return &DemoService{
		pool:         pool,
		universeRepo: universeRepo,
		graphRepo:    graphRepo,
	}
}

func (s *DemoService) CloneUniverse(ctx context.Context, sessionID string) (string, error) {
	// Check if user already has a demo universe for this session
	existing, err := s.universeRepo.FindBySessionID(ctx, sessionID)
	if err == nil && existing != nil {
		return existing.ID.String(), nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get template universe
	templateID := ""
	err = tx.QueryRow(ctx, `SELECT id FROM universes WHERE is_demo_template = TRUE LIMIT 1`).Scan(&templateID)
	if err != nil {
		return "", fmt.Errorf("no demo template found: %w", err)
	}

	// Clone the universe
	newID := uuid.New().String()
	_, err = tx.Exec(ctx, `
		INSERT INTO universes (id, user_id, name, description, genre, format, session_id, is_demo_template, created_at, updated_at)
		SELECT $1, user_id, name, description, genre, format, $2, FALSE, NOW(), NOW()
		FROM universes WHERE id = $3
	`, newID, sessionID, templateID)
	if err != nil {
		return "", fmt.Errorf("clone universe: %w", err)
	}

	// ── Deep-copy all dependent tables ──

	workMap := make(map[string]string)     // oldWorkID → newWorkID
	chapterMap := make(map[string]string)  // oldChapterID → newChapterID
	entityMap := make(map[string]string)   // oldEntityID → newEntityID

	// 1. Works
	workRows, err := tx.Query(ctx, `SELECT id, title, type, order_index, synopsis, status FROM works WHERE universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template works: %w", err)
	}
	defer workRows.Close()
	for workRows.Next() {
		var oldID, title, wtype, synopsis, status string
		var orderIdx int
		if err := workRows.Scan(&oldID, &title, &wtype, &orderIdx, &synopsis, &status); err != nil {
			return "", fmt.Errorf("scan work: %w", err)
		}
		nid := uuid.New().String()
		workMap[oldID] = nid
		_, err = tx.Exec(ctx, `
			INSERT INTO works (id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`,
			nid, newID, title, wtype, orderIdx, synopsis, status)
		if err != nil {
			return "", fmt.Errorf("insert work: %w", err)
		}
	}

	// 2. Chapters
	chapterRows, err := tx.Query(ctx, `
		SELECT c.id, c.work_id, c.title, c.order_index, c.content, c.raw_text, c.word_count, c.status
		FROM chapters c JOIN works w ON c.work_id = w.id
		WHERE w.universe_id = $1 ORDER BY c.order_index`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template chapters: %w", err)
	}
	defer chapterRows.Close()
	for chapterRows.Next() {
		var oldID, oldWorkID, title, content, rawText, status string
		var orderIdx, wordCount int
		if err := chapterRows.Scan(&oldID, &oldWorkID, &title, &orderIdx, &content, &rawText, &wordCount, &status); err != nil {
			return "", fmt.Errorf("scan chapter: %w", err)
		}
		nid := uuid.New().String()
		chapterMap[oldID] = nid
		newWorkID := workMap[oldWorkID]
		_, err = tx.Exec(ctx, `
			INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
			nid, newWorkID, title, orderIdx, content, rawText, wordCount, status)
		if err != nil {
			return "", fmt.Errorf("insert chapter: %w", err)
		}
	}

	// 3. Entities
	entityRows, err := tx.Query(ctx, `
		SELECT id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id
		FROM entities WHERE universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template entities: %w", err)
	}
	defer entityRows.Close()
	for entityRows.Next() {
		var oldID, etype, name, desc, status string
		var aliases []string
		var props []byte
		var relevance float64
		var lastChapterID *string
		if err := entityRows.Scan(&oldID, &etype, &name, &aliases, &desc, &props, &status, &relevance, &lastChapterID); err != nil {
			return "", fmt.Errorf("scan entity: %w", err)
		}
		nid := uuid.New().String()
		entityMap[oldID] = nid

		var newLastChapterID *string
		if lastChapterID != nil {
			remapped := chapterMap[*lastChapterID]
			newLastChapterID = &remapped
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())`,
			nid, newID, etype, name, aliases, desc, props, status, relevance, newLastChapterID)
		if err != nil {
			return "", fmt.Errorf("insert entity: %w", err)
		}
	}

	// 4. Entity mentions
	mentionRows, err := tx.Query(ctx, `
		SELECT em.id, em.entity_id, em.chapter_id, em.paragraph_index, em.paragraph_node_id, em.context_snippet, em.mention_type
		FROM entity_mentions em JOIN entities e ON em.entity_id = e.id
		WHERE e.universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template mentions: %w", err)
	}
	defer mentionRows.Close()
	for mentionRows.Next() {
		var oldID, oldEntityID, oldChapterID, nodeID, snippet, mtype string
		var pIdx int
		if err := mentionRows.Scan(&oldID, &oldEntityID, &oldChapterID, &pIdx, &nodeID, &snippet, &mtype); err != nil {
			return "", fmt.Errorf("scan mention: %w", err)
		}
		newEntityID := entityMap[oldEntityID]
		newChapterID := chapterMap[oldChapterID]
		_, err = tx.Exec(ctx, `
			INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, paragraph_node_id, context_snippet, mention_type, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
			uuid.New().String(), newEntityID, newChapterID, pIdx, nodeID, snippet, mtype)
		if err != nil {
			return "", fmt.Errorf("insert mention: %w", err)
		}
	}

	// 5. Entity embeddings (copy vector verbatim)
	embRows, err := tx.Query(ctx, `
		SELECT ee.id, ee.entity_id, ee.description_embedding
		FROM entity_embeddings ee JOIN entities e ON ee.entity_id = e.id
		WHERE e.universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template entity embeddings: %w", err)
	}
	defer embRows.Close()
	for embRows.Next() {
		var oldID, oldEntityID string
		var embedding []float32
		if err := embRows.Scan(&oldID, &oldEntityID, &embedding); err != nil {
			return "", fmt.Errorf("scan entity embedding: %w", err)
		}
		newEntityID := entityMap[oldEntityID]
		_, err = tx.Exec(ctx, `
			INSERT INTO entity_embeddings (id, entity_id, description_embedding, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())`,
			uuid.New().String(), newEntityID, embedding)
		if err != nil {
			return "", fmt.Errorf("insert entity embedding: %w", err)
		}
	}

	// 6. Paragraph embeddings (copy vector verbatim)
	paraRows, err := tx.Query(ctx, `
		SELECT pe.id, pe.chapter_id, pe.paragraph_index, pe.paragraph_node_id, pe.content, pe.embedding
		FROM paragraph_embeddings pe JOIN chapters c ON pe.chapter_id = c.id
		JOIN works w ON c.work_id = w.id
		WHERE w.universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template paragraph embeddings: %w", err)
	}
	defer paraRows.Close()
	for paraRows.Next() {
		var oldID, oldChapterID, nodeID, content string
		var pIdx int
		var embedding []float32
		if err := paraRows.Scan(&oldID, &oldChapterID, &pIdx, &nodeID, &content, &embedding); err != nil {
			return "", fmt.Errorf("scan paragraph embedding: %w", err)
		}
		newChapterID := chapterMap[oldChapterID]
		_, err = tx.Exec(ctx, `
			INSERT INTO paragraph_embeddings (id, chapter_id, paragraph_index, paragraph_node_id, content, embedding, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
			uuid.New().String(), newChapterID, pIdx, nodeID, content, embedding)
		if err != nil {
			return "", fmt.Errorf("insert paragraph embedding: %w", err)
		}
	}

	// 7. Contradictions
	contraRows, err := tx.Query(ctx, `
		SELECT id, entity_id, severity, description, suggestion,
		       evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id, fingerprint, status
		FROM contradictions WHERE universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template contradictions: %w", err)
	}
	defer contraRows.Close()
	for contraRows.Next() {
		var oldID, severity, desc, suggestion, evidenceA, evidenceB, fingerprint, status string
		var oldEntityID, evAChID, evBChID *string
		if err := contraRows.Scan(&oldID, &oldEntityID, &severity, &desc, &suggestion,
			&evidenceA, &evAChID, &evidenceB, &evBChID, &fingerprint, &status); err != nil {
			return "", fmt.Errorf("scan contradiction: %w", err)
		}

		var newEntityID, newEvAChID, newEvBChID *string
		if oldEntityID != nil {
			remapped := entityMap[*oldEntityID]
			newEntityID = &remapped
		}
		if evAChID != nil {
			remapped := chapterMap[*evAChID]
			newEvAChID = &remapped
		}
		if evBChID != nil {
			remapped := chapterMap[*evBChID]
			newEvBChID = &remapped
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
			       evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id, fingerprint, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())`,
			uuid.New().String(), newID, newEntityID, severity, desc, suggestion,
			evidenceA, newEvAChID, evidenceB, newEvBChID, fingerprint, status)
		if err != nil {
			return "", fmt.Errorf("insert contradiction: %w", err)
		}
	}

	// 8. Timeline events
	tlRows, err := tx.Query(ctx, `
		SELECT id, event_entity_id, title, description, timeline_position, timeline_label,
		       chapter_id, participants
		FROM timeline_events WHERE universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template timeline events: %w", err)
	}
	defer tlRows.Close()
	for tlRows.Next() {
		var oldID, title, desc, label string
		var oldEventEntityID, oldChapterID *string
		var tlPos *float64
		var participants []string
		if err := tlRows.Scan(&oldID, &oldEventEntityID, &title, &desc, &tlPos, &label, &oldChapterID, &participants); err != nil {
			return "", fmt.Errorf("scan timeline event: %w", err)
		}

		var newEventEntityID, newChapterID *string
		if oldEventEntityID != nil {
			remapped := entityMap[*oldEventEntityID]
			newEventEntityID = &remapped
		}
		if oldChapterID != nil {
			remapped := chapterMap[*oldChapterID]
			newChapterID = &remapped
		}
		newParticipants := remapUUIDs(participants, entityMap)

		_, err = tx.Exec(ctx, `
			INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
			       timeline_position, timeline_label, chapter_id, participants, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())`,
			uuid.New().String(), newID, newEventEntityID, title, desc, tlPos, label, newChapterID, newParticipants)
		if err != nil {
			return "", fmt.Errorf("insert timeline event: %w", err)
		}
	}

	// 9. Plot holes
	phRows, err := tx.Query(ctx, `
		SELECT id, title, description, related_entity_ids, first_mentioned_chapter_id, status
		FROM plot_holes WHERE universe_id = $1`, templateID)
	if err != nil {
		return "", fmt.Errorf("query template plot holes: %w", err)
	}
	defer phRows.Close()
	for phRows.Next() {
		var oldID, title, desc, status string
		var relatedIDs []string
		var firstChID *string
		if err := phRows.Scan(&oldID, &title, &desc, &relatedIDs, &firstChID, &status); err != nil {
			return "", fmt.Errorf("scan plot hole: %w", err)
		}

		var newFirstChID *string
		if firstChID != nil {
			remapped := chapterMap[*firstChID]
			newFirstChID = &remapped
		}
		newRelatedIDs := remapUUIDs(relatedIDs, entityMap)

		_, err = tx.Exec(ctx, `
			INSERT INTO plot_holes (id, universe_id, title, description, related_entity_ids, first_mentioned_chapter_id, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
			uuid.New().String(), newID, title, desc, newRelatedIDs, newFirstChID, status)
		if err != nil {
			return "", fmt.Errorf("insert plot hole: %w", err)
		}
	}

	// 10. AGE Graph clone
	if err := s.cloneGraph(ctx, tx, templateID, newID, entityMap); err != nil {
		return "", fmt.Errorf("clone graph: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	return newID, nil
}

// reset / re-clone: delete then deep-copy (uses the expanded CloneUniverse above)
func (s *DemoService) ResetUniverse(ctx context.Context, sessionID string) (string, error) {
	u, err := s.universeRepo.FindBySessionID(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("universe not found for session")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete the session's universe (CASCADE removes all dependent rows)
	_, err = tx.Exec(ctx, `DELETE FROM universes WHERE id = $1`, u.ID)
	if err != nil {
		return "", fmt.Errorf("delete universe: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	// Re-clone with full deep-copy
	return s.CloneUniverse(ctx, sessionID)
}

// ── Helpers ──

// remapUUIDs replaces each UUID in ids using the given mapping.
func remapUUIDs(ids []string, m map[string]string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = m[id]
	}
	return out
}

// cloneGraph copies all vertices and edges from the template universe's AGE graph
// into a new graph for the cloned universe, remapping entity_ids.
func (s *DemoService) cloneGraph(ctx context.Context, tx pgx.Tx, templateID, newID string, entityMap map[string]string) error {
	// 1. Create the new graph
	if err := s.graphRepo.CreateGraph(ctx, newID); err != nil {
		return fmt.Errorf("create graph: %w", err)
	}
	newGraphName := "universe_" + newID

	// 2. Create nodes: derive label from entity type
	entRows, err := tx.Query(ctx, `
		SELECT id, type, name, status, relevance_score
		FROM entities WHERE universe_id = $1`, templateID)
	if err != nil {
		return fmt.Errorf("query entities for graph nodes: %w", err)
	}
	defer entRows.Close()
	for entRows.Next() {
		var oldID, etype, name, status string
		var relevance float64
		if err := entRows.Scan(&oldID, &etype, &name, &status, &relevance); err != nil {
			return fmt.Errorf("scan entity for node: %w", err)
		}
		newEntityID := entityMap[oldID]
		props := map[string]interface{}{
			"entity_id":       newEntityID,
			"name":            name,
			"status":          status,
			"relevance_score": relevance,
		}
		// ponytail: use entity type as graph node label directly
		if err := s.graphRepo.CreateNode(ctx, newGraphName, etype, props); err != nil {
			return fmt.Errorf("create node %s: %w", newEntityID, err)
		}
	}

	// 3. Create edges: query template graph for all relationships
	templateGraphName := "universe_" + templateID
	edgeQuery := fmt.Sprintf(`LOAD 'age'; SET search_path = ag_catalog, "$user", public; SELECT * FROM cypher('%s', $$ MATCH (a)-[r]->(b) WHERE a.entity_id IS NOT NULL AND b.entity_id IS NOT NULL RETURN a.entity_id, type(r), b.entity_id $$) AS (src agtype, rel agtype, tgt agtype)`,
		templateGraphName)
	edgeRows, err := tx.Query(ctx, edgeQuery, pgx.QueryExecModeSimpleProtocol)
	if err != nil {
		return fmt.Errorf("query graph edges: %w", err)
	}
	defer edgeRows.Close()
	for edgeRows.Next() {
		var srcRaw, relRaw, tgtRaw *string
		if err := edgeRows.Scan(&srcRaw, &relRaw, &tgtRaw); err != nil {
			return fmt.Errorf("scan edge row: %w", err)
		}
		if srcRaw == nil || relRaw == nil || tgtRaw == nil {
			continue
		}
		srcID := extractQuotedValue(*srcRaw)
		relType := extractQuotedValue(*relRaw)
		tgtID := extractQuotedValue(*tgtRaw)
		if srcID == "" || tgtID == "" || relType == "" {
			continue
		}

		newSrcID := entityMap[srcID]
		newTgtID := entityMap[tgtID]
		if err := s.graphRepo.CreateEdge(ctx, newGraphName, newSrcID, newTgtID, relType, nil); err != nil {
			return fmt.Errorf("create edge %s-[%s]->%s: %w", newSrcID, relType, newTgtID, err)
		}
	}

	return nil
}

// extractQuotedValue strips surrounding double-quotes from an agtype string value.
// agtype represents strings as "value" (quoted), so we just trim the outer quotes.
func extractQuotedValue(agtypeStr string) string {
	s := strings.TrimSpace(agtypeStr)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
