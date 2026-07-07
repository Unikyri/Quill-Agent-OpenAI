package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
)

// mockEntityLister satisfies entityLister with a fixed set of entities.
type mockEntityLister struct {
	entities []models.Entity
}

func (m *mockEntityLister) ListByUniverseActive(ctx context.Context, universeID uuid.UUID) ([]models.Entity, error) {
	return m.entities, nil
}

// capturingGraphQuerier records the entityID passed to GetNeighbors so
// tests can assert the correct entity was resolved. Distinct from
// mockGraphQuerier (qwen_service_test.go) to avoid redeclaration.
type capturingGraphQuerier struct {
	gotEntityID string
	neighbors   []models.GraphNeighbor
}

func (m *capturingGraphQuerier) GetNeighbors(ctx context.Context, graphName, entityID string) ([]models.GraphNeighbor, error) {
	m.gotEntityID = entityID
	return m.neighbors, nil
}

// TestQueryEntityGraphResolvesByNameOrAlias locks case-insensitive
// name-then-alias resolution (agent-tool-alias-resolution).
func TestQueryEntityGraphResolvesByNameOrAlias(t *testing.T) {
	aragornID := uuid.New()
	gandalfID := uuid.New()

	tests := []struct {
		name      string
		entities  []models.Entity
		queryName string
		wantID    string // resolved id.String(); "" => expect "not found"
	}{
		{
			name:      "exact name match, different case",
			entities:  []models.Entity{{ID: aragornID, Name: "Aragorn", Aliases: []string{"Strider"}}},
			queryName: "aragorn",
			wantID:    aragornID.String(),
		},
		{
			name:      "alias match, different case",
			entities:  []models.Entity{{ID: aragornID, Name: "Aragorn", Aliases: []string{"Strider"}}},
			queryName: "strider",
			wantID:    aragornID.String(),
		},
		{
			name:      "no match at all",
			entities:  []models.Entity{{ID: aragornID, Name: "Aragorn", Aliases: []string{"Strider"}}},
			queryName: "Sauron",
			wantID:    "",
		},
		{
			name: "multiple entities, only one alias matches",
			entities: []models.Entity{
				{ID: gandalfID, Name: "Gandalf", Aliases: []string{"Mithrandir"}},
				{ID: aragornID, Name: "Aragorn", Aliases: []string{"Strider", "Elessar"}},
			},
			queryName: "elessar",
			wantID:    aragornID.String(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			graph := &capturingGraphQuerier{
				neighbors: []models.GraphNeighbor{{Node: `{"properties":{"name":"Legolas"}}`, RelType: "ALLY_OF"}},
			}
			exec := &QuillExecutor{
				EntityRepo: &mockEntityLister{entities: tc.entities},
				GraphRepo:  graph,
			}

			result, err := exec.ExecuteTool("query_entity_graph", `{"entity_name":"`+tc.queryName+`"}`)
			if err != nil {
				t.Fatalf("ExecuteTool: %v", err)
			}

			if tc.wantID == "" {
				if !strings.Contains(result, "not found") {
					t.Errorf("expected 'not found', got %q", result)
				}
				if graph.gotEntityID != "" {
					t.Errorf("GetNeighbors should not be called on no-match, got id %q", graph.gotEntityID)
				}
				return
			}
			if graph.gotEntityID != tc.wantID {
				t.Errorf("resolved wrong entity: got id %q want %q", graph.gotEntityID, tc.wantID)
			}
			if !strings.Contains(result, "Neighbors of") || !strings.Contains(result, "ALLY_OF") {
				t.Errorf("expected formatted neighbor output, got %q", result)
			}
		})
	}
}
