package repositories

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/testutil"
)

func setupHistoryFixtures(t *testing.T, pool *pgxpool.Pool) models.Universe {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "History Test Universe", Format: "novel"}

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

// TestEntityRelevanceHistoryRepoListRecentByUniverseCapsAt30 proves the
// window-function query caps history to the most recent N rows per entity,
// returned oldest-first, even when far more rows exist.
func TestEntityRelevanceHistoryRepoListRecentByUniverseCapsAt30(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "019")
	universe := setupHistoryFixtures(t, pool)
	entity := createTestEntity(t, pool, universe.ID, "Sampled Entity", 0.8, "active")

	ctx := context.Background()
	repo := NewEntityRelevanceHistoryRepo(pool)

	// insert 35 rows, each with an increasing score, oldest first
	const total = 35
	for i := 0; i < total; i++ {
		score := 0.5 + float64(i)*0.01
		_, err := pool.Exec(ctx,
			"INSERT INTO entity_relevance_history (id, entity_id, universe_id, relevance_score, status, recorded_at) VALUES ($1,$2,$3,$4,$5, NOW() + ($6 || ' seconds')::interval)",
			uuid.New(), entity.ID, universe.ID, score, "active", i)
		if err != nil {
			t.Fatalf("seed history row %d: %v", i, err)
		}
	}

	points, err := repo.ListRecentByUniverse(ctx, universe.ID, 30)
	if err != nil {
		t.Fatalf("ListRecentByUniverse: %v", err)
	}

	if len(points) != 30 {
		t.Fatalf("len(points) = %d, want 30 (capped)", len(points))
	}

	// oldest-first: the retained rows are the LAST 30 inserted (i=5..34),
	// so the first returned point must be i=5 (score 0.55) and the last i=34.
	wantFirst := 0.5 + 5*0.01
	wantLast := 0.5 + 34*0.01
	if diff := points[0].RelevanceScore - wantFirst; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("points[0].RelevanceScore = %f, want ~%f", points[0].RelevanceScore, wantFirst)
	}
	if diff := points[len(points)-1].RelevanceScore - wantLast; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("points[last].RelevanceScore = %f, want ~%f", points[len(points)-1].RelevanceScore, wantLast)
	}
	for _, p := range points {
		if p.EntityID != entity.ID {
			t.Errorf("point EntityID = %v, want %v", p.EntityID, entity.ID)
		}
	}
	// ordering: recorded_at must be non-decreasing (oldest-first)
	for i := 1; i < len(points); i++ {
		if points[i].RecordedAt.Before(points[i-1].RecordedAt) {
			t.Errorf("points not oldest-first at index %d", i)
		}
	}
}

// TestEntityRelevanceHistoryRepoAppendOne inserts a single snapshot row
// reflecting the entity's current score/status.
func TestEntityRelevanceHistoryRepoAppendOne(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "019")
	universe := setupHistoryFixtures(t, pool)
	entity := createTestEntity(t, pool, universe.ID, "Solo Entity", 0.8, "active")

	ctx := context.Background()
	repo := NewEntityRelevanceHistoryRepo(pool)

	if err := repo.AppendOne(ctx, entity.ID); err != nil {
		t.Fatalf("AppendOne: %v", err)
	}

	points, err := repo.ListRecentByUniverse(ctx, universe.ID, 30)
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
	if points[0].Status != "active" {
		t.Errorf("Status = %q, want active", points[0].Status)
	}
}

// TestEntityRelevanceHistoryRepoAppendSnapshot inserts one row per entity in
// the universe, matching each entity's current score/status.
func TestEntityRelevanceHistoryRepoAppendSnapshot(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "019")
	universe := setupHistoryFixtures(t, pool)
	e1 := createTestEntity(t, pool, universe.ID, "Snap A", 0.7, "active")
	e2 := createTestEntity(t, pool, universe.ID, "Snap B", 0.1, "archived")

	ctx := context.Background()
	repo := NewEntityRelevanceHistoryRepo(pool)

	if err := repo.AppendSnapshot(ctx, universe.ID); err != nil {
		t.Fatalf("AppendSnapshot: %v", err)
	}

	points, err := repo.ListRecentByUniverse(ctx, universe.ID, 30)
	if err != nil {
		t.Fatalf("ListRecentByUniverse: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("len(points) = %d, want 2 (one per entity)", len(points))
	}

	byEntity := map[uuid.UUID]RelevanceHistoryPoint{}
	for _, p := range points {
		byEntity[p.EntityID] = p
	}
	if p, ok := byEntity[e1.ID]; !ok || p.RelevanceScore != 0.7 || p.Status != "active" {
		t.Errorf("e1 snapshot = %+v, want score=0.7 status=active", p)
	}
	if p, ok := byEntity[e2.ID]; !ok || p.RelevanceScore != 0.1 || p.Status != "archived" {
		t.Errorf("e2 snapshot = %+v, want score=0.1 status=archived", p)
	}
}
