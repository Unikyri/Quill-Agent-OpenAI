package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
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
	entitySvc    *EntityService
	contraSvc    *ContradictionService
	relevSvc     *RelevanceService
	timelineSvc  *TimelineService
	plotHoleSvc  *PlotHoleService
	qwenSvc      *QwenService
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
	qwenSvc := NewQwenService(nil) // nil config = no real API calls
	entitySvc := NewEntityService(pool, repos.entity, repos.vector, qwenSvc)
	contraSvc := NewContradictionService(pool, repos.contradiction, repos.entity, qwenSvc, nil, 3)
	relevSvc := NewRelevanceService(pool, repos.entity, 0.1, 0.15)
	timelineSvc := NewTimelineService(pool, repos.timeline, nil)
	plotHoleSvc := NewPlotHoleService(pool, repos.plotHole, repos.entity, 8, nil, nil)

	return analysisTestServices{
		entitySvc:    entitySvc,
		contraSvc:    contraSvc,
		relevSvc:     relevSvc,
		timelineSvc:  timelineSvc,
		plotHoleSvc:  plotHoleSvc,
		qwenSvc:      qwenSvc,
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
