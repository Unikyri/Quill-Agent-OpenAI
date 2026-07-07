package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/testutil"
)

func setupConsolidationFixtures(t *testing.T, pool *pgxpool.Pool) models.Universe {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Consolidation Test Universe", Format: "novel"}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "INSERT INTO universes (id, user_id, name, format) VALUES ($1,$2,$3,$4)",
		universe.ID, universe.UserID, universe.Name, universe.Format); err != nil {
		t.Fatalf("insert universe: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return universe
}

func TestConsolidationRepoCreateAndFind(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "017")
	universe := setupConsolidationFixtures(t, pool)
	ctx := context.Background()

	entity := createTestEntity(t, pool, universe.ID, "Merlin", 0.8, "active")
	repo := NewConsolidationRepo(pool)

	cm := &models.ConsolidatedMemory{
		ID:        uuid.New(),
		EntityID:  entity.ID,
		Summary:   "Merlin is a powerful wizard who mentored the young king.",
		KeyFacts:  []string{"wizard", "mentor", "ancient"},
		Embedding: make([]float32, 1024),
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.Create(ctx, cm); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// spec: WHEN Create is called then FindByEntityID retrieves matching data
	found, err := repo.FindByEntityID(ctx, entity.ID)
	if err != nil {
		t.Fatalf("FindByEntityID: %v", err)
	}
	if found.EntityID != entity.ID {
		t.Errorf("EntityID = %v, want %v", found.EntityID, entity.ID)
	}
	if found.Summary != cm.Summary {
		t.Errorf("Summary = %q, want %q", found.Summary, cm.Summary)
	}
	if len(found.KeyFacts) != 3 {
		t.Errorf("KeyFacts length = %d, want 3", len(found.KeyFacts))
	}
	if len(found.Embedding) != 1024 {
		t.Errorf("Embedding length = %d, want 1024", len(found.Embedding))
	}
}

// TestConsolidationRepoFindSimilarByEmbeddingFiltersByUniverse proves the
// entities join scopes results to the given universe_id — consolidated
// memories from another universe must never leak into the ranked output.
func TestConsolidationRepoFindSimilarByEmbeddingFiltersByUniverse(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "017")
	ctx := context.Background()
	repo := NewConsolidationRepo(pool)

	universeA := setupConsolidationFixtures(t, pool)
	universeB := setupConsolidationFixtures(t, pool)

	entityA := createTestEntity(t, pool, universeA.ID, "Merlin", 0.8, "archived")
	entityB := createTestEntity(t, pool, universeB.ID, "Gandalf", 0.8, "archived")

	embA := makeEmbedding(0.0)
	embB := makeEmbedding(3.0)

	if err := repo.Create(ctx, &models.ConsolidatedMemory{
		ID: uuid.New(), EntityID: entityA.ID, Summary: "Merlin summary", Embedding: embA,
	}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if err := repo.Create(ctx, &models.ConsolidatedMemory{
		ID: uuid.New(), EntityID: entityB.ID, Summary: "Gandalf summary", Embedding: embB,
	}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	hits, err := repo.FindSimilarByEmbedding(ctx, universeA.ID, embA, 10)
	if err != nil {
		t.Fatalf("FindSimilarByEmbedding: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want 1 (only universe A)", len(hits))
	}
	if hits[0].EntityID != entityA.ID {
		t.Errorf("hits[0].EntityID = %v, want %v", hits[0].EntityID, entityA.ID)
	}
	if hits[0].Summary != "Merlin summary" {
		t.Errorf("hits[0].Summary = %q, want %q", hits[0].Summary, "Merlin summary")
	}
}

// TestConsolidationRepoFindSimilarByEmbeddingRanksByDistance triangulates
// with two same-universe rows at different cosine distances from the query.
func TestConsolidationRepoFindSimilarByEmbeddingRanksByDistance(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "017")
	ctx := context.Background()
	repo := NewConsolidationRepo(pool)

	universe := setupConsolidationFixtures(t, pool)
	near := createTestEntity(t, pool, universe.ID, "Near", 0.8, "archived")
	far := createTestEntity(t, pool, universe.ID, "Far", 0.8, "archived")

	if err := repo.Create(ctx, &models.ConsolidatedMemory{
		ID: uuid.New(), EntityID: near.ID, Summary: "near", Embedding: makeEmbedding(0.1),
	}); err != nil {
		t.Fatalf("create near: %v", err)
	}
	if err := repo.Create(ctx, &models.ConsolidatedMemory{
		ID: uuid.New(), EntityID: far.ID, Summary: "far", Embedding: makeEmbedding(2.0),
	}); err != nil {
		t.Fatalf("create far: %v", err)
	}

	hits, err := repo.FindSimilarByEmbedding(ctx, universe.ID, makeEmbedding(0.0), 10)
	if err != nil {
		t.Fatalf("FindSimilarByEmbedding: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("len(hits) = %d, want 2", len(hits))
	}
	if hits[0].EntityID != near.ID {
		t.Errorf("hits[0].EntityID = %v, want nearest (%v) first", hits[0].EntityID, near.ID)
	}
}

func TestConsolidationRepoDeleteIsIdempotent(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "017")
	universe := setupConsolidationFixtures(t, pool)
	ctx := context.Background()

	entity := createTestEntity(t, pool, universe.ID, "Gandalf", 0.8, "active")
	repo := NewConsolidationRepo(pool)

	// Seed a row
	cm := &models.ConsolidatedMemory{
		ID:        uuid.New(),
		EntityID:  entity.ID,
		Summary:   "Test summary",
		Embedding: make([]float32, 1024),
	}
	if err := repo.Create(ctx, cm); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Delete should succeed
	if err := repo.DeleteByEntityID(ctx, entity.ID); err != nil {
		t.Fatalf("DeleteByEntityID (existing): %v", err)
	}

	// Verify it's gone
	_, err := repo.FindByEntityID(ctx, entity.ID)
	if err == nil {
		t.Error("FindByEntityID should return error after delete")
	}

	// spec: idempotent — deleting a non-existent row returns no error
	if err := repo.DeleteByEntityID(ctx, entity.ID); err != nil {
		t.Errorf("DeleteByEntityID (idempotent): %v", err)
	}
}
