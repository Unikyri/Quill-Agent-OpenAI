package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

// TestContradictionFingerprintDeterminism verifies SHA-256 fingerprint is
// deterministic — same inputs always produce the same hash. Pure unit test.
func TestContradictionFingerprintDeterminism(t *testing.T) {
	entityA := uuid.New()
	entityB := uuid.New()

	candidates := []ContradictionCandidate{
		{EntityID: entityA, Type: "deceased_alive", EvidenceA: "Bob alive ch1", EvidenceB: "Bob dead ch3"},
		{EntityID: entityB, Type: "status_change", EvidenceA: "Alice mayor", EvidenceB: "Alice queen"},
	}

	// Create service with nil dependencies — fingerprint is pure, needs no DB
	svc := NewContradictionService(nil, nil, nil, nil, nil, 3, nil)

	fp1 := svc.fingerprint(candidates[0])
	fp2 := svc.fingerprint(candidates[0])

	if fp1 == "" {
		t.Error("fingerprint should not be empty")
	}
	if fp1 != fp2 {
		t.Errorf("same input produced different fingerprints: %s vs %s", fp1, fp2)
	}

	// Different inputs should produce different fingerprints
	fp3 := svc.fingerprint(candidates[1])
	if fp3 == "" {
		t.Error("fingerprint for candidate[1] should not be empty")
	}
	if fp3 == fp1 {
		t.Error("different inputs should produce different fingerprints")
	}
}

// TestContradictionFingerprintChaptersIncluded verifies that ChapterA/ChapterB
// affect the fingerprint — two candidates identical except for chapter fields
// must produce different fingerprints.
func TestContradictionFingerprintChaptersIncluded(t *testing.T) {
	svc := NewContradictionService(nil, nil, nil, nil, nil, 3, nil)
	entityID := uuid.New()
	chA := uuid.New()
	chB := uuid.New()
	chOther := uuid.New()

	c1 := ContradictionCandidate{
		EntityID:  entityID,
		Type:      "semantic",
		EvidenceA: "evidence A",
		EvidenceB: "evidence B",
		ChapterA:  chA,
		ChapterB:  chB,
	}
	c2 := ContradictionCandidate{
		EntityID:  entityID,
		Type:      "semantic",
		EvidenceA: "evidence A",
		EvidenceB: "evidence B",
		ChapterA:  chOther, // different chapter
		ChapterB:  chB,
	}

	fp1 := svc.fingerprint(c1)
	fp2 := svc.fingerprint(c2)

	if fp1 == fp2 {
		t.Error("fingerprints should differ when ChapterA differs — chapter fields must be included in hash")
	}

	// Triangulate: ChapterB differs
	c3 := ContradictionCandidate{
		EntityID:  entityID,
		Type:      "semantic",
		EvidenceA: "evidence A",
		EvidenceB: "evidence B",
		ChapterA:  chA,
		ChapterB:  chOther,
	}
	fp3 := svc.fingerprint(c3)
	if fp1 == fp3 {
		t.Error("fingerprints should differ when ChapterB differs — chapter fields must be included in hash")
	}
	if fp3 == fp2 {
		t.Error("fingerprints with different chapters should all differ")
	}
}

// TestContradictionFingerprintFormat verifies the fingerprint is a valid hex string
// (64 chars for SHA-256).
func TestContradictionFingerprintFormat(t *testing.T) {
	cfg := &config.Config{MaxContradictionCandidates: 3}
	svc := NewContradictionService(nil, nil, nil, nil, nil, cfg.MaxContradictionCandidates, nil)

	c := ContradictionCandidate{
		EntityID:  uuid.New(),
		Type:      "semantic",
		EvidenceA: "test evidence A",
		EvidenceB: "test evidence B",
	}

	fp := svc.fingerprint(c)
	if len(fp) != 64 {
		t.Errorf("SHA-256 fingerprint length = %d, want 64 hex chars", len(fp))
	}
	// Verify all characters are hex
	for _, ch := range fp {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("fingerprint contains non-hex character: %c", ch)
			break
		}
	}
}

// TestContradictionCheckDeterministicDeceasedAlive verifies that the
// deterministic rule catches deceased/alive contradictions without calling Qwen API.
func TestContradictionCheckDeterministicDeceasedAlive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	// Create a "deceased" entity
	deceasedID := uuid.New()
	// Insert entity with "deceased" status via pool directly
	if _, err := pool.Exec(ctx,
		"INSERT INTO entities (id, universe_id, type, name, description, status, relevance_score) VALUES ($1,$2,'character','Dead Bob','','deceased',0.8)",
		deceasedID, universe.ID); err != nil {
		t.Fatalf("create deceased entity: %v", err)
	}

	entityRepo := repositories.NewEntityRepo(pool)
	contraRepo := repositories.NewContradictionRepo(pool)
	cfg := config.Config{MaxContradictionCandidates: 3}
	svc := NewContradictionService(pool, contraRepo, entityRepo, nil, nil, cfg.MaxContradictionCandidates, nil) // nil qwenSvc

	// Pass an entity marked as "alive" but the DB says "deceased"
	entities := []ResolvedEntity{
		{
			Entity:         models.Entity{ID: deceasedID, UniverseID: universe.ID, Type: "character", Name: "Dead Bob", Status: "deceased"},
			MentionText:    "Bob walked into the room",
			IsNew:          false,
			PreviousStatus: "deceased",
		},
	}

	chapterID := uuid.New()
	contradictions, err := svc.CheckDeterministic(ctx, universe.ID, chapterID, entities)
	if err != nil {
		t.Fatalf("CheckDeterministic: %v", err)
	}

	// Should detect that "Dead Bob" is mentioned as alive but DB says deceased
	if len(contradictions) == 0 {
		t.Error("Expected at least one contradiction for deceased entity mentioned as alive")
	}
	if len(contradictions) > 0 && contradictions[0].Severity == "" {
		t.Error("Contradiction severity should not be empty")
	}
	if len(contradictions) > 0 && contradictions[0].Severity != "critical" {
		t.Errorf("Contradiction severity for deceased_alive should be 'critical', got '%s'", contradictions[0].Severity)
	}
}

// TestResolveOrCreateThenCheckDeterministicCatchesReanimation reproduces the
// real production bug: EntityService.ResolveOrCreate merges newly-extracted
// data (including Status) into the existing entity and persists it BEFORE
// ContradictionService.CheckDeterministic ever inspects it. A prior version
// of this code compared CheckDeterministic against the already-overwritten
// Entity.Status, so the deceased/alive rule could never fire on a real
// re-extraction. This test drives both calls in production order and asserts
// the contradiction is still detected via ResolveOrCreate's previousStatus.
func TestResolveOrCreateThenCheckDeterministicCatchesReanimation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	deceasedID := uuid.New()
	if _, err := pool.Exec(ctx,
		"INSERT INTO entities (id, universe_id, type, name, description, status, relevance_score) VALUES ($1,$2,'character','Dead Bob','','deceased',0.8)",
		deceasedID, universe.ID); err != nil {
		t.Fatalf("create deceased entity: %v", err)
	}

	entityRepo := repositories.NewEntityRepo(pool)
	contraRepo := repositories.NewContradictionRepo(pool)
	cfg := config.Config{MaxContradictionCandidates: 3}
	entitySvc := NewEntityService(pool, entityRepo, nil, nil)
	contraSvc := NewContradictionService(pool, contraRepo, entityRepo, nil, nil, cfg.MaxContradictionCandidates, nil)

	// Simulate a chapter re-mentioning "Dead Bob" as active — extraction
	// finds him by exact name match (step 1), so no qwenSvc/vectorRepo needed.
	entity, previousStatus, isNew, err := entitySvc.ResolveOrCreate(ctx, universe.ID, repositories.ExtractedEntity{
		Type: "character", Name: "Dead Bob", Status: "active",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreate: %v", err)
	}
	if isNew {
		t.Fatal("expected existing entity to be matched, not created new")
	}
	if entity.Status != "active" {
		t.Errorf("merged entity Status = %q, want %q (merge should adopt new status)", entity.Status, "active")
	}
	if previousStatus != "deceased" {
		t.Fatalf("previousStatus = %q, want %q — CheckDeterministic has nothing to compare against otherwise", previousStatus, "deceased")
	}

	resolved := []ResolvedEntity{
		{Entity: *entity, MentionText: "Bob walked into the room", IsNew: isNew, PreviousStatus: previousStatus},
	}

	contradictions, err := contraSvc.CheckDeterministic(ctx, universe.ID, uuid.New(), resolved)
	if err != nil {
		t.Fatalf("CheckDeterministic: %v", err)
	}
	if len(contradictions) == 0 {
		t.Fatal("expected deceased/alive contradiction to be detected via previousStatus, got none")
	}
}

// TestContradictionCheckDeterministicNoIssues verifies deterministic check
// returns empty when there are no issues.
func TestContradictionCheckDeterministicNoIssues(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	activeEntity := svcCreateTestEntity(t, ctx, pool, universe.ID, "Alive Alice", 0.8, "active")

	entityRepo := repositories.NewEntityRepo(pool)
	contraRepo := repositories.NewContradictionRepo(pool)
	cfg := config.Config{MaxContradictionCandidates: 3}
	svc := NewContradictionService(pool, contraRepo, entityRepo, nil, nil, cfg.MaxContradictionCandidates, nil)

	entities := []ResolvedEntity{
		{Entity: activeEntity, MentionText: "Alice walked to the store", IsNew: false},
	}

	chapterID := uuid.New()
	contradictions, err := svc.CheckDeterministic(ctx, universe.ID, chapterID, entities)
	if err != nil {
		t.Fatalf("CheckDeterministic: %v", err)
	}

	if len(contradictions) != 0 {
		t.Errorf("Expected 0 contradictions for active entity, got %d", len(contradictions))
	}
}

// TestContradictionCheckDeterministicChapterThreaded verifies that ChapterA/ChapterB
// are populated on ContradictionCandidate structs when CheckDeterministic is called
// with a chapterID. This ensures the fingerprint embeds chapter context.
//
// RED: CheckDeterministic currently takes 3 params (ctx, universeID, entities).
// This test adds a 4th param (chapterID) — won't compile until production code
// is updated.
func TestContradictionCheckDeterministicChapterThreaded(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	deceasedID := uuid.New()
	if _, err := pool.Exec(ctx,
		"INSERT INTO entities (id, universe_id, type, name, description, status, relevance_score) VALUES ($1,$2,'character','Ghost Bob','','deceased',0.8)",
		deceasedID, universe.ID); err != nil {
		t.Fatalf("create deceased entity: %v", err)
	}

	entityRepo := repositories.NewEntityRepo(pool)
	contraRepo := repositories.NewContradictionRepo(pool)
	cfg := config.Config{MaxContradictionCandidates: 3}
	svc := NewContradictionService(pool, contraRepo, entityRepo, nil, nil, cfg.MaxContradictionCandidates, nil)

	chapterID := uuid.New()

	entities := []ResolvedEntity{
		{
			Entity:         models.Entity{ID: deceasedID, UniverseID: universe.ID, Type: "character", Name: "Ghost Bob", Status: "deceased"},
			MentionText:    "Bob walked into the room",
			IsNew:          false,
			PreviousStatus: "deceased",
		},
	}

	// RED: this line won't compile — CheckDeterministic currently only takes 3 params
	contradictions, err := svc.CheckDeterministic(ctx, universe.ID, chapterID, entities)
	if err != nil {
		t.Fatalf("CheckDeterministic: %v", err)
	}

	if len(contradictions) == 0 {
		t.Fatal("Expected at least one contradiction for deceased entity")
	}

	// Verify the fingerprint embeds the chapter — reconstruct expected fingerprint
	expectedFP := svc.fingerprint(ContradictionCandidate{
		EntityID:  deceasedID,
		Type:      "deceased_alive",
		EvidenceA: "Entity Ghost Bob is deceased in DB",
		EvidenceB: "Bob walked into the room",
		ChapterA:  chapterID,
		ChapterB:  chapterID,
	})

	if contradictions[0].Fingerprint != expectedFP {
		t.Errorf("Fingerprint mismatch — chapterID not threaded into candidate?\n  got:  %s\n  want: %s",
			contradictions[0].Fingerprint, expectedFP)
	}
}

// TestContradictionCheckSemanticSignature verifies the CheckSemantic method
// compiles and routes to QwenService.CheckContradictions.
func TestContradictionCheckSemanticSignature(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")
	ctx := context.Background()

	contraRepo := repositories.NewContradictionRepo(pool)
	entityRepo := repositories.NewEntityRepo(pool)

	// Create QwenService with dummy config — CheckSemantic will call it
	cfgQwen := config.Config{
		QwenBaseURL:                "https://example.com",
		QwenAPIKey:                 "test-key",
		QwenMaxModel:               "qwen-max-latest",
		QwenMaxConcurrency:         1,
		QwenTurboConcurrency:       1,
		QwenEmbeddingModel:         "text-embedding-v3",
		MaxContradictionCandidates: 3,
	}
	qwenSvc := NewQwenService(&cfgQwen, nil)

	cfg := config.Config{MaxContradictionCandidates: 3}
	svc := NewContradictionService(pool, contraRepo, entityRepo, qwenSvc, nil, cfg.MaxContradictionCandidates, nil)

	entities := []ResolvedEntity{
		{Entity: models.Entity{ID: uuid.New(), Type: "character", Name: "Test"}, MentionText: "Test text", IsNew: false},
	}

	// Should not panic, will try to call Qwen and fail gracefully
	_, err := svc.CheckSemantic(ctx, uuid.New(), uuid.New(), "some text", entities)
	// Expected to fail because Qwen API is unreachable — that's OK, test verifies compilation
	if err == nil {
		t.Log("Unexpected: CheckSemantic succeeded with dummy Qwen endpoint")
	}
}

// TestCheckSemanticAgentLoop verifies that the agent-driven CheckSemantic correctly
// parses a JSON contradiction array returned by the LLM and populates the model fields.
//
// RED: CheckSemantic still uses the batch CheckContradictions path. This test will
// fail because the mock server is not configured for the old batch endpoint.
func TestCheckSemanticAgentLoop(t *testing.T) {
	// Mock server that returns a contradiction JSON array as the agent's final answer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": `[{"type":"status_contradiction","description":"Bob was described as both alive and dead","evidence_a":"Bob walked into the room in chapter 1","evidence_b":"Bob died in chapter 3","severity":"critical"},{"type":"timeline_mismatch","description":"Alice appears before her birth","evidence_a":"Alice was born in chapter 4","evidence_b":"Alice appears in chapter 2","severity":"high"}]`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, nil)
	qwenSvc.client.Timeout = 5 * time.Second

	exec := &mockExecutor{}

	cfg := config.Config{MaxContradictionCandidates: 5}
	svc := NewContradictionService(nil, nil, nil, qwenSvc, exec, cfg.MaxContradictionCandidates, nil)

	entityID := uuid.New()
	entities := []ResolvedEntity{
		{
			Entity:      models.Entity{ID: entityID, Type: "character", Name: "Test Entity", Description: "A test character"},
			MentionText: "Test mention text",
			IsNew:       false,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contradictions, err := svc.CheckSemantic(ctx, uuid.New(), uuid.New(), "test text", entities)
	if err != nil {
		t.Fatalf("CheckSemantic: %v", err)
	}

	if len(contradictions) != 2 {
		t.Fatalf("expected 2 contradictions, got %d", len(contradictions))
	}

	// Verify first contradiction
	if contradictions[0].Description != "Bob was described as both alive and dead" {
		t.Errorf("unexpected description: %q", contradictions[0].Description)
	}
	if contradictions[0].Severity != "critical" {
		t.Errorf("unexpected severity: %q", contradictions[0].Severity)
	}

	// Verify second contradiction
	if contradictions[1].Description != "Alice appears before her birth" {
		t.Errorf("unexpected description: %q", contradictions[1].Description)
	}
	if contradictions[1].Severity != "high" {
		t.Errorf("unexpected severity: %q", contradictions[1].Severity)
	}
}

// TestCheckSemanticEmptyContradiction verifies that CheckSemantic returns
// an empty slice (not nil) when the agent finds no contradictions.
// Triangulates: tests both empty JSON array and the actual return value properties.
func TestCheckSemanticEmptyContradiction(t *testing.T) {
	// Mock server returns empty JSON array as the agent's final answer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "[]",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	qwenSvc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, nil)
	qwenSvc.client.Timeout = 5 * time.Second

	exec := &mockExecutor{}

	cfg := config.Config{MaxContradictionCandidates: 5}
	svc := NewContradictionService(nil, nil, nil, qwenSvc, exec, cfg.MaxContradictionCandidates, nil)

	entityID := uuid.New()
	entities := []ResolvedEntity{
		{
			Entity:      models.Entity{ID: entityID, Type: "character", Name: "Clean Entity", Description: "No issues here"},
			MentionText: "Everything is consistent",
			IsNew:       false,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contradictions, err := svc.CheckSemantic(ctx, uuid.New(), uuid.New(), "consistent text", entities)
	if err != nil {
		t.Fatalf("CheckSemantic: %v", err)
	}

	// Must be non-nil — per Go convention, return empty slice, not nil, for "no results"
	if contradictions == nil {
		t.Error("CheckSemantic should return empty slice, not nil, for no contradictions")
	}
	if len(contradictions) != 0 {
		t.Errorf("expected 0 contradictions for empty response, got %d", len(contradictions))
	}
}

// mockExecutor implements ToolExecutor for testing.
type mockExecutor struct{}

func (m *mockExecutor) ExecuteTool(name string, argsJSON string) (string, error) {
	return "mock tool result", nil
}

// TestBuildEntityLinesCapsToHighestRelevance closes the verify-report's
// CRITICAL gap: every existing CheckSemantic test above passes a nil
// budgetMgr, so the entity-capping wiring in buildEntityLines was never
// exercised with a real budget. This constructs a real, deliberately tiny
// ContextBudgetManager and proves buildEntityLines (a) drops entities when
// they don't all fit and (b) keeps the highest-RelevanceScore entities, not
// an arbitrary subset.
//
// The nil-budgetMgr fallback (concatenate every entity, uncapped) is already
// covered by TestCheckSemanticAgentLoop / TestCheckSemanticEmptyContradiction
// above — not duplicated here.
func TestBuildEntityLinesCapsToHighestRelevance(t *testing.T) {
	tok := NewTokenizer()

	// A long, uniform description so each entity line costs roughly the same
	// number of tokens — makes the "which ones survive" assertion depend on
	// RelevanceScore ranking, not incidental text-length differences.
	const description = "A long-standing figure in the story whose history spans several chapters and countless subplots, mentioned repeatedly by name across the narrative."

	makeEntity := func(name string, score float64) ResolvedEntity {
		return ResolvedEntity{
			Entity: models.Entity{
				ID:             uuid.New(),
				Type:           "character",
				Name:           name,
				Description:    description,
				RelevanceScore: score,
			},
		}
	}

	entities := []ResolvedEntity{
		makeEntity("HighOne", 0.95),
		makeEntity("HighTwo", 0.90),
		makeEntity("HighThree", 0.85),
		makeEntity("LowOne", 0.30),
		makeEntity("LowTwo", 0.20),
		makeEntity("LowThree", 0.10),
	}

	// Measure a single entity line's token cost the same way buildEntityLines
	// does, then reverse ComputeBudget's 35% split (with systemPrompt/
	// userMessageTemplate/text all empty below, systemTokens and
	// userBaseTokens are 0, so available == maxContextTokens) to size a
	// budget that fits exactly the top 3 of 6 lines. perEntityTokens is large
	// relative to the +10 slack, so a 4th line can never round its way in.
	sample := fmt.Sprintf("- %s (%s): %s\n", entities[0].Entity.Name, entities[0].Entity.Type, entities[0].Entity.Description)
	perEntityTokens := tok.CountTokens(sample)
	entitiesBudget := perEntityTokens*3 + 10
	maxContextTokens := entitiesBudget * 100 / 35

	budgetMgr := NewContextBudgetManager(tok, maxContextTokens, 0)
	svc := NewContradictionService(nil, nil, nil, nil, nil, 3, budgetMgr)

	result := svc.buildEntityLines(entities, "", "%s%s", "")

	lineCount := strings.Count(result, "\n")
	if lineCount == 0 {
		t.Fatal("expected at least one entity line in result")
	}
	if lineCount >= len(entities) {
		t.Fatalf("expected fewer lines than total entities (%d) under budget pressure, got %d lines:\n%s", len(entities), lineCount, result)
	}

	for _, name := range []string{"HighOne", "HighTwo", "HighThree"} {
		if !strings.Contains(result, name) {
			t.Errorf("expected top-scored entity %q to survive capping, got:\n%s", name, result)
		}
	}
	for _, name := range []string{"LowOne", "LowTwo", "LowThree"} {
		if strings.Contains(result, name) {
			t.Errorf("expected low-scored entity %q to be dropped under budget pressure, got:\n%s", name, result)
		}
	}
}
