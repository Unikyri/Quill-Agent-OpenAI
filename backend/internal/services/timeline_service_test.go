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

// TestTimelineServiceValidateFuture rejects events whose chapter is ahead of the present chapter.
func TestTimelineServiceValidateFuture(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "010")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	work, ch1, ch2 := svcCreateWorkAndChapters(t, ctx, pool, universe, 2)

	svc := NewTimelineService(pool, repositories.NewTimelineRepo(pool), nil)

	// event tied to chapter 2, but we're validating against chapter 1
	event := models.TimelineEvent{
		ID:         uuid.New(),
		UniverseID: universe.ID,
		Title:      "Battle of the Fists",
		ChapterID:  &ch2.ID,
	}
	_ = work

	err := svc.ValidatePosition(ctx, event, ch1.ID)
	if err == nil {
		t.Error("expected error for future event (ch2 > ch1), got nil")
	}
}

// TestTimelineServiceValidatePresent accepts events in the same chapter.
func TestTimelineServiceValidatePresent(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "010")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	_, ch1, _ := svcCreateWorkAndChapters(t, ctx, pool, universe, 2)

	svc := NewTimelineService(pool, repositories.NewTimelineRepo(pool), nil)

	event := models.TimelineEvent{
		ID:         uuid.New(),
		UniverseID: universe.ID,
		Title:      "Present Event",
		ChapterID:  &ch1.ID,
	}

	if err := svc.ValidatePosition(ctx, event, ch1.ID); err != nil {
		t.Errorf("expected no error for same-chapter event, got: %v", err)
	}
}

// TestTimelineServiceValidatePast accepts events from earlier chapters.
func TestTimelineServiceValidatePast(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "010")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	_, ch1, ch2 := svcCreateWorkAndChapters(t, ctx, pool, universe, 2)

	svc := NewTimelineService(pool, repositories.NewTimelineRepo(pool), nil)

	event := models.TimelineEvent{
		ID:         uuid.New(),
		UniverseID: universe.ID,
		Title:      "Past Event",
		ChapterID:  &ch1.ID, // chapter 1
	}

	if err := svc.ValidatePosition(ctx, event, ch2.ID); err != nil {
		t.Errorf("expected no error for past event (ch1 < ch2), got: %v", err)
	}
}

// TestTimelineServiceValidateNoChapter accepts events without a chapter reference.
func TestTimelineServiceValidateNoChapter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "010")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	_, ch1, _ := svcCreateWorkAndChapters(t, ctx, pool, universe, 1)

	svc := NewTimelineService(pool, repositories.NewTimelineRepo(pool), nil)

	event := models.TimelineEvent{
		ID:         uuid.New(),
		UniverseID: universe.ID,
		Title:      "Timeless Event",
		ChapterID:  nil, // no chapter tie
	}

	if err := svc.ValidatePosition(ctx, event, ch1.ID); err != nil {
		t.Errorf("expected no error for event without chapter, got: %v", err)
	}
}

// helpers

// TestTimelineServiceValidateAmbiguousAgentFallback verifies that when
// eventOrder == presentOrder (ambiguous case), the agent is called for
// temporal reasoning instead of blindly accepting.
//
// RED: Needs NewTimelineService with qwenSvc param (task 3.3).
func TestTimelineServiceValidateAmbiguousAgentFallback(t *testing.T) {
	// Mock Qwen server: agent says position IS consistent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "CONSISTENT",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	})
	qwenSvc.client.Timeout = 5 * time.Second

	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "010")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	_, ch1, _ := svcCreateWorkAndChapters(t, ctx, pool, universe, 2)

	svc := NewTimelineService(pool, repositories.NewTimelineRepo(pool), qwenSvc)

	event := models.TimelineEvent{
		ID:         uuid.New(),
		UniverseID: universe.ID,
		Title:      "Ambiguous Event",
		ChapterID:  &ch1.ID,
	}

	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := svc.ValidatePosition(ctx2, event, ch1.ID)
	if err != nil {
		t.Errorf("expected no error for agent-validated same-chapter event, got: %v", err)
	}
}

// TestTimelineServiceValidateAmbiguousAgentReject verifies that when the
// agent says the position is INCONSISTENT, ValidatePosition returns an error.
//
// RED: Needs task 3.3 implementation.
func TestTimelineServiceValidateAmbiguousAgentReject(t *testing.T) {
	// Mock Qwen server: agent says position is INCONSISTENT
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "INCONSISTENT: Event references events that haven't happened yet",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	})
	qwenSvc.client.Timeout = 5 * time.Second

	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "010")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	_, ch1, _ := svcCreateWorkAndChapters(t, ctx, pool, universe, 2)

	svc := NewTimelineService(pool, repositories.NewTimelineRepo(pool), qwenSvc)

	event := models.TimelineEvent{
		ID:         uuid.New(),
		UniverseID: universe.ID,
		Title:      "Inconsistent Event",
		ChapterID:  &ch1.ID,
	}

	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := svc.ValidatePosition(ctx2, event, ch1.ID)
	if err == nil {
		t.Error("expected error for agent-rejected timeline position, got nil")
	}
}

func svcCreateWorkAndChapters(t *testing.T, ctx context.Context, pool *pgxpool.Pool, universe models.Universe, n int) (models.Work, models.Chapter, models.Chapter) {
	t.Helper()
	w := models.Work{ID: uuid.New(), UniverseID: universe.ID, Title: "Test Work", Type: "book", OrderIndex: 1, Status: "draft"}
	if _, err := pool.Exec(ctx, "INSERT INTO works (id, universe_id, title, type, order_index, status) VALUES ($1,$2,$3,$4,$5,$6)",
		w.ID, w.UniverseID, w.Title, w.Type, w.OrderIndex, w.Status); err != nil {
		t.Fatalf("create work: %v", err)
	}

	chapters := make([]models.Chapter, n)
	for i := 0; i < n; i++ {
		ch := models.Chapter{ID: uuid.New(), WorkID: w.ID, Title: "Chapter " + string(rune('A'+i)), OrderIndex: i + 1, Status: "draft"}
		if _, err := pool.Exec(ctx, "INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status) VALUES ($1,$2,$3,$4,'','',0,$5)",
			ch.ID, ch.WorkID, ch.Title, ch.OrderIndex, ch.Status); err != nil {
			t.Fatalf("create chapter: %v", err)
		}
		chapters[i] = ch
	}
	if n >= 2 {
		return w, chapters[0], chapters[1]
	}
	return w, chapters[0], models.Chapter{}
}
