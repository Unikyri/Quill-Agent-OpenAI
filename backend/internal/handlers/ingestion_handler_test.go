package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// mockIngestionSvc returns a fixed job ID and records calls.
type mockIngestionSvc struct {
	jobID uuid.UUID
	err   error
}

func (m *mockIngestionSvc) Start(ctx context.Context, universeID, workID uuid.UUID, reader io.Reader, filename string) (uuid.UUID, error) {
	return m.jobID, m.err
}

// TestIngestionHandlerPost verifies:
// - 202 Accepted response
// - job_id in response body
// - invalid universe ID returns 400
func TestIngestionHandlerPost(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	jobID := uuid.New()
	mockSvc := &mockIngestionSvc{jobID: jobID}
	h := &IngestionHandler{ingestionSvc: mockSvc}
	app.Post("/api/v1/universes/:id/ingest", h.Ingest)

	// Build multipart form with a file
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "document.md")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("# Chapter 1\nTest content."))
	w.Close()

	universeID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+universeID.String()+"/ingest", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["job_id"] != jobID.String() {
		t.Errorf("job_id: got %q, want %q", body["job_id"], jobID.String())
	}
	if body["status"] != "accepted" {
		t.Errorf("status: got %q, want %q", body["status"], "accepted")
	}
}

// TestIngestionHandlerInvalidID verifies 400 for bad UUID.
func TestIngestionHandlerInvalidID(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := &IngestionHandler{}
	app.Post("/api/v1/universes/:id/ingest", h.Ingest)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/not-a-uuid/ingest", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

// TestIngestionHandlerNoFile verifies 400 when no file is attached.
func TestIngestionHandlerNoFile(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})

	h := &IngestionHandler{}
	app.Post("/api/v1/universes/:id/ingest", h.Ingest)

	universeID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+universeID.String()+"/ingest", nil)
	// No multipart form at all
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for no file, got %d", resp.StatusCode)
	}
}
