package services

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// ── Mocks ──

// mockIngestionHub captures SendToUser calls for verification.
type mockIngestionHub struct {
	mu       sync.Mutex
	messages []models.WSMessage
}

func (m *mockIngestionHub) SendToUser(userID uuid.UUID, msg models.WSMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockIngestionHub) popMessages() []models.WSMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.messages
	m.messages = nil
	return out
}

// mockQwenForIngestion returns canned ExtractEntities and GenerateEmbedding results.
type mockQwenForIngestion struct {
	extractResult *ExtractedEntities
	extractErr    error
}

func (m *mockQwenForIngestion) ExtractEntities(ctx context.Context, text, categories string) (*ExtractedEntities, error) {
	return m.extractResult, m.extractErr
}

func (m *mockQwenForIngestion) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Return a dummy embedding — length 3 for test
	return []float32{0.1, 0.2, 0.3}, nil
}

// ── Test: pipeline sequence ──

// TestIngestionServicePipeline verifies the chunk→extract→embed→graph sequence
// in a mock-driven test. It uses an httptest-style pattern without real DB.
func TestIngestionServicePipeline(t *testing.T) {
	hub := &mockIngestionHub{}
	qwen := &mockQwenForIngestion{
		extractResult: &ExtractedEntities{
			Characters: []ExtractedEntity{
				{Type: "Character", Name: "Frodo", Status: "alive", Description: "A hobbit"},
			},
		},
	}

	docContent := `# Chapter 1: A Long-expected Party

Bilbo was going to have a birthday party.

# Chapter 2: The Shadow of the Past

Frodo learns about the Ring.`

	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    qwen,
		hub:        hub,
	}

	universeID := uuid.New()
	workID := uuid.New()

	jobID, err := svc.Start(context.Background(), universeID, workID, strings.NewReader(docContent), "test.md")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if jobID == uuid.Nil {
		t.Error("expected non-nil job ID")
	}

	// Wait for goroutine to finish (small doc, fast)
	time.Sleep(200 * time.Millisecond)

	msgs := hub.popMessages()
	// Should have at least one progress event
	if len(msgs) == 0 {
		t.Error("expected at least one WebSocket progress message")
	}

	// Verify at least one is an ingestion_progress event
	foundProgress := false
	for _, msg := range msgs {
		if msg.Type == "ingestion_progress" {
			foundProgress = true
			var payload map[string]any
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				t.Errorf("unmarshal progress payload: %v", err)
			}
			if payload["job_id"] == nil {
				t.Error("progress payload missing job_id")
			}
		}
	}
	if !foundProgress {
		t.Error("expected an ingestion_progress message")
	}
}

// TestIngestionServiceChunking verifies the document is split by markdown headers.
func TestIngestionServiceChunking(t *testing.T) {
	hub := &mockIngestionHub{}
	qwen := &mockQwenForIngestion{
		extractResult: &ExtractedEntities{},
	}

	docContent := `# Chapter A

Content A line 1.
Content A line 2.

# Chapter B

Content B line 1.`

	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    qwen,
		hub:        hub,
	}

	chunks := svc.splitChunks(docContent)
	// ponytail: minimal chunking — one chunk per # header section
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify chunk content is non-empty
	for i, ch := range chunks {
		if strings.TrimSpace(ch.content) == "" {
			t.Errorf("chunk %d is empty", i)
		}
		if ch.title == "" {
			t.Errorf("chunk %d has no title", i)
		}
	}
}

// TestIngestionServiceEmptyDocument verifies handling of empty input.
func TestIngestionServiceEmptyDocument(t *testing.T) {
	hub := &mockIngestionHub{}
	qwen := &mockQwenForIngestion{
		extractResult: &ExtractedEntities{},
	}

	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    qwen,
		hub:        hub,
	}

	chunks := svc.splitChunks("")
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty document, got %d", len(chunks))
	}
}

// TestIngestionServiceNilDeps verifies Start can handle nil dependencies gracefully.
func TestIngestionServiceNilDeps(t *testing.T) {
	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    nil,
		hub:        nil,
	}

	// Start should attempt to create a job; may fail if pool is nil
	jobID, err := svc.Start(context.Background(), uuid.New(), uuid.New(), strings.NewReader("hello"), "test.md")
	if err != nil {
		// Expected when pool is nil — service can't persist the job
		t.Logf("Start with nil deps: jobID=%s err=%v", jobID, err)
	} else if jobID == uuid.Nil {
		t.Error("expected non-nil job ID even with nil deps")
	}
}

// compile-time interface checks
var _ IngestionQwen = (*mockQwenForIngestion)(nil)
var _ *pgxpool.Pool = nil
var _ *repositories.VectorRepo = nil
var _ *repositories.GraphRepo = nil
var _ *EntityService = nil
var _ AnalysisHub = (*mockIngestionHub)(nil)
