package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Universe struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	GenreTags      []string  `json:"genre_tags"`
	SessionID      *string   `json:"-"`
	IsDemoTemplate bool      `json:"is_demo_template"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Work struct {
	ID         uuid.UUID `json:"id"`
	UniverseID uuid.UUID `json:"universe_id"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	OrderIndex int       `json:"order_index"`
	Synopsis   string    `json:"synopsis,omitempty"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Chapter struct {
	ID         uuid.UUID  `json:"id"`
	WorkID     uuid.UUID  `json:"work_id"`
	UniverseID uuid.UUID  `json:"universe_id"`
	Title      string     `json:"title"`
	OrderIndex int        `json:"order_index"`
	Content    string     `json:"content"`
	RawText    string     `json:"raw_text"`
	WordCount  int        `json:"word_count"`
	Status     string     `json:"status"`
	AnalyzedAt *time.Time `json:"analyzed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Entity struct {
	ID                     uuid.UUID       `json:"id"`
	UniverseID             uuid.UUID       `json:"universe_id"`
	Type                   string          `json:"type"`
	Name                   string          `json:"name"`
	Aliases                []string        `json:"aliases,omitempty"`
	Description            string          `json:"description,omitempty"`
	Properties             json.RawMessage `json:"properties,omitempty"`
	Status                 string          `json:"status"`
	Confidence             float64         `json:"confidence"`
	RelevanceScore         float64         `json:"relevance_score"`
	LastMentionedChapterID *uuid.UUID      `json:"last_mentioned_chapter_id,omitempty"`
	LastMentionedAt        *time.Time      `json:"last_mentioned_at,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

// EntityCandidate is the review-tray projection of an entity whose extraction
// confidence is below the configured auto-accept threshold. Evidence is read
// from the most recent entity mention; candidates do not enter active graph
// memory until accepted or merged by the writer.
type EntityCandidate struct {
	EntityID      uuid.UUID `json:"entity_id"`
	UniverseID    uuid.UUID `json:"universe_id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Aliases       []string  `json:"aliases,omitempty"`
	Description   string    `json:"description,omitempty"`
	Confidence    float64   `json:"confidence"`
	Status        string    `json:"status"`
	EvidenceQuote string    `json:"evidence_quote,omitempty"`
	ChapterID     uuid.UUID `json:"chapter_id,omitempty"`
}

type EntityMention struct {
	ID              uuid.UUID `json:"id"`
	EntityID        uuid.UUID `json:"entity_id"`
	ChapterID       uuid.UUID `json:"chapter_id"`
	ParagraphIndex  int       `json:"paragraph_index"`
	CharacterOffset int       `json:"character_offset"`
	ParagraphNodeID string    `json:"paragraph_node_id,omitempty"`
	ContextSnippet  string    `json:"context_snippet,omitempty"`
	MentionType     string    `json:"mention_type,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Contradiction struct {
	ID                 uuid.UUID  `json:"id"`
	UniverseID         uuid.UUID  `json:"universe_id"`
	EntityID           *uuid.UUID `json:"entity_id,omitempty"`
	Severity           string     `json:"severity"`
	Description        string     `json:"description"`
	Suggestion         string     `json:"suggestion,omitempty"`
	EvidenceA          string     `json:"evidence_a,omitempty"`
	EvidenceAChapterID *uuid.UUID `json:"evidence_a_chapter_id,omitempty"`
	EvidenceB          string     `json:"evidence_b,omitempty"`
	EvidenceBChapterID *uuid.UUID `json:"evidence_b_chapter_id,omitempty"`
	Fingerprint        string     `json:"fingerprint,omitempty"`
	Status             string     `json:"status"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type TimelineEvent struct {
	ID               uuid.UUID   `json:"id"`
	UniverseID       uuid.UUID   `json:"universe_id"`
	EventEntityID    *uuid.UUID  `json:"event_entity_id,omitempty"`
	Title            string      `json:"title"`
	Description      string      `json:"description,omitempty"`
	TimelinePosition *float64    `json:"timeline_position,omitempty"`
	TimelineLabel    string      `json:"timeline_label,omitempty"`
	ChapterID        *uuid.UUID  `json:"chapter_id,omitempty"`
	Participants     []uuid.UUID `json:"participants,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
}

type PlotHole struct {
	ID                      uuid.UUID   `json:"id"`
	UniverseID              uuid.UUID   `json:"universe_id"`
	Title                   string      `json:"title"`
	Description             string      `json:"description,omitempty"`
	RelatedEntityIDs        []uuid.UUID `json:"related_entity_ids,omitempty"`
	FirstMentionedChapterID *uuid.UUID  `json:"first_mentioned_chapter_id,omitempty"`
	Status                  string      `json:"status"`
	CreatedAt               time.Time   `json:"created_at"`
}

type IngestionJob struct {
	ID                    uuid.UUID  `json:"id"`
	UniverseID            uuid.UUID  `json:"universe_id"`
	WorkID                uuid.UUID  `json:"work_id"`
	Filename              string     `json:"filename,omitempty"`
	FileType              string     `json:"file_type,omitempty"`
	Status                string     `json:"status"`
	TotalChaptersDetected int        `json:"total_chapters_detected"`
	ChaptersProcessed     int        `json:"chapters_processed"`
	EntitiesExtracted     int        `json:"entities_extracted"`
	ContentHash           string     `json:"content_hash,omitempty"`
	ErrorMessage          string     `json:"error_message,omitempty"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

type GraphNeighbor struct {
	RelType  string `json:"rel_type"`
	RelProps string `json:"rel_props"`
	Node     string `json:"node"`
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
	Name        string   `json:"name"`
	Description string   `json:"description"`
	GenreTags   []string `json:"genre_tags"`
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

type MergeEntityCandidateRequest struct {
	TargetEntityID uuid.UUID `json:"target_entity_id"`
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

type ConsolidatedMemory struct {
	ID        uuid.UUID `json:"id"`
	EntityID  uuid.UUID `json:"entity_id"`
	Summary   string    `json:"summary"`
	KeyFacts  []string  `json:"key_facts"`
	Embedding []float32 `json:"embedding"`
	CreatedAt time.Time `json:"created_at"`
}

// WriterObservation is a verifiable, zero-LLM stylometry fact. It is
// intentionally separate from WriterPreference: observations never imply
// intent on their own.
type WriterObservation struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	UniverseID *uuid.UUID `json:"universe_id,omitempty"`
	Metric     string     `json:"metric"`
	Value      float64    `json:"value"`
	SampleSize int        `json:"sample_size"`
	ComputedAt time.Time  `json:"computed_at"`
}

// WriterPreference is an inferred belief about writer intent. Relevance and
// lifecycle follow the same decay model as story entities.
type WriterPreference struct {
	ID               uuid.UUID   `json:"id"`
	UserID           uuid.UUID   `json:"user_id"`
	Statement        string      `json:"statement"`
	Scope            string      `json:"scope"`
	GenreTags        []string    `json:"genre_tags"`
	Confidence       float64     `json:"confidence"`
	RelevanceScore   float64     `json:"relevance_score"`
	Lifecycle        string      `json:"lifecycle"`
	Embedding        []float32   `json:"-"`
	LastReinforcedAt time.Time   `json:"last_reinforced_at"`
	ObservationIDs   []uuid.UUID `json:"observation_ids"`
	FeedbackEventIDs []uuid.UUID `json:"feedback_event_ids"`
	CreatedAt        time.Time   `json:"created_at"`
}

// WriterFeedbackEvent is the raw, user-originated intent evidence. Silent
// dismissal is deliberately not representable in Signal.
type WriterFeedbackEvent struct {
	ID           uuid.UUID       `json:"id"`
	UserID       uuid.UUID       `json:"user_id"`
	UniverseID   *uuid.UUID      `json:"universe_id,omitempty"`
	ChapterID    *uuid.UUID      `json:"chapter_id,omitempty"`
	NoteID       *uuid.UUID      `json:"note_id,omitempty"`
	Signal       string          `json:"signal"`
	PreferenceID *uuid.UUID      `json:"preference_id,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    time.Time       `json:"created_at"`
}

// WriterPreferenceHistory is a point-in-time decay/reinforcement snapshot.
type WriterPreferenceHistory struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	PreferenceID   *uuid.UUID `json:"preference_id,omitempty"`
	RelevanceScore float64    `json:"relevance_score"`
	Confidence     float64    `json:"confidence"`
	Lifecycle      string     `json:"lifecycle"`
	RecordedAt     time.Time  `json:"recorded_at"`
}

// WriterPreferenceEvidence is the explainability payload returned by the
// preference evidence endpoint.
type WriterPreferenceEvidence struct {
	Preference   WriterPreference          `json:"preference"`
	Observations []WriterObservation       `json:"observations"`
	Events       []WriterFeedbackEvent     `json:"feedback_events"`
	History      []WriterPreferenceHistory `json:"history"`
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
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []ErrorField `json:"details,omitempty"`
}

type ErrorField struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// --- WebSocket message types (Phase 2a) ---

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client → Server payloads

type AuthInitPayload struct {
	Token string `json:"token"`
}

type ParagraphSubmitPayload struct {
	// SubmissionID correlates every server progress/terminal message with the
	// exact client-side debounce that created it.
	SubmissionID string `json:"submission_id"`
	// ParagraphRef is a client-local reference captured at edit time. It is
	// intentionally opaque to the backend; the editor uses it to render a
	// status beside the paragraph that was actually edited.
	ParagraphRef string    `json:"paragraph_ref"`
	WorkID       uuid.UUID `json:"work_id"`
	ChapterID    uuid.UUID `json:"chapter_id"`
	UniverseID   uuid.UUID `json:"universe_id"`
	Text         string    `json:"text"`
}

type RecallRequestPayload struct {
	UniverseID uuid.UUID `json:"universe_id"`
	Query      string    `json:"query"`
	K          int       `json:"k"`
}

// SkillCatalogueItem is the public, frontmatter-only representation of an
// editorial skill. Skill bodies stay server-side so callers cannot mutate or
// accidentally depend on the prompt implementation.
type SkillCatalogueItem struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	GenreTags   []string `json:"genre_tags"`
	Stage       string   `json:"stage"`
}

type UniverseSkill struct {
	UniverseID  uuid.UUID `json:"universe_id"`
	SkillName   string    `json:"skill_name"`
	ActivatedAt time.Time `json:"activated_at"`
}

type CraftReviewRequestPayload struct {
	UniverseID uuid.UUID `json:"universe_id"`
	WorkID     uuid.UUID `json:"work_id"`
	ChapterID  uuid.UUID `json:"chapter_id"`
	Passage    string    `json:"passage"`
	Context    string    `json:"context,omitempty"`
	// RequestedSkills is optional. An empty list means "let Quill choose"
	// from this universe's active skills; a non-empty list is the writer's
	// explicit, validated selection for this passage.
	RequestedSkills []string `json:"requested_skill_names,omitempty"`
	// RequestID correlates this one-off request to exactly one client action.
	// Craft responses are never safe to apply without it.
	RequestID       string   `json:"request_id"`
}

type CraftReviewSelection struct {
	Skill     string `json:"skill"`
	Rationale string `json:"rationale"`
}

type CraftReviewNote struct {
	ID       uuid.UUID `json:"id"`
	Skill    string    `json:"skill"`
	Quote    string    `json:"quote"`
	Note     string    `json:"note"`
	Severity string    `json:"severity"`
	Category string    `json:"category,omitempty"`
}

type CraftReviewResultPayload struct {
	UniverseID uuid.UUID              `json:"universe_id"`
	WorkID     uuid.UUID              `json:"work_id"`
	ChapterID  uuid.UUID              `json:"chapter_id"`
	RequestID  string                 `json:"request_id"`
	Selections []CraftReviewSelection `json:"selections"`
	Notes      []CraftReviewNote      `json:"notes"`
}

// Server → Client payloads

type AnalysisResultPayload struct {
	SubmissionID   string          `json:"submission_id"`
	ParagraphRef   string          `json:"paragraph_ref"`
	WorkID         uuid.UUID       `json:"work_id"`
	ChapterID      uuid.UUID       `json:"chapter_id"`
	UniverseID     uuid.UUID       `json:"universe_id"`
	Entities       []EntityBrief   `json:"entities"`
	Contradictions []Contradiction `json:"contradictions"`
	PlotHoles      []PlotHole      `json:"plot_holes"`
	ArbiterSummary string          `json:"arbiter_summary,omitempty"`
}

// AnalysisFailedPayload is the terminal failure counterpart of
// AnalysisResultPayload. A successfully accepted paragraph submission emits
// exactly one of these two terminal payloads.
type AnalysisFailedPayload struct {
	SubmissionID string    `json:"submission_id"`
	ParagraphRef string    `json:"paragraph_ref"`
	WorkID       uuid.UUID `json:"work_id"`
	ChapterID    uuid.UUID `json:"chapter_id"`
	UniverseID   uuid.UUID `json:"universe_id"`
	Reason       string    `json:"reason"`
}

type EntityBrief struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Type string    `json:"type"`
}

type ContradictionAlertPayload struct {
	UniverseID    uuid.UUID     `json:"universe_id"`
	Contradiction Contradiction `json:"contradiction"`
}

type EntityDiscoveredPayload struct {
	UniverseID uuid.UUID `json:"universe_id"`
	Entity     Entity    `json:"entity"`
	IsNew      bool      `json:"is_new"`
}

type GraphUpdatedPayload struct {
	UniverseID uuid.UUID `json:"universe_id"`
	Action     string    `json:"action"`
}

// AnalysisProgressPayload is the payload for analysis_progress WS messages,
// emitted at each real processJob pipeline stage. Count fields are pointers
// so a stage that has no meaningful count (e.g. checking_contradictions,
// which fires before the check runs) can omit it from the JSON entirely.
// Budget holds a services.BudgetReport for the context_budget stage; kept as
// interface{} here (not a concrete services type) since models is a
// lower-level package that services depends on, not the other way around.
type AnalysisProgressPayload struct {
	SubmissionID       string      `json:"submission_id"`
	ParagraphRef       string      `json:"paragraph_ref"`
	Stage              string      `json:"stage"`
	ChapterID          uuid.UUID   `json:"chapter_id"`
	UniverseID         uuid.UUID   `json:"universe_id"`
	EntityCount        *int        `json:"entity_count,omitempty"`
	ContradictionCount *int        `json:"contradiction_count,omitempty"`
	PlotHoleCount      *int        `json:"plot_hole_count,omitempty"`
	Budget             interface{} `json:"budget,omitempty"`
}

type ContextualRecallPayload struct {
	UniverseID uuid.UUID    `json:"universe_id"`
	Items      []RecallItem `json:"items"`
}

// IngestionProgressPayload is the authoritative, universe-scoped progress
// event emitted while an uploaded document is processed asynchronously.
type IngestionProgressPayload struct {
	JobID             uuid.UUID `json:"job_id"`
	UniverseID        uuid.UUID `json:"universe_id"`
	Status            string    `json:"status"`
	ChaptersProcessed int       `json:"chapters_processed"`
	TotalChapters     int       `json:"total_chapters"`
	Action            string    `json:"action,omitempty"`
	ETASeconds        *int      `json:"eta_seconds,omitempty"`
}

type RecallItem struct {
	ID       string    `json:"id"`
	EntityID uuid.UUID `json:"entity_id"`
	Fact     string    `json:"fact"`
	Score    float64   `json:"score"`
	Source   string    `json:"source"`
}
