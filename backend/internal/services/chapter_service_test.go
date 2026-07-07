package services

import (
	"context"
	"math"
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

// TestChapterServiceCreateTriggersDecay is a regression test proving that
// ChapterService.Create wires into RelevanceService.DecayAll on the true
// "chapter advance" event. Distinct scenario from TestRelevanceServiceDecayAll
// (which seeds entities at 0.9/0.16): here a fresh entity starts at 1.0 and
// must land at ~e^-0.1 after a single chapter creation.
func TestChapterServiceCreateTriggersDecay(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	entity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Fresh Entity", 1.0, "active")

	entityRepo := repositories.NewEntityRepo(pool)
	chapterRepo := repositories.NewChapterRepo(pool)
	workRepo := repositories.NewWorkRepo(pool)
	relevSvc := NewRelevanceService(pool, entityRepo, 0.1, 0.15, nil)

	svc := NewChapterService(pool, chapterRepo, workRepo, relevSvc)

	if _, err := svc.Create(ctx, work.ID, models.CreateChapterRequest{Title: "Chapter One"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	found, err := entityRepo.FindByID(ctx, entity.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	expected := 1.0 * math.Exp(-0.1)
	if math.Abs(found.RelevanceScore-expected) > 0.001 {
		t.Errorf("RelevanceScore = %f, want ~%f (decay not triggered by chapter creation)", found.RelevanceScore, expected)
	}
}

// TestChapterServiceCreateFailureSkipsDecay triangulates the wiring: when the
// chapter-creation transaction fails to commit (here: FK violation on a
// nonexistent workID), DecayAll must NOT fire.
func TestChapterServiceCreateFailureSkipsDecay(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	entity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Untouched Entity", 1.0, "active")

	entityRepo := repositories.NewEntityRepo(pool)
	chapterRepo := repositories.NewChapterRepo(pool)
	workRepo := repositories.NewWorkRepo(pool)
	relevSvc := NewRelevanceService(pool, entityRepo, 0.1, 0.15, nil)

	svc := NewChapterService(pool, chapterRepo, workRepo, relevSvc)

	nonexistentWorkID := uuid.New()
	if _, err := svc.Create(ctx, nonexistentWorkID, models.CreateChapterRequest{Title: "Orphan Chapter"}); err == nil {
		t.Fatal("expected Create to fail for nonexistent workID (FK violation), got nil error")
	}

	found, err := entityRepo.FindByID(ctx, entity.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.RelevanceScore != 1.0 {
		t.Errorf("RelevanceScore = %f, want unchanged 1.0 (decay must not fire when Create fails)", found.RelevanceScore)
	}
}
