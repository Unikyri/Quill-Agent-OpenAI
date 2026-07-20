package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
)

func TestArbiterServiceNoQwenSvcIsNoOp(t *testing.T) {
	svc := NewArbiterService(nil)

	summary, err := svc.Adjudicate(context.Background(),
		[]models.Contradiction{{Description: "Edric is both dead and alive"}}, nil)
	if err != nil {
		t.Fatalf("expected no error with nil qwenSvc, got: %v", err)
	}
	if summary != "" {
		t.Errorf("expected empty summary with nil qwenSvc, got: %q", summary)
	}
}

func TestArbiterServiceNoFindingsIsNoOp(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"should not be called"}}]}`))
	}))
	defer srv.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL: srv.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1,
	}, nil)
	svc := NewArbiterService(qwenSvc)

	summary, err := svc.Adjudicate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("expected no error for zero findings, got: %v", err)
	}
	if summary != "" {
		t.Errorf("expected empty summary for zero findings, got: %q", summary)
	}
	if called {
		t.Error("expected the model to never be called when there is nothing to adjudicate")
	}
}

func TestArbiterServiceSynthesizesAcrossBothSpecialists(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"role":    "assistant",
					"content": "The contradiction about Edric's death matters most — it directly conflicts with the death record and subsumes the vault plot hole.",
				}},
			},
		})
	}))
	defer srv.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL: srv.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1,
	}, nil)
	svc := NewArbiterService(qwenSvc)

	contradictions := []models.Contradiction{{Description: "Edric Ashvale is recorded dead in chapter 1 but appears alive in chapter 3"}}
	plotHoles := []models.PlotHole{{Description: "The vault's secondary entrance is mentioned but never explained"}}

	summary, err := svc.Adjudicate(context.Background(), contradictions, plotHoles)
	if err != nil {
		t.Fatalf("Adjudicate: %v", err)
	}
	if !strings.Contains(summary, "Edric's death") {
		t.Errorf("expected the model's synthesis in the summary, got: %q", summary)
	}

	// The request sent to the model must actually contain both specialists'
	// findings — otherwise this "adjudicates across both" only in name.
	messages, _ := capturedBody["messages"].([]interface{})
	var userContent string
	for _, m := range messages {
		msg, _ := m.(map[string]interface{})
		if msg["role"] == "user" {
			userContent, _ = msg["content"].(string)
		}
	}
	if !strings.Contains(userContent, "Edric Ashvale") || !strings.Contains(userContent, "vault's secondary entrance") {
		t.Errorf("expected both specialists' findings in the prompt sent to the model, got: %q", userContent)
	}
}

func TestArbiterServicePropagatesModelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL: srv.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1,
	}, nil)
	svc := NewArbiterService(qwenSvc)

	_, err := svc.Adjudicate(context.Background(), []models.Contradiction{{Description: "x"}}, nil)
	if err == nil {
		t.Fatal("expected an error when the model call fails")
	}
}
