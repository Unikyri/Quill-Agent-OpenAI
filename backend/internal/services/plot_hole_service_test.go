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

func TestPlotHoleServiceScanDetectsStaleArc(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	chapters := svcCreateChapters(t, ctx, pool, universe, 10)
	ch1 := chapters[0]   // order_index=1
	ch10 := chapters[9]  // order_index=10

	// Create entity with last_mentioned at chapter 1 (score > 0.5 to pass relevance filter)
	entity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Forgotten Hero", 0.8, "active")
	// Update last_mentioned_chapter_id to ch1
	if _, err := pool.Exec(ctx, "UPDATE entities SET last_mentioned_chapter_id = $1, last_mentioned_at = NOW() WHERE id = $2",
		ch1.ID, entity.ID); err != nil {
		t.Fatalf("set last_mentioned: %v", err)
	}

	svc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), repositories.NewEntityRepo(pool), 8, nil, nil)

	holes, err := svc.Scan(ctx, universe.ID, ch10.ID)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(holes) == 0 {
		t.Error("expected at least 1 plot hole for stale entity (gap=9 ≥ 8)")
	}
	for _, h := range holes {
		if h.Status != "open" {
			t.Errorf("plot hole %s status = %s, want open", h.Title, h.Status)
		}
	}
}

// TestPlotHoleServiceScanNoGap ensures entities within threshold don't trigger.
func TestPlotHoleServiceScanNoGap(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	chapters := svcCreateChapters(t, ctx, pool, universe, 5)
	ch1 := chapters[0]
	ch5 := chapters[4]

	entity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Current Hero", 0.6, "active")
	if _, err := pool.Exec(ctx, "UPDATE entities SET last_mentioned_chapter_id = $1, last_mentioned_at = NOW() WHERE id = $2",
		ch1.ID, entity.ID); err != nil {
		t.Fatalf("set last_mentioned: %v", err)
	}

	svc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), repositories.NewEntityRepo(pool), 8, nil, nil)

	holes, err := svc.Scan(ctx, universe.ID, ch5.ID)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(holes) != 0 {
		t.Errorf("expected 0 plot holes (gap=4 < 8), got %d", len(holes))
	}
}

// TestPlotHoleServiceScanMixed has both stale and current entities.
func TestPlotHoleServiceScanMixed(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	chapters := svcCreateChapters(t, ctx, pool, universe, 10)
	ch1 := chapters[0]
	ch8 := chapters[7]  // order_index=8
	ch10 := chapters[9] // order_index=10

	// Stale entity: last mentioned at chapter 1, scan at chapter 10 → gap 9
	stale := svcCreateTestEntity(t, ctx, pool, universe.ID, "Stale Arc", 0.8, "active")
	pool.Exec(ctx, "UPDATE entities SET last_mentioned_chapter_id = $1, last_mentioned_at = NOW() WHERE id = $2", ch1.ID, stale.ID)

	// Recent entity: last mentioned at chapter 8, scan at chapter 10 → gap 2
	recent := svcCreateTestEntity(t, ctx, pool, universe.ID, "Recent Arc", 0.6, "active")
	pool.Exec(ctx, "UPDATE entities SET last_mentioned_chapter_id = $1, last_mentioned_at = NOW() WHERE id = $2", ch8.ID, recent.ID)

	svc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), repositories.NewEntityRepo(pool), 8, nil, nil)

	holes, err := svc.Scan(ctx, universe.ID, ch10.ID)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(holes) != 1 {
		t.Errorf("expected 1 plot hole, got %d", len(holes))
	}
}

// TestPlotHoleServiceScanNoLastMentioned skips entities without last_mentioned data.
func TestPlotHoleServiceScanNoLastMentioned(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	chapters := svcCreateChapters(t, ctx, pool, universe, 3)
	ch3 := chapters[2]

	// Entity with NULL last_mentioned — never mentioned
	svcCreateTestEntity(t, ctx, pool, universe.ID, "Never Mentioned", 0.8, "active")

	svc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), repositories.NewEntityRepo(pool), 8, nil, nil)

	holes, err := svc.Scan(ctx, universe.ID, ch3.ID)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(holes) != 0 {
		t.Errorf("expected 0 plot holes (null last_mentioned should be skipped), got %d", len(holes))
	}
}

// TestPlotHoleServiceScanRelevanceFilter verifies that entities with
// relevance_score <= 0.5 are skipped during the scan.
//
// RED: NewPlotHoleService with qwenSvc/executor params will fail compilation
// until task 3.1 is implemented. The relevance_score filter needs 3.2.
func TestPlotHoleServiceScanRelevanceFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	chapters := svcCreateChapters(t, ctx, pool, universe, 10)
	ch1 := chapters[0]  // order_index=1
	ch10 := chapters[9] // order_index=10

	// Entity with low relevance (<= 0.5) — should be skipped by filter
	lowEntity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Low Relevance Arc", 0.3, "active")
	if _, err := pool.Exec(ctx, "UPDATE entities SET last_mentioned_chapter_id = $1, last_mentioned_at = NOW() WHERE id = $2",
		ch1.ID, lowEntity.ID); err != nil {
		t.Fatalf("set last_mentioned: %v", err)
	}

	svc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), repositories.NewEntityRepo(pool), 8, nil, nil)

	holes, err := svc.Scan(ctx, universe.ID, ch10.ID)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(holes) != 0 {
		t.Errorf("expected 0 plot holes (relevance_score 0.3 <= 0.5 should be filtered), got %d", len(holes))
	}
}

// TestPlotHoleServiceScanAgentVerdict verifies that for stale entities
// with relevance_score > 0.5, the agent is consulted and its verdict
// determines whether a plot hole is created.
//
// RED: Needs NewPlotHoleService with qwenSvc, executor params (task 3.1)
// and agent evaluation in Scan (task 3.2).
func TestPlotHoleServiceScanAgentVerdict(t *testing.T) {
	// Mock Qwen server: agent says "this IS a plot hole"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "YES",
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
	testutil.RunMigrationsUpTo(t, pool, "011")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	chapters := svcCreateChapters(t, ctx, pool, universe, 10)
	ch1 := chapters[0]  // order_index=1
	ch10 := chapters[9] // order_index=10

	// High relevance, stale entity (gap 9 ≥ 8) — agent should be consulted
	staleEntity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Stale Hero", 0.9, "active")
	if _, err := pool.Exec(ctx, "UPDATE entities SET last_mentioned_chapter_id = $1, last_mentioned_at = NOW() WHERE id = $2",
		ch1.ID, staleEntity.ID); err != nil {
		t.Fatalf("set last_mentioned: %v", err)
	}

	svc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), repositories.NewEntityRepo(pool), 8, qwenSvc, nil)

	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	holes, err := svc.Scan(ctx2, universe.ID, ch10.ID)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Agent said YES → plot hole should be created
	if len(holes) != 1 {
		t.Fatalf("expected 1 plot hole (agent verdict=YES), got %d", len(holes))
	}
	if holes[0].Status != "open" {
		t.Errorf("plot hole status = %s, want open", holes[0].Status)
	}
}

// TestPlotHoleServiceScanAgentNoVerdict verifies that when the agent says
// the arc is naturally concluded (NOT a plot hole), no plot hole is created.
//
// RED: Needs task 3.1 + 3.2 implementation.
func TestPlotHoleServiceScanAgentNoVerdict(t *testing.T) {
	// Mock Qwen server: agent says "this is NOT a plot hole"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "NO — arc is naturally concluded",
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
	testutil.RunMigrationsUpTo(t, pool, "011")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	chapters := svcCreateChapters(t, ctx, pool, universe, 10)
	ch1 := chapters[0]
	ch10 := chapters[9]

	staleEntity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Naturally Concluded Arc", 0.9, "active")
	pool.Exec(ctx, "UPDATE entities SET last_mentioned_chapter_id = $1, last_mentioned_at = NOW() WHERE id = $2",
		ch1.ID, staleEntity.ID)

	svc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), repositories.NewEntityRepo(pool), 8, qwenSvc, nil)

	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	holes, err := svc.Scan(ctx2, universe.ID, ch10.ID)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Agent said NO → no plot hole
	if len(holes) != 0 {
		t.Errorf("expected 0 plot holes (agent verdict=NO), got %d", len(holes))
	}
}

// svcCreateChapters creates a work + N chapters and returns the chapters.
func svcCreateChapters(t *testing.T, ctx context.Context, pool *pgxpool.Pool, universe models.Universe, n int) []models.Chapter {
	t.Helper()
	w := models.Work{ID: uuid.New(), UniverseID: universe.ID, Title: "Test Work", Type: "book", OrderIndex: 1, Status: "draft"}
	if _, err := pool.Exec(ctx, "INSERT INTO works (id, universe_id, title, type, order_index, status) VALUES ($1,$2,$3,$4,$5,$6)",
		w.ID, w.UniverseID, w.Title, w.Type, w.OrderIndex, w.Status); err != nil {
		t.Fatalf("create work: %v", err)
	}

	chapters := make([]models.Chapter, n)
	for i := 0; i < n; i++ {
		ch := models.Chapter{ID: uuid.New(), WorkID: w.ID, Title: "Ch", OrderIndex: i + 1, Status: "draft"}
		if _, err := pool.Exec(ctx, "INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status) VALUES ($1,$2,$3,$4,'','',0,$5)",
			ch.ID, ch.WorkID, ch.Title, ch.OrderIndex, ch.Status); err != nil {
			t.Fatalf("create chapter: %v", err)
		}
		chapters[i] = ch
	}
	return chapters
}
