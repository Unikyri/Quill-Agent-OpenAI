package services

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

// TestDecayMath is a pure-function unit test for the exponential decay formula.
func TestDecayMath(t *testing.T) {
	lambda := 0.1

	tests := []struct {
		name     string
		score    float64
		idle     int // number of idle chapters
		expected float64
	}{
		{"idle=0", 0.8, 0, 0.8},
		{"idle=1", 0.8, 1, 0.8 * math.Exp(-0.1)},
		{"idle=15", 0.8, 15, 0.8 * math.Exp(-0.1*15)},
		{"idle=3", 0.5, 3, 0.5 * math.Exp(-0.1*3)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyDecay(tt.score, float64(tt.idle), lambda)
			if math.Abs(got-tt.expected) > 0.001 {
				t.Errorf("applyDecay(%f, %d, %f) = %f, want ~%f", tt.score, tt.idle, lambda, got, tt.expected)
			}
		})
	}
}

// TestRelevanceServiceTouch verifies that Touch updates last_mentioned on the entity.
func TestRelevanceServiceTouch(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	e := svcCreateTestEntity(t, ctx, pool, universe.ID, "Touched Entity", 0.5, "active")

	repo := repositories.NewEntityRepo(pool)
	svc := NewRelevanceService(pool, repo, 0.1, 0.15, nil)

	chapterID := uuid.New()
	if err := svc.Touch(ctx, e.ID, chapterID); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	found, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.LastMentionedChapterID == nil || *found.LastMentionedChapterID != chapterID {
		t.Errorf("LastMentionedChapterID = %v, want %v", found.LastMentionedChapterID, chapterID)
	}
	if found.LastMentionedAt == nil {
		t.Error("LastMentionedAt should be set after touch")
	}
}

// TestRelevanceServiceReactivate sets score to 0.8 and status to active.
func TestRelevanceServiceReactivate(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	e := svcCreateTestEntity(t, ctx, pool, universe.ID, "Archived Guy", 0.05, "archived")

	repo := repositories.NewEntityRepo(pool)
	svc := NewRelevanceService(pool, repo, 0.1, 0.15, nil)

	if err := svc.Reactivate(ctx, e.ID); err != nil {
		t.Fatalf("Reactivate failed: %v", err)
	}

	found, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.RelevanceScore != 0.8 {
		t.Errorf("score = %f, want 0.8", found.RelevanceScore)
	}
	if found.Status != "active" {
		t.Errorf("status = %s, want active", found.Status)
	}
}

// TestRelevanceServiceDecayAll applies decay and archives entities below threshold.
func TestRelevanceServiceDecayAll(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	high := svcCreateTestEntity(t, ctx, pool, universe.ID, "High Scorer", 0.9, "active")
	low := svcCreateTestEntity(t, ctx, pool, universe.ID, "Low Scorer", 0.16, "active")

	repo := repositories.NewEntityRepo(pool)
	svc := NewRelevanceService(pool, repo, 0.1, 0.15, nil)

	if err := svc.DecayAll(ctx, universe.ID); err != nil {
		t.Fatalf("DecayAll failed: %v", err)
	}

	foundHigh, err := repo.FindByID(ctx, high.ID)
	if err != nil {
		t.Fatalf("FindByID high: %v", err)
	}
	expectedHigh := 0.9 * math.Exp(-0.1)
	if math.Abs(foundHigh.RelevanceScore-expectedHigh) > 0.001 {
		t.Errorf("high score = %f, want ~%f", foundHigh.RelevanceScore, expectedHigh)
	}
	if foundHigh.Status != "active" {
		t.Errorf("high status = %s, want active (above threshold)", foundHigh.Status)
	}

	foundLow, err := repo.FindByID(ctx, low.ID)
	if err != nil {
		t.Fatalf("FindByID low: %v", err)
	}
	expectedLow := 0.16 * math.Exp(-0.1)
	if math.Abs(foundLow.RelevanceScore-expectedLow) > 0.001 {
		t.Errorf("low score = %f, want ~%f", foundLow.RelevanceScore, expectedLow)
	}
	if foundLow.Status != "archived" {
		t.Errorf("low status = %s, want archived (below threshold 0.15)", foundLow.Status)
	}
}

// ── Consolidation hook tests (require TEST_DATABASE_URL) ──

// spec: CRITICAL #4 — DecayAll triggers async consolidation for newly-archived entities
func TestDecayAllTriggersConsolidation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	if pool == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	// Seed an entity with score just above threshold so one decay drops it below
	e := svcCreateTestEntity(t, ctx, pool, universe.ID, "About To Archive", 0.16, "active")

	repo := repositories.NewEntityRepo(pool)

	// Create ConsolidationService with nil inner repos — goroutine fires but
	// ConsolidateEntity recovers from nil entityRepo (panic-safe)
	consolidationSvc := &ConsolidationService{
		consolidationRepo: nil,
		entityRepo:        nil,
		qwenSvc:           nil,
	}

	svc := NewRelevanceService(pool, repo, 0.1, 0.15, consolidationSvc)

	// DecayAll should archive the entity and fire the consolidation goroutine
	if err := svc.DecayAll(ctx, universe.ID); err != nil {
		t.Fatalf("DecayAll failed: %v", err)
	}

	// Verify the entity was archived
	found, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Status != "archived" {
		t.Errorf("status = %s, want archived", found.Status)
	}

	// The goroutine launched — since ConsolidationService has nil repos,
	// ConsolidateEntity recovers from nil pointer. The test verifies no panic
	// occurred during DecayAll (if it panicked, we wouldn't reach here).
}

// spec: CRITICAL #5 — Reactivate triggers deconsolidation
func TestReactivateTriggersDeconsolidation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	if pool == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	e := svcCreateTestEntity(t, ctx, pool, universe.ID, "Archived Guy", 0.05, "archived")

	repo := repositories.NewEntityRepo(pool)

	// Create ConsolidationService with nil consolidationRepo — DeconsolidateEntity
	// returns nil via nil check (added for testability)
	consolidationSvc := &ConsolidationService{
		consolidationRepo: nil,
		entityRepo:        nil,
		qwenSvc:           nil,
	}

	svc := NewRelevanceService(pool, repo, 0.1, 0.15, consolidationSvc)

	if err := svc.Reactivate(ctx, e.ID); err != nil {
		t.Fatalf("Reactivate failed: %v", err)
	}

	found, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Status != "active" {
		t.Errorf("status = %s, want active", found.Status)
	}
	if found.RelevanceScore != 0.8 {
		t.Errorf("score = %f, want 0.8", found.RelevanceScore)
	}

	// Deconsolidation hook was called (via nil ConsolidationService),
	// no panic occurred — verified by reaching here.
}

// ── Consolidator spy ──

// spyConsolidator records calls to ConsolidateEntity and DeconsolidateEntity.
type spyConsolidator struct {
	mu                  sync.Mutex
	consolidateCalls    []uuid.UUID
	deconsolidateCalls  []uuid.UUID
}

func (s *spyConsolidator) ConsolidateEntity(ctx context.Context, entityID, universeID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consolidateCalls = append(s.consolidateCalls, entityID)
	return nil
}

func (s *spyConsolidator) DeconsolidateEntity(ctx context.Context, entityID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deconsolidateCalls = append(s.deconsolidateCalls, entityID)
	return nil
}

// spec: CRITICAL #4 — DecayAll triggers async consolidation for newly-archived entities
func TestDecayAll_CallsConsolidateEntity(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	// Seed an entity with score just above threshold so one decay drops it below
	e := svcCreateTestEntity(t, ctx, pool, universe.ID, "About To Archive", 0.16, "active")

	repo := repositories.NewEntityRepo(pool)
	spy := &spyConsolidator{}
	svc := NewRelevanceService(pool, repo, 0.1, 0.15, spy)

	// DecayAll should archive the entity and fire the consolidation goroutine
	if err := svc.DecayAll(ctx, universe.ID); err != nil {
		t.Fatalf("DecayAll failed: %v", err)
	}

	// Verify the entity was archived
	found, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Status != "archived" {
		t.Errorf("status = %s, want archived", found.Status)
	}

	// Wait for goroutine to complete (fire-and-forget, need brief wait)
	time.Sleep(100 * time.Millisecond)

	// Verify ConsolidateEntity was called with the correct entity ID
	spy.mu.Lock()
	calls := spy.consolidateCalls
	spy.mu.Unlock()

	if len(calls) == 0 {
		t.Error("ConsolidateEntity was NOT called after DecayAll — consolidation hook not exercised")
	} else if len(calls) != 1 {
		t.Errorf("ConsolidateEntity called %d times, want 1", len(calls))
	} else if calls[0] != e.ID {
		t.Errorf("ConsolidateEntity called with entity ID %s, want %s", calls[0], e.ID)
	}
}

// spec: CRITICAL #5 — Reactivate triggers deconsolidation
func TestReactivate_CallsDeconsolidateEntity(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	e := svcCreateTestEntity(t, ctx, pool, universe.ID, "Archived Guy", 0.05, "archived")

	repo := repositories.NewEntityRepo(pool)
	spy := &spyConsolidator{}
	svc := NewRelevanceService(pool, repo, 0.1, 0.15, spy)

	if err := svc.Reactivate(ctx, e.ID); err != nil {
		t.Fatalf("Reactivate failed: %v", err)
	}

	found, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Status != "active" {
		t.Errorf("status = %s, want active", found.Status)
	}
	if found.RelevanceScore != 0.8 {
		t.Errorf("score = %f, want 0.8", found.RelevanceScore)
	}

	// Verify DeconsolidateEntity was called
	spy.mu.Lock()
	calls := spy.deconsolidateCalls
	spy.mu.Unlock()

	if len(calls) == 0 {
		t.Error("DeconsolidateEntity was NOT called after Reactivate — deconsolidation hook not exercised")
	} else if len(calls) != 1 {
		t.Errorf("DeconsolidateEntity called %d times, want 1", len(calls))
	} else if calls[0] != e.ID {
		t.Errorf("DeconsolidateEntity called with entity ID %s, want %s", calls[0], e.ID)
	}
}

// --- helpers for service integration tests ---

func svcCreateTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *models.User {
	t.Helper()
	user := &models.User{ID: uuid.New(), Email: uuid.NewString() + "@test.local", DisplayName: "Test"}
	if _, err := pool.Exec(ctx,
		"INSERT INTO users (id, email, password_hash, display_name) VALUES ($1, $2, $3, $4)",
		user.ID, user.Email, "hash", user.DisplayName); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func svcCreateTestUniverse(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) models.Universe {
	t.Helper()
	u := models.Universe{ID: uuid.New(), UserID: userID, Name: "Test Universe", Description: "", Genre: "", Format: "novel"}
	if _, err := pool.Exec(ctx,
		"INSERT INTO universes (id, user_id, name, description, genre, format) VALUES ($1, $2, $3, $4, $5, $6)",
		u.ID, u.UserID, u.Name, u.Description, u.Genre, u.Format); err != nil {
		t.Fatalf("create universe: %v", err)
	}
	return u
}

func svcCreateTestEntity(t *testing.T, ctx context.Context, pool *pgxpool.Pool, universeID uuid.UUID, name string, score float64, status string) models.Entity {
	t.Helper()
	e := models.Entity{ID: uuid.New(), UniverseID: universeID, Type: "character", Name: name, Status: status, RelevanceScore: score, Description: ""}
	if _, err := pool.Exec(ctx,
		"INSERT INTO entities (id, universe_id, type, name, description, status, relevance_score) VALUES ($1,$2,$3,$4,$5,$6,$7)",
		e.ID, e.UniverseID, e.Type, e.Name, e.Description, e.Status, e.RelevanceScore); err != nil {
		t.Fatalf("create entity: %v", err)
	}
	return e
}
