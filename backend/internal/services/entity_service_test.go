package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

// TestResolveOrCreateNewEntityAppendsHistoryRow proves entity creation (Step
// 4 of ResolveOrCreate, brand-new entity, isNew=true) writes a single
// entity_relevance_history row with the initial score (0.8) and status
// (spec: Relevance history persistence requirement).
func TestResolveOrCreateNewEntityAppendsHistoryRow(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "019")
	ctx := context.Background()

	// Mock Qwen embeddings endpoint so Step 3 (semantic similarity) doesn't
	// need a real API key.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := EmbeddingResponse{Data: []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}{{Embedding: make([]float32, 1024)}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key"}
	qwenSvc := NewQwenService(cfg, nil)

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	entityRepo := repositories.NewEntityRepo(pool)
	entitySvc := NewEntityService(pool, entityRepo, nil, qwenSvc)

	entity, previousStatus, isNew, err := entitySvc.ResolveOrCreate(ctx, universe.ID, repositories.ExtractedEntity{
		Type: "character", Name: "Brand New Wizard",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreate: %v", err)
	}
	if !isNew {
		t.Fatal("expected a brand-new entity to be created")
	}
	if previousStatus != "" {
		t.Errorf("previousStatus = %q, want empty for brand-new entity", previousStatus)
	}

	historyRepo := repositories.NewEntityRelevanceHistoryRepo(pool)
	points, err := historyRepo.ListRecentByUniverse(ctx, universe.ID, 30)
	if err != nil {
		t.Fatalf("ListRecentByUniverse: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("len(points) = %d, want 1", len(points))
	}
	if points[0].EntityID != entity.ID {
		t.Errorf("EntityID = %v, want %v", points[0].EntityID, entity.ID)
	}
	if points[0].RelevanceScore != 0.8 {
		t.Errorf("RelevanceScore = %f, want 0.8", points[0].RelevanceScore)
	}
}
