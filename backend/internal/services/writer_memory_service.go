package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

const (
	DefaultWriterPreferencePromotionThreshold = 3
	writerBehaviourWindow                     = 10 * time.Minute
)

var ErrInvalidWriterFeedback = errors.New("invalid writer feedback")
var ErrInvalidWriterPreference = errors.New("invalid writer preference")
var ErrWriterPreferenceNotFound = errors.New("writer preference not found")

var writerPreferenceSchema = &ResponseFormat{
	Type: "json_schema",
	JSONSchema: &JSONSchemaResponseFormat{
		Name:        "writer_preference_promotion",
		Description: "Classify an intent-backed writer preference without inventing evidence.",
		Strict:      true,
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"statement":  map[string]interface{}{"type": "string"},
				"scope":      map[string]interface{}{"type": "string", "enum": []interface{}{"universal", "genre_bound"}},
				"genre_tags": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"confidence": map[string]interface{}{"type": "number"},
			},
			"required":             []interface{}{"statement", "scope", "genre_tags", "confidence"},
			"additionalProperties": false,
		},
	},
}

// WriterFeedbackInput is the service-level contract used by craft consumers
// and the authenticated feedback endpoint. Silent dismissal has no valid
// signal value and is rejected before persistence.
type WriterFeedbackInput struct {
	UserID       uuid.UUID
	UniverseID   *uuid.UUID
	ChapterID    *uuid.UUID
	NoteID       *uuid.UUID
	PreferenceID *uuid.UUID
	Signal       string
	Payload      map[string]interface{}
}

type writerPromotionResult struct {
	Statement  string   `json:"statement"`
	Scope      string   `json:"scope"`
	GenreTags  []string `json:"genre_tags"`
	Confidence float64  `json:"confidence"`
}

// WriterMemoryService owns intent promotion, feedback reinforcement, decay,
// and suppression. StylometryService is intentionally a separate dependency
// and cannot call this service.
type WriterMemoryService struct {
	repo               *repositories.WriterMemoryRepo
	llm                LLMService
	promotionThreshold int
	lambda             float64
	archiveThreshold   float64
	now                func() time.Time
}

func NewWriterMemoryService(repo *repositories.WriterMemoryRepo, llm LLMService, promotionThreshold int, lambda, archiveThreshold float64) *WriterMemoryService {
	if promotionThreshold <= 0 {
		promotionThreshold = DefaultWriterPreferencePromotionThreshold
	}
	if lambda <= 0 {
		lambda = 0.1
	}
	if archiveThreshold <= 0 {
		archiveThreshold = 0.15
	}
	return &WriterMemoryService{
		repo: repo, llm: llm, promotionThreshold: promotionThreshold,
		lambda: lambda, archiveThreshold: archiveThreshold, now: time.Now,
	}
}

func (s *WriterMemoryService) ListPreferences(ctx context.Context, userID uuid.UUID, activeOnly bool) ([]models.WriterPreference, error) {
	if s == nil || s.repo == nil {
		return []models.WriterPreference{}, nil
	}
	return s.repo.ListPreferences(ctx, userID, activeOnly, 500)
}

// ListPreferencesForUniverse applies the universe owner and genre intersection
// used by craft consumers. It prevents a review from receiving unrelated
// genre-bound preferences from another universe in the same writer profile.
func (s *WriterMemoryService) ListPreferencesForUniverse(ctx context.Context, universeID uuid.UUID) ([]models.WriterPreference, error) {
	if s == nil || s.repo == nil {
		return []models.WriterPreference{}, nil
	}
	return s.repo.ListActiveForUniverse(ctx, universeID, 500)
}

func (s *WriterMemoryService) ListObservations(ctx context.Context, userID uuid.UUID, universeID *uuid.UUID) ([]models.WriterObservation, error) {
	if s == nil || s.repo == nil {
		return []models.WriterObservation{}, nil
	}
	return s.repo.ListObservations(ctx, userID, universeID, 5000)
}

// RecordFeedback persists an explicit intent event and applies its consequence.
// If the event reaches the configured corroboration threshold, promotion is
// attempted from the matching observations. Behavioural acceptance is only
// accepted when a before/after comparison is present within a bounded window.
func (s *WriterMemoryService) RecordFeedback(ctx context.Context, input WriterFeedbackInput) (*models.WriterPreference, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("writer memory is not configured")
	}
	if input.UserID == uuid.Nil {
		return nil, fmt.Errorf("%w: user id is required", ErrInvalidWriterFeedback)
	}
	if input.Signal != "accept" && input.Signal != "reject" && input.Signal != "behavioural_accept" {
		return nil, fmt.Errorf("%w: invalid feedback signal %q", ErrInvalidWriterFeedback, input.Signal)
	}
	if input.Signal == "behavioural_accept" && !validBehaviouralPayload(input.Payload, s.now()) {
		return nil, fmt.Errorf("%w: behavioural_accept requires a changed before/after paragraph within ten minutes", ErrInvalidWriterFeedback)
	}
	if input.Payload == nil {
		input.Payload = map[string]interface{}{}
	}
	if input.PreferenceID != nil {
		if _, err := s.repo.FindPreference(ctx, input.UserID, *input.PreferenceID); err != nil {
			// Validate ownership before inserting the event. Otherwise a caller
			// could attach an event to another user's preference and only fail
			// after the evidence row had already been persisted.
			return nil, fmt.Errorf("%w: %v", ErrWriterPreferenceNotFound, err)
		}
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal feedback payload: %w", err)
	}
	event := &models.WriterFeedbackEvent{
		ID: uuid.New(), UserID: input.UserID, UniverseID: input.UniverseID,
		ChapterID: input.ChapterID, NoteID: input.NoteID, Signal: input.Signal,
		PreferenceID: input.PreferenceID, Payload: payload, CreatedAt: s.now().UTC(),
	}
	if err := s.repo.CreateFeedbackEvent(ctx, event); err != nil {
		return nil, err
	}

	if input.PreferenceID != nil {
		if err := s.applyFeedbackToPreference(ctx, input.UserID, *input.PreferenceID, input.Signal); err != nil {
			return nil, err
		}
		return s.repo.FindPreference(ctx, input.UserID, *input.PreferenceID)
	}

	return s.promoteIfCorroborated(ctx, input.UserID, event)
}

// PromoteObservation is useful to offline/corpus workflows and tests: it
// evaluates all unlinked intent events for one observation using the same
// threshold and structured promotion path as RecordFeedback.
func (s *WriterMemoryService) PromoteObservation(ctx context.Context, userID, observationID uuid.UUID) (*models.WriterPreference, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("writer memory is not configured")
	}
	observations, err := s.repo.ListObservationsByIDs(ctx, userID, []uuid.UUID{observationID})
	if err != nil {
		return nil, err
	}
	if len(observations) == 0 {
		return nil, fmt.Errorf("observation %s not found", observationID)
	}
	events, err := s.repo.ListFeedbackEvents(ctx, userID, nil, 1000)
	if err != nil {
		return nil, err
	}
	return s.promoteFromEvidence(ctx, userID, observations[0], events)
}

func (s *WriterMemoryService) promoteIfCorroborated(ctx context.Context, userID uuid.UUID, event *models.WriterFeedbackEvent) (*models.WriterPreference, error) {
	observations, err := s.repo.ListObservations(ctx, userID, event.UniverseID, 5000)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.ListFeedbackEvents(ctx, userID, nil, 1000)
	if err != nil {
		return nil, err
	}
	if len(observations) == 0 {
		return nil, nil
	}
	// A payload can identify the exact metric/observation. For the simple
	// craft-note flow, the newest observation in the universe is the intended
	// corroboration and keeps the event contract lightweight.
	observation := observations[0]
	if id := payloadUUID(event.Payload, "observation_id"); id != uuid.Nil {
		for _, candidate := range observations {
			if candidate.ID == id {
				observation = candidate
				break
			}
		}
	}
	return s.promoteFromEvidence(ctx, userID, observation, events)
}

func (s *WriterMemoryService) promoteFromEvidence(ctx context.Context, userID uuid.UUID, observation models.WriterObservation, events []models.WriterFeedbackEvent) (*models.WriterPreference, error) {
	matching := make([]models.WriterFeedbackEvent, 0)
	counts := map[string]int{}
	for _, event := range events {
		if event.PreferenceID != nil || event.Signal == "" {
			continue
		}
		if !eventMatchesObservation(event, observation) {
			continue
		}
		matching = append(matching, event)
		counts[event.Signal]++
	}
	if len(matching) < s.promotionThreshold {
		return nil, nil
	}
	winningSignal := ""
	winningCount := 0
	for signal, count := range counts {
		if count > winningCount {
			winningSignal, winningCount = signal, count
		}
	}
	if winningCount < s.promotionThreshold {
		return nil, nil
	}

	promotion, err := s.classifyPromotion(ctx, observation, matching, winningSignal)
	if err != nil {
		return nil, err
	}
	promotion.Scope, promotion.GenreTags = normalizePreferenceScope(promotion.Scope, promotion.GenreTags)
	if err := validatePreferenceScope(promotion.Scope, promotion.GenreTags); err != nil {
		return nil, err
	}
	if strings.TrimSpace(promotion.Statement) == "" {
		promotion.Statement = fallbackPreferenceStatement(observation.Metric, winningSignal)
	}
	promotion.Confidence = clamp01(promotion.Confidence)
	if promotion.Confidence == 0 {
		promotion.Confidence = 0.5
	}

	preference := &models.WriterPreference{
		ID: uuid.New(), UserID: userID, Statement: strings.TrimSpace(promotion.Statement),
		Scope: promotion.Scope, GenreTags: promotion.GenreTags,
		Confidence: promotion.Confidence, RelevanceScore: 1, Lifecycle: "active",
		ObservationIDs: []uuid.UUID{observation.ID}, CreatedAt: s.now().UTC(), LastReinforcedAt: s.now().UTC(),
	}
	if s.llm != nil {
		if embedding, embedErr := s.llm.GenerateEmbedding(ctx, preference.Statement); embedErr == nil {
			preference.Embedding = embedding
		} else {
			log.Printf("[writer-memory] preference embedding skipped: %v", embedErr)
		}
	}
	for _, event := range matching {
		preference.FeedbackEventIDs = append(preference.FeedbackEventIDs, event.ID)
	}
	if err := s.repo.CreatePreference(ctx, preference); err != nil {
		return nil, err
	}
	for _, event := range matching {
		if err := s.repo.AttachFeedbackPreference(ctx, userID, event.ID, preference.ID); err != nil {
			return nil, err
		}
	}
	if err := s.recordPreferenceHistory(ctx, preference); err != nil {
		log.Printf("[writer-memory] append promotion history: %v", err)
	}
	return preference, nil
}

func (s *WriterMemoryService) classifyPromotion(ctx context.Context, observation models.WriterObservation, events []models.WriterFeedbackEvent, signal string) (writerPromotionResult, error) {
	prompt := fmt.Sprintf(`Promote this writer-memory observation only because it has corroborating intent signals. Observation: metric=%s value=%.4f sample_size=%d. Signals: %d consistent %s events. Evidence payloads: %s. Return only JSON with statement, scope (universal or genre_bound), genre_tags, and confidence 0..1. Never infer intent from the observation without the signals.`, observation.Metric, observation.Value, observation.SampleSize, len(events), signal, summarizeFeedbackPayloads(events))
	messages := []QwenMessage{{Role: "system", Content: "You classify intent-backed writer preferences. Return JSON only."}, {Role: "user", Content: prompt}}
	if structured, ok := s.llm.(StructuredChat); ok {
		content, err := structured.ChatStructured(ctx, "", messages, writerPreferenceSchema)
		if err != nil {
			return writerPromotionResult{}, fmt.Errorf("promote writer preference: %w", err)
		}
		var result writerPromotionResult
		if err := parseJSONLoose(content, &result); err != nil {
			return writerPromotionResult{}, fmt.Errorf("parse writer preference: %w", err)
		}
		return result, nil
	}
	if s.llm != nil {
		content, err := s.llm.Chat(ctx, "", messages)
		if err == nil {
			var result writerPromotionResult
			if parseJSONLoose(content, &result) == nil {
				return result, nil
			}
		}
	}
	// Offline/test fallback is deterministic but still requires the intent
	// threshold above; it never promotes from stylometry alone.
	return writerPromotionResult{
		Statement: fallbackPreferenceStatement(observation.Metric, signal),
		Scope:     "universal", Confidence: 0.5,
	}, nil
}

func (s *WriterMemoryService) applyFeedbackToPreference(ctx context.Context, userID, preferenceID uuid.UUID, signal string) error {
	preference, err := s.repo.FindPreference(ctx, userID, preferenceID)
	if err != nil {
		return err
	}
	switch signal {
	case "reject":
		preference.Confidence = clamp01(preference.Confidence * applyDecay(1, 1, s.lambda))
		preference.RelevanceScore = clamp01(preference.RelevanceScore * applyDecay(1, 1, s.lambda))
	case "accept":
		preference.Confidence = clamp01(preference.Confidence + 0.1)
		preference.RelevanceScore = 1
		preference.Lifecycle = "active"
	case "behavioural_accept":
		preference.Confidence = clamp01(preference.Confidence + 0.05)
		preference.RelevanceScore = 1
		preference.Lifecycle = "active"
	}
	if preference.RelevanceScore <= s.archiveThreshold {
		preference.Lifecycle = "archived"
	}
	preference.LastReinforcedAt = s.now().UTC()
	if err := s.repo.UpdatePreference(ctx, preference); err != nil {
		return err
	}
	return s.recordPreferenceHistory(ctx, preference)
}

// DecayForUniverse applies one event-driven tick to the writer profile owned
// by a universe. It uses applyDecay, the same exponential math as entities,
// and appends one history row per preference.
func (s *WriterMemoryService) DecayForUniverse(ctx context.Context, universeID uuid.UUID) error {
	if s == nil || s.repo == nil || universeID == uuid.Nil {
		return nil
	}
	preferences, err := s.repo.ListActiveForUniverseOwner(ctx, universeID, 500)
	if err != nil {
		return err
	}
	for i := range preferences {
		preferences[i].RelevanceScore = clamp01(applyDecay(preferences[i].RelevanceScore, 1, s.lambda))
		if preferences[i].RelevanceScore <= s.archiveThreshold {
			preferences[i].Lifecycle = "archived"
		}
		if err := s.repo.UpdatePreference(ctx, &preferences[i]); err != nil {
			return err
		}
		if err := s.recordPreferenceHistory(ctx, &preferences[i]); err != nil {
			log.Printf("[writer-memory] append decay history: %v", err)
		}
	}
	return nil
}

func (s *WriterMemoryService) recordPreferenceHistory(ctx context.Context, preference *models.WriterPreference) error {
	id := preference.ID
	return s.repo.AppendPreferenceHistory(ctx, &models.WriterPreferenceHistory{
		UserID: preference.UserID, PreferenceID: &id, RelevanceScore: preference.RelevanceScore,
		Confidence: preference.Confidence, Lifecycle: preference.Lifecycle, RecordedAt: s.now().UTC(),
	})
}

func (s *WriterMemoryService) Evidence(ctx context.Context, userID, preferenceID uuid.UUID) (*models.WriterPreferenceEvidence, error) {
	preference, err := s.repo.FindPreference(ctx, userID, preferenceID)
	if err != nil {
		return nil, err
	}
	observations, err := s.repo.ListObservationsByIDs(ctx, userID, preference.ObservationIDs)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.ListFeedbackEvents(ctx, userID, &preferenceID, 1000)
	if err != nil {
		return nil, err
	}
	history, err := s.repo.ListPreferenceHistory(ctx, userID, preferenceID, 1000)
	if err != nil {
		return nil, err
	}
	return &models.WriterPreferenceEvidence{Preference: *preference, Observations: observations, Events: events, History: history}, nil
}

func (s *WriterMemoryService) Correct(ctx context.Context, userID, preferenceID uuid.UUID, scope string, genreTags []string) (*models.WriterPreference, error) {
	if err := validatePreferenceScope(scope, genreTags); err != nil {
		return nil, err
	}
	preference, err := s.repo.FindPreference(ctx, userID, preferenceID)
	if err != nil {
		return nil, err
	}
	preference.Scope, preference.GenreTags = normalizePreferenceScope(scope, genreTags)
	if err := s.repo.UpdatePreference(ctx, preference); err != nil {
		return nil, err
	}
	return preference, nil
}

func (s *WriterMemoryService) Deactivate(ctx context.Context, userID, preferenceID uuid.UUID) error {
	if err := s.repo.DeactivatePreference(ctx, userID, preferenceID); err != nil {
		return err
	}
	preference, err := s.repo.FindPreference(ctx, userID, preferenceID)
	if err == nil {
		if historyErr := s.recordPreferenceHistory(ctx, preference); historyErr != nil {
			log.Printf("[writer-memory] append deactivation history: %v", historyErr)
		}
	}
	return nil
}

// ShouldSuppress is the craft-consumer contract. It returns true only when an
// active preference has the same category rejected at least the configured
// number of times. Silence is never counted because no silent event can be
// persisted by RecordFeedback.
func (s *WriterMemoryService) ShouldSuppress(ctx context.Context, userID, universeID uuid.UUID, category string) (bool, error) {
	if strings.TrimSpace(category) == "" {
		return false, nil
	}
	preferences, err := s.repo.ListActiveForUniverse(ctx, universeID, 500)
	if err != nil {
		return false, err
	}
	for _, preference := range preferences {
		id := preference.ID
		events, err := s.repo.ListFeedbackEvents(ctx, userID, &id, 1000)
		if err != nil {
			return false, err
		}
		rejections := 0
		for _, event := range events {
			if event.Signal != "reject" || !strings.EqualFold(payloadString(event.Payload, "category"), category) {
				continue
			}
			rejections++
		}
		if rejections >= s.promotionThreshold {
			return true, nil
		}
	}
	return false, nil
}

func validBehaviouralPayload(payload map[string]interface{}, now time.Time) bool {
	before, beforeOK := payload["before"].(string)
	after, afterOK := payload["after"].(string)
	if !beforeOK || !afterOK || strings.TrimSpace(before) == "" || before == after {
		return false
	}
	observed, observedOK := payload["observed_at"].(string)
	if !observedOK || observed == "" {
		return false
	}
	when, err := time.Parse(time.RFC3339, observed)
	if err != nil || now.Sub(when) < 0 || now.Sub(when) > writerBehaviourWindow {
		return false
	}
	return true
}

func eventMatchesObservation(event models.WriterFeedbackEvent, observation models.WriterObservation) bool {
	if id := payloadUUID(event.Payload, "observation_id"); id != uuid.Nil {
		return id == observation.ID
	}
	if metric := payloadString(event.Payload, "metric"); metric != "" {
		return metric == observation.Metric
	}
	return true
}

func summarizeFeedbackPayloads(events []models.WriterFeedbackEvent) string {
	parts := make([]string, 0, len(events))
	for _, event := range events {
		if len(event.Payload) == 0 {
			continue
		}
		parts = append(parts, string(event.Payload))
	}
	return strings.Join(parts, "; ")
}

func payloadString(raw json.RawMessage, key string) string {
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	value, _ := payload[key].(string)
	return value
}

func payloadUUID(raw json.RawMessage, key string) uuid.UUID {
	value := payloadString(raw, key)
	id, _ := uuid.Parse(value)
	return id
}

func fallbackPreferenceStatement(metric, signal string) string {
	verb := "values"
	if signal == "reject" {
		verb = "deliberately keeps"
	}
	switch metric {
	case MetricMeanSentenceLength:
		return "The writer " + verb + " long sentence lengths."
	case MetricAdverbDensity:
		return "The writer " + verb + " adverb-rich phrasing."
	case MetricDialogueRatio:
		return "The writer " + verb + " a dialogue-forward balance."
	case MetricLexicalRichness:
		return "The writer " + verb + " varied vocabulary."
	default:
		return "The writer " + verb + " this craft choice."
	}
}

func normalizePreferenceScope(scope string, tags []string) (string, []string) {
	scope = strings.TrimSpace(scope)
	if scope == "universal" {
		return scope, []string{}
	}
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		found := false
		for _, existing := range normalized {
			if existing == tag {
				found = true
				break
			}
		}
		if !found {
			normalized = append(normalized, tag)
		}
	}
	return scope, normalized
}

func validatePreferenceScope(scope string, tags []string) error {
	scope = strings.TrimSpace(scope)
	switch scope {
	case "universal":
		if len(tags) != 0 {
			return fmt.Errorf("%w: universal preferences cannot include genre_tags", ErrInvalidWriterPreference)
		}
		return nil
	case "genre_bound":
		if len(tags) == 0 {
			return fmt.Errorf("%w: genre_bound preferences require at least one genre_tag", ErrInvalidWriterPreference)
		}
		for _, tag := range tags {
			if _, ok := allowedGenreTags[tag]; !ok {
				return fmt.Errorf("%w: invalid genre_tag %q", ErrInvalidWriterPreference, tag)
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: scope must be universal or genre_bound", ErrInvalidWriterPreference)
	}
}

func clamp01(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
