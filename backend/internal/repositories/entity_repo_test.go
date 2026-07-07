package repositories

import (
	"context"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/testutil"
)

func setupEntityRepoFixtures(t *testing.T, pool *pgxpool.Pool) models.Universe {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Entity Test Universe", Format: "novel"}

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

func createTestEntity(t *testing.T, pool *pgxpool.Pool, universeID uuid.UUID, name string, score float64, status string) models.Entity {
	t.Helper()
	ctx := context.Background()
	e := models.Entity{ID: uuid.New(), UniverseID: universeID, Type: "character", Name: name, Status: status, RelevanceScore: score, Description: ""}
	if _, err := pool.Exec(ctx, "INSERT INTO entities (id, universe_id, type, name, description, status, relevance_score) VALUES ($1,$2,$3,$4,$5,$6,$7)",
		e.ID, e.UniverseID, e.Type, e.Name, e.Description, e.Status, e.RelevanceScore); err != nil {
		t.Fatalf("insert entity: %v", err)
	}
	return e
}

// TestDecayAll verifies the exponential decay: score *= e^(-lambda)
func TestEntityRepoDecayAll(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005") // entities table
	universe := setupEntityRepoFixtures(t, pool)

	e1 := createTestEntity(t, pool, universe.ID, "Entity A", 0.8, "active")
	e2 := createTestEntity(t, pool, universe.ID, "Entity B", 0.5, "active")
	_ = createTestEntity(t, pool, universe.ID, "Archived E", 0.1, "archived") // should NOT be decayed

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	lambda := 0.1
	if err := repo.DecayAll(ctx, universe.ID, lambda); err != nil {
		t.Fatalf("DecayAll: %v", err)
	}

	// Verify e1
	found1, err := repo.FindByID(ctx, e1.ID)
	if err != nil {
		t.Fatalf("FindByID e1: %v", err)
	}
	expected1 := 0.8 * math.Exp(-lambda)
	if math.Abs(found1.RelevanceScore-expected1) > 0.001 {
		t.Errorf("e1 score = %f, want ~%f", found1.RelevanceScore, expected1)
	}

	// Verify e2
	found2, err := repo.FindByID(ctx, e2.ID)
	if err != nil {
		t.Fatalf("FindByID e2: %v", err)
	}
	expected2 := 0.5 * math.Exp(-lambda)
	if math.Abs(found2.RelevanceScore-expected2) > 0.001 {
		t.Errorf("e2 score = %f, want ~%f", found2.RelevanceScore, expected2)
	}
}

// TestTouchBatch verifies idle counter reset for multiple entities
func TestEntityRepoTouchBatch(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	universe := setupEntityRepoFixtures(t, pool)

	e1 := createTestEntity(t, pool, universe.ID, "Touch A", 0.5, "active")
	e2 := createTestEntity(t, pool, universe.ID, "Touch B", 0.3, "active")

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	chapterID := uuid.New()
	if err := repo.TouchBatch(ctx, []uuid.UUID{e1.ID, e2.ID}, chapterID); err != nil {
		t.Fatalf("TouchBatch: %v", err)
	}

	// Verify last_mentioned_chapter_id updated
	found1, err := repo.FindByID(ctx, e1.ID)
	if err != nil {
		t.Fatalf("FindByID e1: %v", err)
	}
	if found1.LastMentionedChapterID == nil || *found1.LastMentionedChapterID != chapterID {
		t.Errorf("e1 LastMentionedChapterID = %v, want %v", found1.LastMentionedChapterID, chapterID)
	}
	if found1.LastMentionedAt == nil {
		t.Error("e1 LastMentionedAt should be set")
	}

	found2, err := repo.FindByID(ctx, e2.ID)
	if err != nil {
		t.Fatalf("FindByID e2: %v", err)
	}
	if found2.LastMentionedChapterID == nil || *found2.LastMentionedChapterID != chapterID {
		t.Errorf("e2 LastMentionedChapterID = %v, want %v", found2.LastMentionedChapterID, chapterID)
	}
}

// TestListByUniverseActive filters by status=active
func TestEntityRepoListByUniverseActive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	universe := setupEntityRepoFixtures(t, pool)

	createTestEntity(t, pool, universe.ID, "Active 1", 0.9, "active")
	createTestEntity(t, pool, universe.ID, "Active 2", 0.7, "active")
	createTestEntity(t, pool, universe.ID, "Archived", 0.1, "archived")

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	list, err := repo.ListByUniverseActive(ctx, universe.ID)
	if err != nil {
		t.Fatalf("ListByUniverseActive: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByUniverseActive len = %d, want 2 (only active entities)", len(list))
	}
	for _, e := range list {
		if e.Status != "active" {
			t.Errorf("entity %s has status %q, want active", e.Name, e.Status)
		}
	}
}

// TestFindNewlyArchivable returns active entities below score threshold.
func TestEntityRepoFindNewlyArchivable(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	universe := setupEntityRepoFixtures(t, pool)

	// Entities with varying scores
	high := createTestEntity(t, pool, universe.ID, "HighScore", 0.9, "active")
	mid := createTestEntity(t, pool, universe.ID, "MidScore", 0.2, "active")
	low := createTestEntity(t, pool, universe.ID, "LowScore", 0.05, "active")
	_ = createTestEntity(t, pool, universe.ID, "AlreadyArchived", 0.01, "archived") // should NOT match

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	ids, err := repo.FindNewlyArchivable(ctx, universe.ID, 0.15)
	if err != nil {
		t.Fatalf("FindNewlyArchivable: %v", err)
	}

	// low (0.05) should match; high (0.9) and mid (0.2) should not; archived should not
	got := make(map[uuid.UUID]bool)
	for _, id := range ids {
		got[id] = true
	}

	if !got[low.ID] {
		t.Errorf("low score entity (0.05) should be returned")
	}
	if got[high.ID] {
		t.Errorf("high score entity (0.9) should NOT be returned")
	}
	if got[mid.ID] {
		t.Errorf("mid score entity (0.2) should NOT be returned (above threshold 0.15)")
	}
	if len(ids) != 1 {
		t.Errorf("FindNewlyArchivable returned %d entities, want 1; got IDs: %v", len(ids), ids)
	}
}

// TestFindNewlyArchivableNoMatches returns empty slice when none qualify.
func TestEntityRepoFindNewlyArchivableNoMatches(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	universe := setupEntityRepoFixtures(t, pool)

	createTestEntity(t, pool, universe.ID, "AllHigh", 0.9, "active")

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	ids, err := repo.FindNewlyArchivable(ctx, universe.ID, 0.15)
	if err != nil {
		t.Fatalf("FindNewlyArchivable: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 entities, got %d", len(ids))
	}
}

// setupChapterFixture creates a work+chapter under the given universe so tests
// can seed entity_mentions rows keyed by (chapter_id, paragraph_index).
func setupChapterFixture(t *testing.T, pool *pgxpool.Pool, universeID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	workID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO works (id, universe_id, title, type) VALUES ($1,$2,$3,$4)",
		workID, universeID, "Test Work", "novel"); err != nil {
		t.Fatalf("insert work: %v", err)
	}
	chapterID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO chapters (id, work_id, title, order_index) VALUES ($1,$2,$3,$4)",
		chapterID, workID, "Chapter 1", 0); err != nil {
		t.Fatalf("insert chapter: %v", err)
	}
	return chapterID
}

func createTestMention(t *testing.T, pool *pgxpool.Pool, entityID, chapterID uuid.UUID, paragraphIndex int) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		"INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index) VALUES ($1,$2,$3,$4)",
		uuid.New(), entityID, chapterID, paragraphIndex); err != nil {
		t.Fatalf("insert mention: %v", err)
	}
}

// TestEntityIDsForParagraphsBatchedLookup proves the batched join returns
// entity IDs grouped by (chapter_id, paragraph_index) key, covering multiple
// keys and multiple entities per key in one call.
func TestEntityIDsForParagraphsBatchedLookup(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "006")
	universe := setupEntityRepoFixtures(t, pool)
	chapterID := setupChapterFixture(t, pool, universe.ID)

	entityA := createTestEntity(t, pool, universe.ID, "Entity A", 0.9, "active")
	entityB := createTestEntity(t, pool, universe.ID, "Entity B", 0.8, "active")

	createTestMention(t, pool, entityA.ID, chapterID, 3)
	createTestMention(t, pool, entityB.ID, chapterID, 3)
	createTestMention(t, pool, entityA.ID, chapterID, 7)

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	keys := []ParagraphKey{
		{ChapterID: chapterID, ParagraphIndex: 3},
		{ChapterID: chapterID, ParagraphIndex: 7},
	}
	got, err := repo.EntityIDsForParagraphs(ctx, keys)
	if err != nil {
		t.Fatalf("EntityIDsForParagraphs: %v", err)
	}

	key3 := ParagraphKey{ChapterID: chapterID, ParagraphIndex: 3}
	key7 := ParagraphKey{ChapterID: chapterID, ParagraphIndex: 7}

	entities3 := map[uuid.UUID]bool{}
	for _, id := range got[key3] {
		entities3[id] = true
	}
	if !entities3[entityA.ID] || !entities3[entityB.ID] {
		t.Errorf("key3 entities = %v, want both %v and %v", got[key3], entityA.ID, entityB.ID)
	}
	if len(got[key3]) != 2 {
		t.Errorf("key3 len = %d, want 2", len(got[key3]))
	}

	if len(got[key7]) != 1 || got[key7][0] != entityA.ID {
		t.Errorf("key7 = %v, want [%v]", got[key7], entityA.ID)
	}
}

// TestEntityIDsForParagraphsEmptyKeys proves an empty input short-circuits to
// an empty map without issuing a query.
func TestEntityIDsForParagraphsEmptyKeys(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "006")

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	got, err := repo.EntityIDsForParagraphs(ctx, []ParagraphKey{})
	if err != nil {
		t.Fatalf("EntityIDsForParagraphs empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

// TestTouchBatchEmpty is safe with empty slice
func TestEntityRepoTouchBatchEmpty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	_ = setupEntityRepoFixtures(t, pool)

	ctx := context.Background()
	repo := NewEntityRepo(pool)

	// Should not error on empty slice
	if err := repo.TouchBatch(ctx, []uuid.UUID{}, uuid.New()); err != nil {
		t.Fatalf("TouchBatch empty: %v", err)
	}
}
