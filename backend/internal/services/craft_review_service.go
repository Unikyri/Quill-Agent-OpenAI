package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
)

const maxCraftReviewSkills = 3

type skillActivationReader interface {
	ListActive(ctx context.Context, universeID uuid.UUID) ([]models.UniverseSkill, error)
}

type universeReader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Universe, error)
}

type craftSelectionResponse struct {
	Selected  []string `json:"selected"`
	Rationale string   `json:"rationale,omitempty"`
}

type craftStageTwoResponse struct {
	Notes []models.CraftReviewNote `json:"notes"`
}

// CraftReviewService performs the on-demand two-stage editorial review. The
// first call sees descriptions only and selects at most three active skills;
// the second call receives only those full bodies plus matching genre refs.
type CraftReviewService struct {
	registry       *SkillRegistry
	activations    skillActivationReader
	universes      universeReader
	llm            LLMService
	memory         *MemoryService
	writerMemory   *WriterMemoryService
	selectionModel string
	reviewModel    string
}

func NewCraftReviewService(
	registry *SkillRegistry,
	activations skillActivationReader,
	universes universeReader,
	llm LLMService,
	memory *MemoryService,
	writerMemory *WriterMemoryService,
	selectionModel, reviewModel string,
) *CraftReviewService {
	if strings.TrimSpace(selectionModel) == "" {
		selectionModel = "qwen-turbo"
	}
	if strings.TrimSpace(reviewModel) == "" {
		reviewModel = "qwen-max"
	}
	return &CraftReviewService{
		registry: registry, activations: activations, universes: universes,
		llm: llm, memory: memory, writerMemory: writerMemory,
		selectionModel: selectionModel, reviewModel: reviewModel,
	}
}

func (s *CraftReviewService) Review(ctx context.Context, userID uuid.UUID, req models.CraftReviewRequestPayload) (models.CraftReviewResultPayload, error) {
	result := models.CraftReviewResultPayload{
		UniverseID: req.UniverseID, WorkID: req.WorkID, ChapterID: req.ChapterID,
		Selections: []models.CraftReviewSelection{}, Notes: []models.CraftReviewNote{},
	}
	if req.UniverseID == uuid.Nil || req.WorkID == uuid.Nil || req.ChapterID == uuid.Nil || strings.TrimSpace(req.Passage) == "" {
		return result, errors.New("universe, work, chapter, and passage are required")
	}
	if s.registry == nil || s.activations == nil || s.universes == nil {
		return result, errors.New("craft review is not configured")
	}
	universe, err := s.universes.FindByID(ctx, req.UniverseID)
	if err != nil {
		return result, err
	}
	if universe == nil || universe.UserID != userID {
		return result, ErrUniverseAccessDenied
	}

	activeRows, err := s.activations.ListActive(ctx, req.UniverseID)
	if err != nil {
		return result, fmt.Errorf("list active skills: %w", err)
	}
	active := make([]string, 0, len(activeRows))
	for _, row := range activeRows {
		active = append(active, row.SkillName)
	}
	active, err = s.registry.ValidateNames(active)
	if err != nil {
		return result, err
	}
	if len(active) == 0 {
		return result, nil
	}

	selection, err := s.selectSkills(ctx, req, universe, active)
	if err != nil {
		return result, err
	}
	selected := limitSelected(selection.Selected, active)
	if len(selected) == 0 {
		return result, nil
	}
	for _, name := range selected {
		rationale := selection.Rationale
		result.Selections = append(result.Selections, models.CraftReviewSelection{Skill: name, Rationale: rationale})
		log.Printf("[craft-review] selected skill=%s rationale=%q universe=%s", name, rationale, req.UniverseID)
	}

	fullContext, err := s.registry.PromptContext(selected, universe.GenreTags)
	if err != nil {
		return result, err
	}
	recalled := s.recallContext(ctx, req)
	preferences := s.preferenceContext(ctx, req.UniverseID)
	notes, err := s.reviewSelected(ctx, req, universe, selected, fullContext, recalled, preferences)
	if err != nil {
		return result, err
	}
	result.Notes = s.filterNotes(ctx, userID, req.UniverseID, req.Passage, selected, notes)
	return result, nil
}

func (s *CraftReviewService) selectSkills(ctx context.Context, req models.CraftReviewRequestPayload, universe *models.Universe, active []string) (craftSelectionResponse, error) {
	descriptions := make([]string, 0, len(active))
	for _, name := range active {
		_, item, ok := s.registry.Get(name)
		if !ok {
			continue
		}
		descriptions = append(descriptions, fmt.Sprintf("- %s: %s (stage: %s)", item.Name, item.Description, item.Stage))
	}
	prompt := fmt.Sprintf(`Select up to three editorial skills from the active set for this passage. Return JSON only with {"selected":["skill-name"],"rationale":"brief reason"}. Every selected value MUST be one of the active names. Do not write a review or rewrite.

Active skill descriptions:
%s

Genre tags: %s
Review request context: %s
Passage:
%s`, strings.Join(descriptions, "\n"), strings.Join(universe.GenreTags, ", "), req.Context, req.Passage)
	messages := []QwenMessage{
		{Role: "system", Content: "You are the skill selector for a writing craft review. Return JSON only."},
		{Role: "user", Content: prompt},
	}
	format := craftSelectionFormat(active)
	var response craftSelectionResponse
	if structured, ok := s.llm.(StructuredChat); ok {
		content, err := structured.ChatStructured(ctx, s.selectionModel, messages, format)
		if err != nil {
			return response, fmt.Errorf("select craft skills: %w", err)
		}
		if err := parseJSONLoose(content, &response); err != nil {
			return response, fmt.Errorf("parse craft skill selection: %w", err)
		}
		return response, nil
	}
	if s.llm != nil {
		content, err := s.llm.Chat(ctx, s.selectionModel, messages)
		if err == nil && parseJSONLoose(content, &response) == nil {
			return response, nil
		}
	}
	// Offline fallback keeps the review usable in tests and degraded demos; it
	// still respects the active set and hard cap.
	response.Selected = append([]string(nil), active...)
	if len(response.Selected) > maxCraftReviewSkills {
		response.Selected = response.Selected[:maxCraftReviewSkills]
	}
	response.Rationale = "deterministic fallback: first active skills"
	return response, nil
}

func (s *CraftReviewService) reviewSelected(ctx context.Context, req models.CraftReviewRequestPayload, universe *models.Universe, selected []string, skillContext, recalled, preferences string) ([]models.CraftReviewNote, error) {
	prompt := fmt.Sprintf(`Review the passage using ONLY the selected skill instructions below. Return JSON only as {"notes":[{"skill":"...","quote":"exact passage quote","note":"concise observation","severity":"info|suggestion|warning","category":"optional category"}]}. Never rewrite the passage, never provide replacement prose, and keep quotes verbatim. A note must identify an observation grounded in the quoted text.

Selected skills and matching genre references:
%s

Relevant lore recall:
%s

Active writer preferences (suppress a note when it contradicts one of these preferences):
%s

Genre tags: %s
Additional request context: %s
Passage:
%s`, skillContext, recalled, preferences, strings.Join(universe.GenreTags, ", "), req.Context, req.Passage)
	messages := []QwenMessage{
		{Role: "system", Content: "You are a craft reviewer. Produce diagnostic notes only; never rewrite. Return JSON only."},
		{Role: "user", Content: prompt},
	}
	format := craftNotesFormat(selected)
	var response craftStageTwoResponse
	if structured, ok := s.llm.(StructuredChat); ok {
		content, err := structured.ChatStructured(ctx, s.reviewModel, messages, format)
		if err != nil {
			return nil, fmt.Errorf("review craft passage: %w", err)
		}
		if err := parseJSONLoose(content, &response); err != nil {
			return nil, fmt.Errorf("parse craft notes: %w", err)
		}
		return response.Notes, nil
	}
	if s.llm != nil {
		content, err := s.llm.Chat(ctx, s.reviewModel, messages)
		if err == nil && parseJSONLoose(content, &response) == nil {
			return response.Notes, nil
		}
	}
	return []models.CraftReviewNote{}, nil
}

func (s *CraftReviewService) recallContext(ctx context.Context, req models.CraftReviewRequestPayload) string {
	if s.memory == nil || s.llm == nil {
		return "(none available)"
	}
	embedding, err := s.llm.GenerateEmbedding(ctx, req.Passage)
	if err != nil {
		log.Printf("[craft-review] lore embedding skipped: %v", err)
		return "(none available)"
	}
	items, err := s.memory.RecallWithQuery(ctx, req.UniverseID, embedding, req.Passage, 5)
	if err != nil {
		log.Printf("[craft-review] lore recall skipped: %v", err)
		return "(none available)"
	}
	if len(items) == 0 {
		return "(none available)"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("- %s", item.Fact))
	}
	return strings.Join(parts, "\n")
}

func (s *CraftReviewService) preferenceContext(ctx context.Context, universeID uuid.UUID) string {
	if s.writerMemory == nil {
		return "(none available)"
	}
	preferences, err := s.writerMemory.ListPreferencesForUniverse(ctx, universeID)
	if err != nil || len(preferences) == 0 {
		return "(none available)"
	}
	parts := make([]string, 0, len(preferences))
	for _, preference := range preferences {
		parts = append(parts, "- "+preference.Statement)
	}
	return strings.Join(parts, "\n")
}

func (s *CraftReviewService) filterNotes(ctx context.Context, userID, universeID uuid.UUID, passage string, selected []string, notes []models.CraftReviewNote) []models.CraftReviewNote {
	allowed := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		allowed[name] = struct{}{}
	}
	filtered := make([]models.CraftReviewNote, 0, len(notes))
	for _, note := range notes {
		note.Skill = strings.TrimSpace(note.Skill)
		note.Quote = strings.TrimSpace(note.Quote)
		note.Note = strings.TrimSpace(note.Note)
		note.Category = strings.TrimSpace(note.Category)
		note.Severity = strings.TrimSpace(note.Severity)
		if _, ok := allowed[note.Skill]; !ok || note.Quote == "" || note.Note == "" || !strings.Contains(passage, note.Quote) {
			continue
		}
		if note.Severity == "" {
			note.Severity = "suggestion"
		}
		if craftRewriteOverlap(note.Note, note.Quote) {
			log.Printf("[craft-review] dropped rewrite-like note skill=%s", note.Skill)
			continue
		}
		if s.writerMemory != nil {
			categories := []string{note.Category, note.Skill}
			suppressed := false
			seen := make(map[string]struct{}, len(categories))
			for _, category := range categories {
				category = strings.TrimSpace(category)
				if category == "" {
					continue
				}
				if _, ok := seen[category]; ok {
					continue
				}
				seen[category] = struct{}{}
				suppress, err := s.writerMemory.ShouldSuppress(ctx, userID, universeID, category)
				if err != nil {
					log.Printf("[craft-review] suppression lookup skipped: %v", err)
					continue
				}
				if suppress {
					suppressed = true
					break
				}
			}
			if suppressed {
				continue
			}
		}
		if note.ID == uuid.Nil {
			note.ID = uuid.New()
		}
		filtered = append(filtered, note)
	}
	return filtered
}

func limitSelected(selected, active []string) []string {
	allowed := make(map[string]struct{}, len(active))
	for _, name := range active {
		allowed[name] = struct{}{}
	}
	result := make([]string, 0, maxCraftReviewSkills)
	seen := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		name = strings.TrimSpace(name)
		if _, ok := allowed[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
		if len(result) == maxCraftReviewSkills {
			break
		}
	}
	return result
}

func craftSelectionFormat(active []string) *ResponseFormat {
	enum := make([]interface{}, len(active))
	for i, name := range active {
		enum[i] = name
	}
	return &ResponseFormat{Type: "json_schema", JSONSchema: &JSONSchemaResponseFormat{
		Name: "craft_skill_selection", Description: "Select at most three active skills.", Strict: true,
		Schema: map[string]interface{}{
			"type": "object", "properties": map[string]interface{}{
				"selected":  map[string]interface{}{"type": "array", "maxItems": maxCraftReviewSkills, "items": map[string]interface{}{"type": "string", "enum": enum}},
				"rationale": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"selected"}, "additionalProperties": false,
		},
	}}
}

func craftNotesFormat(selected []string) *ResponseFormat {
	enum := make([]interface{}, len(selected))
	for i, name := range selected {
		enum[i] = name
	}
	return &ResponseFormat{Type: "json_schema", JSONSchema: &JSONSchemaResponseFormat{
		Name: "craft_review_notes", Description: "Diagnostic, non-rewrite craft notes.", Strict: true,
		Schema: map[string]interface{}{
			"type": "object", "properties": map[string]interface{}{
				"notes": map[string]interface{}{"type": "array", "items": map[string]interface{}{
					"type": "object", "properties": map[string]interface{}{
						"skill":    map[string]interface{}{"type": "string", "enum": enum},
						"quote":    map[string]interface{}{"type": "string"},
						"note":     map[string]interface{}{"type": "string"},
						"severity": map[string]interface{}{"type": "string", "enum": []interface{}{"info", "suggestion", "warning"}},
						"category": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"skill", "quote", "note", "severity"}, "additionalProperties": false,
				}},
			},
			"required": []interface{}{"notes"}, "additionalProperties": false,
		},
	}}
}

// craftRewriteOverlap catches an accidental rewrite disguised as a note. It
// compares the smaller token set so a note that mostly repeats its quote is
// rejected while a concise observation remains valid.
func craftRewriteOverlap(note, quote string) bool {
	noteTokens := craftTokens(note)
	quoteTokens := craftTokens(quote)
	if len(noteTokens) < 4 || len(quoteTokens) < 4 {
		return false
	}
	quoteSet := make(map[string]struct{}, len(quoteTokens))
	for _, token := range quoteTokens {
		quoteSet[token] = struct{}{}
	}
	intersection := 0
	seen := make(map[string]struct{}, len(noteTokens))
	for _, token := range noteTokens {
		if _, duplicate := seen[token]; duplicate {
			continue
		}
		seen[token] = struct{}{}
		if _, ok := quoteSet[token]; ok {
			intersection++
		}
	}
	denominator := len(noteTokens)
	if len(quoteTokens) < denominator {
		denominator = len(quoteTokens)
	}
	return denominator > 0 && float64(intersection)/float64(denominator) >= 0.8
}

func craftTokens(value string) []string {
	words := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	result := make([]string, 0, len(words))
	for _, word := range words {
		if word != "" {
			result = append(result, word)
		}
	}
	sort.Strings(result)
	return result
}
