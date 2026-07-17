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
		"submission_id": "submission-1",
		"paragraph_ref": "chapter:12",
		"work_id":       workID,
		"chapter_id":    chapterID,
		"universe_id":   universeID,
		"text":          "Test paragraph",
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
	if mock.submissionID != "submission-1" || mock.paragraphRef != "chapter:12" {
		t.Errorf("submission correlation = %q / %q", mock.submissionID, mock.paragraphRef)
	}
}

func TestHubParagraphSubmitMalformedPayloadDoesNotPanic(t *testing.T) {
	hub := NewHub(nil, &mockParagraphSubmitter{}, nil, nil)
	msg := WSMessage{Type: TypeParagraphSubmit, Payload: json.RawMessage(`{"not":"a submission"}`)}
	// The handler must reject malformed/invalid payloads without reaching the
	// submitter or panicking. There is no registered connection in this unit
	// test, so the best-effort failure send is expected to be logged only.
	hub.handleParagraphSubmit(uuid.New(), msg)
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

func TestHubRecallRejectsForeignUniverse(t *testing.T) {
	mockRecaller := &mockRecallRequester{}
	hub := NewHub(nil, nil, mockRecaller, nil)
	hub.SetUniverseOwnerResolver(mockUniverseOwnerResolver{universe: &models.Universe{UserID: uuid.New()}})

	payload, _ := json.Marshal(map[string]interface{}{
		"universe_id": uuid.New(),
		"query":       "private context",
		"k":           5,
	})
	hub.handleRecallRequest(uuid.New(), WSMessage{Type: TypeRecallRequest, Payload: payload})

	if mockRecaller.called {
		t.Fatal("recall must not reach MemoryService for a foreign universe")
	}
}

func TestHubCraftReviewScopesAndDispatches(t *testing.T) {
	mockReviewer := &mockCraftReviewer{}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetCraftReviewer(mockReviewer)
	userID := uuid.New()
	universeID, workID, chapterID := uuid.New(), uuid.New(), uuid.New()
	hub.SetUniverseOwnerResolver(mockUniverseOwnerResolver{universe: &models.Universe{ID: universeID, UserID: userID}})
	hub.SetParagraphOwnershipResolvers(
		mockWorkOwnershipResolver{work: &models.Work{ID: workID, UniverseID: universeID}},
		mockChapterOwnershipResolver{chapter: &models.Chapter{ID: chapterID, WorkID: workID, UniverseID: universeID}},
	)
	payload, _ := json.Marshal(models.CraftReviewRequestPayload{
		UniverseID: universeID, WorkID: workID, ChapterID: chapterID, Passage: "A passage to review",
	})
	hub.handleCraftReviewRequest(userID, WSMessage{Type: TypeCraftReviewRequest, Payload: payload})
	if !mockReviewer.called {
		t.Fatal("craft reviewer was not called")
	}
	if mockReviewer.request.Passage != "A passage to review" {
		t.Fatalf("passage = %q", mockReviewer.request.Passage)
	}

	foreign := &mockCraftReviewer{}
	hub.SetCraftReviewer(foreign)
	hub.handleCraftReviewRequest(uuid.New(), WSMessage{Type: TypeCraftReviewRequest, Payload: payload})
	if foreign.called {
		t.Fatal("foreign craft review must be rejected before dispatch")
	}
}

func TestHubParagraphRejectsForeignUniverse(t *testing.T) {
	mockSubmitter := &mockParagraphSubmitter{}
	hub := NewHub(nil, mockSubmitter, nil, nil)
	hub.SetUniverseOwnerResolver(mockUniverseOwnerResolver{universe: &models.Universe{UserID: uuid.New()}})

	payload, _ := json.Marshal(models.ParagraphSubmitPayload{
		SubmissionID: "submission-foreign",
		ParagraphRef: "chapter:1",
		WorkID:       uuid.New(),
		ChapterID:    uuid.New(),
		UniverseID:   uuid.New(),
		Text:         "Private paragraph",
	})
	hub.handleParagraphSubmit(uuid.New(), WSMessage{Type: TypeParagraphSubmit, Payload: payload})

	if mockSubmitter.called {
		t.Fatal("paragraph must not reach AnalysisService for a foreign universe")
	}
}

func TestHubParagraphRejectsMismatchedWork(t *testing.T) {
	ownerID := uuid.New()
	universeID := uuid.New()
	mockSubmitter := &mockParagraphSubmitter{}
	hub := NewHub(nil, mockSubmitter, nil, nil)
	hub.SetUniverseOwnerResolver(mockUniverseOwnerResolver{universe: &models.Universe{UserID: ownerID}})
	hub.SetParagraphOwnershipResolvers(mockWorkOwnershipResolver{work: &models.Work{UniverseID: uuid.New()}}, nil)

	payload, _ := json.Marshal(models.ParagraphSubmitPayload{
		SubmissionID: "submission-mismatched-work",
		ParagraphRef: "chapter:1",
		WorkID:       uuid.New(),
		ChapterID:    uuid.New(),
		UniverseID:   universeID,
		Text:         "Mismatched work",
	})
	hub.handleParagraphSubmit(ownerID, WSMessage{Type: TypeParagraphSubmit, Payload: payload})

	if mockSubmitter.called {
		t.Fatal("paragraph must not reach AnalysisService when work belongs to another universe")
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
	called                        bool
	submissionID, paragraphRef    string
	workID, chapterID, universeID uuid.UUID
	text                          string
}

func (m *mockParagraphSubmitter) SubmitParagraph(ctx context.Context, submissionID, paragraphRef string, workID, chapterID, universeID, userID uuid.UUID, text string) error {
	m.called = true
	m.submissionID = submissionID
	m.paragraphRef = paragraphRef
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

type mockUniverseOwnerResolver struct {
	universe *models.Universe
}

type mockChapterOwnershipResolver struct {
	chapter *models.Chapter
}

func (m mockChapterOwnershipResolver) FindByID(_ context.Context, _ uuid.UUID) (*models.Chapter, error) {
	return m.chapter, nil
}

type mockCraftReviewer struct {
	called  bool
	request models.CraftReviewRequestPayload
}

func (m *mockCraftReviewer) Review(_ context.Context, _ uuid.UUID, request models.CraftReviewRequestPayload) (models.CraftReviewResultPayload, error) {
	m.called = true
	m.request = request
	return models.CraftReviewResultPayload{UniverseID: request.UniverseID, WorkID: request.WorkID, ChapterID: request.ChapterID}, nil
}

func (m mockUniverseOwnerResolver) FindByID(_ context.Context, _ uuid.UUID) (*models.Universe, error) {
	return m.universe, nil
}

type mockWorkOwnershipResolver struct {
	work *models.Work
}

func (m mockWorkOwnershipResolver) FindByID(_ context.Context, _ uuid.UUID) (*models.Work, error) {
	return m.work, nil
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
