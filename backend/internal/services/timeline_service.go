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

// TimelineService validates temporal consistency of timeline events.
type TimelineService struct {
	pool         *pgxpool.Pool
	timelineRepo *repositories.TimelineRepo
	qwenSvc      LLMService
	executor     ToolExecutor
}

func NewTimelineService(pool *pgxpool.Pool, timelineRepo *repositories.TimelineRepo, qwenSvc LLMService, executor ToolExecutor) *TimelineService {
	return &TimelineService{pool: pool, timelineRepo: timelineRepo, qwenSvc: qwenSvc, executor: executor}
}

// ValidatePosition checks whether the event's chapter is chronologically before or
// at the present chapter. Returns an error if the event chapter is in the future.
//
// When eventOrder == presentOrder (ambiguous case), the method calls an agent
// for LLM-based chronological validation if qwenSvc is available.
func (s *TimelineService) ValidatePosition(ctx context.Context, event models.TimelineEvent, presentChapterID uuid.UUID) error {
	// ponytail: compare chapter order_index; nil chapter_id = always valid
	if event.ChapterID == nil {
		return nil
	}

	var eventOrder, presentOrder int
	err := s.pool.QueryRow(ctx, "SELECT order_index FROM chapters WHERE id = $1", *event.ChapterID).Scan(&eventOrder)
	if err != nil {
		return fmt.Errorf("get event chapter: %w", err)
	}
	err = s.pool.QueryRow(ctx, "SELECT order_index FROM chapters WHERE id = $1", presentChapterID).Scan(&presentOrder)
	if err != nil {
		return fmt.Errorf("get present chapter: %w", err)
	}

	if eventOrder > presentOrder {
		return fmt.Errorf("timeline event chapter (%d) is after present chapter (%d): future event not allowed", eventOrder, presentOrder)
	}

	// ponytail: agent fallback for equal order_index — ambiguous temporal position.
	// Skip if no qwenSvc (nil-safe for testing and backward compat).
	if eventOrder == presentOrder && s.qwenSvc != nil {
		return s.validateWithAgent(ctx, event)
	}

	return nil
}

// ValidateNewEvent is the entry point for a manually-created timeline event:
// it resolves "present" as the work's most-recently-authored chapter (the
// highest order_index chapter in the same work as the event), then delegates
// to ValidatePosition. Nil-safe — an event with no chapter is trivially valid.
func (s *TimelineService) ValidateNewEvent(ctx context.Context, event models.TimelineEvent) error {
	if event.ChapterID == nil {
		return nil
	}

	var workID uuid.UUID
	if err := s.pool.QueryRow(ctx, "SELECT work_id FROM chapters WHERE id = $1", *event.ChapterID).Scan(&workID); err != nil {
		return fmt.Errorf("resolve event chapter's work: %w", err)
	}

	var latestChapterID uuid.UUID
	if err := s.pool.QueryRow(ctx, "SELECT id FROM chapters WHERE work_id = $1 ORDER BY order_index DESC LIMIT 1", workID).Scan(&latestChapterID); err != nil {
		return fmt.Errorf("resolve work's latest chapter: %w", err)
	}

	return s.ValidatePosition(ctx, event, latestChapterID)
}

// validateWithAgent calls the LLM to evaluate if this timeline position is
// chronologically consistent using vector memory search for context.
func (s *TimelineService) validateWithAgent(ctx context.Context, event models.TimelineEvent) error {
	systemPrompt := `You are a narrative timeline validator. Use search_vector_memory to find chronological context about the story.

Evaluate whether this event's temporal position is chronologically consistent with the narrative flow.

Answer with ONLY "CONSISTENT" if the position makes chronological sense, or "INCONSISTENT: <reason>" if it does not.`

	userPrompt := fmt.Sprintf(`Event: %s
%s

Use search_vector_memory to find relevant chronological context before making your decision.`,
		event.Title, event.Description)

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

	result, err := s.qwenSvc.RunAgentLoop(ctx, messages, tools, s.executor, 2)
	if err != nil {
		return fmt.Errorf("agent validation: %w", err)
	}

	result = strings.TrimSpace(strings.ToUpper(result))
	if strings.HasPrefix(result, "INCONSISTENT") {
		return fmt.Errorf("timeline position inconsistent: %s", strings.TrimPrefix(result, "INCONSISTENT: "))
	}
	return nil
}
