package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ── GraphHandler tests ──

func TestGraphHandlerFullGraph(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := NewGraphHandler(nil, nil, nil)
	app.Get("/api/v1/universes/:universe_id/graph", h.FullGraph)

	universeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/"+universeID.String()+"/graph", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode < 400 {
		t.Errorf("expected error status, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerFullGraphInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(nil, nil, nil)
	app.Get("/api/v1/universes/:universe_id/graph", h.FullGraph)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/bad/graph", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerNeighbors(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(nil, nil, nil)
	app.Get("/api/v1/entities/:id/neighbors", h.Neighbors)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities/"+uuid.New().String()+"/neighbors?universe_id="+uuid.New().String(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode < 400 {
		t.Errorf("expected error status, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerNeighborsInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(nil, nil, nil)
	app.Get("/api/v1/entities/:id/neighbors", h.Neighbors)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities/bad/neighbors?universe_id="+uuid.New().String(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerRecall(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(nil, nil, nil)
	app.Post("/api/v1/universes/:id/recall", h.Recall)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+uuid.New().String()+"/recall", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode < 400 {
		t.Errorf("expected error status, got %d", resp.StatusCode)
	}
}

func TestNewGraphHandler(t *testing.T) {
	h := NewGraphHandler(nil, nil, nil)
	if h == nil {
		t.Fatal("NewGraphHandler returned nil")
	}
}
