package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

type WorkService struct {
	pool     *pgxpool.Pool
	workRepo *repositories.WorkRepo
}

func NewWorkService(pool *pgxpool.Pool, workRepo *repositories.WorkRepo) *WorkService {
	return &WorkService{
		pool:     pool,
		workRepo: workRepo,
	}
}

func (s *WorkService) Create(ctx context.Context, universeID uuid.UUID, input models.CreateWorkRequest) (*models.Work, error) {
	if input.Title == "" {
		return nil, fmt.Errorf("work title is required")
	}
	if input.Type == "" {
		return nil, fmt.Errorf("work type is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	maxOrder, err := s.workRepo.GetMaxOrderIndex(ctx, universeID)
	if err != nil {
		return nil, err
	}

	w := &models.Work{
		ID:         uuid.New(),
		UniverseID: universeID,
		Title:      input.Title,
		Type:       input.Type,
		OrderIndex: maxOrder + 1,
		Synopsis:   input.Synopsis,
		Status:     "in_progress",
	}

	if err := s.workRepo.Create(ctx, tx, w); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return w, nil
}

func (s *WorkService) GetByID(ctx context.Context, id uuid.UUID) (*models.Work, error) {
	return s.workRepo.FindByID(ctx, id)
}

func (s *WorkService) ListByUniverse(ctx context.Context, universeID uuid.UUID) ([]models.Work, error) {
	return s.workRepo.ListByUniverse(ctx, universeID)
}

func (s *WorkService) Update(ctx context.Context, id uuid.UUID, input models.CreateWorkRequest) (*models.Work, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	w, err := s.workRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Title != "" {
		w.Title = input.Title
	}
	if input.Type != "" {
		w.Type = input.Type
	}
	if input.Synopsis != "" {
		w.Synopsis = input.Synopsis
	}

	if err := s.workRepo.Update(ctx, tx, w); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return w, nil
}

func (s *WorkService) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.workRepo.Delete(ctx, tx, id); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
