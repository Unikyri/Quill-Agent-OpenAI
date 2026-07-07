package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// Consolidator is the interface for entity consolidation/deconsolidation.
// ConsolidationService satisfies this interface.
type Consolidator interface {
	ConsolidateEntity(ctx context.Context, entityID, universeID uuid.UUID) error
	DeconsolidateEntity(ctx context.Context, entityID uuid.UUID) error
}

// ConsolidationService handles summarization of archived entities into
// consolidated memories and deconsolidation on reactivation.
type ConsolidationService struct {
	consolidationRepo *repositories.ConsolidationRepo
	entityRepo        *repositories.EntityRepo
	qwenSvc           *QwenService
}

func NewConsolidationService(
	consolidationRepo *repositories.ConsolidationRepo,
	entityRepo *repositories.EntityRepo,
	qwenSvc *QwenService,
) *ConsolidationService {
	return &ConsolidationService{
		consolidationRepo: consolidationRepo,
		entityRepo:        entityRepo,
		qwenSvc:           qwenSvc,
	}
}

// ConsolidateEntity fetches the entity's mentions, summarizes them via
// qwen-turbo, generates an embedding of the summary, and stores the result.
// Zero-mention entities are skipped. Failures are logged but NOT propagated
// (best-effort, fire-and-forget).
func (s *ConsolidationService) ConsolidateEntity(ctx context.Context, entityID, universeID uuid.UUID) error {
	// ponytail: best-effort fire-and-forget — errors are logged, not returned
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[consolidation] panic consolidating entity %s: %v", entityID, r)
		}
	}()

	mentions, err := s.entityRepo.GetMentionsByEntity(ctx, entityID, 100)
	if err != nil {
		log.Printf("[consolidation] get mentions for entity %s: %v", entityID, err)
		return nil // best-effort: don't propagate
	}

	// spec: zero mentions → skip
	if len(mentions) == 0 {
		log.Printf("[consolidation] entity %s has 0 mentions, skipping consolidation", entityID)
		return nil
	}

	entity, err := s.entityRepo.FindByID(ctx, entityID)
	if err != nil {
		log.Printf("[consolidation] find entity %s: %v", entityID, err)
		return nil
	}

	summary, keyFacts, embedding, err := s.generateConsolidation(ctx, entity, mentions)
	if err != nil {
		log.Printf("[consolidation] generate consolidation for entity %s: %v", entityID, err)
		return nil
	}

	cm := &models.ConsolidatedMemory{
		ID:        uuid.New(),
		EntityID:  entityID,
		Summary:   summary,
		KeyFacts:  keyFacts,
		Embedding: embedding,
	}

	if err := s.consolidationRepo.Create(ctx, cm); err != nil {
		log.Printf("[consolidation] store consolidated memory for entity %s: %v", entityID, err)
		return nil
	}

	log.Printf("[consolidation] entity %s (%s) consolidated: %d mentions → %d facts",
		entityID, entity.Name, len(mentions), len(keyFacts))
	return nil
}

// generateConsolidation builds the summarization prompt, calls qwen-turbo,
// parses the response, and generates an embedding. Returns the summary,
// key facts, and embedding without touching the database.
// Exported for testing via ConsolidationService receiver.
func (s *ConsolidationService) generateConsolidation(ctx context.Context, entity *models.Entity, mentions []models.EntityMention) (string, []string, []float32, error) {
	summaryPrompt := buildConsolidationPrompt(entity.Name, mentions)

	messages := []QwenMessage{
		{Role: "system", Content: "You are a narrative memory compressor. Summarize entity history into concise JSON."},
		{Role: "user", Content: summaryPrompt},
	}

	response, err := s.qwenSvc.Chat(ctx, "qwen-turbo", messages)
	if err != nil {
		return "", nil, nil, fmt.Errorf("qwen-turbo chat: %w", err)
	}

	summary, keyFacts, err := parseConsolidationResponse(response)
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse response: %w", err)
	}

	embedding, err := s.qwenSvc.GenerateEmbedding(ctx, summary)
	if err != nil {
		return "", nil, nil, fmt.Errorf("generate embedding: %w", err)
	}

	return summary, keyFacts, embedding, nil
}

// DeconsolidateEntity removes the consolidated memory row for an entity.
// spec: idempotent — no error when no row exists.
func (s *ConsolidationService) DeconsolidateEntity(ctx context.Context, entityID uuid.UUID) error {
	// ponytail: direct call — DeleteByEntityID is already idempotent
	if s.consolidationRepo == nil {
		return nil
	}
	return s.consolidationRepo.DeleteByEntityID(ctx, entityID)
}

// parseConsolidationResponse parses the raw JSON response from qwen-turbo
// into a summary string and key facts slice. Exported for testing.
func parseConsolidationResponse(rawJSON string) (summary string, keyFacts []string, err error) {
	var result struct {
		Summary  string   `json:"summary"`
		KeyFacts []string `json:"key_facts"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return "", nil, fmt.Errorf("unmarshal consolidation response: %w", err)
	}
	if result.Summary == "" {
		return "", nil, fmt.Errorf("consolidation response has empty summary")
	}
	return result.Summary, result.KeyFacts, nil
}

// buildConsolidationPrompt creates the summarization prompt from entity mentions.
func buildConsolidationPrompt(entityName string, mentions []models.EntityMention) string {
	mentionTexts := make([]string, 0, len(mentions))
	for _, m := range mentions {
		if m.ContextSnippet != "" {
			mentionTexts = append(mentionTexts, fmt.Sprintf("- %s", m.ContextSnippet))
		}
	}
	return fmt.Sprintf(
		`Summarize the narrative history of the entity "%s" based on these context snippets.
Return ONLY valid JSON with this structure:
{
  "summary": "A 2-3 sentence summary of who this entity is and what they did",
  "key_facts": ["fact1", "fact2", "fact3"]
}

Context snippets:
%s`,
		entityName, joinStrings(mentionTexts, "\n"),
	)
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return "(no context available)"
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
