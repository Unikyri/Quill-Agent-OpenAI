package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

// TestResolveOrCreateNewEntityAppendsHistoryRow proves entity creation (Step
// 4 of ResolveOrCreate, brand-new entity, isNew=true) writes a single
// entity_relevance_history row with the initial score (0.8) and status
// (spec: Relevance history persistence requirement).
func TestResolveOrCreateNewEntityAppendsHistoryRow(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "019")
	ctx := context.Background()

	// Mock Qwen embeddings endpoint so Step 3 (semantic similarity) doesn't
	// need a real API key.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := EmbeddingResponse{Data: []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}{{Embedding: make([]float32, 1024)}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key"}
	qwenSvc := NewQwenService(cfg, nil)

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	entityRepo := repositories.NewEntityRepo(pool)
	vectorRepo := repositories.NewVectorRepo(pool)
	entitySvc := NewEntityService(pool, entityRepo, vectorRepo, qwenSvc)

	entity, previousStatus, isNew, err := entitySvc.ResolveOrCreate(ctx, universe.ID, repositories.ExtractedEntity{
		Type: "character", Name: "Brand New Wizard",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreate: %v", err)
	}
	if !isNew {
		t.Fatal("expected a brand-new entity to be created")
	}
	if previousStatus != "" {
		t.Errorf("previousStatus = %q, want empty for brand-new entity", previousStatus)
	}

	historyRepo := repositories.NewEntityRelevanceHistoryRepo(pool)
	points, err := historyRepo.ListRecentByUniverse(ctx, universe.ID, 30)
	if err != nil {
		t.Fatalf("ListRecentByUniverse: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("len(points) = %d, want 1", len(points))
	}
	if points[0].EntityID != entity.ID {
		t.Errorf("EntityID = %v, want %v", points[0].EntityID, entity.ID)
	}
	if points[0].RelevanceScore != 0.8 {
		t.Errorf("RelevanceScore = %f, want 0.8", points[0].RelevanceScore)
	}
}

// newErrorQwenService returns a QwenService whose embedding endpoint always
// fails, so ResolveOrCreate falls through to creating a new entity.
func newErrorQwenService(t *testing.T) *QwenService {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key"}
	return NewQwenService(cfg, nil)
}

// seedEntity creates an entity row directly via the repository so tests can
// exercise ResolveOrCreate paths without a real Qwen embedding service.
func seedEntity(t *testing.T, ctx context.Context, pool *pgxpool.Pool, universeID uuid.UUID, name, entityType string) *models.Entity {
	t.Helper()
	repo := repositories.NewEntityRepo(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	e := &models.Entity{
		ID:             uuid.New(),
		UniverseID:     universeID,
		Type:           entityType,
		Name:           name,
		Status:         "active",
		RelevanceScore: 0.8,
	}
	if err := repo.Create(ctx, tx, e); err != nil {
		t.Fatalf("create entity: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return e
}

// TestResolveOrCreateFuzzyMergeShortQuery proves a short incoming name ("Holden")
// merges into an existing longer name ("James Holden") via fuzzy substring match.
func TestResolveOrCreateFuzzyMergeShortQuery(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	entityRepo := repositories.NewEntityRepo(pool)
	entitySvc := NewEntityService(pool, entityRepo, nil, nil)

	existing := seedEntity(t, ctx, pool, universe.ID, "James Holden", "character")

	merged, prevStatus, isNew, err := entitySvc.ResolveOrCreate(ctx, universe.ID, repositories.ExtractedEntity{
		Type: "character", Name: "Holden",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreate: %v", err)
	}
	if isNew {
		t.Fatal("expected fuzzy merge, not a new entity")
	}
	if merged.ID != existing.ID {
		t.Errorf("merged.ID = %v, want %v", merged.ID, existing.ID)
	}
	if prevStatus != existing.Status {
		t.Errorf("prevStatus = %q, want %q", prevStatus, existing.Status)
	}

	list, total, err := entityRepo.ListByUniverse(ctx, universe.ID, repositories.EntityFilters{Page: 1, Limit: 100})
	if err != nil {
		t.Fatalf("ListByUniverse: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("want 1 entity, got total=%d len=%d", total, len(list))
	}
}

// TestResolveOrCreateFuzzyMergeLongQuery proves a long incoming name ("James Holden")
// merges into an existing short name ("Holden") via fuzzy substring match.
func TestResolveOrCreateFuzzyMergeLongQuery(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	entityRepo := repositories.NewEntityRepo(pool)
	entitySvc := NewEntityService(pool, entityRepo, nil, nil)

	existing := seedEntity(t, ctx, pool, universe.ID, "Holden", "character")

	merged, _, isNew, err := entitySvc.ResolveOrCreate(ctx, universe.ID, repositories.ExtractedEntity{
		Type: "character", Name: "James Holden",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreate: %v", err)
	}
	if isNew {
		t.Fatal("expected fuzzy merge, not a new entity")
	}
	if merged.ID != existing.ID {
		t.Errorf("merged.ID = %v, want %v", merged.ID, existing.ID)
	}

	list, total, err := entityRepo.ListByUniverse(ctx, universe.ID, repositories.EntityFilters{Page: 1, Limit: 100})
	if err != nil {
		t.Fatalf("ListByUniverse: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("want 1 entity, got total=%d len=%d", total, len(list))
	}
}

// TestResolveOrCreateFuzzyMergeRespectsType proves fuzzy matching does not merge
// across entity types.
func TestResolveOrCreateFuzzyMergeRespectsType(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	entityRepo := repositories.NewEntityRepo(pool)
	entitySvc := NewEntityService(pool, entityRepo, nil, newErrorQwenService(t))

	_ = seedEntity(t, ctx, pool, universe.ID, "James Holden", "character")

	// Substring match but a different type should create a new entity.
	_, _, isNew, err := entitySvc.ResolveOrCreate(ctx, universe.ID, repositories.ExtractedEntity{
		Type: "location", Name: "Holden",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreate: %v", err)
	}
	if !isNew {
		t.Fatal("expected a new entity for a different type")
	}

	list, total, err := entityRepo.ListByUniverse(ctx, universe.ID, repositories.EntityFilters{Page: 1, Limit: 100})
	if err != nil {
		t.Fatalf("ListByUniverse: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Errorf("want 2 entities, got total=%d len=%d", total, len(list))
	}
}

func TestResolveOrCreateRecoversFromNaturalKeyRace(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "022")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Natural Key Race", GenreTags: []string{"fantasy"}}
	universeRepo := repositories.NewUniverseRepo(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin universe tx: %v", err)
	}
	if err := universeRepo.Create(ctx, tx, &universe); err != nil {
		t.Fatalf("create universe: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit universe: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		CREATE SEQUENCE entity_natural_key_race_seq;
		CREATE FUNCTION delay_first_natural_key_insert() RETURNS trigger AS $$
		BEGIN
			IF NEW.name = 'Race Entity' AND nextval('entity_natural_key_race_seq') = 1 THEN
				PERFORM pg_sleep(0.25);
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_first_natural_key_insert
		BEFORE INSERT ON entities
		FOR EACH ROW EXECUTE FUNCTION delay_first_natural_key_insert();
	`); err != nil {
		t.Fatalf("create race trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS delay_first_natural_key_insert ON entities; DROP FUNCTION IF EXISTS delay_first_natural_key_insert(); DROP SEQUENCE IF EXISTS entity_natural_key_race_seq;")
	})

	svc := NewEntityService(pool, repositories.NewEntityRepo(pool), nil, newErrorQwenService(t))
	type result struct {
		entity *models.Entity
		isNew  bool
		err    error
		data   repositories.ExtractedEntity
	}
	results := make(chan result, 2)
	inputs := []repositories.ExtractedEntity{
		{Type: "character", Name: "Race Entity", Aliases: []string{"First Alias"}, Description: "short", Status: "active"},
		{Type: "character", Name: "Race Entity", Aliases: []string{"Second Alias"}, Description: "the longer race description", Status: "archived"},
	}
	run := func(input repositories.ExtractedEntity) {
		entity, _, isNew, err := svc.ResolveOrCreate(ctx, universe.ID, input)
		results <- result{entity: entity, isNew: isNew, err: err, data: input}
	}
	go run(inputs[0])

	deadline := time.Now().Add(time.Second)
	for {
		var called bool
		if err := pool.QueryRow(ctx, "SELECT is_called FROM entity_natural_key_race_seq").Scan(&called); err != nil {
			t.Fatalf("read race sequence: %v", err)
		}
		if called {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first ResolveOrCreate did not reach the insert trigger")
		}
		time.Sleep(10 * time.Millisecond)
	}
	go run(inputs[1])

	var got []result
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("ResolveOrCreate race: %v", result.err)
		}
		got = append(got, result)
	}
	if len(got) != 2 || got[0].entity.ID != got[1].entity.ID {
		t.Fatalf("race winners = %#v, want the same entity", got)
	}
	newCount := 0
	for _, result := range got {
		if result.isNew {
			newCount++
		}
	}
	if newCount != 1 {
		t.Fatalf("new entity results = %d, want one creator and one recovered winner", newCount)
	}
	finalEntity, err := repositories.NewEntityRepo(pool).FindByNaturalKey(ctx, universe.ID, "Race Entity", "character")
	if err != nil {
		t.Fatalf("find final race winner: %v", err)
	}
	aliases := map[string]bool{}
	for _, alias := range finalEntity.Aliases {
		aliases[alias] = true
	}
	if !aliases["First Alias"] || !aliases["Second Alias"] {
		t.Errorf("winner aliases = %v, want both concurrent aliases", finalEntity.Aliases)
	}
	if finalEntity.Description != "the longer race description" {
		t.Errorf("winner description = %q, want the longest incoming description", finalEntity.Description)
	}
	for _, result := range got {
		if !result.isNew && result.entity.Status != result.data.Status {
			t.Errorf("recovered winner status = %q, want incoming status %q", result.entity.Status, result.data.Status)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM entities WHERE universe_id = $1 AND LOWER(name) = LOWER($2) AND type = $3", universe.ID, "Race Entity", "character").Scan(&count); err != nil {
		t.Fatalf("count race entities: %v", err)
	}
	if count != 1 {
		t.Errorf("natural-key entity count = %d, want 1", count)
	}
}
