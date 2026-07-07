package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
