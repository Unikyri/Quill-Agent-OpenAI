package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

// ── GraphHandler tests ──

func TestGraphHandlerFullGraphInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
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

func TestGraphHandlerNeighborsInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
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
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
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

func TestGraphHandlerRecallExplainInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
	app.Post("/api/v1/universes/:id/recall/explain", h.RecallExplain)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/bad/recall/explain", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid universe id, got %d", resp.StatusCode)
	}
}

// fakeQueryEmbedder is a test double for queryEmbedder that records whether
// GenerateEmbedding was invoked and with what query text. It returns errSentinel
// so callers short-circuit at the embed step (500) instead of proceeding into
// MemoryService.RecallExplain, which would otherwise dereference the nil-pool
// repos these tests construct handlers with.
type fakeQueryEmbedder struct {
	called   bool
	gotQuery string
}

var errSentinelEmbed = fmt.Errorf("sentinel embed error")

func (f *fakeQueryEmbedder) GenerateEmbedding(_ context.Context, text string) ([]float32, error) {
	f.called = true
	f.gotQuery = text
	return nil, errSentinelEmbed
}

// TestGraphHandlerRecallExplainKClamp proves out-of-range K (500) is clamped
// rather than rejected with its own 400 — the request proceeds past
// validation to the embed step (observable via the sentinel 500, not 400).
func TestGraphHandlerRecallExplainKClamp(t *testing.T) {
	app := fiber.New()
	fake := &fakeQueryEmbedder{}
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), fake)
	app.Post("/api/v1/universes/:id/recall/explain", h.RecallExplain)

	body := strings.NewReader(`{"query":"who is the king","k":500}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+uuid.New().String()+"/recall/explain", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == http.StatusBadRequest {
		t.Errorf("expected K=500 to be clamped, not rejected as 400, got %d", resp.StatusCode)
	}
	if !fake.called {
		t.Fatal("expected the request to reach the embed step past K validation")
	}
}

// TestGraphHandlerRecallExplainEmbedderInvoked proves the handler embeds
// req.Query via the injected embedder for a non-empty query (spec: "the
// query MUST NOT be ignored").
func TestGraphHandlerRecallExplainEmbedderInvoked(t *testing.T) {
	app := fiber.New()
	fake := &fakeQueryEmbedder{}
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), fake)
	app.Post("/api/v1/universes/:id/recall/explain", h.RecallExplain)

	body := strings.NewReader(`{"query":"who is the king","k":5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+uuid.New().String()+"/recall/explain", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if !fake.called {
		t.Fatal("expected embedder.GenerateEmbedding to be invoked for a non-empty query")
	}
	if fake.gotQuery != "who is the king" {
		t.Errorf("expected embedder called with query %q, got %q", "who is the king", fake.gotQuery)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected embed failure to surface as 500, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerMemoryStatusInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
	app.Get("/api/v1/universes/:id/memory-status", h.MemoryStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/bad/memory-status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerNeighborsMissingGraph(t *testing.T) {
	app := fiber.New()

	stub := &stubGraphQuerier{errorMsg: `graph "universe_123e4567-e89b-12d3-a456-426614174000" does not exist`}
	h := &GraphHandler{
		graphRepo:  stub,
		memorySvc:  services.NewMemoryService(nil, nil, nil),
		entityRepo: repositories.NewEntityRepo(nil),
	}
	app.Get("/api/v1/entities/:id/neighbors", h.Neighbors)

	validID := "123e4567-e89b-12d3-a456-426614174000"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities/"+validID+"/neighbors?universe_id="+validID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for missing graph neighbors, got %d", resp.StatusCode)
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	nodes, ok := body["nodes"]
	if !ok {
		t.Fatal("response missing 'nodes'")
	}
	edges, ok := body["edges"]
	if !ok {
		t.Fatal("response missing 'edges'")
	}
	if string(nodes) != "[]" || string(edges) != "[]" {
		t.Errorf("expected empty arrays, got nodes=%s edges=%s", nodes, edges)
	}
}

func TestNewGraphHandler(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil graphRepo")
		}
	}()
	NewGraphHandler(nil, nil, nil, nil)
}

// ── stub graph querier for testing error paths ──

type stubGraphQuerier struct{ errorMsg string }

func (s *stubGraphQuerier) FullQuery(_ context.Context, _ string) ([]repositories.GraphNode, []repositories.GraphEdge, error) {
	return nil, nil, &stubQuerierErr{msg: s.errorMsg}
}
func (s *stubGraphQuerier) NHopTraversal(_ context.Context, _ string, _ string, _ int) ([]repositories.GraphNode, []repositories.GraphEdge, error) {
	return nil, nil, &stubQuerierErr{msg: s.errorMsg}
}

type stubQuerierErr struct{ msg string }

func (e *stubQuerierErr) Error() string { return e.msg }

// fakeDecayer is a test double for the Decayer interface.
type fakeDecayer struct {
	called bool
	gotID  uuid.UUID
	err    error
}

func (f *fakeDecayer) DecayAll(_ context.Context, universeID uuid.UUID) error {
	f.called = true
	f.gotID = universeID
	return f.err
}

func TestGraphHandlerRunDecayInvalidID(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
	app.Post("/api/v1/universes/:id/decay", h.RunDecay)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/bad/decay", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerRunDecaySuccess(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
	fake := &fakeDecayer{}
	h.SetDecayer(fake)
	app.Post("/api/v1/universes/:id/decay", h.RunDecay)

	universeID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+universeID.String()+"/decay", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if !fake.called {
		t.Fatal("expected DecayAll to be called")
	}
	if fake.gotID != universeID {
		t.Errorf("expected DecayAll called with %s, got %s", universeID, fake.gotID)
	}

	var body map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body["ok"] {
		t.Errorf(`expected {"ok":true}, got %v`, body)
	}
}

func TestGraphHandlerRunDecayNoDecayer(t *testing.T) {
	app := fiber.New()
	h := NewGraphHandler(repositories.NewGraphRepo(nil), services.NewMemoryService(nil, nil, nil), repositories.NewEntityRepo(nil), nil)
	app.Post("/api/v1/universes/:id/decay", h.RunDecay)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+uuid.New().String()+"/decay", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode < 500 {
		t.Errorf("expected 5xx when no decayer wired, got %d", resp.StatusCode)
	}
}

func TestGraphHandlerFullGraphMissingGraph(t *testing.T) {
	app := fiber.New()

	stub := &stubGraphQuerier{errorMsg: `graph "universe_123e4567-e89b-12d3-a456-426614174000" does not exist`}
	h := &GraphHandler{
		graphRepo:  stub,
		memorySvc:  services.NewMemoryService(nil, nil, nil),
		entityRepo: repositories.NewEntityRepo(nil),
	}
	app.Get("/api/v1/universes/:universe_id/graph", h.FullGraph)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/universes/123e4567-e89b-12d3-a456-426614174000/graph", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for missing graph, got %d", resp.StatusCode)
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	nodes, ok := body["nodes"]
	if !ok {
		t.Fatal("response missing 'nodes'")
	}
	edges, ok := body["edges"]
	if !ok {
		t.Fatal("response missing 'edges'")
	}
	if string(nodes) != "[]" || string(edges) != "[]" {
		t.Errorf("expected empty arrays, got nodes=%s edges=%s", nodes, edges)
	}
}
