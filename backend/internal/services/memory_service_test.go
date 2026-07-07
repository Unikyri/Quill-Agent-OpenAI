package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

// TestMemoryServiceRecallNormalModeVectorSeedsGraph verifies that, given a
// non-empty embedding, the graph pipeline seeds from the entities mentioned
// in the top vector-ranked paragraphs (not from global recency) — an entity
// only reachable via that vector-derived seed must appear in the fused
// result, sourced from the graph pipeline.
func TestMemoryServiceRecallNormalModeVectorSeedsGraph(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "018")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	seedEntity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Seed Entity", 0.9, "active")
	neighborEntity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Neighbor Entity", 0.1, "active")

	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	chapter := svcCreateTestChapter(t, ctx, pool, work.ID, "Chapter 1", 0)

	entityRepo := repositories.NewEntityRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	vectorRepo := repositories.NewVectorRepo(pool)

	queryEmb := make([]float32, 1024)
	queryEmb[0] = 1.0
	const paragraphIndex = 2
	if err := vectorRepo.SaveParagraphEmbedding(ctx, chapter.ID, paragraphIndex, "node-1", "The seed entity does something notable.", queryEmb); err != nil {
		t.Fatalf("save paragraph embedding: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index) VALUES ($1,$2,$3,$4)",
		uuid.New(), seedEntity.ID, chapter.ID, paragraphIndex); err != nil {
		t.Fatalf("insert mention: %v", err)
	}

	graphName := "universe_" + universe.ID.String()
	if err := graphRepo.CreateGraph(ctx, universe.ID.String()); err != nil {
		t.Fatalf("create graph: %v", err)
	}
	_ = graphRepo.CreateNode(ctx, graphName, "Entity", map[string]interface{}{
		"entity_id": seedEntity.ID.String(), "name": seedEntity.Name, "status": "active", "relevance_score": 0.9,
	})
	_ = graphRepo.CreateNode(ctx, graphName, "Entity", map[string]interface{}{
		"entity_id": neighborEntity.ID.String(), "name": neighborEntity.Name, "status": "active", "relevance_score": 0.1,
	})
	if err := graphRepo.CreateEdge(ctx, graphName, seedEntity.ID.String(), neighborEntity.ID.String(), "KNOWS", nil); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	svc := NewMemoryService(graphRepo, entityRepo, vectorRepo)

	items, err := svc.RecallWithQuery(ctx, universe.ID, queryEmb, "seed entity", 10)
	if err != nil {
		t.Fatalf("RecallWithQuery failed: %v", err)
	}

	var foundNeighborViaGraph bool
	for _, item := range items {
		if item.EntityID == neighborEntity.ID && strings.Contains(item.Source, "graph") {
			foundNeighborViaGraph = true
		}
	}
	if !foundNeighborViaGraph {
		t.Errorf("expected neighborEntity (%s) to be found via graph pipeline seeded from the vector hit's mentioned entity; got items: %+v", neighborEntity.ID, items)
	}

	var foundVectorHit bool
	for _, item := range items {
		if strings.Contains(item.Source, "vector") {
			foundVectorHit = true
		}
	}
	if !foundVectorHit {
		t.Errorf("expected a vector-sourced item in results, got: %+v", items)
	}
}

// TestMemoryServiceRecallDegradedModeSkipsVectorKeywordConsolidated verifies
// that a nil embedding + empty queryText call skips the vector, keyword, and
// consolidated pipelines, falls back to recency-seeded graph traversal, and
// returns no error.
func TestMemoryServiceRecallDegradedModeSkipsVectorKeywordConsolidated(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "018")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	entityA := svcCreateTestEntity(t, ctx, pool, universe.ID, "Entity A", 0.9, "active")
	entityB := svcCreateTestEntity(t, ctx, pool, universe.ID, "Entity B", 0.5, "active")
	_ = svcCreateTestEntity(t, ctx, pool, universe.ID, "Entity C", 0.2, "archived")

	entityRepo := repositories.NewEntityRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	vectorRepo := repositories.NewVectorRepo(pool)

	graphName := "universe_" + universe.ID.String()
	if err := graphRepo.CreateGraph(ctx, universe.ID.String()); err != nil {
		t.Fatalf("create graph: %v", err)
	}
	_ = graphRepo.CreateNode(ctx, graphName, "Entity", map[string]interface{}{
		"entity_id": entityA.ID.String(), "name": entityA.Name, "status": "active", "relevance_score": 0.9,
	})
	_ = graphRepo.CreateNode(ctx, graphName, "Entity", map[string]interface{}{
		"entity_id": entityB.ID.String(), "name": entityB.Name, "status": "active", "relevance_score": 0.5,
	})
	_ = graphRepo.CreateEdge(ctx, graphName, entityA.ID.String(), entityB.ID.String(), "KNOWS", nil)

	svc := NewMemoryService(graphRepo, entityRepo, vectorRepo)

	items, err := svc.RecallWithQuery(ctx, universe.ID, nil, "", 10)
	if err != nil {
		t.Fatalf("RecallWithQuery failed on degraded input: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected non-empty degraded-mode results (graph + recency)")
	}
	for _, item := range items {
		if strings.Contains(item.Source, "vector") || strings.Contains(item.Source, "keyword") || strings.Contains(item.Source, "consolidated") {
			t.Errorf("degraded mode must skip vector/keyword/consolidated pipelines, got item sourced %q", item.Source)
		}
	}
}

// TestMemoryServiceRecallBudgetAndKCap verifies that when a budgetMgr is
// wired, FitToBudget trims the fused list to VectorTokens before the k cap
// is applied, and that k still bounds the final result size.
func TestMemoryServiceRecallBudgetAndKCap(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "018")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	// 20 entities → 20 recency facts. A tiny VectorTokens budget below will
	// only fit a handful, forcing FitToBudget to actually drop items.
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("Entity Number %02d", i)
		svcCreateTestEntity(t, ctx, pool, universe.ID, name, 0.5+float64(i)*0.01, "active")
	}

	entityRepo := repositories.NewEntityRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	vectorRepo := repositories.NewVectorRepo(pool)

	svc := NewMemoryService(graphRepo, entityRepo, vectorRepo)

	baseline, err := svc.RecallWithQuery(ctx, universe.ID, nil, "", 100)
	if err != nil {
		t.Fatalf("baseline RecallWithQuery failed: %v", err)
	}
	if len(baseline) == 0 {
		t.Fatal("expected non-empty baseline results")
	}

	tinyBudget := NewContextBudgetManager(NewTokenizer(), 100, 0) // available=100, VectorTokens=40
	svc.SetBudgetMgr(tinyBudget)

	fitted, err := svc.RecallWithQuery(ctx, universe.ID, nil, "", 100)
	if err != nil {
		t.Fatalf("budget-fitted RecallWithQuery failed: %v", err)
	}
	if len(fitted) >= len(baseline) {
		t.Errorf("expected budget fit to drop items: baseline=%d fitted=%d", len(baseline), len(fitted))
	}

	tok := NewTokenizer()
	var totalTokens int
	for _, item := range fitted {
		totalTokens += tok.CountTokens(item.Fact)
	}
	if totalTokens > 40 {
		t.Errorf("fitted results exceed VectorTokens budget: %d tokens > 40", totalTokens)
	}

	capped, err := svc.RecallWithQuery(ctx, universe.ID, nil, "", 2)
	if err != nil {
		t.Fatalf("k-capped RecallWithQuery failed: %v", err)
	}
	if len(capped) > 2 {
		t.Errorf("expected k=2 to cap results, got %d", len(capped))
	}
}
