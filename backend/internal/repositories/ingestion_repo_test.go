package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/testutil"
)

// TestIngestionRepoCRUD verifies Create, FindByID, and UpdateStatus
// against the ingestion_jobs table (migration 012).
func TestIngestionRepoCRUD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "012")

	repo := NewIngestionRepo(pool)
	ctx := context.Background()

	jobID := uuid.New()
	universeID := uuid.New()
	workID := uuid.New()
	filename := "test_document.md"

	// Create
	err := repo.Create(ctx, jobID, universeID, workID, "pending", filename)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// FindByID
	job, err := repo.FindByID(ctx, jobID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if job.ID != jobID {
		t.Errorf("ID: got %s, want %s", job.ID, jobID)
	}
	if job.UniverseID != universeID {
		t.Errorf("UniverseID: got %s, want %s", job.UniverseID, universeID)
	}
	if job.WorkID != workID {
		t.Errorf("WorkID: got %s, want %s", job.WorkID, workID)
	}
	if job.Filename != filename {
		t.Errorf("Filename: got %q, want %q", job.Filename, filename)
	}
	if job.Status != "pending" {
		t.Errorf("Status: got %q, want %q", job.Status, "pending")
	}

	// UpdateStatus — running
	now := time.Now().UTC()
	err = repo.UpdateStatus(ctx, jobID, "running", "")
	if err != nil {
		t.Fatalf("UpdateStatus running: %v", err)
	}
	job, err = repo.FindByID(ctx, jobID)
	if err != nil {
		t.Fatalf("FindByID after running update: %v", err)
	}
	if job.Status != "running" {
		t.Errorf("Status after running: got %q, want %q", job.Status, "running")
	}
	if job.StartedAt == nil || job.StartedAt.Before(now.Add(-time.Second)) {
		t.Errorf("StartedAt should be set: %v", job.StartedAt)
	}

	// UpdateStatus — completed
	err = repo.UpdateStatus(ctx, jobID, "completed", "")
	if err != nil {
		t.Fatalf("UpdateStatus completed: %v", err)
	}
	job, err = repo.FindByID(ctx, jobID)
	if err != nil {
		t.Fatalf("FindByID after completed update: %v", err)
	}
	if job.Status != "completed" {
		t.Errorf("Status after completed: got %q, want %q", job.Status, "completed")
	}
	if job.CompletedAt == nil || job.CompletedAt.Before(now.Add(-time.Second)) {
		t.Errorf("CompletedAt should be set: %v", job.CompletedAt)
	}

	// UpdateStatus — failed with error message
	errMsg := "parse error on line 42"
	err = repo.UpdateStatus(ctx, jobID, "failed", errMsg)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	job, err = repo.FindByID(ctx, jobID)
	if err != nil {
		t.Fatalf("FindByID after failed update: %v", err)
	}
	if job.Status != "failed" {
		t.Errorf("Status after failed: got %q, want %q", job.Status, "failed")
	}
	if job.ErrorMessage != errMsg {
		t.Errorf("ErrorMessage: got %q, want %q", job.ErrorMessage, errMsg)
	}
}

// TestIngestionRepoFindByIDNotFound verifies FindByID returns an error
// for non-existent jobs.
func TestIngestionRepoFindByIDNotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "012")

	repo := NewIngestionRepo(pool)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Error("expected error for non-existent job, got nil")
	}
}
