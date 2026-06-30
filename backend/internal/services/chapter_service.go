package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

type ChapterService struct {
	pool        *pgxpool.Pool
	chapterRepo *repositories.ChapterRepo
}

func NewChapterService(pool *pgxpool.Pool, chapterRepo *repositories.ChapterRepo) *ChapterService {
	return &ChapterService{
		pool:        pool,
		chapterRepo: chapterRepo,
	}
}

func (s *ChapterService) Create(ctx context.Context, workID uuid.UUID, input models.CreateChapterRequest) (*models.Chapter, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	maxOrder, err := s.chapterRepo.GetMaxOrderIndex(ctx, workID)
	if err != nil {
		return nil, err
	}

	c := &models.Chapter{
		ID:         uuid.New(),
		WorkID:     workID,
		Title:      input.Title,
		OrderIndex: maxOrder + 1,
		Content:    "",
		RawText:    "",
		WordCount:  0,
		Status:     "draft",
	}

	if err := s.chapterRepo.Create(ctx, tx, c); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return c, nil
}

func (s *ChapterService) GetByID(ctx context.Context, id uuid.UUID) (*models.Chapter, error) {
	return s.chapterRepo.FindByID(ctx, id)
}

func (s *ChapterService) ListByWork(ctx context.Context, workID uuid.UUID) ([]models.Chapter, error) {
	return s.chapterRepo.ListByWork(ctx, workID)
}

func (s *ChapterService) Update(ctx context.Context, id uuid.UUID, input models.UpdateChapterRequest) (*models.Chapter, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	c, err := s.chapterRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Title != "" {
		c.Title = input.Title
	}
	if input.Content != "" {
		c.Content = input.Content
		c.WordCount = s.chapterRepo.CountWords(input.Content)
	}
	if input.RawText != "" {
		c.RawText = input.RawText
	}

	if err := s.chapterRepo.Update(ctx, tx, c); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return c, nil
}

func (s *ChapterService) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.chapterRepo.Delete(ctx, tx, id); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
