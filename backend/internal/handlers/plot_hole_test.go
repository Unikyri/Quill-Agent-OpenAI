package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ── PlotHolesHandler tests ──

func TestPlotHolesHandlerList(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := NewPlotHoleHandler(nil)
	app.Get("/api/v1/universes/:universe_id/plot-holes", h.ListByUniverse)

	universeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/"+universeID.String()+"/plot-holes", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode < 400 {
		t.Errorf("expected error status, got %d", resp.StatusCode)
	}
}

func TestPlotHolesHandlerListInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewPlotHoleHandler(nil)
	app.Get("/api/v1/universes/:universe_id/plot-holes", h.ListByUniverse)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/bad/plot-holes", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestNewPlotHoleHandler(t *testing.T) {
	h := NewPlotHoleHandler(nil)
	if h == nil {
		t.Fatal("NewPlotHoleHandler returned nil")
	}
}
