package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// PlotHoleService detects narrative arcs that have been stale for too many chapters.
type PlotHoleService struct {
	pool         *pgxpool.Pool
	plotHoleRepo *repositories.PlotHoleRepo
	entityRepo   *repositories.EntityRepo
	chapters     int // inactivity threshold
	qwenSvc      *QwenService
	executor     *QuillExecutor
}

func NewPlotHoleService(pool *pgxpool.Pool, plotHoleRepo *repositories.PlotHoleRepo, entityRepo *repositories.EntityRepo, chapters int, qwenSvc *QwenService, executor *QuillExecutor) *PlotHoleService {
	return &PlotHoleService{
		pool:         pool,
		plotHoleRepo: plotHoleRepo,
		entityRepo:   entityRepo,
		chapters:     chapters,
		qwenSvc:      qwenSvc,
		executor:     executor,
	}
}

// Scan checks all active entities and creates plot holes for those whose
// last_mentioned chapter is at least `chapters` behind the current chapter.
//
// ponytail: single-pass scan over active entities; O(n) where n is entity count.
// Gap calculation: current_order - last_mentioned_order.
// ponytail: N+1 query per entity for chapter lookup — fine for hackathon scale;
// batch preload chapters if entity count exceeds 1k.
func (s *PlotHoleService) Scan(ctx context.Context, universeID, currentChapterID uuid.UUID) ([]models.PlotHole, error) {
	// Get current chapter's order_index
	var currentOrder int
	if err := s.pool.QueryRow(ctx, "SELECT order_index FROM chapters WHERE id = $1", currentChapterID).Scan(&currentOrder); err != nil {
		return nil, fmt.Errorf("get current chapter order: %w", err)
	}

	// Get all active entities
	entities, err := s.entityRepo.ListByUniverseActive(ctx, universeID)
	if err != nil {
		return nil, fmt.Errorf("list active entities: %w", err)
	}

	var holes []models.PlotHole
	for _, e := range entities {
		if e.LastMentionedChapterID == nil {
			continue // never mentioned → skip
		}

		// ponytail: relevance_score filter — low-relevance entities
		// (<= 0.5) are background noise, skip agent evaluation.
		if e.RelevanceScore <= 0.5 {
			continue
		}

		// Get the last mentioned chapter's order_index
		var lastOrder int
		if err := s.pool.QueryRow(ctx, "SELECT order_index FROM chapters WHERE id = $1", *e.LastMentionedChapterID).Scan(&lastOrder); err != nil {
			continue // chapter may have been deleted
		}

		gap := currentOrder - lastOrder
		if gap >= s.chapters {
			// ponytail: agent evaluation for semantic plot hole verdict.
			// Skip if no qwenSvc — caller can pass nil for testing.
			if s.qwenSvc != nil {
				isPlotHole, err := s.evaluatePlotHole(ctx, universeID, e, gap)
				if err != nil {
					continue // agent call failed, skip gracefully
				}
				if !isPlotHole {
					continue // agent says arc is naturally concluded
				}
			}

			hole := models.PlotHole{
				ID:                      uuid.New(),
				UniverseID:              universeID,
				Title:                   fmt.Sprintf("Stale arc: %s (gap %d chapters)", e.Name, gap),
				Description:             fmt.Sprintf("Entity '%s' has not been mentioned for %d chapters (last seen in chapter %d, currently at chapter %d)", e.Name, gap, lastOrder, currentOrder),
				RelatedEntityIDs:        []uuid.UUID{e.ID},
				FirstMentionedChapterID: e.LastMentionedChapterID,
				Status:                  "open",
			}
			if err := s.plotHoleRepo.Create(ctx, &hole); err != nil {
				return nil, fmt.Errorf("create plot hole: %w", err)
			}
			holes = append(holes, hole)
		}
	}

	return holes, nil
}

// evaluatePlotHole calls the agent to determine if a stale entity's arc is
// a forgotten plot thread or naturally concluded.
//
// The agent uses search_vector_memory to find contextual information about the
// entity before making its verdict.
func (s *PlotHoleService) evaluatePlotHole(ctx context.Context, universeID uuid.UUID, e models.Entity, gap int) (bool, error) {
	// ponytail: set UniverseID on the executor so tool calls resolve in the right universe.
	if s.executor != nil {
		s.executor.UniverseID = universeID
	}

	systemPrompt := `You are a narrative analysis AI. Evaluate whether this entity's story arc is a plot hole.

You have access to search_vector_memory to look up contextual facts about this entity in the story.

Determine if the arc is naturally concluded or a forgotten plot thread that the author should address.
Answer with ONLY "YES" if this is a plot hole (forgotten/incomplete arc) or "NO" if the arc is naturally concluded.`

	userPrompt := fmt.Sprintf(`Entity: %s (type: %s, status: %s)
Last mentioned %d chapters ago.

Use search_vector_memory to find relevant context before making your decision.`, e.Name, e.Type, e.Status, gap)

	messages := []QwenMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	tools := []QwenTool{
		{
			Type: "function",
			Function: QwenToolFunction{
				Name:        "search_vector_memory",
				Description: "Search the story's vector memory for facts related to a query. Returns relevant paragraphs with similarity scores.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The search query to find relevant facts",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}

	var exec ToolExecutor
	if s.executor != nil {
		exec = s.executor
	}

	result, err := s.qwenSvc.RunAgentLoop(ctx, messages, tools, exec, 3)
	if err != nil {
		return false, fmt.Errorf("agent evaluation: %w", err)
	}

	result = strings.TrimSpace(strings.ToUpper(result))
	return strings.HasPrefix(result, "YES"), nil
}
