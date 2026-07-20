package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
	"github.com/quill/backend/internal/testutil"
)

// ── TimelineHandler tests ──

func TestTimelineHandlerListInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewTimelineHandler(nil, repositories.NewTimelineRepo(nil))
	app.Get("/api/v1/universes/:universe_id/timeline", h.ListByUniverse)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/bad/timeline", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestTimelineHandlerCreate(t *testing.T) {
	app := fiber.New()
	h := NewTimelineHandler(nil, repositories.NewTimelineRepo(nil))
	app.Post("/api/v1/universes/:universe_id/timeline", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+uuid.New().String()+"/timeline", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode < 400 {
		t.Errorf("expected error on empty body, got %d", resp.StatusCode)
	}
}

// TestTimelineHandlerCreateSurfacesTimelineWarning proves the previously
// dead TimelineService agent path is now wired into the real create
// endpoint: an event landing on a work's latest (ambiguous) chapter gets a
// non-blocking "timeline_warning" in the response when the agent flags it,
// while the event is still created (advisory, not a hard rejection).
func TestTimelineHandlerCreateSurfacesTimelineWarning(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "010")
	ctx := context.Background()

	userID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, display_name) VALUES ($1,$2,$3,$4)",
		userID, uuid.NewString()+"@test.local", "hash", "Test"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	universeID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO universes (id, user_id, name, description, genre, format) VALUES ($1,$2,$3,$4,$5,$6)",
		universeID, userID, "Test Universe", "", "", "novel"); err != nil {
		t.Fatalf("create universe: %v", err)
	}
	workID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO works (id, universe_id, title, type, order_index, status) VALUES ($1,$2,$3,$4,$5,$6)",
		workID, universeID, "Test Work", "book", 1, "draft"); err != nil {
		t.Fatalf("create work: %v", err)
	}
	chapterID := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status) VALUES ($1,$2,$3,$4,'','',0,$5)",
		chapterID, workID, "Chapter A", 1, "draft"); err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"role": "assistant", "content": "INCONSISTENT: contradicts an earlier chapter"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	qwenSvc := services.NewQwenService(&config.Config{
		QwenBaseURL: srv.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1,
	}, nil)
	timelineSvc := services.NewTimelineService(pool, repositories.NewTimelineRepo(pool), qwenSvc, nil)
	h := NewTimelineHandler(timelineSvc, repositories.NewTimelineRepo(pool))

	app := fiber.New()
	app.Post("/api/v1/universes/:universe_id/timeline", h.Create)

	body, _ := json.Marshal(models.TimelineEvent{Title: "Present Event", ChapterID: &chapterID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+universeID.String()+"/timeline", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 (advisory, not blocking), got %d", resp.StatusCode)
	}

	var parsed map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	warning, _ := parsed["timeline_warning"].(string)
	if warning == "" {
		t.Fatal("expected a timeline_warning in the response, got none")
	}
	if _, ok := parsed["event"]; !ok {
		t.Error("expected the event to still be present in the response despite the warning")
	}
}

func TestNewTimelineHandler(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil timelineRepo")
		}
	}()
	NewTimelineHandler(nil, nil)
}
