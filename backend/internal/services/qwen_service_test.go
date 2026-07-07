package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quill/backend/internal/config"
)

func TestQwenServiceChatResponseParsing(t *testing.T) {
	// Mock Qwen API that returns a valid chat completion response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		// Build QwenResponse using the exact anonymous struct shape
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
				}{Content: `{"summary":"test","key_facts":["a","b"]}`}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:           server.URL,
		QwenAPIKey:            "test-key",
		QwenMaxConcurrency:    1,
		QwenTurboConcurrency:  1,
	}
	svc := NewQwenService(cfg, nil)

	ctx := context.Background()
	result, err := svc.Chat(ctx, "qwen-turbo", []QwenMessage{
		{Role: "user", Content: "Summarize this"},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result == "" {
		t.Error("Chat returned empty result")
	}
	// spec: Chat returns the content string from choices[0].message.content
	expected := `{"summary":"test","key_facts":["a","b"]}`
	if result != expected {
		t.Errorf("Chat result = %q, want %q", result, expected)
	}
}

// captureRequestBody returns a handler that decodes the raw request body into
// QwenRequest (stashed via out) before responding with a minimal valid
// QwenResponse so the caller's parsing does not fail.
func captureRequestBody(t *testing.T, out *QwenRequest, content string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(out); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
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
				}{Content: content}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func TestExtractEntitiesSetsJSONResponseFormat(t *testing.T) {
	var captured QwenRequest
	server := httptest.NewServer(captureRequestBody(t, &captured, `{"characters":[],"places":[],"events":[],"factions":[],"world_rules":[],"plot_developments":[]}`))
	defer server.Close()

	cfg := &config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}
	svc := NewQwenService(cfg, nil)

	_, err := svc.ExtractEntities(context.Background(), "some text", "some context")
	if err != nil {
		t.Fatalf("ExtractEntities: %v", err)
	}
	if captured.ResponseFormat == nil || captured.ResponseFormat.Type != "json_object" {
		t.Errorf("ExtractEntities request ResponseFormat = %+v, want {Type: json_object}", captured.ResponseFormat)
	}
}

func TestAnalyzeRelationshipsSetsJSONResponseFormat(t *testing.T) {
	var captured QwenRequest
	server := httptest.NewServer(captureRequestBody(t, &captured, `[]`))
	defer server.Close()

	cfg := &config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}
	svc := NewQwenService(cfg, nil)

	_, err := svc.AnalyzeRelationships(context.Background(), "some text", []string{"A", "B"})
	if err != nil {
		t.Fatalf("AnalyzeRelationships: %v", err)
	}
	if captured.ResponseFormat == nil || captured.ResponseFormat.Type != "json_object" {
		t.Errorf("AnalyzeRelationships request ResponseFormat = %+v, want {Type: json_object}", captured.ResponseFormat)
	}
}

func TestCheckContradictionsSetsJSONResponseFormat(t *testing.T) {
	var captured QwenRequest
	server := httptest.NewServer(captureRequestBody(t, &captured, `[]`))
	defer server.Close()

	cfg := &config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}
	svc := NewQwenService(cfg, nil)

	candidates := []ContradictionCandidate{{Type: "semantic", EvidenceA: "a", EvidenceB: "b"}}
	_, err := svc.CheckContradictions(context.Background(), candidates)
	if err != nil {
		t.Fatalf("CheckContradictions: %v", err)
	}
	if captured.ResponseFormat == nil || captured.ResponseFormat.Type != "json_object" {
		t.Errorf("CheckContradictions request ResponseFormat = %+v, want {Type: json_object}", captured.ResponseFormat)
	}
}

func TestCheckContradictionsFallsBackOnMalformedJSON(t *testing.T) {
	server := httptest.NewServer(captureRequestBody(t, &QwenRequest{}, "not json at all"))
	defer server.Close()

	cfg := &config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}
	svc := NewQwenService(cfg, nil)

	candidates := []ContradictionCandidate{{Type: "semantic", EvidenceA: "a", EvidenceB: "b"}}
	_, err := svc.CheckContradictions(context.Background(), candidates)
	if err == nil {
		t.Error("CheckContradictions should return an error on malformed JSON, not panic or silently succeed")
	}
}

func TestNewQwenServiceSetsModelsFromConfig(t *testing.T) {
	cfg := &config.Config{
		QwenBaseURL:          "http://example.invalid",
		QwenAPIKey:           "test-key",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
		QwenMaxModel:         "custom-max",
		QwenTurboModel:       "custom-turbo",
		QwenEmbeddingModel:   "custom-embedding",
	}
	svc := NewQwenService(cfg, nil)

	if svc.maxModel != "custom-max" {
		t.Errorf("maxModel = %q, want %q", svc.maxModel, "custom-max")
	}
	if svc.turboModel != "custom-turbo" {
		t.Errorf("turboModel = %q, want %q", svc.turboModel, "custom-turbo")
	}
	if svc.embModel != "custom-embedding" {
		t.Errorf("embModel = %q, want %q", svc.embModel, "custom-embedding")
	}
}

// requestModelCases enumerates every QwenService method that issues a
// chat/embedding request, asserting the request's Model field is sourced
// from the configured field rather than a hardcoded literal. Configuring
// distinct sentinel model names and asserting on the captured request body
// is the regression guard against reintroducing hardcoded model strings.
func TestRequestsUseConfiguredModelsNotHardcodedLiterals(t *testing.T) {
	cfg := &config.Config{
		QwenAPIKey:           "test-key",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
		QwenMaxModel:         "sentinel-max",
		QwenTurboModel:       "sentinel-turbo",
		QwenEmbeddingModel:   "sentinel-embedding",
	}

	t.Run("ExtractEntities uses turboModel", func(t *testing.T) {
		var captured QwenRequest
		server := httptest.NewServer(captureRequestBody(t, &captured, `{"characters":[],"places":[],"events":[],"factions":[],"world_rules":[],"plot_developments":[]}`))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		svc := NewQwenService(&c, nil)
		if _, err := svc.ExtractEntities(context.Background(), "text", "context"); err != nil {
			t.Fatalf("ExtractEntities: %v", err)
		}
		if captured.Model != "sentinel-turbo" {
			t.Errorf("ExtractEntities request Model = %q, want %q", captured.Model, "sentinel-turbo")
		}
	})

	t.Run("AnalyzeRelationships uses turboModel", func(t *testing.T) {
		var captured QwenRequest
		server := httptest.NewServer(captureRequestBody(t, &captured, `[]`))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		svc := NewQwenService(&c, nil)
		if _, err := svc.AnalyzeRelationships(context.Background(), "text", []string{"A"}); err != nil {
			t.Fatalf("AnalyzeRelationships: %v", err)
		}
		if captured.Model != "sentinel-turbo" {
			t.Errorf("AnalyzeRelationships request Model = %q, want %q", captured.Model, "sentinel-turbo")
		}
	})

	t.Run("CheckContradictions uses maxModel", func(t *testing.T) {
		var captured QwenRequest
		server := httptest.NewServer(captureRequestBody(t, &captured, `[]`))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		svc := NewQwenService(&c, nil)
		candidates := []ContradictionCandidate{{Type: "semantic", EvidenceA: "a", EvidenceB: "b"}}
		if _, err := svc.CheckContradictions(context.Background(), candidates); err != nil {
			t.Fatalf("CheckContradictions: %v", err)
		}
		if captured.Model != "sentinel-max" {
			t.Errorf("CheckContradictions request Model = %q, want %q", captured.Model, "sentinel-max")
		}
	})

	t.Run("RunAgentLoop no-tools fallback uses maxModel", func(t *testing.T) {
		var captured QwenRequest
		server := httptest.NewServer(captureRequestBody(t, &captured, "final answer"))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		svc := NewQwenService(&c, nil)
		_, err := svc.RunAgentLoop(context.Background(), []QwenMessage{{Role: "user", Content: "hi"}}, nil, nil, 0)
		if err != nil {
			t.Fatalf("RunAgentLoop: %v", err)
		}
		if captured.Model != "sentinel-max" {
			t.Errorf("RunAgentLoop (no tools) request Model = %q, want %q", captured.Model, "sentinel-max")
		}
	})

	t.Run("RunAgentLoop tool-loop path uses maxModel", func(t *testing.T) {
		var captured QwenRequest
		server := httptest.NewServer(captureRequestBody(t, &captured, "final answer"))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		svc := NewQwenService(&c, nil)
		tools := []QwenTool{{Type: "function", Function: QwenToolFunction{Name: "noop"}}}
		_, err := svc.RunAgentLoop(context.Background(), []QwenMessage{{Role: "user", Content: "hi"}}, tools, nil, 1)
		if err != nil {
			t.Fatalf("RunAgentLoop: %v", err)
		}
		if captured.Model != "sentinel-max" {
			t.Errorf("RunAgentLoop (with tools) request Model = %q, want %q", captured.Model, "sentinel-max")
		}
	})

	t.Run("GenerateEmbedding uses embModel", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req EmbeddingRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode embedding request: %v", err)
			}
			if req.Model != "sentinel-embedding" {
				t.Errorf("GenerateEmbedding request Model = %q, want %q", req.Model, "sentinel-embedding")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(EmbeddingResponse{Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{{Embedding: []float32{0.1}, Index: 0}}})
		}))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		svc := NewQwenService(&c, nil)
		if _, err := svc.GenerateEmbedding(context.Background(), "text"); err != nil {
			t.Fatalf("GenerateEmbedding: %v", err)
		}
	})

	t.Run("GenerateEmbeddingBatch uses embModel", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req EmbeddingRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode embedding request: %v", err)
			}
			if req.Model != "sentinel-embedding" {
				t.Errorf("GenerateEmbeddingBatch request Model = %q, want %q", req.Model, "sentinel-embedding")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(EmbeddingResponse{Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{{Embedding: []float32{0.1}, Index: 0}}})
		}))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		svc := NewQwenService(&c, nil)
		if _, err := svc.GenerateEmbeddingBatch(context.Background(), []string{"text"}); err != nil {
			t.Fatalf("GenerateEmbeddingBatch: %v", err)
		}
	})

	t.Run("compressToolResults uses turboModel", func(t *testing.T) {
		var captured QwenRequest
		server := httptest.NewServer(captureRequestBody(t, &captured, "summary"))
		defer server.Close()
		c := *cfg
		c.QwenBaseURL = server.URL
		// Tiny budget forces compressToolResults' 80%-usage threshold to trip
		// immediately regardless of message content size.
		budgetMgr := NewContextBudgetManager(NewTokenizer(), 10, 0)
		svc := NewQwenService(&c, budgetMgr)

		msgs := []QwenMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "usr"},
			{Role: "assistant", ToolCalls: []QwenToolCall{{ID: "1", Function: QwenToolCallFunction{Name: "f"}}}},
			{Role: "tool", ToolCallID: "1", Content: "result A"},
			{Role: "assistant", ToolCalls: []QwenToolCall{{ID: "2", Function: QwenToolCallFunction{Name: "f"}}}},
			{Role: "tool", ToolCallID: "2", Content: "result B"},
		}
		_, compressed := svc.compressToolResults(context.Background(), msgs)
		if !compressed {
			t.Fatal("compressToolResults did not attempt compression — test setup did not exceed threshold")
		}
		if captured.Model != "sentinel-turbo" {
			t.Errorf("compressToolResults request Model = %q, want %q", captured.Model, "sentinel-turbo")
		}
	})
}

func TestNoHardcodedModelLiteralsOutsideConstructor(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "qwen_service.go"))
	if err != nil {
		t.Fatalf("read qwen_service.go: %v", err)
	}

	// Isolate NewQwenService's body — the only place literal defaults are
	// permitted to appear (as fallback getEnv defaults live in config, not
	// here; this file only assigns from cfg fields).
	src := string(data)
	ctorStart := strings.Index(src, "func NewQwenService(")
	if ctorStart == -1 {
		t.Fatal("NewQwenService not found in qwen_service.go")
	}
	ctorEnd := strings.Index(src[ctorStart:], "\n}\n")
	if ctorEnd == -1 {
		t.Fatal("could not find end of NewQwenService")
	}
	bodyBeforeCtor := src[:ctorStart]
	bodyAfterCtor := src[ctorStart+ctorEnd+len("\n}\n"):]
	outsideCtor := bodyBeforeCtor + bodyAfterCtor

	for _, literal := range []string{`"qwen-turbo"`, `"qwen-max"`, `"text-embedding-v3"`} {
		if strings.Contains(outsideCtor, literal) {
			t.Errorf("hardcoded model literal %s found outside NewQwenService — use s.turboModel/s.maxModel/s.embModel instead", literal)
		}
	}
}

func TestParseJSONLoose(t *testing.T) {
	type target struct {
		Name string `json:"name"`
	}

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "raw JSON", input: `{"name":"raw"}`, want: "raw"},
		{name: "fenced JSON", input: "```json\n{\"name\":\"fenced\"}\n```", want: "fenced"},
		{name: "garbage", input: "not json at all", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got target
			err := parseJSONLoose(tc.input, &got)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseJSONLoose(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseJSONLoose(%q) unexpected error: %v", tc.input, err)
			}
			if got.Name != tc.want {
				t.Errorf("parseJSONLoose(%q).Name = %q, want %q", tc.input, got.Name, tc.want)
			}
		})
	}
}

func TestQwenServiceChatAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:           server.URL,
		QwenAPIKey:            "test-key",
		QwenMaxConcurrency:    1,
		QwenTurboConcurrency:  1,
	}
	svc := NewQwenService(cfg, nil)

	ctx := context.Background()
	_, err := svc.Chat(ctx, "qwen-turbo", []QwenMessage{
		{Role: "user", Content: "Test"},
	})
	if err == nil {
		t.Error("Chat should return error on non-200 response")
	}
}
