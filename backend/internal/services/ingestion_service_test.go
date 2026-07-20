package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

// ── Mocks ──

// mockIngestionHub captures SendToUser calls for verification.
type mockIngestionHub struct {
	mu       sync.Mutex
	messages []models.WSMessage
	userIDs  []uuid.UUID
}

func (m *mockIngestionHub) SendToUser(userID uuid.UUID, msg models.WSMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	m.userIDs = append(m.userIDs, userID)
	return nil
}

func (m *mockIngestionHub) popMessages() []models.WSMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.messages
	m.messages = nil
	return out
}

func (m *mockIngestionHub) popUserIDs() []uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.userIDs
	m.userIDs = nil
	return out
}

// mockQwenForIngestion returns canned ExtractEntities and GenerateEmbedding results.
type mockQwenForIngestion struct {
	extractResult         *ExtractedEntities
	extractErr            error
	relationships         []map[string]interface{}
	relationshipErr       error
	relationshipMu        sync.Mutex
	relationshipCallCount int
	embeddingBatchSizes   []int
}

type recordingIngestionRelevance struct {
	decayCalls int
	mentioned  []uuid.UUID
	deadline   time.Time
	ctxErr     error
	err        error
}

func (m *recordingIngestionRelevance) Touch(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *recordingIngestionRelevance) DecayExcept(ctx context.Context, _ uuid.UUID, mentioned []uuid.UUID) error {
	m.decayCalls++
	m.mentioned = append([]uuid.UUID(nil), mentioned...)
	m.deadline, _ = ctx.Deadline()
	m.ctxErr = ctx.Err()
	if m.err != nil {
		return m.err
	}
	return ctx.Err()
}

// boundedMapQwen blocks each provider task until the test releases it. This
// makes MAP parallelism observable without timing sleeps.
type boundedMapQwen struct {
	mu      sync.Mutex
	active  int
	max     int
	started chan struct{}
	release <-chan struct{}
}

type middleBatchFailureQwen struct{ calls int }

func (m *middleBatchFailureQwen) ExtractEntities(context.Context, string, string) (*ExtractedEntities, error) {
	return nil, nil
}
func (m *middleBatchFailureQwen) AnalyzeRelationships(context.Context, string, []string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *middleBatchFailureQwen) GenerateEmbedding(context.Context, string) ([]float32, error) {
	return make([]float32, 1024), nil
}
func (m *middleBatchFailureQwen) GenerateEmbeddingBatch(_ context.Context, texts []string) ([][]float32, error) {
	m.calls++
	if m.calls == 2 {
		return nil, errors.New("middle batch unavailable")
	}
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = make([]float32, 1024)
	}
	return result, nil
}

func (m *boundedMapQwen) IngestionConcurrency() int { return 2 }
func (m *boundedMapQwen) begin() {
	m.mu.Lock()
	m.active++
	if m.active > m.max {
		m.max = m.active
	}
	m.mu.Unlock()
	m.started <- struct{}{}
	<-m.release
	m.mu.Lock()
	m.active--
	m.mu.Unlock()
}
func (m *boundedMapQwen) ExtractEntities(_ context.Context, text, _ string) (*ExtractedEntities, error) {
	m.begin()
	return &ExtractedEntities{Characters: []ExtractedEntity{{Type: "character", Name: text, Description: text}}}, nil
}
func (m *boundedMapQwen) AnalyzeRelationships(context.Context, string, []string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *boundedMapQwen) GenerateEmbedding(context.Context, string) ([]float32, error) {
	return []float32{0.1}, nil
}
func (m *boundedMapQwen) GenerateEmbeddingBatch(_ context.Context, texts []string) ([][]float32, error) {
	m.begin()
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{0.1}
	}
	return result, nil
}

func (m *boundedMapQwen) maxActive() int { m.mu.Lock(); defer m.mu.Unlock(); return m.max }

type recordingRelationshipEdgeWriter struct {
	edges []recordedRelationshipEdge
}

type recordedRelationshipEdge struct {
	sourceID string
	targetID string
	relType  string
}

func (w *recordingRelationshipEdgeWriter) CreateEdge(_ context.Context, _ string, sourceID, targetID, relType string, _ map[string]interface{}) error {
	w.edges = append(w.edges, recordedRelationshipEdge{sourceID: sourceID, targetID: targetID, relType: relType})
	return nil
}

func (m *mockQwenForIngestion) ExtractEntities(ctx context.Context, text, categories string) (*ExtractedEntities, error) {
	return m.extractResult, m.extractErr
}

func (m *mockQwenForIngestion) AnalyzeRelationships(ctx context.Context, text string, entityNames []string) ([]map[string]interface{}, error) {
	m.relationshipMu.Lock()
	defer m.relationshipMu.Unlock()
	m.relationshipCallCount++
	return m.relationships, m.relationshipErr
}

func (m *mockQwenForIngestion) relationshipCalls() int {
	m.relationshipMu.Lock()
	defer m.relationshipMu.Unlock()
	return m.relationshipCallCount
}

func (m *mockQwenForIngestion) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Return a dummy embedding — length 3 for test
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockQwenForIngestion) GenerateEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.relationshipMu.Lock()
	m.embeddingBatchSizes = append(m.embeddingBatchSizes, len(texts))
	m.relationshipMu.Unlock()
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, nil
}

func (m *mockQwenForIngestion) embeddingBatches() []int {
	m.relationshipMu.Lock()
	defer m.relationshipMu.Unlock()
	return append([]int(nil), m.embeddingBatchSizes...)
}

func TestMapChunksIsWriteFreeAndPreservesMentionCoordinates(t *testing.T) {
	qwen := &mockQwenForIngestion{extractResult: &ExtractedEntities{Characters: []ExtractedEntity{{Type: "character", Name: "Mira", Description: "captain"}}}}
	// A nil pool/entity service is deliberate: MAP must still call only Qwen
	// and return DTOs, never trying to resolve/write domain state.
	svc := &IngestionService{qwenSvc: qwen}
	chunks := []ingestionChunk{{title: "Arrival", content: "Mira arrives at Aurelia.\n\nThe ship departs."}}
	results := svc.mapChunks(context.Background(), chunks, nil)
	if len(results) != 1 || len(results[0].Embeddings) != 2 {
		t.Fatalf("MAP result = %#v", results)
	}
	if len(results[0].Mentions) != 1 {
		t.Fatalf("mentions = %#v", results[0].Mentions)
	}
	mention := results[0].Mentions[0]
	if mention.ParagraphIndex != 0 || mention.Offset != 0 || !strings.Contains(mention.Snippet, "Mira") {
		t.Fatalf("mention coordinates = %#v", mention)
	}
}

func TestEmbedParagraphBatchesCapsProviderBatchesAtTen(t *testing.T) {
	qwen := &mockQwenForIngestion{}
	svc := &IngestionService{qwenSvc: qwen}
	paragraphs := make([]mappedParagraph, 21)
	for i := range paragraphs {
		paragraphs[i] = mappedParagraph{Index: i, Text: fmt.Sprintf("paragraph %d", i)}
	}
	embeddings, err := svc.embedParagraphBatches(context.Background(), paragraphs)
	if err != nil {
		t.Fatalf("embedParagraphBatches: %v", err)
	}
	if got := qwen.embeddingBatches(); !reflect.DeepEqual(got, []int{10, 10, 1}) {
		t.Fatalf("batch sizes = %v, want [10 10 1]", got)
	}
	if len(embeddings) != 21 {
		t.Fatalf("embeddings = %d, want 21", len(embeddings))
	}
	for index, embedding := range embeddings {
		if embedding == nil {
			t.Fatalf("embedding %d was not retained", index)
		}
	}
}

func TestMapMentionsTrimLeadingWhitespaceAndSortForReduce(t *testing.T) {
	paragraphs := mapParagraphs(0, "   Port greets Mira.")
	extracted := &ExtractedEntities{
		Characters: []ExtractedEntity{{Type: "character", Name: "Mira"}},
		Places:     []ExtractedEntity{{Type: "place", Name: "Port"}},
	}
	mentions := mapExtractedMentions(extracted, paragraphs)
	if len(mentions) != 2 || mentions[0].Entity.Name != "Mira" || mentions[0].Offset != 15 {
		t.Fatalf("leading whitespace mention = %#v", mentions)
	}
	ordered := sortMentionsForReduce(mentions)
	if ordered[0].Entity.Name != "Port" || ordered[0].Offset != 3 || ordered[1].Entity.Name != "Mira" {
		t.Fatalf("document order = %#v", ordered)
	}
	reversed := sortMentionsForReduce([]extractedMention{
		{Entity: repositories.ExtractedEntity{Name: "Late"}, ParagraphIndex: 0, Offset: 20},
		{Entity: repositories.ExtractedEntity{Name: "Early"}, ParagraphIndex: 0, Offset: 2},
	})
	if reversed[0].Entity.Name != "Early" {
		t.Fatalf("reduce order = %#v", reversed)
	}
}

func TestReducePersistsSuccessfulEmbeddingBatchesAfterMiddleFailure(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "023")
	ctx := context.Background()
	user := svcCreateTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Embedding Universe", GenreTags: []string{"fantasy"}}
	if _, err := pool.Exec(ctx, "INSERT INTO universes (id,user_id,name,description,genre_tags) VALUES ($1,$2,$3,$4,$5)", universe.ID, universe.UserID, universe.Name, "", universe.GenreTags); err != nil {
		t.Fatalf("create universe: %v", err)
	}
	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	chapter := svcCreateTestChapter(t, ctx, pool, work.ID, "Chapter", 1)
	qwen := &middleBatchFailureQwen{}
	svc := &IngestionService{vectorRepo: repositories.NewVectorRepo(pool), qwenSvc: qwen}
	paragraphs := make([]mappedParagraph, 21)
	for i := range paragraphs {
		paragraphs[i] = mappedParagraph{Index: i, Text: fmt.Sprintf("paragraph %d", i), NodeID: fmt.Sprintf("node-%d", i)}
	}
	embeddings, err := svc.embedParagraphBatches(ctx, paragraphs)
	if err == nil {
		t.Fatal("expected middle batch failure")
	}
	if embeddings[0] == nil || embeddings[10] != nil || embeddings[20] == nil {
		t.Fatalf("partial embeddings were not retained: first=%t middle=%t last=%t", embeddings[0] != nil, embeddings[10] != nil, embeddings[20] != nil)
	}
	svc.persistMappedEmbeddings(ctx, chapter.ID, paragraphs, embeddings)
	rows, err := pool.Query(ctx, "SELECT paragraph_index FROM paragraph_embeddings WHERE chapter_id=$1 ORDER BY paragraph_index", chapter.ID)
	if err != nil {
		t.Fatalf("query persisted embeddings: %v", err)
	}
	defer rows.Close()
	var indexes []int
	for rows.Next() {
		var index int
		if err := rows.Scan(&index); err != nil {
			t.Fatalf("scan embedding: %v", err)
		}
		indexes = append(indexes, index)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate embeddings: %v", err)
	}
	if len(indexes) != 11 || indexes[0] != 0 || indexes[9] != 9 || indexes[10] != 20 {
		t.Fatalf("persisted indexes = %v, want 0..9,20", indexes)
	}
}

func TestMapChunksBoundsAllProviderTasksAndReturnsDocumentOrder(t *testing.T) {
	release := make(chan struct{})
	qwen := &boundedMapQwen{started: make(chan struct{}, 16), release: release}
	svc := &IngestionService{qwenSvc: qwen}
	chunks := []ingestionChunk{{title: "first", content: "First"}, {title: "second", content: "Second"}, {title: "third", content: "Third"}}
	done := make(chan []ingestionMapResult, 1)
	go func() { done <- svc.mapChunks(context.Background(), chunks, nil) }()

	// Exactly the throttle-derived bound may enter provider work before release.
	<-qwen.started
	<-qwen.started
	if got := qwen.maxActive(); got != 2 {
		t.Fatalf("active MAP provider tasks = %d, want 2", got)
	}
	close(release)
	results := <-done
	if len(results) != len(chunks) {
		t.Fatalf("results = %d, want %d", len(results), len(chunks))
	}
	for index, result := range results {
		if result.Index != index || result.Chunk.title != chunks[index].title {
			t.Fatalf("result %d = %#v, want chunk %q", index, result, chunks[index].title)
		}
	}
}

func TestReduceMentionsSeriallyResolvesDuplicatesAndPersistsMentions(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	// Use the latest schema so this also proves the character offset survives
	// REDUCE persistence. Natural-key conflict recovery itself is covered by
	// the Phase 1 EntityService integration test.
	testutil.RunMigrationsUpTo(t, pool, "023")
	ctx := context.Background()
	user := svcCreateTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Test Universe", GenreTags: []string{"fantasy"}}
	if _, err := pool.Exec(ctx, "INSERT INTO universes (id, user_id, name, description, genre_tags) VALUES ($1,$2,$3,$4,$5)", universe.ID, universe.UserID, universe.Name, "", universe.GenreTags); err != nil {
		t.Fatalf("create universe: %v", err)
	}
	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	chapter := svcCreateTestChapter(t, ctx, pool, work.ID, "Arrival", 1)
	entityRepo := repositories.NewEntityRepo(pool)
	svc := &IngestionService{
		pool:      pool,
		entitySvc: NewEntityService(pool, entityRepo, nil, newErrorQwenService(t)),
	}
	mentions := []extractedMention{
		{Entity: repositories.ExtractedEntity{Type: "character", Name: "James Holden", Description: "captain"}, ParagraphIndex: 0, Offset: 3, Snippet: "James Holden arrives."},
		{Entity: repositories.ExtractedEntity{Type: "character", Name: "James Holden", Description: "captain of the Rocinante"}, ParagraphIndex: 1, Offset: 8, Snippet: "James Holden speaks."},
	}

	resolved := svc.reduceMentions(ctx, universe.ID, chapter.ID, mentions)
	if len(resolved) != 2 {
		t.Fatalf("resolved = %d, want 2", len(resolved))
	}
	if resolved[0].Entity.ID != resolved[1].Entity.ID {
		t.Fatalf("duplicate mentions resolved to %s and %s", resolved[0].Entity.ID, resolved[1].Entity.ID)
	}
	entities, total, err := entityRepo.ListByUniverse(ctx, universe.ID, repositories.EntityFilters{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListByUniverse: %v", err)
	}
	if total != 1 || len(entities) != 1 {
		t.Fatalf("entities = %d/%d, want one natural-key entity", total, len(entities))
	}
	mentionsCount, err := entityRepo.CountMentions(ctx, resolved[0].Entity.ID)
	if err != nil {
		t.Fatalf("CountMentions: %v", err)
	}
	if mentionsCount != 2 {
		t.Fatalf("mentions = %d, want 2", mentionsCount)
	}
	var offset int
	if err := pool.QueryRow(ctx, "SELECT character_offset FROM entity_mentions WHERE entity_id=$1 AND paragraph_index=0", resolved[0].Entity.ID).Scan(&offset); err != nil {
		t.Fatalf("select persisted offset: %v", err)
	}
	if offset != 3 {
		t.Fatalf("persisted offset = %d, want 3", offset)
	}
}

// TestCreateTimelineEventsForChapterSeedsEventEntitiesWithParticipants proves
// an imported manuscript's Timeline is no longer permanently empty: an
// "event"-type entity resolved during ingestion gets a real timeline_events
// row, using the chapter's position as a first-pass chronological order and
// attaching any characters resolved in the same chapter as participants.
func TestCreateTimelineEventsForChapterSeedsEventEntitiesWithParticipants(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "023")
	ctx := context.Background()
	user := svcCreateTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Test Universe", GenreTags: []string{"fantasy"}}
	if _, err := pool.Exec(ctx, "INSERT INTO universes (id, user_id, name, description, genre_tags) VALUES ($1,$2,$3,$4,$5)", universe.ID, universe.UserID, universe.Name, "", universe.GenreTags); err != nil {
		t.Fatalf("create universe: %v", err)
	}
	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	chapter := svcCreateTestChapter(t, ctx, pool, work.ID, "The Fall", 1)

	svc := &IngestionService{pool: pool, timelineRepo: repositories.NewTimelineRepo(pool)}

	characterID := uuid.New()
	eventID := uuid.New()
	insertEntity := func(id uuid.UUID, entityType, name, description string) {
		if _, err := pool.Exec(ctx,
			"INSERT INTO entities (id, universe_id, type, name, description, status, relevance_score) VALUES ($1,$2,$3,$4,$5,'active',0.5)",
			id, universe.ID, entityType, name, description,
		); err != nil {
			t.Fatalf("create test entity %s: %v", name, err)
		}
	}
	insertEntity(characterID, "character", "Edric Ashvale", "")
	insertEntity(eventID, "event", "The Fall of Ironmere", "The fortress burns.")

	resolved := []ResolvedEntity{
		{Entity: models.Entity{ID: characterID, UniverseID: universe.ID, Type: "character", Name: "Edric Ashvale"}},
		{Entity: models.Entity{ID: eventID, UniverseID: universe.ID, Type: "event", Name: "The Fall of Ironmere", Description: "The fortress burns."}},
	}

	svc.createTimelineEventsForChapter(ctx, universe.ID, chapter.ID, 1, resolved)

	var title, description string
	var position float64
	var participants []uuid.UUID
	err := pool.QueryRow(ctx,
		"SELECT title, description, timeline_position, participants FROM timeline_events WHERE event_entity_id=$1",
		eventID,
	).Scan(&title, &description, &position, &participants)
	if err != nil {
		t.Fatalf("expected a timeline_events row for the event entity, got: %v", err)
	}
	if title != "The Fall of Ironmere" {
		t.Errorf("title = %q, want %q", title, "The Fall of Ironmere")
	}
	if description != "The fortress burns." {
		t.Errorf("description = %q, want %q", description, "The fortress burns.")
	}
	if position != 1 {
		t.Errorf("timeline_position = %v, want 1", position)
	}
	if len(participants) != 1 || participants[0] != characterID {
		t.Errorf("participants = %v, want [%s]", participants, characterID)
	}
}

// TestCreateTimelineEventsForChapterNilSafe verifies the default (no
// SetTimelineRepo call) stays a silent no-op, matching every other optional
// dependency on IngestionService.
func TestCreateTimelineEventsForChapterNilSafe(t *testing.T) {
	svc := &IngestionService{}
	svc.createTimelineEventsForChapter(context.Background(), uuid.New(), uuid.New(), 1, []ResolvedEntity{
		{Entity: models.Entity{ID: uuid.New(), Type: "event", Name: "Should not panic"}},
	})
}

// TestReduceMentionsReinforcesImportedEntities proves imported entities get
// relevance from their actual mention frequency instead of every extracted
// entity remaining at the same creation default.
func TestReduceMentionsReinforcesImportedEntities(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "023")
	ctx := context.Background()
	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	work := svcCreateTestWork(t, ctx, pool, universe.ID)
	chapter := svcCreateTestChapter(t, ctx, pool, work.ID, "Arrival", 1)
	entityRepo := repositories.NewEntityRepo(pool)

	svc := &IngestionService{pool: pool, entitySvc: NewEntityService(pool, entityRepo, nil, newErrorQwenService(t))}
	svc.SetRelevance(NewRelevanceService(pool, entityRepo, 0.1, 0.15, nil))
	mentions := []extractedMention{
		{Entity: repositories.ExtractedEntity{Type: "character", Name: "James Holden"}, ParagraphIndex: 0, Offset: 0, Snippet: "Holden arrives."},
		{Entity: repositories.ExtractedEntity{Type: "character", Name: "James Holden"}, ParagraphIndex: 1, Offset: 0, Snippet: "Holden speaks."},
		{Entity: repositories.ExtractedEntity{Type: "character", Name: "James Holden"}, ParagraphIndex: 2, Offset: 0, Snippet: "Holden leaves."},
		{Entity: repositories.ExtractedEntity{Type: "character", Name: "Naomi Nagata"}, ParagraphIndex: 3, Offset: 0, Snippet: "Naomi watches."},
	}
	svc.reduceMentions(ctx, universe.ID, chapter.ID, mentions)

	holden, err := entityRepo.FindByNaturalKey(ctx, universe.ID, "James Holden", "character")
	if err != nil {
		t.Fatalf("find Holden: %v", err)
	}
	naomi, err := entityRepo.FindByNaturalKey(ctx, universe.ID, "Naomi Nagata", "character")
	if err != nil {
		t.Fatalf("find Naomi: %v", err)
	}
	if math.Abs(holden.RelevanceScore-0.95) > 0.0001 || math.Abs(naomi.RelevanceScore-0.65) > 0.0001 {
		t.Fatalf("imported relevance Holden=%f Naomi=%f, want 0.95/0.65", holden.RelevanceScore, naomi.RelevanceScore)
	}
}

func TestDecayCompletedIngestionRunsOnceAndExcludesFinalChapterEntities(t *testing.T) {
	firstID := uuid.New()
	finalID := uuid.New()
	relevance := &recordingIngestionRelevance{}
	svc := &IngestionService{relevance: relevance}

	svc.decayCompletedIngestion(context.Background(), uuid.New(), []ingestedChapter{
		{ID: uuid.New(), Entities: []ResolvedEntity{{Entity: models.Entity{ID: firstID}}}},
		{ID: uuid.New(), Entities: []ResolvedEntity{
			{Entity: models.Entity{ID: finalID}},
			{Entity: models.Entity{ID: finalID}},
		}},
	})

	if relevance.decayCalls != 1 {
		t.Fatalf("decay calls = %d, want one per completed import", relevance.decayCalls)
	}
	if !reflect.DeepEqual(relevance.mentioned, []uuid.UUID{finalID}) {
		t.Fatalf("excluded entities = %v, want final chapter entity %v", relevance.mentioned, finalID)
	}
}

func TestDecayCompletedIngestionUsesBoundedCancelableContextAndIsBestEffort(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	relevance := &recordingIngestionRelevance{err: context.Canceled}
	svc := &IngestionService{relevance: relevance}

	svc.decayCompletedIngestion(parent, uuid.New(), []ingestedChapter{{ID: uuid.New()}})

	if relevance.decayCalls != 1 {
		t.Fatalf("decay calls = %d, want one", relevance.decayCalls)
	}
	if relevance.deadline.IsZero() {
		t.Fatal("decay context has no deadline")
	}
	if !errors.Is(relevance.ctxErr, context.Canceled) {
		t.Fatalf("decay context error = %v, want context canceled", relevance.ctxErr)
	}
	if remaining := time.Until(relevance.deadline); remaining <= 0 || remaining > ingestionDecayTimeout {
		t.Fatalf("decay deadline remaining = %s, want within %s", remaining, ingestionDecayTimeout)
	}
}

// ── Test: pipeline sequence ──

// TestIngestionServicePipeline verifies the chunk→extract→embed→graph sequence
// in a mock-driven test. It uses an httptest-style pattern without real DB.
func TestIngestionServicePipeline(t *testing.T) {
	hub := &mockIngestionHub{}
	qwen := &mockQwenForIngestion{
		extractResult: &ExtractedEntities{
			Characters: []ExtractedEntity{
				{Type: "Character", Name: "Frodo", Status: "alive", Description: "A hobbit"},
			},
		},
	}

	docContent := `# Chapter 1: A Long-expected Party

Bilbo was going to have a birthday party.

# Chapter 2: The Shadow of the Past

Frodo learns about the Ring.`

	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    qwen,
		hub:        hub,
	}

	universeID := uuid.New()

	jobID, duplicate, err := svc.Start(context.Background(), universeID, strings.NewReader(docContent), "test.md")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if duplicate {
		t.Error("expected duplicate=false for first upload")
	}
	if jobID == uuid.Nil {
		t.Error("expected non-nil job ID")
	}

	// Wait for goroutine to finish (small doc, fast)
	time.Sleep(200 * time.Millisecond)

	msgs := hub.popMessages()
	// Should have at least one progress event
	if len(msgs) == 0 {
		t.Error("expected at least one WebSocket progress message")
	}

	// Verify at least one is an ingestion_progress event
	foundProgress := false
	for _, msg := range msgs {
		if msg.Type == "ingestion_progress" {
			foundProgress = true
			var payload map[string]any
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				t.Errorf("unmarshal progress payload: %v", err)
			}
			if payload["job_id"] == nil {
				t.Error("progress payload missing job_id")
			}
			if payload["universe_id"] != universeID.String() {
				t.Errorf("progress universe_id = %v, want %s", payload["universe_id"], universeID)
			}
		}
	}
	if !foundProgress {
		t.Error("expected an ingestion_progress message")
	}
}

func TestIngestionServicePersistsRelationshipEdge(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "020")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	graphRepo := repositories.NewGraphRepo(pool)
	if err := graphRepo.CreateGraph(ctx, universe.ID.String()); err != nil {
		t.Fatalf("create graph: %v", err)
	}

	source := svcCreateTestEntity(t, ctx, pool, universe.ID, "Mira", 0.8, "alive")
	target := svcCreateTestEntity(t, ctx, pool, universe.ID, "Aurelia", 0.8, "active")
	graphName := "universe_" + universe.ID.String()
	for _, entity := range []models.Entity{source, target} {
		if err := graphRepo.CreateNode(ctx, graphName, entity.Type, map[string]interface{}{
			"entity_id": entity.ID.String(), "name": entity.Name, "status": entity.Status, "relevance_score": entity.RelevanceScore,
		}); err != nil {
			t.Fatalf("create graph node %s: %v", entity.Name, err)
		}
	}

	qwen := &mockQwenForIngestion{relationships: []map[string]interface{}{
		{"source": "Mira", "target": "Aurelia", "type": "LOCATED_AT"},
	}}
	svc := &IngestionService{graphRepo: graphRepo, qwenSvc: qwen}
	if _, err := svc.persistRelationships(ctx, universe.ID, "Mira arrives at Aurelia.", []ResolvedEntity{{Entity: source}, {Entity: target}}); err != nil {
		t.Fatalf("persistRelationships: %v", err)
	}

	neighbors, err := graphRepo.GetNeighbors(ctx, graphName, source.ID.String())
	if err != nil {
		t.Fatalf("get graph neighbors: %v", err)
	}
	for _, neighbor := range neighbors {
		if neighbor.RelType == "LOCATED_AT" {
			return
		}
	}
	t.Fatalf("expected LOCATED_AT edge from %s, got neighbors %#v", source.Name, neighbors)
}

func TestResolveIngestionRelationshipEntity(t *testing.T) {
	entities := []models.Entity{
		{ID: uuid.New(), Name: "Mira Voss", Aliases: []string{"The Navigator"}},
		{ID: uuid.New(), Name: "Aurelia Station"},
	}
	tests := []struct {
		name       string
		query      string
		entities   []models.Entity
		wantName   string
		wantFound  bool
		wantReason string
	}{
		{name: "canonical", query: "Mira Voss", entities: entities, wantName: "Mira Voss", wantFound: true},
		{name: "case and space", query: "  mira voss  ", entities: entities, wantName: "Mira Voss", wantFound: true},
		{name: "unique alias", query: "the navigator", entities: entities, wantName: "Mira Voss", wantFound: true},
		{name: "unique shortened source", query: "Mira", entities: entities, wantName: "Mira Voss", wantFound: true},
		{name: "unique shortened target", query: "Aurelia", entities: entities, wantName: "Aurelia Station", wantFound: true},
		{name: "ambiguous shortened name", query: "Mira", entities: []models.Entity{{ID: uuid.New(), Name: "Mira Voss"}, {ID: uuid.New(), Name: "Mira Sol"}}, wantFound: false, wantReason: "ambiguous"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entity, found, reason := resolveIngestionRelationshipEntity(tc.query, tc.entities)
			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v (reason: %s)", found, tc.wantFound, reason)
			}
			if tc.wantFound && entity.Name != tc.wantName {
				t.Errorf("entity = %q, want %q", entity.Name, tc.wantName)
			}
			if tc.wantReason != "" && !strings.Contains(reason, tc.wantReason) {
				t.Errorf("reason = %q, want containing %q", reason, tc.wantReason)
			}
		})
	}
}

func TestPersistRelationshipsRejectsAmbiguousNamesWithoutCreatingEdge(t *testing.T) {
	edgeWriter := &recordingRelationshipEdgeWriter{}
	svc := &IngestionService{
		qwenSvc: &mockQwenForIngestion{relationships: []map[string]interface{}{
			{"source": "Mira", "target": "Aurelia", "type": "LOCATED_AT"},
		}},
		relationshipEdges: edgeWriter,
	}
	resolved := []ResolvedEntity{
		{Entity: models.Entity{ID: uuid.New(), Name: "Mira Voss"}},
		{Entity: models.Entity{ID: uuid.New(), Name: "Mira Sol"}},
		{Entity: models.Entity{ID: uuid.New(), Name: "Aurelia Station"}},
	}
	if _, err := svc.persistRelationships(context.Background(), uuid.New(), "Mira arrives at Aurelia.", resolved); err != nil {
		t.Fatalf("persistRelationships: %v", err)
	}
	if len(edgeWriter.edges) != 0 {
		t.Fatalf("CreateEdge calls = %d, want 0 for an ambiguous source", len(edgeWriter.edges))
	}
}

func TestEnrichRelationshipsFallsBackToCooccurrenceAfterModelTimeout(t *testing.T) {
	source := models.Entity{ID: uuid.New(), Name: "Mira Voss"}
	target := models.Entity{ID: uuid.New(), Name: "Aurelia Station"}
	edgeWriter := &recordingRelationshipEdgeWriter{}
	svc := &IngestionService{
		qwenSvc:           &mockQwenForIngestion{relationshipErr: context.DeadlineExceeded},
		relationshipEdges: edgeWriter,
	}
	resolved := []ResolvedEntity{{Entity: source}, {Entity: target}}
	svc.enrichRelationships(context.Background(), uuid.New(), "Mira arrives at Aurelia.", resolved, [][]ResolvedEntity{resolved, resolved})

	if len(edgeWriter.edges) != 1 || edgeWriter.edges[0].relType != "CO_OCCURS_WITH" {
		t.Fatalf("fallback edges = %#v, want one deduplicated CO_OCCURS_WITH edge", edgeWriter.edges)
	}
}

func TestEnrichRelationshipsSkipsFallbackWhenModelEdgePersists(t *testing.T) {
	source := models.Entity{ID: uuid.New(), Name: "Mira Voss"}
	target := models.Entity{ID: uuid.New(), Name: "Aurelia Station"}
	edgeWriter := &recordingRelationshipEdgeWriter{}
	svc := &IngestionService{
		qwenSvc: &mockQwenForIngestion{relationships: []map[string]interface{}{
			{"source": "Mira Voss", "target": "Aurelia Station", "type": "LOCATED_AT"},
		}},
		relationshipEdges: edgeWriter,
	}
	resolved := []ResolvedEntity{{Entity: source}, {Entity: target}}
	svc.enrichRelationships(context.Background(), uuid.New(), "Mira arrives at Aurelia.", resolved, [][]ResolvedEntity{resolved})

	if len(edgeWriter.edges) != 1 || edgeWriter.edges[0].relType != "LOCATED_AT" {
		t.Fatalf("edges = %#v, want only persisted model relation", edgeWriter.edges)
	}
}

func TestPersistCooccurrenceEdgesDeduplicatesAndExcludesSelfPairs(t *testing.T) {
	a := models.Entity{ID: uuid.New(), Name: "A"}
	b := models.Entity{ID: uuid.New(), Name: "B"}
	c := models.Entity{ID: uuid.New(), Name: "C"}
	edgeWriter := &recordingRelationshipEdgeWriter{}
	svc := &IngestionService{relationshipEdges: edgeWriter}
	_, err := svc.persistCooccurrenceEdges(context.Background(), uuid.New(), [][]ResolvedEntity{
		{{Entity: a}, {Entity: a}, {Entity: b}},
		{{Entity: b}, {Entity: a}},
		{{Entity: a}, {Entity: c}},
	})
	if err != nil {
		t.Fatalf("persistCooccurrenceEdges: %v", err)
	}
	if len(edgeWriter.edges) != 2 {
		t.Fatalf("fallback edges = %#v, want deduplicated A-B and A-C pairs", edgeWriter.edges)
	}
	seen := make(map[string]bool)
	for _, edge := range edgeWriter.edges {
		if edge.relType != "CO_OCCURS_WITH" || edge.sourceID == edge.targetID {
			t.Fatalf("unsafe fallback edge = %#v", edge)
		}
		key := edge.sourceID + ":" + edge.targetID
		if seen[key] {
			t.Fatalf("duplicate fallback edge = %#v", edge)
		}
		seen[key] = true
	}
}

func TestPersistCooccurrenceEdgesBoundsGenerationAndWritesDeterministically(t *testing.T) {
	makeEntity := func(id string) models.Entity {
		return models.Entity{ID: uuid.MustParse(id), Name: id}
	}
	firstChunk := make([]ResolvedEntity, 0, 15)
	for i := 1; i <= 15; i++ { // 15 choose 2 = 105 candidate pairs.
		firstChunk = append(firstChunk, ResolvedEntity{Entity: makeEntity(fmt.Sprintf("10000000-0000-0000-0000-%012d", i))})
	}
	overflowSource := makeEntity("00000000-0000-0000-0000-000000000001")
	overflowTarget := makeEntity("00000000-0000-0000-0000-000000000002")
	chunks := [][]ResolvedEntity{
		firstChunk,
		{{Entity: overflowSource}, {Entity: overflowTarget}},
	}

	run := func() []recordedRelationshipEdge {
		edgeWriter := &recordingRelationshipEdgeWriter{}
		svc := &IngestionService{relationshipEdges: edgeWriter}
		persisted, err := svc.persistCooccurrenceEdges(context.Background(), uuid.New(), chunks)
		if err != nil {
			t.Fatalf("persistCooccurrenceEdges: %v", err)
		}
		if persisted != ingestionFallbackEdgeLimit || len(edgeWriter.edges) != ingestionFallbackEdgeLimit {
			t.Fatalf("persisted/writes = %d/%d, want %d", persisted, len(edgeWriter.edges), ingestionFallbackEdgeLimit)
		}
		return edgeWriter.edges
	}

	first := run()
	second := run()
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("fallback order differs at %d: first=%#v second=%#v", i, first[i], second[i])
		}
	}
	for _, edge := range first {
		if edge.sourceID == overflowSource.ID.String() && edge.targetID == overflowTarget.ID.String() {
			t.Fatalf("generated pair after cap: %#v", edge)
		}
	}
}

func TestIngestionServiceRelationshipAnalysisFailureIsBestEffort(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "020")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)
	graphRepo := repositories.NewGraphRepo(pool)
	if err := graphRepo.CreateGraph(ctx, universe.ID.String()); err != nil {
		t.Fatalf("create graph: %v", err)
	}
	// Precreate the entities and AGE nodes so ResolveOrCreate takes the exact
	// match path. This keeps the worker entirely local: no embedding call and
	// no Qwen HTTP client are reachable from this best-effort test.
	graphName := "universe_" + universe.ID.String()
	for _, entity := range []models.Entity{
		svcCreateTestEntity(t, ctx, pool, universe.ID, "Mira", 0.8, "alive"),
		svcCreateTestEntity(t, ctx, pool, universe.ID, "Aurelia", 0.8, "active"),
	} {
		if err := graphRepo.CreateNode(ctx, graphName, entity.Type, map[string]interface{}{
			"entity_id": entity.ID.String(), "name": entity.Name, "status": entity.Status, "relevance_score": entity.RelevanceScore,
		}); err != nil {
			t.Fatalf("create graph node %s: %v", entity.Name, err)
		}
	}

	qwen := &mockQwenForIngestion{
		extractResult: &ExtractedEntities{Characters: []ExtractedEntity{
			{Type: "character", Name: "Mira", Status: "alive"},
			{Type: "character", Name: "Aurelia", Status: "active"},
		}},
		relationshipErr: errors.New("relationship model unavailable"),
	}
	entitySvc := NewEntityService(pool, repositories.NewEntityRepo(pool), nil, nil)
	svc := NewIngestionService(pool, entitySvc, nil, graphRepo, qwen, nil)
	jobID, _, err := svc.Start(ctx, universe.ID, strings.NewReader("# Arrival\n\nMira arrives at Aurelia.\n\n# Return\n\nMira returns to Aurelia."), "arrival.md")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	jobRepo := repositories.NewIngestionRepo(pool)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobRepo.FindByID(ctx, jobID)
		if err != nil {
			t.Fatalf("find ingestion job: %v", err)
		}
		if job != nil && job.Status != "pending" && job.Status != "running" {
			if job.Status != "completed" {
				t.Fatalf("job status = %q, want completed despite relationship error (%s)", job.Status, job.ErrorMessage)
			}
			if qwen.relationshipCalls() != 1 {
				t.Fatalf("relationship analysis calls = %d, want one bounded manuscript pass", qwen.relationshipCalls())
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timed out waiting for ingestion to complete")
}

// TestIngestionServiceChunking verifies the document is split by markdown headers.
func TestIngestionServiceChunking(t *testing.T) {
	hub := &mockIngestionHub{}
	qwen := &mockQwenForIngestion{
		extractResult: &ExtractedEntities{},
	}

	docContent := `# Chapter A

Content A line 1.
Content A line 2.

# Chapter B

Content B line 1.`

	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    qwen,
		hub:        hub,
	}

	chunks := svc.splitChunks(docContent)
	// ponytail: minimal chunking — one chunk per # header section
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify chunk content is non-empty
	for i, ch := range chunks {
		if strings.TrimSpace(ch.content) == "" {
			t.Errorf("chunk %d is empty", i)
		}
		if ch.title == "" {
			t.Errorf("chunk %d has no title", i)
		}
	}
}

// TestIngestionServiceEmptyDocument verifies handling of empty input.
func TestIngestionServiceEmptyDocument(t *testing.T) {
	hub := &mockIngestionHub{}
	qwen := &mockQwenForIngestion{
		extractResult: &ExtractedEntities{},
	}

	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    qwen,
		hub:        hub,
	}

	chunks := svc.splitChunks("")
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty document, got %d", len(chunks))
	}
}

// TestIngestionServiceHeaderlessFallback verifies a headerless document is split
// into ~50K-char paragraph-boundary chunks titled "Part N".
func TestIngestionServiceHeaderlessFallback(t *testing.T) {
	svc := &IngestionService{}

	para := strings.Repeat("A paragraph with enough characters to fill space. ", 200)
	var b strings.Builder
	for i := 0; i < 12; i++ {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(fmt.Sprintf("Paragraph %d: %s", i, para))
	}
	content := b.String()

	chunks := svc.splitChunks(content)
	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks for headerless large doc, got %d", len(chunks))
	}

	joined := ""
	for i, ch := range chunks {
		joined += ch.content
		if len(ch.content) == 0 {
			t.Errorf("chunk %d is empty", i)
		}
		if len(ch.content) > 50_000 {
			t.Errorf("chunk %d length %d exceeds 50K", i, len(ch.content))
		}
		wantTitle := fmt.Sprintf("Part %d", i+1)
		if ch.title != wantTitle {
			t.Errorf("chunk %d title = %q, want %q", i, ch.title, wantTitle)
		}
	}

	for i := 0; i < 12; i++ {
		marker := fmt.Sprintf("Paragraph %d:", i)
		if !strings.Contains(joined, marker) {
			t.Errorf("missing paragraph marker %q", marker)
		}
	}
}

// TestSplitByParagraphsSmallDoc verifies a small headerless document stays as
// a single chunk.
func TestSplitByParagraphsSmallDoc(t *testing.T) {
	content := "A short document.\n\nWith two paragraphs."
	chunks := splitByParagraphs(content)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].title != "Untitled" {
		t.Errorf("title = %q, want Untitled", chunks[0].title)
	}
	if chunks[0].content != strings.TrimSpace(content) {
		t.Errorf("content mismatch: got %d chars, want %d chars", len(chunks[0].content), len(strings.TrimSpace(content)))
	}
}

// TestSplitByParagraphsOversizedParagraph verifies a single paragraph larger
// than the chunk cap becomes its own chunk rather than being dropped.
func TestSplitByParagraphsOversizedParagraph(t *testing.T) {
	big := strings.Repeat("a", 60_000)
	content := big + "\n\nsmall"
	chunks := splitByParagraphs(content)
	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk, got %d", len(chunks))
	}
	if chunks[0].title != "Part 1" {
		t.Errorf("first chunk title = %q, want Part 1", chunks[0].title)
	}
	if len(chunks[0].content) != 60_000 {
		t.Errorf("first chunk length = %d, want 60000", len(chunks[0].content))
	}
}

// TestIngestionServiceNilDeps verifies Start can handle nil dependencies gracefully.
func TestIngestionServiceNilDeps(t *testing.T) {
	svc := &IngestionService{
		pool:       nil,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    nil,
		hub:        nil,
	}

	// Start should attempt to create a job; may fail if pool is nil
	jobID, _, err := svc.Start(context.Background(), uuid.New(), strings.NewReader("hello"), "test.md")
	if err != nil {
		// Expected when pool is nil — service can't persist the job
		t.Logf("Start with nil deps: jobID=%s err=%v", jobID, err)
	} else if jobID == uuid.Nil {
		t.Error("expected non-nil job ID even with nil deps")
	}
}

// TestIngestionProgressDeliveredToUniverseOwner is a DB-backed regression test
// proving ingestion_progress events are routed to the real universe owner,
// not uuid.Nil (see sdd/fix-ingestion-progress-delivery).
func TestIngestionProgressDeliveredToUniverseOwner(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "021")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	// This regression exercises UniverseRepo.FindByID, whose current contract
	// requires migration 021's genre_tags schema. Keep the older shared helper
	// for tests intentionally pinned to pre-021 migrations.
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Ingestion Progress Universe", GenreTags: []string{"fantasy"}}
	if _, err := pool.Exec(ctx, `
		INSERT INTO universes (id, user_id, name, description, genre_tags)
		VALUES ($1, $2, $3, $4, $5)
	`, universe.ID, universe.UserID, universe.Name, "", universe.GenreTags); err != nil {
		t.Fatalf("create current-schema universe: %v", err)
	}

	hub := &mockIngestionHub{}
	svc := &IngestionService{
		pool:       pool,
		entitySvc:  nil,
		vectorRepo: nil,
		graphRepo:  nil,
		qwenSvc:    nil,
		hub:        hub,
	}

	docContent := "# Chapter 1\n\nBody text."

	_, _, err := svc.Start(ctx, universe.ID, strings.NewReader(docContent), "t.md")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	msgs := hub.popMessages()
	userIDs := hub.popUserIDs()

	foundProgress := false
	for i, msg := range msgs {
		if msg.Type != "ingestion_progress" {
			continue
		}
		foundProgress = true
		if userIDs[i] == uuid.Nil {
			t.Errorf("ingestion_progress message %d delivered to uuid.Nil, want universe owner %s", i, universe.UserID)
		}
		if userIDs[i] != universe.UserID {
			t.Errorf("ingestion_progress message %d userID = %s, want %s", i, userIDs[i], universe.UserID)
		}
		var payload models.IngestionProgressPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("unmarshal ingestion progress: %v", err)
		}
		if payload.UniverseID != universe.ID {
			t.Errorf("ingestion_progress universe_id = %s, want %s", payload.UniverseID, universe.ID)
		}
	}
	if !foundProgress {
		t.Fatal("expected at least one ingestion_progress message")
	}
}

// TestStartRejectsLegacyDoc verifies Start rejects a .doc filename
// synchronously, before creating any job row, per the whitelist check (D1/D2).
func TestStartRejectsLegacyDoc(t *testing.T) {
	svc := &IngestionService{}
	_, _, err := svc.Start(context.Background(), uuid.New(), strings.NewReader("binary junk"), "manuscript.doc")
	if err == nil {
		t.Fatal("expected error for .doc filename, got nil")
	}
	if !errors.Is(err, ErrUnsupportedFileType) {
		t.Errorf("expected ErrUnsupportedFileType, got: %v", err)
	}
}

func TestStartRejectsUnknownExtension(t *testing.T) {
	svc := &IngestionService{}
	_, _, err := svc.Start(context.Background(), uuid.New(), strings.NewReader("binary junk"), "manuscript.rtf")
	if err == nil {
		t.Fatal("expected error for unknown extension, got nil")
	}
}

// TestStartWorkTitleFromFilenameStem is a DB-backed regression test verifying
// the created Work's title is the filename stem (D3), replacing the old
// hardcoded "Imported Manuscript".
func TestStartWorkTitleFromFilenameStem(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "020")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	svc := &IngestionService{pool: pool}
	_, _, err := svc.Start(ctx, universe.ID, strings.NewReader("# Chapter 1\n\nBody."), "manuscript.pdf")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	works, err := repositories.NewWorkRepo(pool).ListByUniverse(ctx, universe.ID)
	if err != nil {
		t.Fatalf("ListByUniverse: %v", err)
	}
	if len(works) != 1 {
		t.Fatalf("expected 1 work, got %d", len(works))
	}
	if works[0].Title != "manuscript" {
		t.Errorf("work title = %q, want %q", works[0].Title, "manuscript")
	}
}

// TestIngestionServicePersistsParsedDocumentTitle verifies a newly-created
// Work is renamed from its provisional filename stem to the document title
// after the asynchronous import parses valid title metadata.
func TestIngestionServicePersistsParsedDocumentTitle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "021")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Parsed Title Universe", GenreTags: []string{"fantasy"}}
	if _, err := pool.Exec(ctx, `
		INSERT INTO universes (id, user_id, name, description, genre_tags)
		VALUES ($1, $2, $3, $4, $5)
	`, universe.ID, universe.UserID, universe.Name, "", universe.GenreTags); err != nil {
		t.Fatalf("create current-schema universe: %v", err)
	}

	svc := &IngestionService{pool: pool}
	jobID, duplicate, err := svc.Start(ctx, universe.ID, strings.NewReader("---\ntitle: The Glass Atlas\n---\n\n# Chapter One\n\nThe journey begins."), "draft-v7.md")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if duplicate {
		t.Fatal("Start unexpectedly reported a duplicate import")
	}

	jobRepo := repositories.NewIngestionRepo(pool)
	deadline := time.Now().Add(5 * time.Second)
	var job *models.IngestionJob
	for time.Now().Before(deadline) {
		job, err = jobRepo.FindByID(ctx, jobID)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if job.Status != "pending" && job.Status != "running" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if job == nil || job.Status != "completed" {
		if job == nil {
			t.Fatal("timed out waiting for ingestion job to be created")
		}
		t.Fatalf("job status = %q, want completed (%s)", job.Status, job.ErrorMessage)
	}

	work, err := repositories.NewWorkRepo(pool).FindByID(ctx, job.WorkID)
	if err != nil {
		t.Fatalf("FindByID work: %v", err)
	}
	if work.Title != "The Glass Atlas" {
		t.Errorf("work title = %q, want parsed document title %q", work.Title, "The Glass Atlas")
	}
}

// TestIngestionServicePersistsPDFTitleAndCharacterChapters exercises the
// complete PDF path: Info metadata names the Work, while short ALL-CAPS
// character names become separate durable chapters.
func TestIngestionServicePersistsPDFTitleAndCharacterChapters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "021")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	// Migration 021 replaces universes.genre with genre_tags, so this test
	// must use the current fixture shape instead of the legacy shared helper.
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "PDF Title Universe", GenreTags: []string{"fantasy"}}
	if _, err := pool.Exec(ctx, `
		INSERT INTO universes (id, user_id, name, description, genre_tags)
		VALUES ($1, $2, $3, $4, $5)
	`, universe.ID, universe.UserID, universe.Name, "", universe.GenreTags); err != nil {
		t.Fatalf("create current-schema universe: %v", err)
	}
	svc := &IngestionService{pool: pool}
	pdf := buildTestPDFWithTitle("AMOS\n\nHe met Amos at dusk.\n\nBOB\n\nShe met Bob at dawn.", "The Glass City")
	jobID, duplicate, err := svc.Start(ctx, universe.ID, strings.NewReader(string(pdf)), "draft.pdf")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if duplicate {
		t.Fatal("Start unexpectedly reported a duplicate import")
	}

	jobRepo := repositories.NewIngestionRepo(pool)
	deadline := time.Now().Add(5 * time.Second)
	var job *models.IngestionJob
	for time.Now().Before(deadline) {
		job, err = jobRepo.FindByID(ctx, jobID)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if job.Status != "pending" && job.Status != "running" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if job == nil || job.Status != "completed" {
		if job == nil {
			t.Fatal("timed out waiting for ingestion job")
		}
		t.Fatalf("job status = %q, want completed (%s)", job.Status, job.ErrorMessage)
	}

	work, err := repositories.NewWorkRepo(pool).FindByID(ctx, job.WorkID)
	if err != nil {
		t.Fatalf("FindByID work: %v", err)
	}
	if work.Title != "The Glass City" {
		t.Fatalf("work title = %q, want %q", work.Title, "The Glass City")
	}
	chapters, err := repositories.NewChapterRepo(pool).ListByWork(ctx, work.ID)
	if err != nil {
		t.Fatalf("ListByWork: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("chapter count = %d, want 2", len(chapters))
	}
	if chapters[0].Title != "AMOS" || !strings.Contains(chapters[0].Content, "met Amos at dusk") {
		t.Errorf("first chapter = %#v, want AMOS with its body", chapters[0])
	}
	if chapters[1].Title != "BOB" || !strings.Contains(chapters[1].Content, "met Bob at dawn") {
		t.Errorf("second chapter = %#v, want BOB with its body", chapters[1])
	}
}

// TestRunWorkerPreservesManualRenameDuringAsynchronousImport verifies the
// conditional title update cannot overwrite a title the writer saved while a
// newly-created import was pending parsing.
func TestRunWorkerPreservesManualRenameDuringAsynchronousImport(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "021")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := models.Universe{ID: uuid.New(), UserID: user.ID, Name: "Manual Rename Universe", GenreTags: []string{"fantasy"}}
	if _, err := pool.Exec(ctx, `
		INSERT INTO universes (id, user_id, name, description, genre_tags)
		VALUES ($1, $2, $3, $4, $5)
	`, universe.ID, universe.UserID, universe.Name, "", universe.GenreTags); err != nil {
		t.Fatalf("create current-schema universe: %v", err)
	}
	workRepo := repositories.NewWorkRepo(pool)
	work := models.Work{
		ID:         uuid.New(),
		UniverseID: universe.ID,
		Title:      provisionalWorkTitle("draft-v7.md"),
		Type:       "novel",
		Status:     "in_progress",
		OrderIndex: 1,
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin create work: %v", err)
	}
	if err := workRepo.Create(ctx, tx, &work); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("create work: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit create work: %v", err)
	}

	jobID := uuid.New()
	if err := repositories.NewIngestionRepo(pool).Create(ctx, jobID, universe.ID, work.ID, "pending", "draft-v7.md", "md", uuid.NewString()); err != nil {
		t.Fatalf("create ingestion job: %v", err)
	}

	// Model the writer's normal update while the queued worker has not yet
	// parsed metadata. The worker must preserve this newer explicit choice.
	work.Title = "Writer's chosen title"
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin manual rename: %v", err)
	}
	if err := workRepo.Update(ctx, tx, universe.ID, &work); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("manual rename: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit manual rename: %v", err)
	}

	(&IngestionService{pool: pool}).runWorker(
		jobID,
		universe.ID,
		work.ID,
		true,
		[]byte("---\ntitle: The Glass Atlas\n---\n\n# Chapter One\n\nThe journey begins."),
		"draft-v7.md",
	)

	updatedWork, err := workRepo.FindByID(ctx, work.ID)
	if err != nil {
		t.Fatalf("FindByID work: %v", err)
	}
	if updatedWork.Title != "Writer's chosen title" {
		t.Errorf("work title = %q, want manual title %q", updatedWork.Title, "Writer's chosen title")
	}
}

// TestRunWorkerParseFailure is a DB-backed regression test verifying a
// corrupt/unparseable upload delivers a "failed" WS status, creates no
// chapters (raw binary must never reach chapters.content, D1), and — crucially
// — leaves a durable failed job row with a non-empty error_message so a reload
// shows the failure instead of nothing.
func TestRunWorkerParseFailure(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "020")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	hub := &mockIngestionHub{}
	svc := &IngestionService{pool: pool, hub: hub}
	jobID, _, err := svc.Start(ctx, universe.ID, strings.NewReader("not a pdf at all"), "manuscript.pdf")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	var sawFailed bool
	for _, msg := range hub.popMessages() {
		if msg.Type != "ingestion_progress" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("unmarshal progress payload: %v", err)
		}
		if payload["job_id"] == jobID.String() && payload["status"] == "failed" {
			if payload["universe_id"] != universe.ID.String() {
				t.Errorf("failed progress universe_id = %v, want %s", payload["universe_id"], universe.ID)
			}
			sawFailed = true
		}
	}
	if !sawFailed {
		t.Error("expected an ingestion_progress WS message with status=failed for this job")
	}

	var chapterCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM chapters c JOIN works w ON c.work_id = w.id WHERE w.universe_id = $1", universe.ID).Scan(&chapterCount); err != nil {
		t.Fatalf("count chapters: %v", err)
	}
	if chapterCount != 0 {
		t.Errorf("expected 0 chapters after parse failure, got %d", chapterCount)
	}

	// The failed job row must survive with its error_message — deleting the
	// orphan Work would cascade-delete it and hide the failure from the user.
	job, err := repositories.NewIngestionRepo(pool).FindByID(ctx, jobID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if job == nil {
		t.Fatal("expected the failed job row to survive, but it was gone")
	}
	if job.Status != "failed" {
		t.Errorf("job status = %q, want %q", job.Status, "failed")
	}
	if job.ErrorMessage == "" {
		t.Error("expected a non-empty error_message on the failed job")
	}
}

// TestIngestionServiceOrphanWork_ReusedWorkNotDeleted is a DB-backed
// regression test: a parse failure when ingesting into an *existing* Work
// (the works[0]-reuse branch) must never delete that Work.
func TestIngestionServiceOrphanWork_ReusedWorkNotDeleted(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "020")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	// Seed an existing work so Start takes the works[0]-reuse branch.
	workRepo := repositories.NewWorkRepo(pool)
	existingWork := models.Work{ID: uuid.New(), UniverseID: universe.ID, Title: "Existing Work", Type: "novel", Status: "in_progress"}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := workRepo.Create(ctx, tx, &existingWork); err != nil {
		t.Fatalf("create existing work: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	svc := &IngestionService{pool: pool}
	_, _, err = svc.Start(ctx, universe.ID, strings.NewReader("not a pdf at all"), "manuscript.pdf")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	got, err := workRepo.FindByID(ctx, existingWork.ID)
	if err != nil {
		t.Fatalf("expected the reused work to survive, but FindByID errored: %v", err)
	}
	if got.ID != existingWork.ID {
		t.Errorf("FindByID returned unexpected work: %+v", got)
	}
}

// TestSplitChunksCascade table-tests the heading-pattern priority cascade.
func TestSplitChunksCascade(t *testing.T) {
	svc := &IngestionService{}

	cases := []struct {
		name         string
		content      string
		minChunks    int
		wantContains string // a chunk title expected to appear
	}{
		{
			name:         "markdown",
			content:      "# Chapter One\n\nBody one.\n\n# Chapter Two\n\nBody two.",
			minChunks:    2,
			wantContains: "Chapter One",
		},
		{
			name:         "english chapter N",
			content:      "Chapter 1\n\nBody one.\n\nChapter 2\n\nBody two.",
			minChunks:    2,
			wantContains: "Chapter 1",
		},
		{
			name:         "english chapter spelled out",
			content:      "Chapter One\n\nBody one.\n\nChapter Two\n\nBody two.",
			minChunks:    2,
			wantContains: "Chapter One",
		},
		{
			name:         "spanish capitulo",
			content:      "Capítulo I\n\nCuerpo uno.\n\nCapítulo II\n\nCuerpo dos.",
			minChunks:    2,
			wantContains: "Capítulo I",
		},
		{
			name:         "spanish capitulo spelled out",
			content:      "Capítulo Uno\n\nCuerpo uno.\n\nCapítulo Dos\n\nCuerpo dos.",
			minChunks:    2,
			wantContains: "Capítulo Uno",
		},
		{
			name:         "bare roman numerals",
			content:      "I\n\nBody one.\n\nII\n\nBody two.",
			minChunks:    2,
			wantContains: "I",
		},
		{
			name:         "all caps headings",
			content:      "CHAPTER ONE: THE BEGINNING\n\nBody one.\n\nCHAPTER TWO: THE END\n\nBody two.",
			minChunks:    2,
			wantContains: "CHAPTER ONE: THE BEGINNING",
		},
		{
			name:         "title case single word",
			content:      "Holden\n\nBody one.\n\nMiller\n\nBody two.",
			minChunks:    2,
			wantContains: "Holden",
		},
		{
			name:         "uppercase character names",
			content:      "HOLDEN\n\nThey met Holden.\n\nMILLER\n\nThey met Miller.",
			minChunks:    2,
			wantContains: "HOLDEN",
		},
		{
			name:         "short uppercase dialogue stays in fallback content",
			content:      "YES\n\nThe answer landed between them.\n\nNOPE\n\nNobody moved.",
			minChunks:    1,
			wantContains: "Untitled",
		},
		{
			name:         "title case multi-word",
			content:      "The Rocinante\n\nBody one.\n\nThe Canterbury\n\nBody two.",
			minChunks:    2,
			wantContains: "The Rocinante",
		},
		{
			name:         "no pattern falls back to paragraphs",
			content:      "Just some prose.\n\nMore prose, no headings at all here.",
			minChunks:    1,
			wantContains: "Untitled",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := svc.splitChunks(tc.content)
			if len(chunks) < tc.minChunks {
				t.Fatalf("got %d chunks, want >= %d", len(chunks), tc.minChunks)
			}
			found := false
			for _, ch := range chunks {
				if ch.title == tc.wantContains {
					found = true
				}
			}
			if !found {
				titles := make([]string, len(chunks))
				for i, ch := range chunks {
					titles[i] = ch.title
				}
				t.Errorf("expected a chunk titled %q, got titles: %v", tc.wantContains, titles)
			}
		})
	}
}

func TestSplitChunksPreservesShortUppercaseDialogue(t *testing.T) {
	content := "YES\n\nThe answer landed between them.\n\nNOPE\n\nNobody moved."
	chunks := (&IngestionService{}).splitChunks(content)
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want one paragraph fallback", len(chunks))
	}
	if chunks[0].title != "Untitled" {
		t.Fatalf("title = %q, want Untitled", chunks[0].title)
	}
	if chunks[0].content != content {
		t.Fatalf("short dialogue was lost or reclassified: %q", chunks[0].content)
	}
}

func TestSplitChunksRecognizesPDFNarrativeHeadingsAndPreservesFrontMatter(t *testing.T) {
	content := "EL DESPERTAR DEL LEVIATÁN\nJAMES S. A. COREY\n\nPrólogo: Julie\n\nHabían tomado la Scopuli.\n\n1\n\nHolden\n\nCiento cincuenta años antes, Holden miró las estrellas.\n\n2\n\nMiller\n\nEl inspector Miller siguió la pista."

	chunks := (&IngestionService{}).splitChunks(content)
	wantTitles := []string{"Front Matter", "Prólogo: Julie", "Holden", "Miller"}
	if len(chunks) != len(wantTitles) {
		t.Fatalf("chunks = %d, want %d: %#v", len(chunks), len(wantTitles), chunks)
	}
	for i, want := range wantTitles {
		if chunks[i].title != want {
			t.Errorf("chunk %d title = %q, want %q", i, chunks[i].title, want)
		}
	}
	if !strings.Contains(chunks[0].content, "EL DESPERTAR DEL LEVIATÁN") ||
		!strings.Contains(chunks[1].content, "Habían tomado la Scopuli") ||
		!strings.Contains(chunks[2].content, "Ciento cincuenta años antes") ||
		!strings.Contains(chunks[3].content, "El inspector Miller") {
		t.Errorf("PDF excerpt content was not retained: %#v", chunks)
	}
}

func TestSplitChunksPreservesWaitAndStopWithinCharacterChapters(t *testing.T) {
	content := "AMOS\n\nHe met Amos in the hall.\n\nWAIT\n\nHELP\n\nPLEASE\n\nFINE\n\nSURE\n\nThe room went quiet.\n\nBOB\n\nShe met Bob before STOP.\n\nNobody moved."
	chunks := (&IngestionService{}).splitChunks(content)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want AMOS and BOB chapters", len(chunks))
	}
	if chunks[0].title != "AMOS" {
		t.Errorf("first chapter title = %q, want AMOS", chunks[0].title)
	}
	for _, dialogue := range []string{"WAIT", "HELP", "PLEASE", "FINE", "SURE"} {
		if !strings.Contains(chunks[0].content, dialogue) {
			t.Errorf("AMOS chapter lost %q dialogue: %#v", dialogue, chunks[0])
		}
	}
	if chunks[1].title != "BOB" || !strings.Contains(chunks[1].content, "STOP") {
		t.Errorf("second chapter = %#v, want BOB retaining STOP dialogue", chunks[1])
	}
}

func TestSplitChunksDoesNotTreatSentenceInitialDialogueAsCharacterEvidence(t *testing.T) {
	content := "YES\n\nYes, he nodded.\n\nNO\n\nNo, she replied."
	chunks := (&IngestionService{}).splitChunks(content)
	if len(chunks) != 1 || chunks[0].title != "Untitled" || chunks[0].content != content {
		t.Fatalf("sentence-initial dialogue became headings: %#v", chunks)
	}
}

// TestIsAllCapsHeadingLine verifies the ALL-CAPS fallback heuristic.
func TestIsAllCapsHeadingLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"CHAPTER ONE: THE BEGINNING", true},
		{"THE BEGINNING", true}, // 13 chars, no punctuation
		{"A", false},            // too short
		{"THE END.", false},     // sentence punctuation
		{"WHAT?", false},        // sentence punctuation
		{"LOOK!", false},        // sentence punctuation
		{"QUOTE,", false},       // sentence punctuation
		{"SEMICOLON;", false},   // sentence punctuation
		{"COLON:", false},       // sentence punctuation
		{"DIALOGUE", true},      // short uppercase headings may be character names
	}

	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := isAllCapsHeadingLine(tc.line); got != tc.want {
				t.Errorf("isAllCapsHeadingLine(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// TestSplitChunksCascadeMixedPrefersHighestPriority verifies that when a
// document contains heading styles from multiple pattern classes, the
// highest-priority class (earliest in the cascade) with >= 2 matches wins.
func TestSplitChunksCascadeMixedPrefersHighestPriority(t *testing.T) {
	svc := &IngestionService{}
	// Markdown headers present (2 matches) alongside a coincidental line that
	// looks like "Chapter N" prose — markdown must win since it's earlier in
	// the cascade and already has >= 2 matches.
	content := "# Chapter One\n\nChapter 9 was a turning point in the plot.\n\n# Chapter Two\n\nMore body."
	chunks := svc.splitChunks(content)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 markdown-driven chunks, got %d", len(chunks))
	}
	if chunks[0].title != "Chapter One" || chunks[1].title != "Chapter Two" {
		t.Errorf("unexpected titles: %q, %q", chunks[0].title, chunks[1].title)
	}
}

// TestSplitChunksCascadeGuardsAgainstFalsePositive verifies a pattern
// matching an unreasonable number of lines (a likely false positive) is
// skipped in favor of the next pattern in the cascade.
func TestSplitChunksCascadeGuardsAgainstFalsePositive(t *testing.T) {
	// 600 bare-roman-looking lines — over maxSaneHeadingMatches (500) — must
	// not be treated as chapter headings; falls through to paragraph split.
	var b strings.Builder
	for i := 0; i < 600; i++ {
		b.WriteString("I\n\nSome body text to keep this a real paragraph so it survives trimming.\n\n")
	}
	chunks := (&IngestionService{}).splitChunks(b.String())
	for _, ch := range chunks {
		if ch.title == "I" && len(chunks) > 500 {
			t.Fatalf("expected the >500-match guard to reject the bare-roman pattern, got %d chunks titled %q", len(chunks), ch.title)
		}
	}
}

// TestSelectAnalysisChapters verifies the K cap and zero-entity skip.
func TestSelectAnalysisChapters(t *testing.T) {
	mk := func(n int, hasEntity bool) ingestedChapter {
		ch := ingestedChapter{ID: uuid.New(), Content: fmt.Sprintf("chapter %d", n)}
		if hasEntity {
			ch.Entities = []ResolvedEntity{{MentionText: fmt.Sprintf("chapter %d", n)}}
		}
		return ch
	}

	chapters := []ingestedChapter{
		mk(1, true),
		mk(2, false), // skipped: zero entities
		mk(3, true),
		mk(4, true),
		mk(5, true),
	}

	selected := selectAnalysisChapters(chapters, 2)
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected chapters (K cap), got %d", len(selected))
	}
	// Last-K among the ones with entities: chapters 1,3,4,5 have entities;
	// last 2 of those are 4 and 5.
	if selected[0].Content != "chapter 4" || selected[1].Content != "chapter 5" {
		t.Errorf("unexpected selection: %+v", selected)
	}

	// K larger than available: returns all chapters with entities.
	selected = selectAnalysisChapters(chapters, 10)
	if len(selected) != 4 {
		t.Errorf("expected 4 chapters with entities, got %d", len(selected))
	}

	if got := selectAnalysisChapters(chapters, 0); got != nil {
		t.Errorf("expected nil for k=0, got %+v", got)
	}
}

// TestRunPostIngestAnalysis is a DB-backed wiring test: nil qwenSvc makes
// CheckSemantic and the plot-hole agent evaluation no-op, so this exercises
// the K cap / zero-entity skip / never-fails-the-job contract without
// needing a live Qwen API.
func TestRunPostIngestAnalysis(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "020")
	ctx := context.Background()

	user := svcCreateTestUser(t, ctx, pool)
	universe := svcCreateTestUniverse(t, ctx, pool, user.ID)

	workRepo := repositories.NewWorkRepo(pool)
	work := models.Work{ID: uuid.New(), UniverseID: universe.ID, Title: "Test Work", Type: "novel", Status: "in_progress"}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := workRepo.Create(ctx, tx, &work); err != nil {
		t.Fatalf("create work: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	chRepo := repositories.NewChapterRepo(pool)
	entityRepo := repositories.NewEntityRepo(pool)
	var chapters []ingestedChapter
	for i := 1; i <= 3; i++ {
		ch := models.Chapter{
			ID:         uuid.New(),
			WorkID:     work.ID,
			Title:      fmt.Sprintf("Chapter %d", i),
			OrderIndex: i,
			Content:    fmt.Sprintf("Chapter %d content.", i),
			RawText:    fmt.Sprintf("Chapter %d content.", i),
			Status:     "draft",
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if err := chRepo.Create(ctx, tx, &ch); err != nil {
			t.Fatalf("create chapter: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}

		entity := models.Entity{ID: uuid.New(), UniverseID: universe.ID, Type: "character", Name: fmt.Sprintf("Entity %d", i), Status: "alive"}
		tx2, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if err := entityRepo.Create(ctx, tx2, &entity); err != nil {
			t.Fatalf("create entity: %v", err)
		}
		if err := tx2.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}

		chapters = append(chapters, ingestedChapter{
			ID:      ch.ID,
			Content: ch.Content,
			Entities: []ResolvedEntity{
				{Entity: entity, MentionText: ch.Content, IsNew: true},
			},
		})
	}

	tok := NewTokenizer()
	budgetMgr := NewContextBudgetManager(tok, 30000, 2000)
	contraSvc := NewContradictionService(pool, repositories.NewContradictionRepo(pool), entityRepo, nil, nil, 3, budgetMgr, 3)
	plotHoleSvc := NewPlotHoleService(pool, repositories.NewPlotHoleRepo(pool), entityRepo, 8, nil, nil, 2)

	hub := &mockIngestionHub{}
	svc := &IngestionService{pool: pool, hub: hub}
	svc.SetPostIngestAnalysis(contraSvc, plotHoleSvc, budgetMgr, 2)

	svc.runPostIngestAnalysis(ctx, universe.ID, chapters, user.ID)

	// K cap of 2 respected — with a nil qwenSvc, CheckSemantic no-ops (no
	// contradictions), so the only observable check here is that it didn't
	// panic/error/hang across all 3 candidate chapters despite the cap.
}

// compile-time interface checks
var _ IngestionQwen = (*mockQwenForIngestion)(nil)
var _ *pgxpool.Pool = nil
var _ *repositories.VectorRepo = nil
var _ *repositories.GraphRepo = nil
var _ *EntityService = nil
var _ AnalysisHub = (*mockIngestionHub)(nil)
