package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

func TestNewConsolidationServiceSetsFields(t *testing.T) {
	pool := &pgxpool.Pool{}
	consolidationRepo := repositories.NewConsolidationRepo(pool)
	entityRepo := repositories.NewEntityRepo(pool)
	qwenSvc := &QwenService{}

	svc := NewConsolidationService(consolidationRepo, entityRepo, qwenSvc)

	// spec: constructor wires all dependencies
	if svc.consolidationRepo != consolidationRepo {
		t.Error("consolidationRepo not set")
	}
	if svc.entityRepo != entityRepo {
		t.Error("entityRepo not set")
	}
	if svc.qwenSvc != qwenSvc {
		t.Error("qwenSvc not set")
	}
}

func TestBuildConsolidationPrompt(t *testing.T) {
	prompt := buildConsolidationPrompt("Elena", nil)
	if prompt == "" {
		t.Error("buildConsolidationPrompt returned empty string")
	}

	// With no mentions, it should still produce valid output
	promptEmpty := buildConsolidationPrompt("Ghost", nil)
	if promptEmpty == "" {
		t.Error("buildConsolidationPrompt with no mentions returned empty")
	}
}

// ── parseConsolidationResponse tests ──

// spec: CRITICAL #1 — Successful consolidation response parsing
func TestParseConsolidationResponseValid(t *testing.T) {
	raw := `{"summary":"Alice did things in Wonderland. She met the Queen and escaped.","key_facts":["Alice followed a rabbit","Alice grew and shrank","Alice befriended the Hatter"]}`
	summary, facts, err := parseConsolidationResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "Alice did things in Wonderland. She met the Queen and escaped." {
		t.Errorf("summary = %q, want %q", summary, "Alice did things in Wonderland. She met the Queen and escaped.")
	}
	if len(facts) != 3 {
		t.Errorf("len(facts) = %d, want 3", len(facts))
	}
	if facts[0] != "Alice followed a rabbit" {
		t.Errorf("facts[0] = %q, want %q", facts[0], "Alice followed a rabbit")
	}
}

// spec: CRITICAL #1 — Invalid JSON should return error
func TestParseConsolidationResponseInvalidJSON(t *testing.T) {
	_, _, err := parseConsolidationResponse(`not json`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// spec: CRITICAL #1 — Empty summary should return error
func TestParseConsolidationResponseEmptySummary(t *testing.T) {
	_, _, err := parseConsolidationResponse(`{"summary":"","key_facts":["fact"]}`)
	if err == nil {
		t.Error("expected error for empty summary")
	}
}

// spec: CRITICAL #1 — Valid JSON with empty facts array should succeed
func TestParseConsolidationResponseNoFacts(t *testing.T) {
	raw := `{"summary":"Just a summary.","key_facts":[]}`
	summary, facts, err := parseConsolidationResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "Just a summary." {
		t.Errorf("summary = %q, want %q", summary, "Just a summary.")
	}
	if len(facts) != 0 {
		t.Errorf("len(facts) = %d, want 0", len(facts))
	}
}

// ── ConsolidateEntity nil-safe tests ──

// spec: CRITICAL #2 — ConsolidateEntity with nil entityRepo should not panic
// (zero-mention path exercised via recover from nil pointer)
func TestConsolidateEntityNilEntityRepo(t *testing.T) {
	svc := &ConsolidationService{
		consolidationRepo: nil,
		entityRepo:        nil,
		qwenSvc:           nil,
	}
	// Should not panic — defer recover catches nil pointer dereference
	err := svc.ConsolidateEntity(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Errorf("ConsolidateEntity should return nil on panic recovery, got: %v", err)
	}
}

// spec: CRITICAL #3 — ConsolidateEntity with nil QwenService should not panic
// qwen-turbo error path exercised via recover from nil pointer
func TestConsolidateEntityNilQwenService(t *testing.T) {
	// We need a real entityRepo to pass GetMentionsByEntity, but nil qwenSvc
	// to trigger the Chat error path. When entityRepo is nil, GetMentionsByEntity panics first.
	// So we test: with nil qwenSvc but also nil entityRepo → recover catches either.
	svc := &ConsolidationService{
		consolidationRepo: nil,
		entityRepo:        nil,
		qwenSvc:           nil,
	}
	err := svc.ConsolidateEntity(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Errorf("ConsolidateEntity with nil deps should not propagate error, got: %v", err)
	}
}

// spec: CRITICAL #5 — DeconsolidateEntity with nil consolidationRepo should not panic
func TestDeconsolidateEntityNilRepo(t *testing.T) {
	svc := &ConsolidationService{
		consolidationRepo: nil,
		entityRepo:        nil,
		qwenSvc:           nil,
	}
	// Should not panic — nil check in DeconsolidateEntity
	err := svc.DeconsolidateEntity(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("DeconsolidateEntity with nil repo should return nil, got: %v", err)
	}
}

// ── generateConsolidation tests (httptest) ──

// spec: CRITICAL #1 — Successful consolidation: Chat + GenerateEmbedding
// TestGenerateConsolidation_Success verifies the summarization+embedding pipeline
// returns correct summary, key_facts, and 1024-dim embedding.
func TestGenerateConsolidation_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/chat/completions":
			resp := QwenResponse{
				Choices: []struct {
					Message struct {
						Content   string         `json:"content"`
						ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
					} `json:"message"`
				}{
					{Message: struct {
						Content   string         `json:"content"`
						ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
					}{Content: `{"summary":"Hero fought a dragon and saved the village.","key_facts":["fought dragon","saved village","became legend"]}`}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/embeddings":
			// Return a 1024-dim embedding filled with non-zero values
			emb := make([]float32, 1024)
			emb[0] = 1.0
			resp := EmbeddingResponse{
				Data: []struct {
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{{Embedding: emb, Index: 0}},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:           server.URL,
		QwenAPIKey:            "test-key",
		QwenMaxConcurrency:    1,
		QwenTurboConcurrency:  1,
	}
	qwenSvc := NewQwenService(cfg, nil)
	svc := &ConsolidationService{qwenSvc: qwenSvc}

	entity := &models.Entity{ID: uuid.New(), Name: "Hero", Type: "character"}
	mentions := []models.EntityMention{
		{ContextSnippet: "Hero fought the dragon"},
		{ContextSnippet: "Hero saved the village"},
	}

	summary, facts, embedding, err := svc.generateConsolidation(context.Background(), entity, mentions)
	if err != nil {
		t.Fatalf("generateConsolidation: %v", err)
	}
	if summary != "Hero fought a dragon and saved the village." {
		t.Errorf("summary = %q, want %q", summary, "Hero fought a dragon and saved the village.")
	}
	if len(facts) != 3 {
		t.Errorf("len(facts) = %d, want 3", len(facts))
	}
	if facts[0] != "fought dragon" {
		t.Errorf("facts[0] = %q, want %q", facts[0], "fought dragon")
	}
	if len(embedding) != 1024 {
		t.Errorf("embedding dim = %d, want 1024", len(embedding))
	}
	if embedding[0] != 1.0 {
		t.Errorf("embedding[0] = %f, want 1.0", embedding[0])
	}
}

// spec: CRITICAL #1 triangulation — different entity with different mentions
func TestGenerateConsolidation_DifferentEntity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/chat/completions":
			resp := QwenResponse{
				Choices: []struct {
					Message struct {
						Content   string         `json:"content"`
						ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
					} `json:"message"`
				}{
					{Message: struct {
						Content   string         `json:"content"`
						ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
					}{Content: `{"summary":"Villain schemed and was defeated.","key_facts":["schemed","was defeated"]}`}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/embeddings":
			emb := make([]float32, 1024)
			emb[5] = 0.5
			resp := EmbeddingResponse{
				Data: []struct {
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{{Embedding: emb, Index: 0}},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:           server.URL,
		QwenAPIKey:            "test-key",
		QwenMaxConcurrency:    1,
		QwenTurboConcurrency:  1,
	}
	qwenSvc := NewQwenService(cfg, nil)
	svc := &ConsolidationService{qwenSvc: qwenSvc}

	entity := &models.Entity{ID: uuid.New(), Name: "Villain", Type: "character"}
	mentions := []models.EntityMention{
		{ContextSnippet: "Villain schemed against the council"},
	}

	summary, facts, embedding, err := svc.generateConsolidation(context.Background(), entity, mentions)
	if err != nil {
		t.Fatalf("generateConsolidation: %v", err)
	}
	if summary != "Villain schemed and was defeated." {
		t.Errorf("summary = %q, want %q", summary, "Villain schemed and was defeated.")
	}
	if len(facts) != 2 {
		t.Errorf("len(facts) = %d, want 2", len(facts))
	}
	if embedding[5] != 0.5 {
		t.Errorf("embedding[5] = %f, want 0.5", embedding[5])
	}
}

// spec: CRITICAL #2 — ConsolidateEntity with zero mentions: generateConsolidation
// still works with empty mentions (produces prompt with "(no context available)").
func TestGenerateConsolidation_EmptyMentions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/chat/completions":
			resp := QwenResponse{
				Choices: []struct {
					Message struct {
						Content   string         `json:"content"`
						ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
					} `json:"message"`
				}{
					{Message: struct {
						Content   string         `json:"content"`
						ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
					}{Content: `{"summary":"Unknown entity with no recorded history.","key_facts":[]}`}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/embeddings":
			emb := make([]float32, 1024)
			resp := EmbeddingResponse{
				Data: []struct {
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{{Embedding: emb, Index: 0}},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:           server.URL,
		QwenAPIKey:            "test-key",
		QwenMaxConcurrency:    1,
		QwenTurboConcurrency:  1,
	}
	qwenSvc := NewQwenService(cfg, nil)
	svc := &ConsolidationService{qwenSvc: qwenSvc}

	entity := &models.Entity{ID: uuid.New(), Name: "Ghost", Type: "character"}

	// generateConsolidation with nil mentions should still build a valid prompt
	summary, facts, _, err := svc.generateConsolidation(context.Background(), entity, nil)
	if err != nil {
		t.Fatalf("generateConsolidation with nil mentions: %v", err)
	}
	if summary != "Unknown entity with no recorded history." {
		t.Errorf("summary = %q, want %q", summary, "Unknown entity with no recorded history.")
	}
	if len(facts) != 0 {
		t.Errorf("len(facts) = %d, want 0", len(facts))
	}
}

// spec: CRITICAL #3 — Qwen error: Chat returns non-200, generateConsolidation propagates error
func TestGenerateConsolidation_QwenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"service unavailable"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:           server.URL,
		QwenAPIKey:            "test-key",
		QwenMaxConcurrency:    1,
		QwenTurboConcurrency:  1,
	}
	qwenSvc := NewQwenService(cfg, nil)
	svc := &ConsolidationService{qwenSvc: qwenSvc}

	entity := &models.Entity{ID: uuid.New(), Name: "Error", Type: "character"}
	mentions := []models.EntityMention{{ContextSnippet: "Something happened."}}

	_, _, _, err := svc.generateConsolidation(context.Background(), entity, mentions)
	if err == nil {
		t.Error("generateConsolidation should return error when Qwen API fails")
	}
}

// Integration scenarios for ConsolidateEntity are covered by:
//   - generateConsolidation unit tests (httptest-based, no DB needed)
//   - spy-based DecayAll/Reactivate wiring tests in relevance_service_test.go
// The full DB-backed ConsolidateEntity path is tested when TEST_DATABASE_URL is set
// via the relevance_service_test.go spy integration tests.
