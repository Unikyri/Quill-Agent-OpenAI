package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

// IngestionRepo manages the ingestion_jobs table (migration 012).
type IngestionRepo struct {
	pool *pgxpool.Pool
}

// NewIngestionRepo creates a new IngestionRepo backed by the connection pool.
func NewIngestionRepo(pool *pgxpool.Pool) *IngestionRepo {
	return &IngestionRepo{pool: pool}
}

// Create inserts a new ingestion job with the given initial status.
// contentHash may be empty (stored as NULL, exempt from the dedup index).
// fileType may be empty (stored as NULL).
func (r *IngestionRepo) Create(ctx context.Context, jobID, universeID, workID uuid.UUID, status, filename, fileType, contentHash string) error {
	query := `
		INSERT INTO ingestion_jobs (id, universe_id, work_id, filename, file_type, status, content_hash, created_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), NOW())
	`
	_, err := r.pool.Exec(ctx, query, jobID, universeID, workID, filename, fileType, status, contentHash)
	if err != nil {
		return fmt.Errorf("create ingestion job: %w", err)
	}
	return nil
}

// FindByContentHash returns the most recent non-failed job for the given
// universe and content hash, or (nil, nil) when no such job exists.
func (r *IngestionRepo) FindByContentHash(ctx context.Context, universeID uuid.UUID, hash string) (*models.IngestionJob, error) {
	query := `
		SELECT id, universe_id, work_id, filename, COALESCE(file_type, ''),
		       status, total_chapters_detected, chapters_processed,
		       entities_extracted, COALESCE(error_message, ''), started_at, completed_at, created_at
		FROM ingestion_jobs
		WHERE universe_id = $1 AND content_hash = $2 AND status <> 'failed'
		ORDER BY created_at DESC
		LIMIT 1
	`
	job := &models.IngestionJob{}
	err := r.pool.QueryRow(ctx, query, universeID, hash).Scan(
		&job.ID,
		&job.UniverseID,
		&job.WorkID,
		&job.Filename,
		&job.FileType,
		&job.Status,
		&job.TotalChaptersDetected,
		&job.ChaptersProcessed,
		&job.EntitiesExtracted,
		&job.ErrorMessage,
		&job.StartedAt,
		&job.CompletedAt,
		&job.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find ingestion job by hash: %w", err)
	}
	return job, nil
}

// ListByUniverse returns the 50 most recent ingestion jobs for a universe.
func (r *IngestionRepo) ListByUniverse(ctx context.Context, universeID uuid.UUID) ([]models.IngestionJob, error) {
	query := `
		SELECT id, universe_id, work_id, filename, COALESCE(file_type, ''),
		       status, total_chapters_detected, chapters_processed,
		       entities_extracted, COALESCE(error_message, ''), started_at, completed_at, created_at
		FROM ingestion_jobs
		WHERE universe_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`
	rows, err := r.pool.Query(ctx, query, universeID)
	if err != nil {
		return nil, fmt.Errorf("list ingestion jobs: %w", err)
	}
	defer rows.Close()

	jobs := []models.IngestionJob{}
	for rows.Next() {
		var job models.IngestionJob
		if err := rows.Scan(
			&job.ID,
			&job.UniverseID,
			&job.WorkID,
			&job.Filename,
			&job.FileType,
			&job.Status,
			&job.TotalChaptersDetected,
			&job.ChaptersProcessed,
			&job.EntitiesExtracted,
			&job.ErrorMessage,
			&job.StartedAt,
			&job.CompletedAt,
			&job.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ingestion job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// UpdateProgress persists the chapter/entity counters for a job.
func (r *IngestionRepo) UpdateProgress(ctx context.Context, jobID uuid.UUID, totalDetected, processed, entities int) error {
	query := `
		UPDATE ingestion_jobs
		SET total_chapters_detected = $2, chapters_processed = $3, entities_extracted = $4
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, jobID, totalDetected, processed, entities)
	if err != nil {
		return fmt.Errorf("update ingestion job progress: %w", err)
	}
	return nil
}

// FindByID fetches a single ingestion job by its ID.
func (r *IngestionRepo) FindByID(ctx context.Context, jobID uuid.UUID) (*models.IngestionJob, error) {
	query := `
		SELECT id, universe_id, work_id, filename, COALESCE(file_type, ''),
		       status, total_chapters_detected, chapters_processed,
		       entities_extracted, COALESCE(error_message, ''), started_at, completed_at, created_at
		FROM ingestion_jobs
		WHERE id = $1
	`
	job := &models.IngestionJob{}
	err := r.pool.QueryRow(ctx, query, jobID).Scan(
		&job.ID,
		&job.UniverseID,
		&job.WorkID,
		&job.Filename,
		&job.FileType,
		&job.Status,
		&job.TotalChaptersDetected,
		&job.ChaptersProcessed,
		&job.EntitiesExtracted,
		&job.ErrorMessage,
		&job.StartedAt,
		&job.CompletedAt,
		&job.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("ingestion job not found: %s", jobID)
	}
	if err != nil {
		return nil, fmt.Errorf("find ingestion job: %w", err)
	}
	return job, nil
}

// UpdateStatus updates the job status and optionally sets an error message.
// When transitioning to running, started_at is set. When transitioning to
// completed or failed, completed_at is set.
func (r *IngestionRepo) UpdateStatus(ctx context.Context, jobID uuid.UUID, status, errorMsg string) error {
	now := time.Now().UTC()

	query := `
		UPDATE ingestion_jobs
		SET status = $2,
		    error_message = CASE WHEN $3 = '' THEN error_message ELSE $3 END,
		    started_at   = CASE WHEN $2::varchar = 'running' THEN $4 ELSE started_at END,
		    completed_at = CASE WHEN $2::varchar IN ('completed', 'failed') THEN $4 ELSE completed_at END
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, jobID, status, errorMsg, now)
	if err != nil {
		return fmt.Errorf("update ingestion job status: %w", err)
	}
	return nil
}
