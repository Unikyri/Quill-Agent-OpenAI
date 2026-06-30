package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/repositories"
)

type DemoService struct {
	pool         *pgxpool.Pool
	universeRepo *repositories.UniverseRepo
}

func NewDemoService(pool *pgxpool.Pool, universeRepo *repositories.UniverseRepo) *DemoService {
	return &DemoService{
		pool:         pool,
		universeRepo: universeRepo,
	}
}

func (s *DemoService) CloneUniverse(ctx context.Context, sessionID string) (string, error) {
	// Check if user already has a demo universe for this session
	existing, err := s.universeRepo.FindBySessionID(ctx, sessionID)
	if err == nil && existing != nil {
		return existing.ID.String(), nil
	}

	// Find the demo template
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get template universe
	templateID := ""
	err = tx.QueryRow(ctx, `SELECT id FROM universes WHERE is_demo_template = TRUE LIMIT 1`).Scan(&templateID)
	if err != nil {
		return "", fmt.Errorf("no demo template found: %w", err)
	}

	// Clone the universe
	newID := uuid.New().String()
	_, err = tx.Exec(ctx, `
		INSERT INTO universes (id, user_id, name, description, genre, format, session_id, is_demo_template, created_at, updated_at)
		SELECT $1, user_id, name, description, genre, format, $2, FALSE, NOW(), NOW()
		FROM universes WHERE id = $3
	`, newID, sessionID, templateID)
	if err != nil {
		return "", fmt.Errorf("clone universe: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	return newID, nil
}

func (s *DemoService) ResetUniverse(ctx context.Context, sessionID string) (string, error) {
	u, err := s.universeRepo.FindBySessionID(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("universe not found for session")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete the session's universe
	_, err = tx.Exec(ctx, `DELETE FROM universes WHERE id = $1`, u.ID)
	if err != nil {
		return "", fmt.Errorf("delete universe: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	// Re-clone
	return s.CloneUniverse(ctx, sessionID)
}
