package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ── TimelineHandler tests ──

func TestTimelineHandlerList(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := NewTimelineHandler(nil, nil)
	app.Get("/api/v1/universes/:universe_id/timeline", h.ListByUniverse)

	universeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/"+universeID.String()+"/timeline", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode < 400 {
		t.Errorf("expected error status, got %d", resp.StatusCode)
	}
}

func TestTimelineHandlerListInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewTimelineHandler(nil, nil)
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
	h := NewTimelineHandler(nil, nil)
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

func TestNewTimelineHandler(t *testing.T) {
	h := NewTimelineHandler(nil, nil)
	if h == nil {
		t.Fatal("NewTimelineHandler returned nil")
	}
}
