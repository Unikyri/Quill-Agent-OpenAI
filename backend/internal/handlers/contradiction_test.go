package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ── ContradictionsHandler tests ──

// TestContradictionsHandlerList validates the ListByUniverse endpoint.
func TestContradictionsHandlerList(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := NewContradictionHandler(nil, nil)
	app.Get("/api/v1/universes/:universe_id/contradictions", h.ListByUniverse)

	universeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/"+universeID.String()+"/contradictions", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	// Without services, handler should return an error
	if resp.StatusCode < 400 {
		t.Errorf("expected error status, got %d", resp.StatusCode)
	}
}

// TestContradictionsHandlerListInvalidID validates error on bad UUID.
func TestContradictionsHandlerListInvalidID(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := NewContradictionHandler(nil, nil)
	app.Get("/api/v1/universes/:universe_id/contradictions", h.ListByUniverse)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/not-a-uuid/contradictions", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID, got %d", resp.StatusCode)
	}
}

// TestContradictionsHandlerResolve validates the Resolve endpoint.
func TestContradictionsHandlerResolve(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := NewContradictionHandler(nil, nil)
	app.Put("/api/v1/universes/:universe_id/contradictions/:id/resolve", h.Resolve)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/universes/"+uuid.New().String()+"/contradictions/"+uuid.New().String()+"/resolve", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode < 400 {
		t.Errorf("expected error status, got %d", resp.StatusCode)
	}
}

// TestContradictionsHandlerResolveInvalidID validates error on bad UUID.
func TestContradictionsHandlerResolveInvalidID(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := NewContradictionHandler(nil, nil)
	app.Put("/api/v1/universes/:universe_id/contradictions/:id/resolve", h.Resolve)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/universes/"+uuid.New().String()+"/contradictions/not-a-uuid/resolve", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID, got %d", resp.StatusCode)
	}
}

// TestNewContradictionHandler ensures handler creation doesn't panic.
func TestNewContradictionHandler(t *testing.T) {
	h := NewContradictionHandler(nil, nil)
	if h == nil {
		t.Fatal("NewContradictionHandler returned nil")
	}
}


