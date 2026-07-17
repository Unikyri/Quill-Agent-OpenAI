package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

var (
	allowedGenreTags = map[string]struct{}{
		"fantasy": {}, "epic-fantasy": {}, "urban-fantasy": {}, "romantasy": {},
		"science-fiction": {}, "space-opera": {}, "dystopian": {}, "horror": {},
		"gothic": {}, "paranormal": {}, "romance": {}, "mystery": {},
		"cozy-mystery": {}, "thriller": {}, "crime": {}, "historical": {},
		"literary": {}, "adventure": {}, "young-adult": {}, "coming-of-age": {},
	}
)

var ErrUniverseAccessDenied = errors.New("universe access denied")

func validateUniverseEnums(input models.CreateUniverseRequest) error {
	for _, tag := range input.GenreTags {
		if _, ok := allowedGenreTags[tag]; !ok {
			return fmt.Errorf("invalid genre tag %q: must be one of %s", tag, joinKeys(allowedGenreTags))
		}
	}
	return nil
}

func joinKeys(m map[string]struct{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

type UniverseService struct {
	pool          *pgxpool.Pool
	universeRepo  *repositories.UniverseRepo
	graphRepo     *repositories.GraphRepo
	skillRepo     *repositories.SkillRepo
	skillRegistry *SkillRegistry
}

func NewUniverseService(pool *pgxpool.Pool, universeRepo *repositories.UniverseRepo, graphRepo *repositories.GraphRepo) *UniverseService {
	return &UniverseService{
		pool:         pool,
		universeRepo: universeRepo,
		graphRepo:    graphRepo,
	}
}

// SetSkillActivation wires optional per-universe skill persistence. Keeping it
// as a setter preserves existing service/test constructors while allowing
// universe creation to seed the registry defaults transactionally.
func (s *UniverseService) SetSkillActivation(repo *repositories.SkillRepo, registry *SkillRegistry) {
	s.skillRepo = repo
	s.skillRegistry = registry
}

func (s *UniverseService) Create(ctx context.Context, userID uuid.UUID, input models.CreateUniverseRequest) (*models.Universe, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("universe name is required")
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
		GenreTags:   input.GenreTags,
	}

	if err := s.universeRepo.Create(ctx, tx, u); err != nil {
		return nil, err
	}
	if s.skillRepo != nil && s.skillRegistry != nil {
		if err := s.skillRepo.ReplaceTx(ctx, tx, u.ID, s.skillRegistry.DefaultSkillNames()); err != nil {
			return nil, err
		}
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
	if input.GenreTags != nil {
		u.GenreTags = input.GenreTags
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

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Best-effort: drop the universe's AGE graph so it doesn't leak into
	// ag_catalog forever. The relational delete already committed.
	if s.graphRepo != nil {
		if err := s.graphRepo.DropGraph(ctx, "universe_"+id.String()); err != nil {
			log.Printf("[universe] drop graph for %s: %v", id, err)
		}
	}
	return nil
}

func (s *UniverseService) ListSkills(ctx context.Context, userID, universeID uuid.UUID) ([]models.UniverseSkill, error) {
	if err := s.authorizeUniverse(ctx, userID, universeID); err != nil {
		return nil, err
	}
	if s.skillRepo == nil {
		return []models.UniverseSkill{}, nil
	}
	return s.skillRepo.ListActive(ctx, universeID)
}

func (s *UniverseService) ReplaceSkills(ctx context.Context, userID, universeID uuid.UUID, names []string) ([]models.UniverseSkill, error) {
	if err := s.authorizeUniverse(ctx, userID, universeID); err != nil {
		return nil, err
	}
	if s.skillRepo == nil || s.skillRegistry == nil {
		return nil, errors.New("skill activation is not configured")
	}
	validated, err := s.skillRegistry.ValidateNames(names)
	if err != nil {
		return nil, err
	}
	if err := s.skillRepo.Replace(ctx, universeID, validated); err != nil {
		return nil, err
	}
	return s.skillRepo.ListActive(ctx, universeID)
}

func (s *UniverseService) authorizeUniverse(ctx context.Context, userID, universeID uuid.UUID) error {
	u, err := s.universeRepo.FindByID(ctx, universeID)
	if err != nil {
		return err
	}
	if u == nil || u.UserID != userID {
		return ErrUniverseAccessDenied
	}
	return nil
}
