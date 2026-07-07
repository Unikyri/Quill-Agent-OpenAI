package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// ContradictionService checks for narrative contradictions using rule-based
// deterministic checks and batched semantic checks via Qwen-Max.
//
// ponytail: SHA-256 fingerprint dedup — no DB round-trip for duplicate check.
// Deterministic rules catch deceased/alive without API call.
type ContradictionService struct {
	pool          *pgxpool.Pool
	contraRepo    *repositories.ContradictionRepo
	entityRepo    *repositories.EntityRepo
	qwenSvc       *QwenService
	executor      ToolExecutor
	maxCandidates int
	budgetMgr     *ContextBudgetManager
}

// NewContradictionService creates a contradiction service.
// qwenSvc may be nil — CheckSemantic will be a no-op in that case.
// executor may be nil — CheckSemantic falls back to batch mode without agent loop.
// budgetMgr may be nil — CheckSemantic then concatenates all entities uncapped
// (current behavior).
func NewContradictionService(pool *pgxpool.Pool, contraRepo *repositories.ContradictionRepo, entityRepo *repositories.EntityRepo, qwenSvc *QwenService, executor ToolExecutor, maxCandidates int, budgetMgr *ContextBudgetManager) *ContradictionService {
	return &ContradictionService{
		pool:          pool,
		contraRepo:    contraRepo,
		entityRepo:    entityRepo,
		qwenSvc:       qwenSvc,
		executor:      executor,
		maxCandidates: maxCandidates,
		budgetMgr:     budgetMgr,
	}
}

// fingerprint produces a SHA-256 hex fingerprint for deduplication.
// Exported for testing.
func (s *ContradictionService) fingerprint(c ContradictionCandidate) string {
	h := sha256.New()
	h.Write([]byte(c.EntityID.String()))
	h.Write([]byte(c.Type))
	h.Write([]byte(c.EvidenceA))
	h.Write([]byte(c.EvidenceB))
	h.Write([]byte(c.ChapterA.String()))
	h.Write([]byte(c.ChapterB.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// CheckDeterministic runs fast rule-based contradiction checks (no API call):
//   - deceased/alive: if entity is marked "deceased" but mentioned as alive in new text
//
// chapterID is threaded into the candidate fingerprint so contradictions are
// scoped to the originating chapter context.
//
// ponytail: rule-based checks only; semantic checks go through Qwen-Max.
func (s *ContradictionService) CheckDeterministic(ctx context.Context, universeID uuid.UUID, chapterID uuid.UUID, entities []ResolvedEntity) ([]models.Contradiction, error) {
	var results []models.Contradiction

	for _, re := range entities {
		e := re.Entity

		// Deceased / alive check: if entity DB status was "deceased" *before*
		// this mention's data was merged in, and the new mention suggests
		// they're alive, flag it. Must use PreviousStatus, not e.Status —
		// ResolveOrCreate already overwrote Status with the newly-extracted
		// value by the time this runs.
		if re.PreviousStatus == "deceased" {
			fp := s.fingerprint(ContradictionCandidate{
				EntityID:  e.ID,
				Type:      "deceased_alive",
				EvidenceA: fmt.Sprintf("Entity %s is deceased in DB", e.Name),
				EvidenceB: re.MentionText,
				ChapterA:  chapterID,
				ChapterB:  chapterID,
			})

			// Check if we already have this contradiction
			existing, _ := s.contraRepo.FindByFingerprint(ctx, fp)
			if existing != nil {
				continue // already recorded
			}

			c := models.Contradiction{
				ID:         uuid.New(),
				UniverseID: universeID,
				EntityID:   &e.ID,
				Severity:   "critical",
				Description: fmt.Sprintf(
					"Entity '%s' is marked as deceased but was mentioned as active: \"%s\"",
					e.Name, truncate(re.MentionText, 80),
				),
				Suggestion:  "Review timeline: was this entity revived, or is this a continuity error?",
				EvidenceA:   fmt.Sprintf("Entity %s status: deceased", e.Name),
				EvidenceB:   re.MentionText,
				Fingerprint: fp,
				Status:      "open",
			}
			results = append(results, c)
		}
	}

	return results, nil
}

// CheckSemantic uses an agent loop with tool access to detect narrative contradictions.
// The agent is instructed to use search_vector_memory and query_entity_graph tools
// before making a decision. The final answer is parsed as a JSON contradiction array.
//
// Falls back to no-op when qwenSvc or executor is nil.
//
// chapterID is threaded into the candidate fingerprint so contradictions are
// scoped to the originating chapter context.
//
// onProgress is optional (variadic so existing call sites are unaffected).
// When a non-nil progress sink is provided, CheckSemantic drives the agent
// loop via RunAgentLoopStream instead of the synchronous RunAgentLoop, and
// forwards each completed-tool-call event to it.
func (s *ContradictionService) CheckSemantic(ctx context.Context, universeID uuid.UUID, chapterID uuid.UUID, text string, entities []ResolvedEntity, onProgress ...func(stage string, tc *QwenToolCall)) ([]models.Contradiction, error) {
	if s.qwenSvc == nil || s.executor == nil {
		return nil, nil
	}

	if len(entities) == 0 {
		return nil, nil
	}

	// ponytail: set UniverseID on the executor so tool calls resolve in the right universe.
	// QuillExecutor has a public UniverseID field; other executors may ignore it.
	if qe, ok := s.executor.(*QuillExecutor); ok {
		qe.UniverseID = universeID
	}

	systemPrompt := `You are a narrative consistency analyst. Your job is to detect contradictions in a story.

You have access to these tools:
- search_vector_memory: search the story's memory for contextual facts
- query_entity_graph: explore how entities are connected in the knowledge graph

For each contradiction you find, output a JSON array. Each contradiction must have:
- "type": the category (e.g. "semantic", "status_contradiction", "timeline_mismatch")
- "description": a human-readable explanation of the contradiction
- "evidence_a": the first piece of conflicting evidence
- "evidence_b": the second piece of conflicting evidence
- "severity": "low", "medium", "high", or "critical"

IMPORTANT: Use the tools to gather context BEFORE making your decision. Only return contradictions that are actually supported by the evidence. If no contradictions exist, return an empty array: []`

	const userMessageTemplate = "Analyze the following new text for contradictions with existing story data:\n\nNew text: %s\n\nKnown entities:\n%s"

	entityLines := s.buildEntityLines(entities, systemPrompt, userMessageTemplate, text)

	userMessage := fmt.Sprintf(userMessageTemplate, text, entityLines)

	messages := []QwenMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
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
		{
			Type: "function",
			Function: QwenToolFunction{
				Name:        "query_entity_graph",
				Description: "Query the entity knowledge graph to find connections and neighbors of a named entity.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"entity_name": map[string]interface{}{
							"type":        "string",
							"description": "The exact name of the entity to look up",
						},
					},
					"required": []string{"entity_name"},
				},
			},
		},
	}

	var progress func(stage string, tc *QwenToolCall)
	if len(onProgress) > 0 {
		progress = onProgress[0]
	}

	var answer string
	var err error
	if progress != nil {
		answer, err = s.qwenSvc.RunAgentLoopStream(ctx, messages, tools, s.executor, 5, progress)
	} else {
		answer, err = s.qwenSvc.RunAgentLoop(ctx, messages, tools, s.executor, 5)
	}
	if err != nil {
		return nil, fmt.Errorf("check semantic: %w", err)
	}

	// Parse JSON contradiction array from final answer
	type agentContradiction struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		EvidenceA   string `json:"evidence_a"`
		EvidenceB   string `json:"evidence_b"`
		Severity    string `json:"severity"`
	}

	var agentContras []agentContradiction
	if err := parseJSONLoose(answer, &agentContras); err != nil {
		return nil, fmt.Errorf("parse agent contradiction JSON: %w", err)
	}

	contradictions := make([]models.Contradiction, 0, len(agentContras))
	for _, ac := range agentContras {
		c := models.Contradiction{
			ID:          uuid.New(),
			UniverseID:  universeID,
			Severity:    ac.Severity,
			Description: ac.Description,
			EvidenceA:   ac.EvidenceA,
			EvidenceB:   ac.EvidenceB,
			Status:      "open",
		}

		// Compute fingerprint for dedup — use first entity ID when available
		if len(entities) > 0 {
			c.Fingerprint = s.fingerprint(ContradictionCandidate{
				EntityID:  entities[0].Entity.ID,
				Type:      ac.Type,
				EvidenceA: ac.EvidenceA,
				EvidenceB: ac.EvidenceB,
				ChapterA:  chapterID,
				ChapterB:  chapterID,
			})
		}

		// Deduplicate and persist if repos are available
		if s.contraRepo != nil && c.Fingerprint != "" {
			existing, _ := s.contraRepo.FindByFingerprint(ctx, c.Fingerprint)
			if existing != nil {
				continue
			}
			if err := s.contraRepo.Create(ctx, &c); err != nil {
				continue // best-effort persistence
			}
		}

		contradictions = append(contradictions, c)
	}

	return contradictions, nil
}

// buildEntityLines renders the known-entities block for the semantic-check
// user message. With no budgetMgr it concatenates every entity uncapped
// (current behavior). With a budgetMgr, entities are ranked by
// RelevanceScore and fit into the entities share of the token budget so the
// prompt can't blow the context window on large casts.
func (s *ContradictionService) buildEntityLines(entities []ResolvedEntity, systemPrompt, userMessageTemplate, text string) string {
	if s.budgetMgr == nil {
		var entityLines string
		for _, re := range entities {
			entityLines += fmt.Sprintf("- %s (%s): %s\n", re.Entity.Name, re.Entity.Type, re.Entity.Description)
		}
		return entityLines
	}

	items := make([]RankedItem, len(entities))
	for i, re := range entities {
		items[i] = RankedItem{
			Text:  fmt.Sprintf("- %s (%s): %s\n", re.Entity.Name, re.Entity.Type, re.Entity.Description),
			Score: re.Entity.RelevanceScore,
		}
	}

	systemTokens := s.budgetMgr.tok.CountTokens(systemPrompt)
	userBaseTokens := s.budgetMgr.tok.CountTokens(fmt.Sprintf(userMessageTemplate, text, ""))
	alloc := s.budgetMgr.ComputeBudget(systemTokens, userBaseTokens)

	fitted, _, _ := s.budgetMgr.FitToBudget(items, alloc.EntitiesTokens)

	var entityLines string
	for _, item := range fitted {
		entityLines += item.Text
	}
	return entityLines
}

// truncate shortens text to maxLen characters, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}
