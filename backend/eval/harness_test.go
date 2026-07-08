package eval

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
	"github.com/quill/backend/internal/testutil"
)

// evalFixture holds the shared setup for memory evaluation tests.
type evalFixture struct {
	pool                *pgxpool.Pool
	svc                 *services.MemoryService
	qwen                *services.QwenService
	universeID          uuid.UUID
	gold                *GoldSet
	paragraphToEntities map[string][]uuid.UUID
}

// setupSagaEval loads the gold corpus, builds a real MemoryService, and
// backfills paragraph embeddings for the saga universe.
func setupSagaEval(t *testing.T) *evalFixture {
	t.Helper()

	pool := testutil.SetupTestDB(t)

	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE required for corpus migration 014")
	}
	if os.Getenv("QWEN_API_KEY") == "" {
		t.Skip("QWEN_API_KEY required for semantic recall eval")
	}

	gold, err := LoadGold("corpus/saga_gold.json")
	if err != nil {
		t.Fatalf("load gold corpus: %v", err)
	}

	svc, universeID := buildRealMemoryService(t, pool)
	resolveGoldIDs(t, pool, gold)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	qwen := services.NewQwenService(cfg, nil)

	ctx := context.Background()
	paragraphToEntities := backfillParagraphEmbeddings(t, ctx, pool, qwen, universeID)

	return &evalFixture{
		pool:                pool,
		svc:                 svc,
		qwen:                qwen,
		universeID:          universeID,
		gold:                gold,
		paragraphToEntities: paragraphToEntities,
	}
}

// backfillParagraphEmbeddings splits each saga chapter into paragraphs,
// generates real embeddings for them, and persists paragraph_embeddings rows.
// It returns a map from paragraph content to the entity IDs mentioned in it.
func backfillParagraphEmbeddings(t *testing.T, ctx context.Context, pool *pgxpool.Pool, qwen *services.QwenService, universeID uuid.UUID) map[string][]uuid.UUID {
	t.Helper()

	vectorRepo := repositories.NewVectorRepo(pool)

	rows, err := pool.Query(ctx, `
		SELECT c.id, c.content
		FROM chapters c
		JOIN works w ON c.work_id = w.id
		WHERE w.universe_id = $1
		ORDER BY c.order_index
	`, universeID)
	if err != nil {
		t.Fatalf("query chapters: %v", err)
	}
	defer rows.Close()

	type paragraph struct {
		chapterID uuid.UUID
		index     int
		content   string
	}

	var paragraphs []paragraph
	var keys []repositories.ParagraphKey
	for rows.Next() {
		var chapterID uuid.UUID
		var content string
		if err := rows.Scan(&chapterID, &content); err != nil {
			t.Fatalf("scan chapter: %v", err)
		}
		// paragraph_index is 1-based to match the entity_mentions seeded by migration 014.
		for i, part := range strings.Split(content, "\n\n") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			pIdx := i + 1
			paragraphs = append(paragraphs, paragraph{chapterID: chapterID, index: pIdx, content: part})
			keys = append(keys, repositories.ParagraphKey{ChapterID: chapterID, ParagraphIndex: pIdx})
		}
	}

	const batchSize = 10
	for offset := 0; offset < len(paragraphs); offset += batchSize {
		end := offset + batchSize
		if end > len(paragraphs) {
			end = len(paragraphs)
		}
		batch := paragraphs[offset:end]
		contents := make([]string, len(batch))
		for i, p := range batch {
			contents[i] = p.content
		}

		embeddings, err := qwen.GenerateEmbeddingBatch(ctx, contents)
		if err != nil {
			t.Fatalf("embed paragraphs batch %d: %v", offset/batchSize, err)
		}
		if len(embeddings) != len(batch) {
			t.Fatalf("embeddings count mismatch in batch %d: got %d, want %d", offset/batchSize, len(embeddings), len(batch))
		}

		for i, p := range batch {
			nodeID := fmt.Sprintf("eval-node-%d", offset+i)
			if err := vectorRepo.SaveParagraphEmbedding(ctx, p.chapterID, p.index, nodeID, p.content, embeddings[i]); err != nil {
				t.Fatalf("save paragraph embedding: %v", err)
			}
		}
	}

	entityRepo := repositories.NewEntityRepo(pool)
	mentions, err := entityRepo.EntityIDsForParagraphs(ctx, keys)
	if err != nil {
		t.Fatalf("entity ids for paragraphs: %v", err)
	}

	result := make(map[string][]uuid.UUID, len(paragraphs))
	for _, p := range paragraphs {
		key := repositories.ParagraphKey{ChapterID: p.chapterID, ParagraphIndex: p.index}
		result[p.content] = mentions[key]
	}
	return result
}

// itemEntityIDs resolves a recall item to the entity IDs it represents.
// Paragraph-sourced items (nil EntityID) are mapped via paragraph content.
func itemEntityIDs(item models.RecallItem, paragraphToEntities map[string][]uuid.UUID) []uuid.UUID {
	if item.EntityID != uuid.Nil {
		return []uuid.UUID{item.EntityID}
	}
	return paragraphToEntities[item.Fact]
}

// retrievedEntityIDs flattens recall items into a slice of entity ID strings.
func retrievedEntityIDs(items []models.RecallItem, paragraphToEntities map[string][]uuid.UUID) []string {
	var out []string
	for _, item := range items {
		for _, id := range itemEntityIDs(item, paragraphToEntities) {
			out = append(out, id.String())
		}
	}
	return out
}

// relevantSet builds a string set from a gold query's resolved entity IDs.
func relevantSet(q GoldQuery) map[string]bool {
	set := make(map[string]bool, len(q.RelevantEntityIDs))
	for _, id := range q.RelevantEntityIDs {
		set[id.String()] = true
	}
	return set
}

func TestMemoryEvalRecall(t *testing.T) {
	fx := setupSagaEval(t)
	ctx := context.Background()

	var report RecallReport
	for _, q := range fx.gold.Queries {
		emb, err := fx.qwen.GenerateEmbedding(ctx, q.Query)
		if err != nil {
			t.Fatalf("embed query %s: %v", q.ID, err)
		}

		items, err := fx.svc.RecallWithQuery(ctx, fx.universeID, emb, q.Query, 10)
		if err != nil {
			t.Fatalf("recall query %s: %v", q.ID, err)
		}

		retrieved := retrievedEntityIDs(items, fx.paragraphToEntities)
		relevant := relevantSet(q)

		report.Queries = append(report.Queries, QueryReport{
			ID:           q.ID,
			Query:        q.Query,
			RecallAt5:    recallAtK(retrieved, relevant, 5),
			PrecisionAt5: precisionAtK(retrieved, relevant, 5),
			MRR:          mrr(retrieved, relevant),
			NDCGAt5:      ndcgAtK(retrieved, relevant, 5),
		})
	}

	writeRecallReport(t, "../../Docs/eval/results.md", report)
}

func TestMemoryEvalAblation(t *testing.T) {
	fx := setupSagaEval(t)
	ctx := context.Background()

	pipelineSets := []struct {
		name      string
		pipelines []string
	}{
		{"vector", []string{"vector"}},
		{"graph", []string{"graph"}},
		{"recency", []string{"recency"}},
		{"keyword", []string{"keyword"}},
		{"vector+graph", []string{"vector", "graph"}},
		{"all", nil},
	}

	queryEmbeddings := make(map[string][]float32, len(fx.gold.Queries))
	for _, q := range fx.gold.Queries {
		emb, err := fx.qwen.GenerateEmbedding(ctx, q.Query)
		if err != nil {
			t.Fatalf("embed query %s: %v", q.ID, err)
		}
		queryEmbeddings[q.Query] = emb
	}

	t.Logf("Memory eval ablation (average recall@5 over %d queries):", len(fx.gold.Queries))
	for _, ps := range pipelineSets {
		var totalRecall float64
		for _, q := range fx.gold.Queries {
			items, err := fx.svc.RecallWithPipelines(ctx, fx.universeID, queryEmbeddings[q.Query], q.Query, 5, ps.pipelines)
			if err != nil {
				t.Fatalf("recall query %s with pipelines %v: %v", q.ID, ps.pipelines, err)
			}

			retrieved := retrievedEntityIDs(items, fx.paragraphToEntities)
			totalRecall += recallAtK(retrieved, relevantSet(q), 5)
		}
		avg := totalRecall / float64(len(fx.gold.Queries))
		t.Logf("  %-14s %.3f", ps.name, avg)
	}
}
