package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	PasswordHash string   `json:"-"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Universe struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Genre          string    `json:"genre,omitempty"`
	Format         string    `json:"format"`
	SessionID      string    `json:"session_id,omitempty"`
	IsDemoTemplate bool      `json:"is_demo_template"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Work struct {
	ID          uuid.UUID `json:"id"`
	UniverseID  uuid.UUID `json:"universe_id"`
	Title       string    `json:"title"`
	Type        string    `json:"type"`
	OrderIndex  int       `json:"order_index"`
	Synopsis    string    `json:"synopsis,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Chapter struct {
	ID          uuid.UUID  `json:"id"`
	WorkID      uuid.UUID  `json:"work_id"`
	Title       string     `json:"title"`
	OrderIndex  int        `json:"order_index"`
	Content     string     `json:"content"`
	RawText     string     `json:"raw_text"`
	WordCount   int        `json:"word_count"`
	Status      string     `json:"status"`
	AnalyzedAt  *time.Time `json:"analyzed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Entity struct {
	ID                     uuid.UUID          `json:"id"`
	UniverseID             uuid.UUID          `json:"universe_id"`
	Type                   string             `json:"type"`
	Name                   string             `json:"name"`
	Aliases                []string           `json:"aliases,omitempty"`
	Description            string             `json:"description,omitempty"`
	Properties             json.RawMessage    `json:"properties,omitempty"`
	Status                 string             `json:"status"`
	RelevanceScore         float64            `json:"relevance_score"`
	LastMentionedChapterID *uuid.UUID         `json:"last_mentioned_chapter_id,omitempty"`
	LastMentionedAt        *time.Time         `json:"last_mentioned_at,omitempty"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
}

type EntityMention struct {
	ID              uuid.UUID `json:"id"`
	EntityID        uuid.UUID `json:"entity_id"`
	ChapterID       uuid.UUID `json:"chapter_id"`
	ParagraphIndex  int       `json:"paragraph_index"`
	ParagraphNodeID string    `json:"paragraph_node_id,omitempty"`
	ContextSnippet  string    `json:"context_snippet,omitempty"`
	MentionType     string    `json:"mention_type,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Contradiction struct {
	ID                    uuid.UUID  `json:"id"`
	UniverseID            uuid.UUID  `json:"universe_id"`
	EntityID              *uuid.UUID `json:"entity_id,omitempty"`
	Severity              string     `json:"severity"`
	Description           string     `json:"description"`
	Suggestion            string     `json:"suggestion,omitempty"`
	EvidenceA             string     `json:"evidence_a,omitempty"`
	EvidenceAChapterID    *uuid.UUID `json:"evidence_a_chapter_id,omitempty"`
	EvidenceB             string     `json:"evidence_b,omitempty"`
	EvidenceBChapterID    *uuid.UUID `json:"evidence_b_chapter_id,omitempty"`
	Fingerprint           string     `json:"fingerprint,omitempty"`
	Status                string     `json:"status"`
	ResolvedAt            *time.Time `json:"resolved_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

type TimelineEvent struct {
	ID              uuid.UUID   `json:"id"`
	UniverseID      uuid.UUID   `json:"universe_id"`
	EventEntityID   *uuid.UUID  `json:"event_entity_id,omitempty"`
	Title           string      `json:"title"`
	Description     string      `json:"description,omitempty"`
	TimelinePosition *float64   `json:"timeline_position,omitempty"`
	TimelineLabel   string      `json:"timeline_label,omitempty"`
	ChapterID       *uuid.UUID  `json:"chapter_id,omitempty"`
	Participants    []uuid.UUID `json:"participants,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
}

type PlotHole struct {
	ID                       uuid.UUID   `json:"id"`
	UniverseID               uuid.UUID   `json:"universe_id"`
	Title                    string      `json:"title"`
	Description              string      `json:"description,omitempty"`
	RelatedEntityIDs         []uuid.UUID `json:"related_entity_ids,omitempty"`
	FirstMentionedChapterID  *uuid.UUID  `json:"first_mentioned_chapter_id,omitempty"`
	Status                   string      `json:"status"`
	CreatedAt                time.Time   `json:"created_at"`
}

type IngestionJob struct {
	ID                     uuid.UUID  `json:"id"`
	UniverseID             uuid.UUID  `json:"universe_id"`
	WorkID                 uuid.UUID  `json:"work_id"`
	Filename               string     `json:"filename,omitempty"`
	FileType               string     `json:"file_type,omitempty"`
	Status                 string     `json:"status"`
	TotalChaptersDetected  int        `json:"total_chapters_detected"`
	ChaptersProcessed      int        `json:"chapters_processed"`
	EntitiesExtracted      int        `json:"entities_extracted"`
	ErrorMessage           string     `json:"error_message,omitempty"`
	StartedAt              *time.Time `json:"started_at,omitempty"`
	CompletedAt            *time.Time `json:"completed_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
}

// API request/response types

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}

type CreateUniverseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Genre       string `json:"genre"`
	Format      string `json:"format"`
}

type CreateWorkRequest struct {
	Title    string `json:"title"`
	Type     string `json:"type"`
	Synopsis string `json:"synopsis"`
}

type CreateChapterRequest struct {
	Title string `json:"title"`
}

type UpdateChapterRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	RawText string `json:"raw_text"`
}

type UpdateEntityRequest struct {
	Name        string          `json:"name"`
	Aliases     []string        `json:"aliases"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Properties  json.RawMessage `json:"properties"`
}

type PaginationParams struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

type PaginatedResponse[T any] struct {
	Data       []T            `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
}

type PaginationInfo struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type HealthResponse struct {
	Status        string `json:"status"`
	DB            string `json:"db"`
	AGE           string `json:"age"`
	PGVector      string `json:"pgvector"`
	QwenAPI       string `json:"qwen_api"`
	DiskFreeMB    int64  `json:"disk_free_mb"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details []ErrorField    `json:"details,omitempty"`
}

type ErrorField struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
