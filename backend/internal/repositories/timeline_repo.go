package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

type TimelineRepo struct {
	pool *pgxpool.Pool
}

func NewTimelineRepo(pool *pgxpool.Pool) *TimelineRepo {
	return &TimelineRepo{pool: pool}
}

func (r *TimelineRepo) Create(ctx context.Context, evt *models.TimelineEvent) error {
	query := `
		INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
			timeline_position, timeline_label, chapter_id, participants, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
	`
	_, err := r.pool.Exec(ctx, query,
		evt.ID, evt.UniverseID, evt.EventEntityID, evt.Title, evt.Description,
		evt.TimelinePosition, evt.TimelineLabel, evt.ChapterID, evt.Participants,
	)
	if err != nil {
		return fmt.Errorf("create timeline event: %w", err)
	}
	return nil
}

func (r *TimelineRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.TimelineEvent, error) {
	query := `
		SELECT id, universe_id, event_entity_id, title, description,
		       timeline_position, timeline_label, chapter_id, participants, created_at
		FROM timeline_events WHERE id = $1
	`
	evt := &models.TimelineEvent{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&evt.ID, &evt.UniverseID, &evt.EventEntityID, &evt.Title, &evt.Description,
		&evt.TimelinePosition, &evt.TimelineLabel, &evt.ChapterID, &evt.Participants, &evt.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("timeline event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find timeline event: %w", err)
	}
	return evt, nil
}

func (r *TimelineRepo) ListByUniverse(ctx context.Context, universeID uuid.UUID) ([]models.TimelineEvent, error) {
	query := `
		SELECT id, universe_id, event_entity_id, title, description,
		       timeline_position, timeline_label, chapter_id, participants, created_at
		FROM timeline_events WHERE universe_id = $1
		ORDER BY timeline_position ASC NULLS LAST
	`
	rows, err := r.pool.Query(ctx, query, universeID)
	if err != nil {
		return nil, fmt.Errorf("list timeline events: %w", err)
	}
	defer rows.Close()

	result := []models.TimelineEvent{}
	for rows.Next() {
		var evt models.TimelineEvent
		if err := rows.Scan(
			&evt.ID, &evt.UniverseID, &evt.EventEntityID, &evt.Title, &evt.Description,
			&evt.TimelinePosition, &evt.TimelineLabel, &evt.ChapterID, &evt.Participants, &evt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan timeline event: %w", err)
		}
		result = append(result, evt)
	}
	return result, nil
}
