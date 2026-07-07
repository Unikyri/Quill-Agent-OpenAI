package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestConsolidatedMemorySerialization(t *testing.T) {
	entityID := uuid.New()
	cm := ConsolidatedMemory{
		ID:        uuid.New(),
		EntityID:  entityID,
		Summary:   "Elena is a sorceress from the northern kingdom who betrayed her mentor.",
		KeyFacts:  []string{"sorceress", "northern kingdom", "betrayed mentor"},
		Embedding: make([]float32, 1024),
		CreatedAt: time.Now().UTC(),
	}

	// Verify all six fields are populated
	if cm.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if cm.EntityID != entityID {
		t.Errorf("EntityID = %v, want %v", cm.EntityID, entityID)
	}
	if cm.Summary == "" {
		t.Error("Summary should not be empty")
	}
	if len(cm.KeyFacts) != 3 {
		t.Errorf("KeyFacts length = %d, want 3", len(cm.KeyFacts))
	}
	if len(cm.Embedding) != 1024 {
		t.Errorf("Embedding length = %d, want 1024", len(cm.Embedding))
	}
	if cm.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Verify JSON round-trip (spec: MUST be serializable)
	data, err := json.Marshal(cm)
	if err != nil {
		t.Fatalf("marshal ConsolidatedMemory: %v", err)
	}

	var restored ConsolidatedMemory
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal ConsolidatedMemory: %v", err)
	}
	if restored.EntityID != entityID {
		t.Errorf("after round-trip: EntityID = %v, want %v", restored.EntityID, entityID)
	}
	if restored.Summary != cm.Summary {
		t.Errorf("after round-trip: Summary = %q, want %q", restored.Summary, cm.Summary)
	}
}

// TestRecallItemGenericID proves RecallItem carries a generic ID identifying
// the result regardless of source (vector/paragraph hits have no EntityID,
// so ID is their only identity).
func TestRecallItemGenericID(t *testing.T) {
	vectorItem := RecallItem{
		ID:       "chapter-123:some snippet",
		EntityID: uuid.Nil,
		Fact:     "some snippet",
		Score:    0.8,
		Source:   "vector",
	}
	if vectorItem.ID == "" {
		t.Error("vector-sourced RecallItem.ID should not be empty")
	}
	if vectorItem.EntityID != uuid.Nil {
		t.Error("vector-sourced RecallItem.EntityID should be uuid.Nil")
	}

	entityID := uuid.New()
	graphItem := RecallItem{
		ID:       entityID.String(),
		EntityID: entityID,
		Fact:     "Related: Bob",
		Score:    0.5,
		Source:   "graph",
	}
	if graphItem.ID != entityID.String() {
		t.Errorf("graph-sourced RecallItem.ID = %q, want %q", graphItem.ID, entityID.String())
	}

	data, err := json.Marshal(vectorItem)
	if err != nil {
		t.Fatalf("marshal RecallItem: %v", err)
	}
	var restored RecallItem
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal RecallItem: %v", err)
	}
	if restored.ID != vectorItem.ID {
		t.Errorf("after round-trip: ID = %q, want %q", restored.ID, vectorItem.ID)
	}
}
