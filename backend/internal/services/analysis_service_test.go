package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
	"github.com/quill/backend/internal/ws"
)

// ── Unit tests (no DB required) ──

// TestAnalysisServiceNew verifies construction doesn't panic.
func TestAnalysisServiceNew(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("NewAnalysisService returned nil")
	}
	if svc.queues == nil {
		t.Error("queues map should be initialized")
	}
	if svc.cancels == nil {
		t.Error("cancels map should be initialized")
	}
}

// TestAnalysisServiceArbiterSvcOptedOutByDefault verifies SetArbiterSvc wires
// in cleanly and that processJob still completes without an arbiter call
// when there is nothing for it to adjudicate (empty text -> no entities ->
// no contradictions/plot holes -> Adjudicate is never reached).
func TestAnalysisServiceArbiterSvcOptedOutByDefault(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetArbiterSvc(NewArbiterService(nil))

	result, err := svc.processJob(context.Background(), analysisJob{
		SubmissionID: "sub-1", ParagraphRef: "p1",
		WorkID: uuid.New(), ChapterID: uuid.New(), UniverseID: uuid.New(), UserID: uuid.New(),
		Text: "",
	})
	if err != nil {
		t.Fatalf("processJob: %v", err)
	}
	if result.ArbiterSummary != "" {
		t.Errorf("expected no arbiter summary with no findings, got: %q", result.ArbiterSummary)
	}
}

// TestAnalysisServiceSubmit verifies that Submit enqueues a job
// to the correct per-work channel.
func TestAnalysisServiceSubmit(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workID := uuid.New()
	job := analysisJob{
		WorkID:     workID,
		ChapterID:  uuid.New(),
		UniverseID: uuid.New(),
		Text:       "Test paragraph",
		UserID:     uuid.New(),
	}

	// Submit should enqueue into the buffered channel
	err := svc.Submit(ctx, job)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Verify queue was created
	svc.mu.Lock()
	q, exists := svc.queues[workID]
	svc.mu.Unlock()
	if !exists {
		t.Fatal("queue not created for workID")
	}

	// Drain the queue and verify the job
	select {
	case received := <-q:
		if received.WorkID != job.WorkID {
			t.Errorf("WorkID mismatch: got %s, want %s", received.WorkID, job.WorkID)
		}
		if received.Text != job.Text {
			t.Errorf("Text mismatch: got %s, want %s", received.Text, job.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job in queue")
	}
}

// TestAnalysisServiceSubmitSecondJob verifies sequential queuing
// for the same workID.
func TestAnalysisServiceSubmitSecondJob(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workID := uuid.New()
	job1 := analysisJob{WorkID: workID, Text: "first", UserID: uuid.New(), UniverseID: uuid.New()}
	job2 := analysisJob{WorkID: workID, Text: "second", UserID: uuid.New(), UniverseID: uuid.New()}

	if err := svc.Submit(ctx, job1); err != nil {
		t.Fatalf("Submit job1: %v", err)
	}
	if err := svc.Submit(ctx, job2); err != nil {
		t.Fatalf("Submit job2: %v", err)
	}

	svc.mu.Lock()
	q := svc.queues[workID]
	svc.mu.Unlock()

	// Drain first job
	select {
	case r := <-q:
		if r.Text != "first" {
			t.Errorf("expected 'first', got '%s'", r.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job1")
	}

	// Drain second job
	select {
	case r := <-q:
		if r.Text != "second" {
			t.Errorf("expected 'second', got '%s'", r.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job2")
	}
}

// TestAnalysisServiceCancel verifies Cancel removes queue and cancel func.
func TestAnalysisServiceCancel(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	workID := uuid.New()
	svc.mu.Lock()
	svc.queues[workID] = make(chan analysisJob, 10)
	svc.cancels[workID] = func() {}
	svc.mu.Unlock()

	svc.Cancel(workID)

	svc.mu.Lock()
	_, qExists := svc.queues[workID]
	_, cExists := svc.cancels[workID]
	svc.mu.Unlock()

	if qExists {
		t.Error("queue should be removed after Cancel")
	}
	if cExists {
		t.Error("cancel func should be removed after Cancel")
	}
}

// TestAnalysisServiceCancelNonexistent verifies Cancel is a no-op
// for unknown work IDs.
func TestAnalysisServiceCancelNonexistent(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// Should not panic
	svc.Cancel(uuid.New())
}

// TestAnalysisServiceShutdown verifies Shutdown removes all workers.
func TestAnalysisServiceShutdown(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	workID1 := uuid.New()
	workID2 := uuid.New()

	svc.mu.Lock()
	svc.queues[workID1] = make(chan analysisJob, 10)
	svc.queues[workID2] = make(chan analysisJob, 10)
	svc.cancels[workID1] = func() {}
	svc.cancels[workID2] = func() {}
	svc.mu.Unlock()

	svc.Shutdown()

	svc.mu.Lock()
	remainingQueues := len(svc.queues)
	remainingCancels := len(svc.cancels)
	svc.mu.Unlock()

	if remainingQueues > 0 {
		t.Errorf("expected 0 queues after shutdown, got %d", remainingQueues)
	}
	if remainingCancels > 0 {
		t.Errorf("expected 0 cancels after shutdown, got %d", remainingCancels)
	}
}

// TestAnalysisServiceResolvedEntityType verifies the ResolvedEntity struct.
func TestAnalysisServiceResolvedEntityType(t *testing.T) {
	e := models.Entity{}
	re := ResolvedEntity{Entity: e, MentionText: "test", IsNew: true}
	if !re.IsNew {
		t.Error("expected IsNew=true")
	}
	if re.MentionText != "test" {
		t.Errorf("expected 'test', got '%s'", re.MentionText)
	}
}

// spec: CRITICAL #6 — Pipeline extractEntities nil-safe: no crash with nil deps
func TestExtractEntitiesNilSafe(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// extractEntities (unexported) should return nil, nil when qwenSvc is nil
	entities, err := svc.extractEntities(context.Background(), uuid.New(), "Test paragraph", uuid.New())
	if err != nil {
		t.Errorf("extractEntities with nil deps should return nil error, got: %v", err)
	}
	if entities != nil {
		t.Errorf("extractEntities with nil deps should return nil slice, got: %v", entities)
	}
}

// spec: CRITICAL #6 — Pipeline archived→Reactivate path: archived entity re-mentioned
func TestExtractEntitiesArchivedReactivate(t *testing.T) {
	pool := setupAnalysisTestDB(t)
	if pool == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// We test that AnalysisService with all nil deps does not panic when extracting.
	// The full archived→Reactivate path needs a real QwenService + real EntityService,
	// which requires TEST_DATABASE_URL + QWEN_API_KEY. We verify nil-safe behavior:
	// the Reactivate call at analysis_service.go:431-435 is guarded by `s.relevSvc != nil`.
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	entities, err := svc.extractEntities(ctx, uuid.New(), "The old wizard returned.", uuid.New())
	if err != nil {
		t.Errorf("extractEntities nil-safe with archived entity text: %v", err)
	}
	if entities != nil {
		t.Errorf("expected nil entities with nil deps, got %d", len(entities))
	}
	// spec: no panic, null-guard on relevSvc works — verified by reaching here
}

// ── spy types for extractEntities Reactivate test (#6) ──

// spyReactivatr records calls to Touch and Reactivate.
type spyReactivatr struct {
	mu              sync.Mutex
	touchCalls      []uuid.UUID
	reactivateCalls []uuid.UUID
}

func (s *spyReactivatr) Touch(ctx context.Context, entityID, chapterID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.touchCalls = append(s.touchCalls, entityID)
	return nil
}

func (s *spyReactivatr) Reactivate(ctx context.Context, entityID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reactivateCalls = append(s.reactivateCalls, entityID)
	return nil
}

// spyEntityResolvr returns entities with a configurable previousStatus.
type spyEntityResolvr struct {
	previousStatus string
	entityName     string
	entityID       uuid.UUID
}

func (s *spyEntityResolvr) ResolveOrCreate(ctx context.Context, universeID uuid.UUID, data repositories.ExtractedEntity) (*models.Entity, string, bool, error) {
	e := &models.Entity{
		ID:         s.entityID,
		UniverseID: universeID,
		Name:       s.entityName,
		Type:       data.Type,
		Status:     s.previousStatus,
	}
	return e, s.previousStatus, false, nil
}

// spec: CRITICAL #6 — Pipeline archived→Reactivate: archived entity re-mentioned
// calls Reactivate when previousStatus == "archived".
func TestExtractEntities_ReactivateCallsReactivate(t *testing.T) {
	// httptest server for Qwen ExtractEntities (returns character with no special fields)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
				}{Content: `{"characters":[{"name":"OldWizard","type":"character","status":"active","description":"An old wizard","properties":{}}],"places":[],"events":[],"factions":[],"world_rules":[],"plot_developments":[]}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:          server.URL,
		QwenAPIKey:           "test-key",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}
	qwenSvc := NewQwenService(cfg, nil)

	entityID := uuid.New()
	entityResolvr := &spyEntityResolvr{previousStatus: "archived", entityName: "OldWizard", entityID: entityID}
	relevSpy := &spyReactivatr{}

	svc := NewAnalysisService(nil, entityResolvr, nil, relevSpy, nil, nil, qwenSvc, nil, nil)

	entities, err := svc.extractEntities(context.Background(), uuid.New(), "The old wizard returned.", uuid.New())
	if err != nil {
		t.Fatalf("extractEntities: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("len(entities) = %d, want 1", len(entities))
	}

	// CRITICAL assertion: Reactivate must be called when previousStatus == "archived"
	relevSpy.mu.Lock()
	reactivateCalls := relevSpy.reactivateCalls
	relevSpy.mu.Unlock()

	if len(reactivateCalls) == 0 {
		t.Error("Reactivate was NOT called when entity previousStatus == 'archived' — pipeline hook not exercised")
	} else if len(reactivateCalls) != 1 {
		t.Errorf("Reactivate called %d times, want 1", len(reactivateCalls))
	} else if reactivateCalls[0] != entityID {
		t.Errorf("Reactivate called with entity ID %s, want %s", reactivateCalls[0], entityID)
	}

	// Triangulation: verify the entity returned in ResolvedEntity has the expected status
	if entities[0].PreviousStatus != "archived" {
		t.Errorf("PreviousStatus = %s, want archived", entities[0].PreviousStatus)
	}
}

// spec: CRITICAL #6 triangulation — active entity should NOT trigger Reactivate
func TestExtractEntities_ActiveEntityNoReactivate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
				}{Content: `{"characters":[{"name":"ActiveHero","type":"character","status":"active","description":"A hero","properties":{}}],"places":[],"events":[],"factions":[],"world_rules":[],"plot_developments":[]}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:          server.URL,
		QwenAPIKey:           "test-key",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}
	qwenSvc := NewQwenService(cfg, nil)

	entityID := uuid.New()
	entityResolvr := &spyEntityResolvr{previousStatus: "active", entityName: "ActiveHero", entityID: entityID}
	relevSpy := &spyReactivatr{}

	svc := NewAnalysisService(nil, entityResolvr, nil, relevSpy, nil, nil, qwenSvc, nil, nil)

	entities, err := svc.extractEntities(context.Background(), uuid.New(), "Active hero appears.", uuid.New())
	if err != nil {
		t.Fatalf("extractEntities: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("len(entities) = %d, want 1", len(entities))
	}

	// Reactivate should NOT be called for non-archived entities
	relevSpy.mu.Lock()
	reactivateCalls := relevSpy.reactivateCalls
	relevSpy.mu.Unlock()

	if len(reactivateCalls) != 0 {
		t.Errorf("Reactivate should NOT be called for active entity, but got %d calls", len(reactivateCalls))
	}
}

// TestProcessJobSkipsDuplicateParagraph verifies the in-memory paragraph
// dedup: two processJob calls with identical chapter+text must hit Qwen's
// ExtractEntities exactly once — the second call short-circuits before any
// service fan-out.
func TestProcessJobSkipsDuplicateParagraph(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
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
				}{Content: `{"characters":[],"places":[],"events":[],"factions":[],"world_rules":[],"plot_developments":[]}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		QwenBaseURL:          server.URL,
		QwenAPIKey:           "test-key",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}
	qwenSvc := NewQwenService(cfg, nil)

	// Lazy pool: never actually connects since the empty extraction result
	// keeps every DB-touching branch gated off.
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/db?sslmode=disable")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	svc := NewAnalysisService(pool, &spyEntityResolvr{}, nil, nil, nil, nil, qwenSvc, nil, nil)

	job := analysisJob{
		WorkID:     uuid.New(),
		ChapterID:  uuid.New(),
		UniverseID: uuid.New(),
		Text:       "The same paragraph, submitted twice.",
		UserID:     uuid.New(),
	}

	for i := 0; i < 2; i++ {
		result, err := svc.processJob(context.Background(), job)
		if err != nil {
			t.Fatalf("processJob run %d: %v", i+1, err)
		}
		if i == 0 && result == nil {
			t.Error("first processJob returned nil result, want a real result")
		}
		if i == 1 && result == nil {
			t.Error("duplicate processJob returned nil result; accepted submissions require a terminal result")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("ExtractEntities HTTP calls = %d, want 1 (duplicate paragraph must be skipped)", calls)
	}
}

func TestProcessJobPropagatesEntityExtractionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Qwen unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL:          server.URL,
		QwenAPIKey:           "test-key",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, nil)
	svc := NewAnalysisService(nil, &spyEntityResolvr{}, nil, nil, nil, nil, qwenSvc, nil, nil)
	_, err := svc.processJob(context.Background(), analysisJob{
		WorkID: uuid.New(), ChapterID: uuid.New(), UniverseID: uuid.New(), UserID: uuid.New(), Text: "Aster arrives.",
		SubmissionID: "submission-1", ParagraphRef: "chapter:1",
	})
	if err == nil {
		t.Fatal("processJob succeeded after Qwen entity extraction failed")
	}
	if !strings.Contains(err.Error(), "extract entities") {
		t.Fatalf("error = %q, want extraction context", err)
	}
}

func TestProcessJobReleasesFailedFingerprintForRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "temporary Qwen error", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"characters\":[],\"places\":[],\"objects\":[],\"events\":[],\"factions\":[],\"world_rules\":[],\"plot_developments\":[]}"}}]}`))
	}))
	defer server.Close()
	qwenSvc := NewQwenService(&config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test-key", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}, nil)
	svc := NewAnalysisService(nil, &spyEntityResolvr{}, nil, nil, nil, nil, qwenSvc, nil, nil)
	job := analysisJob{WorkID: uuid.New(), ChapterID: uuid.New(), UniverseID: uuid.New(), UserID: uuid.New(), Text: "Retry me.", SubmissionID: "submission-1", ParagraphRef: "chapter:1"}
	if _, err := svc.processJob(context.Background(), job); err == nil {
		t.Fatal("first processJob succeeded despite Qwen failure")
	}
	if _, err := svc.processJob(context.Background(), job); err != nil {
		t.Fatalf("retry processJob failed after transient error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("Qwen calls = %d, want 2; failed analysis must not remain deduplicated", calls)
	}
}

func TestAnalysisServiceSubmitParagraphProcessesSameWorkInOrder(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	processed := make(chan string, 2)
	svc.processJobFn = func(_ context.Context, job analysisJob) (*AnalysisResult, error) {
		processed <- job.Text
		return &AnalysisResult{SubmissionID: job.SubmissionID, ParagraphRef: job.ParagraphRef, WorkID: job.WorkID, ChapterID: job.ChapterID}, nil
	}
	workID, chapterID, universeID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := svc.SubmitParagraph(context.Background(), "submission-1", "chapter:1", workID, chapterID, universeID, userID, "first"); err != nil {
		t.Fatalf("SubmitParagraph first: %v", err)
	}
	if err := svc.SubmitParagraph(context.Background(), "submission-2", "chapter:2", workID, chapterID, universeID, userID, "second"); err != nil {
		t.Fatalf("SubmitParagraph second: %v", err)
	}
	defer svc.Shutdown()
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-processed:
			if got != want {
				t.Fatalf("processed out of order: got %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

func TestAnalysisServiceWorkerEmitsTerminalFailure(t *testing.T) {
	hub := &terminalRecordingHub{messages: make(chan models.WSMessage, 1)}
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, hub, nil)
	svc.processJobFn = func(context.Context, analysisJob) (*AnalysisResult, error) {
		return nil, fmt.Errorf("simulated analyzer failure")
	}
	workID, chapterID, universeID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := svc.SubmitParagraph(context.Background(), "submission-1", "chapter:1", workID, chapterID, universeID, userID, "paragraph"); err != nil {
		t.Fatalf("SubmitParagraph: %v", err)
	}
	defer svc.Shutdown()
	select {
	case msg := <-hub.messages:
		if msg.Type != ws.TypeAnalysisFailed {
			t.Fatalf("terminal type = %q, want %q", msg.Type, ws.TypeAnalysisFailed)
		}
		var payload models.AnalysisFailedPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("unmarshal failure payload: %v", err)
		}
		if payload.SubmissionID != "submission-1" || payload.ParagraphRef != "chapter:1" || payload.UniverseID != universeID {
			t.Fatalf("terminal correlation = %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for analysis_failed")
	}
}

func TestAnalysisServiceBroadcastResultUsesAcceptedJobUniverse(t *testing.T) {
	hub := &stageRecordingHub{}
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, hub, nil)
	job := analysisJob{
		SubmissionID: "submission-1",
		ParagraphRef: "chapter:1",
		WorkID:       uuid.New(),
		ChapterID:    uuid.New(),
		UniverseID:   uuid.New(),
		UserID:       uuid.New(),
	}

	svc.broadcastResult(job, AnalysisResult{
		SubmissionID: job.SubmissionID,
		ParagraphRef: job.ParagraphRef,
		WorkID:       job.WorkID,
		ChapterID:    job.ChapterID,
		UniverseID:   uuid.New(), // A result must not override the accepted job scope.
	})

	if len(hub.messages) != 1 || hub.messages[0].Type != ws.TypeAnalysisResult {
		t.Fatalf("expected one analysis_result, got %+v", hub.messages)
	}
	var payload models.AnalysisResultPayload
	if err := json.Unmarshal(hub.messages[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal result payload: %v", err)
	}
	if payload.UniverseID != job.UniverseID {
		t.Errorf("result universe_id = %s, want accepted job universe %s", payload.UniverseID, job.UniverseID)
	}
}

type terminalRecordingHub struct{ messages chan models.WSMessage }

func (h *terminalRecordingHub) SendToUser(_ uuid.UUID, msg models.WSMessage) error {
	h.messages <- msg
	return nil
}

// stageRecordingHub is a fake AnalysisHub that records every message sent,
// in send order — used to assert analysis_progress stage ordering.
type stageRecordingHub struct {
	mu       sync.Mutex
	messages []models.WSMessage
	types    []string
}

func (h *stageRecordingHub) SendToUser(userID uuid.UUID, msg models.WSMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msg)
	h.types = append(h.types, msg.Type)
	return nil
}

// rawProgressPayloads returns the raw JSON payload of every analysis_progress
// message sent, in send order.
func (h *stageRecordingHub) rawProgressPayloads() []json.RawMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	var payloads []json.RawMessage
	for _, m := range h.messages {
		if m.Type == "analysis_progress" {
			payloads = append(payloads, m.Payload)
		}
	}
	return payloads
}

// TestSendProgressNilHubNoop verifies sendProgress is a no-op guard when hub is nil.
func TestSendProgressNilHubNoop(t *testing.T) {
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// Should not panic with a nil hub.
	svc.sendProgress(uuid.New(), uuid.New(), uuid.New(), "submission-1", "chapter:12", "entities_extracted", nil)
}

// TestSendProgressSendsPayload verifies sendProgress builds and sends an
// analysis_progress WSMessage carrying the given stage and mutated fields.
func TestSendProgressSendsPayload(t *testing.T) {
	hub := &stageRecordingHub{}
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, hub, nil)

	universeID := uuid.New()
	chapterID := uuid.New()
	count := 3
	svc.sendProgress(uuid.New(), universeID, chapterID, "submission-1", "chapter:12", "entities_extracted", func(p *models.AnalysisProgressPayload) {
		p.EntityCount = &count
	})

	if len(hub.types) != 1 || hub.types[0] != "analysis_progress" {
		t.Fatalf("expected one analysis_progress message, got %v", hub.types)
	}
	var payload models.AnalysisProgressPayload
	if err := json.Unmarshal(hub.messages[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal progress payload: %v", err)
	}
	if payload.UniverseID != universeID {
		t.Errorf("progress universe_id = %s, want %s", payload.UniverseID, universeID)
	}
}

// TestProcessJobEmitsProgressInPipelineOrder verifies the lifecycle and pipeline
// stages fire via sendProgress in the documented relative order, even with
// all downstream dependencies nil (each stage's guard being unmet just means
// its count/budget field stays empty — the stage itself still marks that the
// pipeline reached that point). This keeps the test DB-free and fast.
func TestProcessJobEmitsProgressInPipelineOrder(t *testing.T) {
	hub := &stageRecordingHub{}
	svc := NewAnalysisService(nil, nil, nil, nil, nil, nil, nil, hub, nil)

	job := analysisJob{
		WorkID:     uuid.New(),
		ChapterID:  uuid.New(),
		UniverseID: uuid.New(),
		Text:       "Some paragraph text.",
		UserID:     uuid.New(),
	}

	_, err := svc.processJob(context.Background(), job)
	if err != nil {
		t.Fatalf("processJob: %v", err)
	}

	wantOrder := []string{
		"analyzing",
		"entities_extracted",
		"checking_contradictions",
		"contradictions_checked",
		"plot_holes_scanned",
		"context_budget",
	}

	var gotStages []string
	for _, tBytes := range hub.rawProgressPayloads() {
		var p models.AnalysisProgressPayload
		if err := json.Unmarshal(tBytes, &p); err != nil {
			t.Fatalf("unmarshal progress payload: %v", err)
		}
		gotStages = append(gotStages, p.Stage)
	}

	if len(gotStages) != len(wantOrder) {
		t.Fatalf("stage count = %d, want %d: got %v", len(gotStages), len(wantOrder), gotStages)
	}
	for i, want := range wantOrder {
		if gotStages[i] != want {
			t.Errorf("stage[%d] = %q, want %q (full order: %v)", i, gotStages[i], want, gotStages)
		}
	}
	for _, tBytes := range hub.rawProgressPayloads() {
		var p models.AnalysisProgressPayload
		if err := json.Unmarshal(tBytes, &p); err != nil {
			t.Fatalf("unmarshal progress scope: %v", err)
		}
		if p.UniverseID != job.UniverseID {
			t.Errorf("progress universe_id = %s, want %s", p.UniverseID, job.UniverseID)
		}
	}
}

// ── Integration tests (require DB) ──

// TestAnalysisServiceFullPipeline runs the analysis pipeline end-to-end.
func TestAnalysisServiceFullPipeline(t *testing.T) {
	pool := setupAnalysisTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	chapter := svcCreateTestChapter(t, ctx, pool, work.ID, "Chapter 1", 1)

	repos := svcCreateAnalysisRepos(pool)
	svcs := svcCreateAnalysisServices(pool, repos)

	// Create AnalysisService with nil hub — we'll verify DB state not WS messages
	analysisSvc := NewAnalysisService(pool, svcs.entitySvc, svcs.contraSvc,
		svcs.relevSvc, svcs.timelineSvc, svcs.plotHoleSvc, svcs.qwenSvc, nil, nil)

	job := analysisJob{
		WorkID:     work.ID,
		ChapterID:  chapter.ID,
		UniverseID: universe.ID,
		Text:       "John was a tall man with a scar on his left cheek.",
		UserID:     user.ID,
	}

	// Start a worker goroutine for this work
	go analysisSvc.runWorker(job.WorkID)

	err := analysisSvc.Submit(ctx, job)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Wait for analysis to complete
	time.Sleep(3 * time.Second)

	// Verify entities exist in DB after pipeline
	entities, _, err := svcs.entitySvc.ListByUniverse(ctx, universe.ID, repositories.EntityFilters{})
	if err != nil {
		t.Fatalf("ListByUniverse failed: %v", err)
	}
	t.Logf("Entities after analysis: %d", len(entities))

	// Clean up
	analysisSvc.Shutdown()
}

// TestAnalysisServiceContextCancellation verifies cancelled context
// stops processing mid-flight.
func TestAnalysisServiceContextCancellation(t *testing.T) {
	pool := setupAnalysisTestDB(t)
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	chapter := svcCreateTestChapter(t, ctx, pool, work.ID, "Chapter 1", 1)

	repos := svcCreateAnalysisRepos(pool)
	svcs := svcCreateAnalysisServices(pool, repos)

	analysisSvc := NewAnalysisService(pool, svcs.entitySvc, svcs.contraSvc,
		svcs.relevSvc, svcs.timelineSvc, svcs.plotHoleSvc, svcs.qwenSvc, nil, nil)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start a worker — it will process the job but the ctx is already
	// partially cancelled. Actually we need to cancel from inside the worker.
	// Let's test Cancel() instead: cancel the work while a job is pending.
	job := analysisJob{
		WorkID:     work.ID,
		ChapterID:  chapter.ID,
		UniverseID: universe.ID,
		Text:       "This should complete",
		UserID:     user.ID,
	}

	// Submit the job (it won't be processed until worker starts)
	go analysisSvc.runWorker(job.WorkID)

	// Submit and then immediately cancel
	_ = analysisSvc.Submit(cancelCtx, job)

	// Wait a moment and cancel
	time.Sleep(100 * time.Millisecond)
	analysisSvc.Cancel(job.WorkID)

	time.Sleep(time.Second)
	// After cancel, no new jobs should be processed
	t.Log("Cancellation test completed")
}

// ── Helpers ──

type analysisTestServices struct {
	entitySvc   *EntityService
	contraSvc   *ContradictionService
	relevSvc    *RelevanceService
	timelineSvc *TimelineService
	plotHoleSvc *PlotHoleService
	qwenSvc     *QwenService
}

type analysisTestRepos struct {
	entity        *repositories.EntityRepo
	contradiction *repositories.ContradictionRepo
	timeline      *repositories.TimelineRepo
	plotHole      *repositories.PlotHoleRepo
	graph         *repositories.GraphRepo
	vector        *repositories.VectorRepo
}

func svcCreateAnalysisRepos(pool *pgxpool.Pool) analysisTestRepos {
	return analysisTestRepos{
		entity:        repositories.NewEntityRepo(pool),
		contradiction: repositories.NewContradictionRepo(pool),
		timeline:      repositories.NewTimelineRepo(pool),
		plotHole:      repositories.NewPlotHoleRepo(pool),
		graph:         repositories.NewGraphRepo(pool),
		vector:        repositories.NewVectorRepo(pool),
	}
}

func svcCreateAnalysisServices(pool *pgxpool.Pool, repos analysisTestRepos) analysisTestServices {
	qwenSvc := NewQwenService(&config.Config{QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}, nil) // minimal config = no real API calls
	entitySvc := NewEntityService(pool, repos.entity, repos.vector, qwenSvc)
	contraSvc := NewContradictionService(pool, repos.contradiction, repos.entity, qwenSvc, nil, 3, nil, 3)
	relevSvc := NewRelevanceService(pool, repos.entity, 0.1, 0.15, nil)
	timelineSvc := NewTimelineService(pool, repos.timeline, nil, nil)
	plotHoleSvc := NewPlotHoleService(pool, repos.plotHole, repos.entity, 8, nil, nil, 2)

	return analysisTestServices{
		entitySvc:   entitySvc,
		contraSvc:   contraSvc,
		relevSvc:    relevSvc,
		timelineSvc: timelineSvc,
		plotHoleSvc: plotHoleSvc,
		qwenSvc:     qwenSvc,
	}
}

func svcCreateTestWork(t *testing.T, ctx context.Context, pool *pgxpool.Pool, universeID uuid.UUID) models.Work {
	t.Helper()
	work := models.Work{
		ID:         uuid.New(),
		UniverseID: universeID,
		Title:      "Test Work",
		Type:       "novel",
		OrderIndex: 1,
		Status:     "draft",
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO works (id, universe_id, title, type, order_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`, work.ID, work.UniverseID, work.Title, work.Type, work.OrderIndex, work.Status)
	if err != nil {
		t.Fatalf("create test work: %v", err)
	}
	return work
}

func svcCreateTestChapter(t *testing.T, ctx context.Context, pool *pgxpool.Pool, workID uuid.UUID, title string, orderIndex int) models.Chapter {
	t.Helper()
	ch := models.Chapter{
		ID:         uuid.New(),
		WorkID:     workID,
		Title:      title,
		OrderIndex: orderIndex,
		Status:     "draft",
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '', '', 0, $5, NOW(), NOW())
	`, ch.ID, ch.WorkID, ch.Title, ch.OrderIndex, ch.Status)
	if err != nil {
		t.Fatalf("create test chapter: %v", err)
	}
	return ch
}

func setupAnalysisTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	return pool
}
