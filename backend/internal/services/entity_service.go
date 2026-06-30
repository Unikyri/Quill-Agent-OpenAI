package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

type EntityService struct {
	pool       *pgxpool.Pool
	entityRepo *repositories.EntityRepo
	vectorRepo *repositories.VectorRepo
	qwenSvc    *QwenService
}

func NewEntityService(pool *pgxpool.Pool, entityRepo *repositories.EntityRepo, vectorRepo *repositories.VectorRepo, qwenSvc *QwenService) *EntityService {
	return &EntityService{
		pool:       pool,
		entityRepo: entityRepo,
		vectorRepo: vectorRepo,
		qwenSvc:    qwenSvc,
	}
}

func (s *EntityService) GetByID(ctx context.Context, id uuid.UUID) (*models.Entity, error) {
	return s.entityRepo.FindByID(ctx, id)
}

func (s *EntityService) ListByUniverse(ctx context.Context, universeID uuid.UUID, filters repositories.EntityFilters) ([]models.Entity, int, error) {
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.Limit < 1 || filters.Limit > 100 {
		filters.Limit = 50
	}
	return s.entityRepo.ListByUniverse(ctx, universeID, filters)
}

func (s *EntityService) Update(ctx context.Context, id uuid.UUID, input models.UpdateEntityRequest) (*models.Entity, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	e, err := s.entityRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		e.Name = input.Name
	}
	if len(input.Aliases) > 0 {
		e.Aliases = input.Aliases
	}
	if input.Description != "" {
		e.Description = input.Description
	}
	if input.Status != "" {
		e.Status = input.Status
	}
	if input.Properties != nil {
		e.Properties = s.entityRepo.MergeProperties(e.Properties, input.Properties)
	}

	if err := s.entityRepo.Update(ctx, tx, e); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return e, nil
}

func (s *EntityService) ResolveOrCreate(ctx context.Context, universeID uuid.UUID, data repositories.ExtractedEntity) (*models.Entity, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Step 1: Exact name match
	existing, err := s.entityRepo.FindByName(ctx, universeID, data.Name)
	if err == nil && existing != nil {
		merged := s.mergeEntity(existing, data)
		if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
			return nil, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return merged, false, nil
	}

	// Step 2: Alias match
	for _, alias := range data.Aliases {
		existing, err = s.entityRepo.FindByAlias(ctx, universeID, alias)
		if err == nil && existing != nil {
			merged := s.mergeEntity(existing, data)
			if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
				return nil, false, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, false, err
			}
			return merged, false, nil
		}
	}

	// Step 3: Semantic similarity
	embedding, err := s.qwenSvc.GenerateEmbedding(ctx, data.Name)
	if err == nil {
		similarID, distance, err := s.vectorRepo.FindSimilarEntity(ctx, universeID, embedding, 0.15)
		if err == nil && similarID != nil {
			existing, err = s.entityRepo.FindByID(ctx, *similarID)
			if err == nil {
				merged := s.mergeEntity(existing, data)
				if err := s.entityRepo.Update(ctx, tx, merged); err != nil {
					return nil, false, err
				}
				if err := tx.Commit(ctx); err != nil {
					return nil, false, err
				}
				return merged, false, nil
			}
		}
	}

	// Step 4: Create new entity
	props, _ := json.Marshal(data.Properties)
	newEntity := &models.Entity{
		ID:             uuid.New(),
		UniverseID:     universeID,
		Type:           data.Type,
		Name:           data.Name,
		Aliases:        data.Aliases,
		Description:    data.Description,
		Properties:     props,
		Status:         data.Status,
		RelevanceScore: 0.8,
	}

	if err := s.entityRepo.Create(ctx, tx, newEntity); err != nil {
		return nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}

	return newEntity, true, nil
}

func (s *EntityService) mergeEntity(existing *models.Entity, newData repositories.ExtractedEntity) *models.Entity {
	merged := *existing

	// Merge aliases
	if len(newData.Aliases) > 0 {
		aliasSet := make(map[string]bool)
		for _, a := range merged.Aliases {
			aliasSet[a] = true
		}
		for _, a := range newData.Aliases {
			if !aliasSet[a] {
				merged.Aliases = append(merged.Aliases, a)
			}
		}
	}

	// Merge description (longer wins)
	if len(newData.Description) > len(merged.Description) {
		merged.Description = newData.Description
	}

	// Merge properties
	if newData.Properties != nil {
		merged.Properties = s.entityRepo.MergeProperties(merged.Properties, mustMarshal(newData.Properties))
	}

	// Update status if provided
	if newData.Status != "" {
		merged.Status = newData.Status
	}

	return &merged
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
