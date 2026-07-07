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
