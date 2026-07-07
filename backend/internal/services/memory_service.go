package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// rrfK is the Reciprocal Rank Fusion constant (Cormack et al.'s canonical
// choice): score += 1/(rrfK+rank) per pipeline appearance, rank 1-indexed.
const rrfK = 60

// rankedEntry is one pipeline's ranked output before fusion. id is the
// dedupe/identity key: an entity UUID string for entity-keyed sources
// (graph, recency, consolidated), or a synthetic "chapterID:snippet" key for
// vector/paragraph-sourced hits (entityID stays uuid.Nil for those).
type rankedEntry struct {
	id       string
	entityID uuid.UUID
	fact     string
	source   string
}

// HybridRecallItem is a fused, ranked recall result after RRF combines one
// or more pipeline appearances for the same id.
type HybridRecallItem struct {
	ID       string
	EntityID uuid.UUID
	Fact     string
	RRFScore float64
	Sources  []string
}

// fuseRRF combines any number of independently-ranked pipelines by
// Reciprocal Rank Fusion: score[id] += 1/(rrfK+rank) summed across every
// list the id appears in, rank 1-indexed within each list. Items are
// deduped by id; the first non-empty entityID/fact seen wins. Result is
// sorted by RRFScore descending, ties broken by ID ascending for
// determinism.
func fuseRRF(lists ...[]rankedEntry) []HybridRecallItem {
	byID := make(map[string]*HybridRecallItem)
	order := make([]string, 0)

	for _, list := range lists {
		for i, e := range list {
			item, exists := byID[e.id]
			if !exists {
				item = &HybridRecallItem{ID: e.id}
				byID[e.id] = item
				order = append(order, e.id)
			}
			item.RRFScore += 1.0 / float64(rrfK+i+1)
			item.Sources = append(item.Sources, e.source)
			if item.EntityID == uuid.Nil && e.entityID != uuid.Nil {
				item.EntityID = e.entityID
			}
			if item.Fact == "" && e.fact != "" {
				item.Fact = e.fact
			}
		}
	}

	result := make([]HybridRecallItem, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].RRFScore == result[j].RRFScore {
			return result[i].ID < result[j].ID
		}
		return result[i].RRFScore > result[j].RRFScore
	})
	return result
}

// ResolvedEntity is a shared type between MemoryService, ContradictionService,
// and AnalysisService. It pairs an entity with the mention text that triggered
// the resolution.
type ResolvedEntity struct {
	Entity      models.Entity
	MentionText string
	IsNew       bool
	// PreviousStatus is the entity's status as it was in the DB before this
	// mention's data was merged in (see EntityService.ResolveOrCreate).
	// Deterministic contradiction checks must compare against this, not
	// Entity.Status, which already reflects the newly-merged value.
	PreviousStatus string
}

// graphSeedCap bounds how many vector-derived (or, in degraded mode,
// recency-ranked) entity IDs seed the graph pipeline's neighbor lookup.
const graphSeedCap = 5

// MemoryService provides contextual recall by fusing up to four
// independently-ranked pipelines (vector similarity, graph context,
// recency/keyword, consolidated memories) via Reciprocal Rank Fusion.
//
// consolidationRepo and budgetMgr are optional (nil-safe): consolidationRepo
// nil skips the consolidated-memory pipeline, budgetMgr nil skips budget
// fitting. They are wired via SetConsolidationRepo/SetBudgetMgr rather than
// the constructor so existing call sites (main.go, tests) don't need to
// change until they're ready to supply real values.
type MemoryService struct {
	graphRepo         *repositories.GraphRepo
	entityRepo        *repositories.EntityRepo
	vectorRepo        *repositories.VectorRepo
	consolidationRepo *repositories.ConsolidationRepo
	budgetMgr         *ContextBudgetManager
}

// NewMemoryService creates a memory service with the given repos.
func NewMemoryService(graphRepo *repositories.GraphRepo, entityRepo *repositories.EntityRepo, vectorRepo *repositories.VectorRepo) *MemoryService {
	return &MemoryService{
		graphRepo:  graphRepo,
		entityRepo: entityRepo,
		vectorRepo: vectorRepo,
	}
}

// SetConsolidationRepo wires the consolidated-memory pipeline. Optional —
// nil-safe; the pipeline is skipped when unset.
func (s *MemoryService) SetConsolidationRepo(r *repositories.ConsolidationRepo) {
	s.consolidationRepo = r
}

// SetBudgetMgr wires context-budget fitting via FitToBudget/VectorTokens.
// Optional — nil-safe; budget fitting is skipped when unset.
func (s *MemoryService) SetBudgetMgr(b *ContextBudgetManager) {
	s.budgetMgr = b
}

// Recall is the caller-compatible entrypoint kept for existing callers
// (ws/hub.go, handlers/graph.go, analysis_service.go) that don't yet pass a
// queryText. It delegates to RecallWithQuery with an empty queryText, which
// is exactly the degraded/normal split those callers already rely on
// (embedding present → normal vector-seeded mode; embedding absent →
// degraded recency-seeded mode).
func (s *MemoryService) Recall(ctx context.Context, universeID uuid.UUID, queryEmbedding []float32, k int) ([]models.RecallItem, error) {
	return s.RecallWithQuery(ctx, universeID, queryEmbedding, "", k)
}

// RecallWithQuery returns up to k RecallItems for a universe, fusing vector
// similarity, graph context, recency, keyword, and consolidated-memory
// pipelines via Reciprocal Rank Fusion (see fuseRRF).
//
// Normal mode (embedding present): the vector pipeline runs first; the
// entities mentioned in its top paragraph hits seed the graph pipeline
// (capped at graphSeedCap), so graph context follows query semantics.
// Degraded mode (embedding absent AND queryText empty): vector, keyword, and
// consolidated pipelines are skipped; the graph pipeline instead seeds from
// the top graphSeedCap recency-ranked active entities.
func (s *MemoryService) RecallWithQuery(ctx context.Context, universeID uuid.UUID, queryEmbedding []float32, queryText string, k int) ([]models.RecallItem, error) {
	entities, err := s.entityRepo.ListByUniverseActive(ctx, universeID)
	if err != nil {
		return nil, fmt.Errorf("recall: list active entities: %w", err)
	}

	hasEmbedding := len(queryEmbedding) > 0
	hasText := queryText != ""
	graphName := "universe_" + universeID.String()

	sortedEntities := make([]models.Entity, len(entities))
	copy(sortedEntities, entities)
	sort.Slice(sortedEntities, func(i, j int) bool {
		return sortedEntities[i].RelevanceScore > sortedEntities[j].RelevanceScore
	})

	var (
		vectorRanked       []rankedEntry
		graphRanked        []rankedEntry
		recencyRanked      []rankedEntry
		keywordRanked      []rankedEntry
		consolidatedRanked []rankedEntry
		graphSeeds         []uuid.UUID
	)

	if hasEmbedding {
		vectorRanked, graphSeeds = s.vectorPipelineAndSeeds(ctx, universeID, queryEmbedding, k)
	} else {
		for i := 0; i < len(sortedEntities) && i < graphSeedCap; i++ {
			graphSeeds = append(graphSeeds, sortedEntities[i].ID)
		}
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		graphRanked = s.graphPipeline(ctx, graphName, graphSeeds)
	}()

	for _, e := range sortedEntities {
		recencyRanked = append(recencyRanked, rankedEntry{
			id:       e.ID.String(),
			entityID: e.ID,
			fact:     fmt.Sprintf("Recently active entity: %s", e.Name),
			source:   "recency",
		})
	}

	if hasText {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, err := s.vectorRepo.KeywordSearch(ctx, universeID, queryText, k)
			if err != nil {
				return
			}
			for _, h := range hits {
				keywordRanked = append(keywordRanked, rankedEntry{
					id:     h.ChapterID.String() + ":" + h.Content,
					fact:   h.Content,
					source: "keyword",
				})
			}
		}()
	}

	if hasEmbedding && s.consolidationRepo != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, err := s.consolidationRepo.FindSimilarByEmbedding(ctx, universeID, queryEmbedding, k)
			if err != nil {
				return
			}
			for _, h := range hits {
				consolidatedRanked = append(consolidatedRanked, rankedEntry{
					id:       h.EntityID.String(),
					entityID: h.EntityID,
					fact:     h.Summary,
					source:   "consolidated",
				})
			}
		}()
	}

	wg.Wait()

	fused := fuseRRF(vectorRanked, graphRanked, recencyRanked, keywordRanked, consolidatedRanked)

	if s.budgetMgr != nil {
		fused = s.fitToBudget(fused)
	}

	if k > 0 && len(fused) > k {
		fused = fused[:k]
	}

	items := make([]models.RecallItem, len(fused))
	for i, f := range fused {
		items[i] = models.RecallItem{
			ID:       f.ID,
			EntityID: f.EntityID,
			Fact:     f.Fact,
			Score:    f.RRFScore,
			Source:   strings.Join(f.Sources, ","),
		}
	}
	return items, nil
}

// vectorPipelineAndSeeds runs the vector-similarity pipeline and derives
// graph seeds from the entities mentioned in the top-ranked paragraph hits
// (ADR-2: vector-seeded graph pipeline), preserving vector rank order and
// capping at graphSeedCap. excludeChapterID is uuid.Nil — Recall has no
// "current chapter" context to exclude, matching agent_tools.go's usage.
func (s *MemoryService) vectorPipelineAndSeeds(ctx context.Context, universeID uuid.UUID, queryEmbedding []float32, k int) ([]rankedEntry, []uuid.UUID) {
	paragraphs, err := s.vectorRepo.FindSimilarParagraphs(ctx, universeID, queryEmbedding, uuid.Nil, k)
	if err != nil || len(paragraphs) == 0 {
		return nil, nil
	}

	ranked := make([]rankedEntry, 0, len(paragraphs))
	keys := make([]repositories.ParagraphKey, 0, len(paragraphs))
	for _, p := range paragraphs {
		ranked = append(ranked, rankedEntry{
			id:     p.ChapterID.String() + ":" + p.Content,
			fact:   p.Content,
			source: "vector",
		})
		keys = append(keys, repositories.ParagraphKey{ChapterID: p.ChapterID, ParagraphIndex: p.ParagraphIndex})
	}

	mentions, err := s.entityRepo.EntityIDsForParagraphs(ctx, keys)
	if err != nil {
		return ranked, nil
	}

	seen := make(map[uuid.UUID]bool)
	var seeds []uuid.UUID
	for _, key := range keys {
		for _, eid := range mentions[key] {
			if seen[eid] {
				continue
			}
			seen[eid] = true
			seeds = append(seeds, eid)
			if len(seeds) >= graphSeedCap {
				return ranked, seeds
			}
		}
	}
	return ranked, seeds
}

// graphPipeline resolves 1-hop neighbors for all seeds via a single batched
// GetNeighborsBatch call (spec: "Graph Pipeline Uses Batched Neighbor
// Lookup" — not one GetNeighbors call per seed) and ranks the union by
// co-citation count (neighbors reachable from more than one seed rank
// higher).
func (s *MemoryService) graphPipeline(ctx context.Context, graphName string, seeds []uuid.UUID) []rankedEntry {
	coCitation := make(map[uuid.UUID]int)
	order := make([]uuid.UUID, 0)

	if len(seeds) == 0 {
		return nil
	}

	seedIDs := make([]string, len(seeds))
	for i, seed := range seeds {
		seedIDs[i] = seed.String()
	}

	neighborsBySeed, err := s.graphRepo.GetNeighborsBatch(ctx, graphName, seedIDs)
	if err != nil {
		return nil
	}

	for _, seedID := range seedIDs {
		for _, n := range neighborsBySeed[seedID] {
			nid, ok := extractEntityIDFromNode(n.Node)
			if !ok {
				continue
			}
			if _, exists := coCitation[nid]; !exists {
				order = append(order, nid)
			}
			coCitation[nid]++
		}
	}

	sort.Slice(order, func(i, j int) bool { return coCitation[order[i]] > coCitation[order[j]] })

	entries := make([]rankedEntry, 0, len(order))
	for _, nid := range order {
		entries = append(entries, rankedEntry{
			id:       nid.String(),
			entityID: nid,
			fact:     "Related entity",
			source:   "graph",
		})
	}
	return entries
}

// extractEntityIDFromNode pulls the entity_id property out of an AGE agtype
// node string via plain substring search — the same sharp-edge tolerant
// approach as repositories.extractProp (AGE nodes serialize to JSON-ish
// text, not structured Go values).
func extractEntityIDFromNode(nodeStr string) (uuid.UUID, bool) {
	const key = `"entity_id": "`
	idx := strings.Index(nodeStr, key)
	if idx < 0 {
		return uuid.Nil, false
	}
	start := idx + len(key)
	end := strings.Index(nodeStr[start:], `"`)
	if end < 0 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(nodeStr[start : start+end])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// fitToBudget trims the fused, ranked list to BudgetAllocation.VectorTokens
// via ContextBudgetManager.FitToBudget, mapping RankedItem.Text back to its
// HybridRecallItem by Fact (ADR-4: assumes near-unique facts per call).
func (s *MemoryService) fitToBudget(fused []HybridRecallItem) []HybridRecallItem {
	alloc := s.budgetMgr.ComputeBudget(0, 0)

	ranked := make([]RankedItem, len(fused))
	byFact := make(map[string]*HybridRecallItem, len(fused))
	for i := range fused {
		ranked[i] = RankedItem{Text: fused[i].Fact, Score: fused[i].RRFScore}
		byFact[fused[i].Fact] = &fused[i]
	}

	fitted, _, _ := s.budgetMgr.FitToBudget(ranked, alloc.VectorTokens)

	result := make([]HybridRecallItem, 0, len(fitted))
	for _, r := range fitted {
		if item, ok := byFact[r.Text]; ok {
			result = append(result, *item)
		}
	}
	return result
}
