package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"github.com/quill/backend/internal/testutil"
)

// makeEmbedding builds a 1024-dim unit vector on the plane spanned by axes 0/1,
// so cosine distance from the zero-angle vector is exactly 1-cos(theta) — precise
// and cheap to reason about instead of seeding random high-dim vectors.
func makeEmbedding(theta float64) []float32 {
	v := make([]float32, 1024)
	v[0] = float32(math.Cos(theta))
	v[1] = float32(math.Sin(theta))
	return v
}

func TestFindSimilarEntitiesReturnsTopK(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "016")
	universe := setupEntityRepoFixtures(t, pool)
	ctx := context.Background()
	vecRepo := NewVectorRepo(pool)

	thetas := []float64{0.1, 0.3, 0.5, 0.7, 0.9}
	for i, theta := range thetas {
		e := createTestEntity(t, pool, universe.ID, fmt.Sprintf("Entity %d", i), 1.0, "active")
		if err := vecRepo.SaveEntityEmbedding(ctx, e.ID, makeEmbedding(theta)); err != nil {
			t.Fatalf("save embedding %d: %v", i, err)
		}
	}

	results, err := vecRepo.FindSimilarEntities(ctx, universe.ID, makeEmbedding(0.0), 3)
	if err != nil {
		t.Fatalf("FindSimilarEntities: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i].Distance < results[i-1].Distance {
			t.Errorf("results not ascending by distance: %+v", results)
		}
	}

	wantClosest := map[string]bool{"Entity 0": true, "Entity 1": true, "Entity 2": true}
	for _, r := range results {
		if !wantClosest[r.Name] {
			t.Errorf("unexpected entity in top-3: %s (distance %f)", r.Name, r.Distance)
		}
	}
}

func TestFindSimilarEntitiesRespectsThreshold(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "016")
	universe := setupEntityRepoFixtures(t, pool)
	ctx := context.Background()
	vecRepo := NewVectorRepo(pool)

	// Distances (1-cos(theta)): ~0.005, ~0.460, ~1.416, ~1.990
	thetas := []float64{0.1, 1.0, 2.0, 3.0}
	for i, theta := range thetas {
		e := createTestEntity(t, pool, universe.ID, fmt.Sprintf("T%d", i), 1.0, "active")
		if err := vecRepo.SaveEntityEmbedding(ctx, e.ID, makeEmbedding(theta)); err != nil {
			t.Fatalf("save embedding %d: %v", i, err)
		}
	}

	// FindSimilarEntities has no SQL-side threshold (top-k only, mirrors
	// FindSimilarParagraphs) — callers filter by Distance themselves.
	results, err := vecRepo.FindSimilarEntities(ctx, universe.ID, makeEmbedding(0.0), 10)
	if err != nil {
		t.Fatalf("FindSimilarEntities: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("len(results) = %d, want 4 (all entities, unfiltered)", len(results))
	}

	const threshold = 0.5
	var filtered []SimilarEntity
	for _, r := range results {
		if r.Distance <= threshold {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 || len(filtered) >= len(results) {
		t.Fatalf("expected some but not all entities to pass threshold %f, got %d/%d", threshold, len(filtered), len(results))
	}
	for _, r := range filtered {
		if r.Distance > threshold {
			t.Errorf("filtered result exceeds threshold: %+v", r)
		}
	}
}

// TestMigration018AddsFullTextIndex proves the up migration adds content_tsv
// (GENERATED tsvector) + a GIN index on paragraph_embeddings, and the down
// migration removes both cleanly.
func TestMigration018AddsFullTextIndex(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "018")
	ctx := context.Background()

	var colExists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='paragraph_embeddings' AND column_name='content_tsv')`,
	).Scan(&colExists); err != nil {
		t.Fatalf("check content_tsv column: %v", err)
	}
	if !colExists {
		t.Error("expected paragraph_embeddings.content_tsv column to exist post-up")
	}

	var idxExists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename='paragraph_embeddings' AND indexname='idx_paragraph_embeddings_content_tsv')`,
	).Scan(&idxExists); err != nil {
		t.Fatalf("check GIN index: %v", err)
	}
	if !idxExists {
		t.Error("expected idx_paragraph_embeddings_content_tsv GIN index to exist post-up")
	}

	// Run the down migration directly and verify rollback.
	downSQL, err := os.ReadFile(filepath.Join("..", "..", "migrations", "018_add_fulltext_index.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("execute down migration: %v", err)
	}

	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='paragraph_embeddings' AND column_name='content_tsv')`,
	).Scan(&colExists); err != nil {
		t.Fatalf("check content_tsv column post-down: %v", err)
	}
	if colExists {
		t.Error("expected paragraph_embeddings.content_tsv column to be gone post-down")
	}

	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename='paragraph_embeddings' AND indexname='idx_paragraph_embeddings_content_tsv')`,
	).Scan(&idxExists); err != nil {
		t.Fatalf("check GIN index post-down: %v", err)
	}
	if idxExists {
		t.Error("expected idx_paragraph_embeddings_content_tsv to be gone post-down")
	}
}

// TestFindSimilarParagraphsPopulatesParagraphIndex proves the SELECT/Scan
// carries paragraph_index through to SimilarParagraph.ParagraphIndex (needed
// to join entity_mentions for vector-seeded graph context).
func TestFindSimilarParagraphsPopulatesParagraphIndex(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "018")
	universe := setupEntityRepoFixtures(t, pool)
	ctx := context.Background()

	workID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO works (id, universe_id, title, type) VALUES ($1,$2,$3,$4)", workID, universe.ID, "Test Work", "novel"); err != nil {
		t.Fatalf("insert work: %v", err)
	}
	chapterID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO chapters (id, work_id, title, order_index) VALUES ($1,$2,$3,$4)", chapterID, workID, "Chapter 1", 0); err != nil {
		t.Fatalf("insert chapter: %v", err)
	}

	vecRepo := NewVectorRepo(pool)
	if err := vecRepo.SaveParagraphEmbedding(ctx, chapterID, 3, "node-3", "third paragraph text", makeEmbedding(0.1)); err != nil {
		t.Fatalf("save paragraph embedding: %v", err)
	}
	if err := vecRepo.SaveParagraphEmbedding(ctx, chapterID, 7, "node-7", "seventh paragraph text", makeEmbedding(0.2)); err != nil {
		t.Fatalf("save paragraph embedding: %v", err)
	}

	results, err := vecRepo.FindSimilarParagraphs(ctx, universe.ID, makeEmbedding(0.0), uuid.Nil, 10)
	if err != nil {
		t.Fatalf("FindSimilarParagraphs: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	gotIndexes := map[int]bool{}
	for _, r := range results {
		gotIndexes[r.ParagraphIndex] = true
	}
	if !gotIndexes[3] || !gotIndexes[7] {
		t.Errorf("expected ParagraphIndex 3 and 7 populated, got %+v", results)
	}
}

// TestKeywordSearchFindsTextMatch proves KeywordSearch finds a paragraph via
// full-text match (websearch_to_tsquery) using the migration 018 tsvector/GIN
// index — a query embedding never enters this path.
func TestKeywordSearchFindsTextMatch(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "018")
	universe := setupEntityRepoFixtures(t, pool)
	ctx := context.Background()

	workID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO works (id, universe_id, title, type) VALUES ($1,$2,$3,$4)", workID, universe.ID, "Test Work", "novel"); err != nil {
		t.Fatalf("insert work: %v", err)
	}
	chapterID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO chapters (id, work_id, title, order_index) VALUES ($1,$2,$3,$4)", chapterID, workID, "Chapter 1", 0); err != nil {
		t.Fatalf("insert chapter: %v", err)
	}

	vecRepo := NewVectorRepo(pool)
	if err := vecRepo.SaveParagraphEmbedding(ctx, chapterID, 0, "node-0", "the dragon breathed fire over the castle", makeEmbedding(0.1)); err != nil {
		t.Fatalf("save paragraph embedding: %v", err)
	}
	if err := vecRepo.SaveParagraphEmbedding(ctx, chapterID, 1, "node-1", "completely unrelated content about taxes", makeEmbedding(0.2)); err != nil {
		t.Fatalf("save paragraph embedding: %v", err)
	}

	hits, err := vecRepo.KeywordSearch(ctx, universe.ID, "dragon castle", 10)
	if err != nil {
		t.Fatalf("KeywordSearch: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want 1", len(hits))
	}
	if hits[0].Content != "the dragon breathed fire over the castle" {
		t.Errorf("hits[0].Content = %q, want the dragon paragraph", hits[0].Content)
	}
	if hits[0].ChapterID != chapterID {
		t.Errorf("hits[0].ChapterID = %v, want %v", hits[0].ChapterID, chapterID)
	}
}

// TestKeywordSearchNoMatchReturnsEmpty triangulates with a query that matches
// nothing — proves the ranking path doesn't return unrelated rows.
func TestKeywordSearchNoMatchReturnsEmpty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "018")
	universe := setupEntityRepoFixtures(t, pool)
	ctx := context.Background()

	workID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO works (id, universe_id, title, type) VALUES ($1,$2,$3,$4)", workID, universe.ID, "Test Work", "novel"); err != nil {
		t.Fatalf("insert work: %v", err)
	}
	chapterID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO chapters (id, work_id, title, order_index) VALUES ($1,$2,$3,$4)", chapterID, workID, "Chapter 1", 0); err != nil {
		t.Fatalf("insert chapter: %v", err)
	}

	vecRepo := NewVectorRepo(pool)
	if err := vecRepo.SaveParagraphEmbedding(ctx, chapterID, 0, "node-0", "the dragon breathed fire over the castle", makeEmbedding(0.1)); err != nil {
		t.Fatalf("save paragraph embedding: %v", err)
	}

	hits, err := vecRepo.KeywordSearch(ctx, universe.ID, "spreadsheet accounting ledger", 10)
	if err != nil {
		t.Fatalf("KeywordSearch: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected no matches, got %d: %+v", len(hits), hits)
	}
}

func TestSetHNSWSearchParams(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "016")
	ctx := context.Background()
	vecRepo := NewVectorRepo(pool)

	if err := vecRepo.SetHNSWSearchParams(ctx, 40); err != nil {
		t.Errorf("SetHNSWSearchParams(40): %v", err)
	}
	if err := vecRepo.SetHNSWSearchParams(ctx, 100); err != nil {
		t.Errorf("SetHNSWSearchParams(100): %v", err)
	}
}

// TestHNSWIndexIsUsedByQueryPlanner is the proof-of-value test for this change:
// it seeds enough rows that, with sequential scans penalized, the planner must
// route the cosine-distance ORDER BY / LIMIT query through the HNSW index.
func TestHNSWIndexIsUsedByQueryPlanner(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "016")
	universe := setupEntityRepoFixtures(t, pool)
	ctx := context.Background()
	vecRepo := NewVectorRepo(pool)

	for i := 0; i < 50; i++ {
		e := createTestEntity(t, pool, universe.ID, fmt.Sprintf("Bulk %d", i), 1.0, "active")
		theta := float64(i) * 0.05
		if err := vecRepo.SaveEntityEmbedding(ctx, e.ID, makeEmbedding(theta)); err != nil {
			t.Fatalf("seed embedding %d: %v", i, err)
		}
	}

	// Freshly inserted rows have no planner statistics yet; without ANALYZE the
	// planner assumes ~1 row per universe_id and picks a nested-loop + sort
	// plan regardless of enable_seqscan, never touching the HNSW index.
	if _, err := pool.Exec(ctx, "ANALYZE entities, entity_embeddings"); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	// enable_seqscan=off / enable_sort=off are soft cost penalties (not hard
	// prohibitions), so the planner still has valid alternative paths (a
	// Bitmap Heap Scan + explicit Sort competes with the HNSW's pre-ordered
	// index scan at this small row count) but strongly prefers the HNSW index
	// scan once both are penalized — a deterministic signal without needing
	// thousands of rows.
	if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatalf("set enable_seqscan off: %v", err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL enable_sort = off"); err != nil {
		t.Fatalf("set enable_sort off: %v", err)
	}

	explainSQL := `
		EXPLAIN (FORMAT JSON)
		SELECT e.id, e.name, ee.description_embedding <=> $1 AS distance
		FROM entities e
		JOIN entity_embeddings ee ON e.id = ee.entity_id
		WHERE e.universe_id = $2
		ORDER BY distance ASC
		LIMIT $3
	`
	var planJSON string
	if err := tx.QueryRow(ctx, explainSQL, pgvector.NewVector(makeEmbedding(0.0)), universe.ID, 5).Scan(&planJSON); err != nil {
		t.Fatalf("explain query: %v", err)
	}

	var plan []map[string]interface{}
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		t.Fatalf("unmarshal plan json: %v\nplan: %s", err, planJSON)
	}
	if len(plan) == 0 {
		t.Fatalf("empty plan: %s", planJSON)
	}
	root, ok := plan[0]["Plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("plan missing root Plan node: %s", planJSON)
	}

	var usesHNSWIndex, hasSeqScanOnEmbeddings bool
	var walk func(node map[string]interface{})
	walk = func(node map[string]interface{}) {
		nodeType, _ := node["Node Type"].(string)
		relation, _ := node["Relation Name"].(string)
		indexName, _ := node["Index Name"].(string)

		if (nodeType == "Index Scan" || nodeType == "Bitmap Index Scan") && indexName == "idx_entity_embeddings_hnsw" {
			usesHNSWIndex = true
		}
		if nodeType == "Seq Scan" && relation == "entity_embeddings" {
			hasSeqScanOnEmbeddings = true
		}
		if children, ok := node["Plans"].([]interface{}); ok {
			for _, c := range children {
				if childNode, ok := c.(map[string]interface{}); ok {
					walk(childNode)
				}
			}
		}
	}
	walk(root)

	if !usesHNSWIndex {
		t.Errorf("expected plan to use idx_entity_embeddings_hnsw\nplan: %s", planJSON)
	}
	if hasSeqScanOnEmbeddings {
		t.Errorf("expected no Seq Scan on entity_embeddings\nplan: %s", planJSON)
	}
}
