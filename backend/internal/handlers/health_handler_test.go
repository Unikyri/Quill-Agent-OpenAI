package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/quill/backend/internal/testutil"
)

func TestHealthHandlerCheck(t *testing.T) {
	pool := testutil.SetupTestDB(t)

	app := fiber.New()
	// QwenService is not required for the fields returned by Check.
	h := NewHealthHandler(pool, nil)
	app.Get("/api/v1/health", h.Check)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	for _, key := range []string{"status", "db", "age", "pgvector", "qwen_api", "disk_free_mb", "uptime_seconds"} {
		if _, ok := result[key]; !ok {
			t.Errorf("missing key %q in health response", key)
		}
	}
}
