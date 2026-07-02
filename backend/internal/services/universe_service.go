package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

var (
	allowedGenres = map[string]struct{}{
		"sci-fi":      {},
		"fantasy":     {},
		"mystery":     {},
		"romance":     {},
		"horror":      {},
		"non-fiction": {},
		"thriller":    {},
		"historical":  {},
		"adventure":   {},
		"comedy":      {},
		"drama":       {},
	}

	allowedFormats = map[string]struct{}{
		"novel":         {},
		"short-story":   {},
		"screenplay":    {},
		"poetry":        {},
		"essay":         {},
		"article":       {},
		"graphic-novel": {},
	}
)

func validateUniverseEnums(input models.CreateUniverseRequest) error {
	if input.Genre != "" {
		if _, ok := allowedGenres[input.Genre]; !ok {
			return fmt.Errorf("invalid genre %q: must be one of %s", input.Genre, joinKeys(allowedGenres))
		}
	}
	if input.Format != "" {
		if _, ok := allowedFormats[input.Format]; !ok {
			return fmt.Errorf("invalid format %q: must be one of %s", input.Format, joinKeys(allowedFormats))
		}
	}
	return nil
}

func joinKeys(m map[string]struct{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

type UniverseService struct {
	pool         *pgxpool.Pool
	universeRepo *repositories.UniverseRepo
	graphRepo    *repositories.GraphRepo
}

func NewUniverseService(pool *pgxpool.Pool, universeRepo *repositories.UniverseRepo, graphRepo *repositories.GraphRepo) *UniverseService {
	return &UniverseService{
		pool:         pool,
		universeRepo: universeRepo,
		graphRepo:    graphRepo,
	}
}

func (s *UniverseService) Create(ctx context.Context, userID uuid.UUID, input models.CreateUniverseRequest) (*models.Universe, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("universe name is required")
	}
	if input.Format == "" {
		return nil, fmt.Errorf("universe format is required")
	}
	if err := validateUniverseEnums(input); err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	u := &models.Universe{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        input.Name,
		Description: input.Description,
		Genre:       input.Genre,
		Format:      input.Format,
	}

	if err := s.universeRepo.Create(ctx, tx, u); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Create AGE graph for the new universe
	if s.graphRepo != nil {
		if err := s.graphRepo.CreateGraph(ctx, u.ID.String()); err != nil {
			log.Printf("[universe] create AGE graph for %s: %v", u.ID, err)
			// non-fatal — the graph can be created later
		}
	}

	return u, nil
}

func (s *UniverseService) GetByID(ctx context.Context, id uuid.UUID) (*models.Universe, error) {
	return s.universeRepo.FindByID(ctx, id)
}

func (s *UniverseService) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]models.Universe, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.universeRepo.ListByUser(ctx, userID, page, limit)
}

func (s *UniverseService) Update(ctx context.Context, id uuid.UUID, input models.CreateUniverseRequest) (*models.Universe, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	u, err := s.universeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := validateUniverseEnums(input); err != nil {
		return nil, err
	}

	if input.Name != "" {
		u.Name = input.Name
	}
	if input.Description != "" {
		u.Description = input.Description
	}
	if input.Genre != "" {
		u.Genre = input.Genre
	}
	if input.Format != "" {
		u.Format = input.Format
	}

	if err := s.universeRepo.Update(ctx, tx, u); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return u, nil
}

func (s *UniverseService) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.universeRepo.Delete(ctx, tx, id); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
