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
	qwenSvc      *QwenService
}

func NewTimelineService(pool *pgxpool.Pool, timelineRepo *repositories.TimelineRepo, qwenSvc *QwenService) *TimelineService {
	return &TimelineService{pool: pool, timelineRepo: timelineRepo, qwenSvc: qwenSvc}
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

// validateWithAgent calls the LLM to evaluate if this timeline position is
// chronologically consistent.
//
// ponytail: single chat completion — yes/no consistency verdict.
func (s *TimelineService) validateWithAgent(ctx context.Context, event models.TimelineEvent) error {
	prompt := fmt.Sprintf(`You are a narrative timeline validator. Evaluate this event's temporal position:

Event: %s
%s

Based on the text context, is this timeline position chronologically consistent with the narrative flow?

Answer with ONLY "CONSISTENT" if the position makes chronological sense, or "INCONSISTENT: <reason>" if it does not.`,
		event.Title, event.Description)

	messages := []QwenMessage{
		{Role: "system", Content: "You validate narrative timeline consistency. Answer CONSIStent or INCONSISTENT."},
		{Role: "user", Content: prompt},
	}

	result, err := s.qwenSvc.RunAgentLoop(ctx, messages, nil, nil, 1)
	if err != nil {
		return fmt.Errorf("agent validation: %w", err)
	}

	result = strings.TrimSpace(strings.ToUpper(result))
	if strings.HasPrefix(result, "INCONSISTENT") {
		return fmt.Errorf("timeline position inconsistent: %s", strings.TrimPrefix(result, "INCONSISTENT: "))
	}
	return nil
}
