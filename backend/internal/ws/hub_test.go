package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
)

// TestHubNew creates a Hub and verifies its internal state.
func TestHubNew(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil) // nil authSvc — auth init will reject all tokens
	if hub == nil {
		t.Fatal("NewHub returned nil")
	}
	if hub.conns == nil {
		t.Error("hub.conns should be initialized")
	}
}

// TestHubSendToUserNoConn verifies SendToUser returns an error when no
// connection exists for the given userID.
func TestHubSendToUserNoConn(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)

	msg := WSMessage{
		Type:    TypeAuthOK,
		Payload: json.RawMessage(`{"status":"ok"}`),
	}

	err := hub.SendToUser(uuid.New(), msg)
	if err == nil {
		t.Error("Expected error when sending to non-existent user connection")
	}
}

// TestHubRegisterAndSend verifies a connection can be registered and messages sent.
func TestHubRegisterAndSend(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	userID := uuid.New()

	// Register a mock connection
	conn := &Conn{
		userID: userID,
		done:   make(chan struct{}),
	}
	hub.Register(userID, conn)

	// Verify connection exists
	found := hub.GetConn(userID)
	if found == nil {
		t.Fatal("GetConn should return the registered connection")
	}
	if found.userID != userID {
		t.Errorf("GetConn returned wrong userID: %v", found.userID)
	}

	// Remove and verify cleanup
	hub.Remove(userID)
	if hub.GetConn(userID) != nil {
		t.Error("GetConn should return nil after Remove")
	}
}

// TestHubRemoveCleanup verifies that removing a connection works correctly
// and the removed user can no longer be found.
func TestHubRemoveCleanup(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	userID := uuid.New()

	conn := &Conn{userID: userID, done: make(chan struct{})}
	hub.Register(userID, conn)

	// Verify it exists
	if hub.GetConn(userID) == nil {
		t.Fatal("connection should exist after Register")
	}

	// Remove
	hub.Remove(userID)

	// Verify it's gone
	if hub.GetConn(userID) != nil {
		t.Error("connection should be nil after Remove")
	}

	// SendToUser on removed user should fail
	err := hub.SendToUser(userID, WSMessage{Type: TypeAuthOK})
	if err == nil {
		t.Error("Expected error sending to removed user")
	}
}

// TestHubParagraphSubmitterInterface verifies the Hub accepts and calls a
// ParagraphSubmitter. The handler must call SubmitParagraph with the parsed
// payload instead of echoing a fake result.
func TestHubParagraphSubmitterInterface(t *testing.T) {
	mock := &mockParagraphSubmitter{}
	hub := NewHub(nil, mock, nil, nil)

	workID := uuid.New()
	chapterID := uuid.New()
	universeID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"work_id":     workID,
		"chapter_id":  chapterID,
		"universe_id": universeID,
		"text":        "Test paragraph",
	})
	msg := WSMessage{Type: TypeParagraphSubmit, Payload: payload}

	hub.handleParagraphSubmit(uuid.New(), msg)

	if !mock.called {
		t.Error("ParagraphSubmitter.SubmitParagraph was not called — handler is still a stub")
	}
	if mock.workID != workID {
		t.Errorf("workID = %v, want %v", mock.workID, workID)
	}
	if mock.chapterID != chapterID {
		t.Errorf("chapterID = %v, want %v", mock.chapterID, chapterID)
	}
	if mock.universeID != universeID {
		t.Errorf("universeID = %v, want %v", mock.universeID, universeID)
	}
	if mock.text != "Test paragraph" {
		t.Errorf("text = %q, want %q", mock.text, "Test paragraph")
	}
}

// TestHubRecallRequesterInterface verifies the Hub accepts and calls a
// RecallRequester. The handler must call Recall with the parsed payload
// instead of echoing a fake result.
func TestHubRecallRequesterInterface(t *testing.T) {
	mock := &mockRecallRequester{}
	hub := NewHub(nil, nil, mock, nil)

	universeID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"universe_id": universeID,
		"query":       "test query",
		"k":           5,
	})
	msg := WSMessage{Type: TypeRecallRequest, Payload: payload}

	hub.handleRecallRequest(uuid.New(), msg)

	if !mock.called {
		t.Error("RecallRequester.Recall was not called — handler is still a stub")
	}
	if mock.universeID != universeID {
		t.Errorf("universeID = %v, want %v", mock.universeID, universeID)
	}
	if mock.k != 5 {
		t.Errorf("k = %d, want 5", mock.k)
	}
}

// TestHubRecallEmbeddingProviderUsed verifies that when handleRecallRequest
// processes a query, the EmbeddingProvider is called and its output is passed
// to RecallRequester.Recall (instead of nil).
//
// RED: Hub currently has no embedder field — NewHub only accepts 3 params.
// This test adds a 4th param — won't compile until production code is updated.
func TestHubRecallEmbeddingProviderUsed(t *testing.T) {
	mockEmbedder := &mockEmbeddingProvider{}
	mockRecaller := &mockRecallRequesterWithEmbedding{}

	// RED: NewHub currently takes 3 args — adding embedder as 4th
	hub := NewHub(nil, nil, mockRecaller, mockEmbedder)

	universeID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"universe_id": universeID,
		"query":       "test query for embedding",
		"k":           5,
	})
	msg := WSMessage{Type: TypeRecallRequest, Payload: payload}

	hub.handleRecallRequest(uuid.New(), msg)

	if !mockEmbedder.called {
		t.Error("EmbeddingProvider.GenerateEmbedding was not called — query string was not embedded")
	}
	if mockEmbedder.text != "test query for embedding" {
		t.Errorf("embedding text = %q, want %q", mockEmbedder.text, "test query for embedding")
	}
	if !mockRecaller.called {
		t.Error("RecallRequester.Recall was not called")
	}
	if mockRecaller.embedding == nil {
		t.Error("Recall was called with nil embedding — embedding provider output was not threaded through")
	}
	if len(mockRecaller.embedding) == 0 {
		t.Error("Recall was called with empty embedding — expected non-empty vector from embedder")
	}
}

// TestHubRecallEmbeddingProviderError verifies that when the EmbeddingProvider
// returns an error, handleRecallRequest sends an error message and does NOT
// call RecallRequester.Recall.
func TestHubRecallEmbeddingProviderError(t *testing.T) {
	mockEmbedder := &mockEmbeddingProviderError{}
	mockRecaller := &mockRecallRequesterWithEmbedding{}

	hub := NewHub(nil, nil, mockRecaller, mockEmbedder)

	universeID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"universe_id": universeID,
		"query":       "bad query",
		"k":           5,
	})
	msg := WSMessage{Type: TypeRecallRequest, Payload: payload}

	hub.handleRecallRequest(uuid.New(), msg)

	if !mockEmbedder.called {
		t.Error("EmbeddingProvider.GenerateEmbedding was not called")
	}
	if mockRecaller.called {
		t.Error("RecallRequester.Recall should NOT be called when embedding fails")
	}
}

// TestWSMessageTypeConstantsMatch verifies the ws constants are aligned.
func TestWSMessageTypeConstantsMatch(t *testing.T) {
	// Verify we can construct the well-known messages
	msg := WSMessage{Type: "auth_init", Payload: json.RawMessage(`{"token":"x"}`)}
	if msg.Type != TypeAuthInit {
		t.Errorf("expected auth_init constant")
	}
}

// ── Mocks ──

// mockParagraphSubmitter records calls to SubmitParagraph.
type mockParagraphSubmitter struct {
	called                     bool
	workID, chapterID, universeID uuid.UUID
	text                       string
}

func (m *mockParagraphSubmitter) SubmitParagraph(ctx context.Context, workID, chapterID, universeID, userID uuid.UUID, text string) error {
	m.called = true
	m.workID = workID
	m.chapterID = chapterID
	m.universeID = universeID
	m.text = text
	return nil
}

// mockRecallRequester records calls to RecallWithQuery.
type mockRecallRequester struct {
	called     bool
	universeID uuid.UUID
	k          int
}

func (m *mockRecallRequester) RecallWithQuery(ctx context.Context, universeID uuid.UUID, queryEmbedding []float32, queryText string, k int) ([]models.RecallItem, error) {
	m.called = true
	m.universeID = universeID
	m.k = k
	return nil, nil
}

// mockRecallRequesterWithEmbedding records the embedding passed to RecallWithQuery.
type mockRecallRequesterWithEmbedding struct {
	called     bool
	universeID uuid.UUID
	embedding  []float32
	k          int
}

func (m *mockRecallRequesterWithEmbedding) RecallWithQuery(ctx context.Context, universeID uuid.UUID, queryEmbedding []float32, queryText string, k int) ([]models.RecallItem, error) {
	m.called = true
	m.universeID = universeID
	m.embedding = queryEmbedding
	m.k = k
	return nil, nil
}

// mockEmbeddingProvider records calls to GenerateEmbedding.
type mockEmbeddingProvider struct {
	called bool
	text   string
}

func (m *mockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.called = true
	m.text = text
	return []float32{0.1, 0.2, 0.3}, nil
}

// mockEmbeddingProviderError always returns an error.
type mockEmbeddingProviderError struct {
	called bool
	text   string
}

func (m *mockEmbeddingProviderError) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.called = true
	m.text = text
	return nil, fmt.Errorf("embedding service unavailable")
}
