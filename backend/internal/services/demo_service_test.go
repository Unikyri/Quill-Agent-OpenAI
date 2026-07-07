package services

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"

	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/testutil"
)

const templateUniverseID = "00000000-0000-0000-0000-000000000002"

func TestRemapUUIDsCorrect(t *testing.T) {
	m := map[string]string{
		"aaa": "AAA",
		"bbb": "BBB",
		"ccc": "CCC",
	}
	got := remapUUIDs([]string{"aaa", "bbb", "ccc"}, m)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != "AAA" || got[1] != "BBB" || got[2] != "CCC" {
		t.Errorf("got %v, want [AAA BBB CCC]", got)
	}
}

func TestRemapUUIDsEmpty(t *testing.T) {
	got := remapUUIDs([]string{}, map[string]string{})
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestCloneUniverseDeepCopy(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "014")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()

	universeRepo := repositories.NewUniverseRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	svc := NewDemoService(pool, universeRepo, graphRepo)

	sessionID := uuid.NewString()

	// Clone
	newID, err := svc.CloneUniverse(ctx, sessionID)
	if err != nil {
		t.Fatalf("CloneUniverse: %v", err)
	}
	if newID == "" || newID == templateUniverseID {
		t.Fatalf("newID = %q, want non-empty and different from template", newID)
	}

	// Verify works (≥3)
	var workCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM works WHERE universe_id = $1`, newID).Scan(&workCount); err != nil {
		t.Fatalf("count works: %v", err)
	}
	if workCount < 3 {
		t.Errorf("works = %d, want ≥3", workCount)
	}

	// Verify chapters (≥3)
	var chapterCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM chapters c
		JOIN works w ON c.work_id = w.id
		WHERE w.universe_id = $1`, newID).Scan(&chapterCount); err != nil {
		t.Fatalf("count chapters: %v", err)
	}
	if chapterCount < 3 {
		t.Errorf("chapters = %d, want ≥3", chapterCount)
	}

	// Verify entities (≥20)
	var entityCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM entities WHERE universe_id = $1`, newID).Scan(&entityCount); err != nil {
		t.Fatalf("count entities: %v", err)
	}
	if entityCount < 20 {
		t.Errorf("entities = %d, want ≥20", entityCount)
	}

	// Verify contradictions (≥6)
	var contraCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM contradictions WHERE universe_id = $1`, newID).Scan(&contraCount); err != nil {
		t.Fatalf("count contradictions: %v", err)
	}
	if contraCount < 6 {
		t.Errorf("contradictions = %d, want ≥6", contraCount)
	}

	// Verify entity mentions (≥1 per chapter average)
	var mentionCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM entity_mentions em
		JOIN entities e ON em.entity_id = e.id
		WHERE e.universe_id = $1`, newID).Scan(&mentionCount); err != nil {
		t.Fatalf("count mentions: %v", err)
	}
	if mentionCount < 10 {
		t.Errorf("mentions = %d, want ≥10", mentionCount)
	}

	// Verify entity embeddings exist
	var embCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM entity_embeddings ee
		JOIN entities e ON ee.entity_id = e.id
		WHERE e.universe_id = $1`, newID).Scan(&embCount); err != nil {
		t.Fatalf("count entity embeddings: %v", err)
	}
	if embCount < 20 {
		t.Errorf("entity embeddings = %d, want ≥20", embCount)
	}

	// Verify timeline events
	var tlCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM timeline_events WHERE universe_id = $1`, newID).Scan(&tlCount); err != nil {
		t.Fatalf("count timeline events: %v", err)
	}
	if tlCount < 5 {
		t.Errorf("timeline events = %d, want ≥5", tlCount)
	}

	// Verify plot holes
	var phCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM plot_holes WHERE universe_id = $1`, newID).Scan(&phCount); err != nil {
		t.Fatalf("count plot holes: %v", err)
	}
	if phCount < 3 {
		t.Errorf("plot holes = %d, want ≥3", phCount)
	}
}

func TestCloneUniverseIdempotent(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "014")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()

	universeRepo := repositories.NewUniverseRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	svc := NewDemoService(pool, universeRepo, graphRepo)

	sessionID := uuid.NewString()

	// First clone
	id1, err := svc.CloneUniverse(ctx, sessionID)
	if err != nil {
		t.Fatalf("first CloneUniverse: %v", err)
	}

	// Second clone (same session) — should return same ID
	id2, err := svc.CloneUniverse(ctx, sessionID)
	if err != nil {
		t.Fatalf("second CloneUniverse: %v", err)
	}

	if id1 != id2 {
		t.Errorf("idempotent clone: id1=%s id2=%s, want same", id1, id2)
	}

	// Verify only one universe exists for this session
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM universes WHERE session_id = $1`, sessionID).Scan(&count); err != nil {
		t.Fatalf("count session universes: %v", err)
	}
	if count != 1 {
		t.Errorf("session universes = %d, want 1", count)
	}
}

func TestCloneUniverseUUIDRemapNoOrphans(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "014")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()

	universeRepo := repositories.NewUniverseRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	svc := NewDemoService(pool, universeRepo, graphRepo)

	newID, err := svc.CloneUniverse(ctx, uuid.NewString())
	if err != nil {
		t.Fatalf("CloneUniverse: %v", err)
	}

	// Verify every entity_mention belonging to the CLONED universe (reached via
	// chapter->work->universe, independent of entity_id) references an entity
	// in that same cloned universe.
	// ponytail: scoping via chapter_id/work_id (not entity_id) is deliberate —
	// entity_id is exactly the field under test, so it can't be used to select
	// "which mentions belong to this clone" without begging the question. An
	// unscoped WHERE e.universe_id != $1 here would always count the
	// template's own untouched seed mentions as false-positive orphans.
	var orphanMentions int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM entity_mentions em
		JOIN chapters c ON em.chapter_id = c.id
		JOIN works w ON c.work_id = w.id
		JOIN entities e ON em.entity_id = e.id
		WHERE w.universe_id = $1 AND e.universe_id != $1`, newID).Scan(&orphanMentions); err != nil {
		t.Fatalf("count orphan mentions: %v", err)
	}
	if orphanMentions > 0 {
		t.Errorf("orphan entity_mentions (wrong universe) = %d, want 0", orphanMentions)
	}

	// Verify every contradiction references entities in the SAME universe
	var orphanContras int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM contradictions c
		LEFT JOIN entities e ON c.entity_id = e.id
		WHERE c.universe_id = $1 AND c.entity_id IS NOT NULL AND e.id IS NULL`, newID).Scan(&orphanContras); err != nil {
		t.Fatalf("count orphan contradictions: %v", err)
	}
	if orphanContras > 0 {
		t.Errorf("orphan contradiction entity refs = %d, want 0", orphanContras)
	}

	// Verify every timeline_event with participants references valid entities
	var orphanTlParticipants int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM timeline_events t
		WHERE t.universe_id = $1 AND NOT (t.participants <@ ARRAY(SELECT id FROM entities WHERE universe_id = $1))`, newID).Scan(&orphanTlParticipants); err != nil {
		t.Fatalf("count orphan timeline participants: %v", err)
	}
	if orphanTlParticipants > 0 {
		t.Errorf("orphan timeline participants = %d, want 0", orphanTlParticipants)
	}
}

func TestCloneUniverseEmbeddingCopy(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "014")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()

	universeRepo := repositories.NewUniverseRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	svc := NewDemoService(pool, universeRepo, graphRepo)

	newID, err := svc.CloneUniverse(ctx, uuid.NewString())
	if err != nil {
		t.Fatalf("CloneUniverse: %v", err)
	}

	// Find a template entity and its embedding, compare with clone
	var oldEntityID, newEntityID string
	var oldVecRaw, newVecRaw pgvector.Vector

	// Get one template entity + embedding
	if err := pool.QueryRow(ctx, `
		SELECT e.id, ee.description_embedding
		FROM entities e
		JOIN entity_embeddings ee ON e.id = ee.entity_id
		WHERE e.universe_id = $1 AND e.name = 'Lyra Vane'
		LIMIT 1`, templateUniverseID).Scan(&oldEntityID, &oldVecRaw); err != nil {
		t.Fatalf("query template Lyra embedding: %v", err)
	}

	// Get matching clone entity + embedding
	if err := pool.QueryRow(ctx, `
		SELECT e.id, ee.description_embedding
		FROM entities e
		JOIN entity_embeddings ee ON e.id = ee.entity_id
		WHERE e.universe_id = $1 AND e.name = 'Lyra Vane'
		LIMIT 1`, newID).Scan(&newEntityID, &newVecRaw); err != nil {
		t.Fatalf("query clone Lyra embedding: %v", err)
	}
	oldVec, newVec := oldVecRaw.Slice(), newVecRaw.Slice()

	// Vectors should match byte-for-byte
	if len(oldVec) != len(newVec) {
		t.Errorf("embedding dim mismatch: template=%d clone=%d", len(oldVec), len(newVec))
	}
	for i := range oldVec {
		if oldVec[i] != newVec[i] {
			t.Errorf("embedding[%d] differs: template=%f clone=%f", i, oldVec[i], newVec[i])
			break
		}
	}

	// Verify UUIDs are different
	if oldEntityID == newEntityID {
		t.Error("template and clone entity IDs should differ")
	}
}

func TestResetUniverse(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "014")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()

	universeRepo := repositories.NewUniverseRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	svc := NewDemoService(pool, universeRepo, graphRepo)

	sessionID := uuid.NewString()

	// Clone
	firstID, err := svc.CloneUniverse(ctx, sessionID)
	if err != nil {
		t.Fatalf("first CloneUniverse: %v", err)
	}

	// Modify an entity name in the cloned universe
	_, err = pool.Exec(ctx, `UPDATE entities SET name = 'Modified Name' WHERE universe_id = $1 AND name = 'Lyra Vane'`, firstID)
	if err != nil {
		t.Fatalf("modify entity: %v", err)
	}

	// Reset
	resetID, err := svc.ResetUniverse(ctx, sessionID)
	if err != nil {
		t.Fatalf("ResetUniverse: %v", err)
	}
	if resetID == "" {
		t.Fatal("resetID is empty")
	}

	// Verify new universe has different ID
	if resetID == firstID {
		t.Errorf("reset should produce a new universe ID, got same: %s", resetID)
	}

	// Verify Lyra's name is restored to original
	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM entities WHERE universe_id = $1 AND name = 'Lyra Vane'`, resetID).Scan(&name); err != nil {
		t.Fatalf("find Lyra after reset: %v", err)
	}
	if name != "Lyra Vane" {
		t.Errorf("entity name = %q, want 'Lyra Vane'", name)
	}

	// Verify contradiction count is restored (≥6)
	var contraCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM contradictions WHERE universe_id = $1`, resetID).Scan(&contraCount); err != nil {
		t.Fatalf("count contradictions after reset: %v", err)
	}
	if contraCount < 6 {
		t.Errorf("contradictions after reset = %d, want ≥6", contraCount)
	}
}

func TestCloneGraph(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "014")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()

	universeRepo := repositories.NewUniverseRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	svc := NewDemoService(pool, universeRepo, graphRepo)

	newID, err := svc.CloneUniverse(ctx, uuid.NewString())
	if err != nil {
		t.Fatalf("CloneUniverse: %v", err)
	}

	// Query the cloned graph
	newGraphName := "universe_" + newID
	nodes, edges, err := graphRepo.FullQuery(ctx, newGraphName)
	if err != nil {
		t.Fatalf("FullQuery cloned graph: %v", err)
	}

	// Should have ≥20 nodes (one per entity)
	if len(nodes) < 20 {
		t.Errorf("cloned graph nodes = %d, want ≥20", len(nodes))
	}

	// Should have edges
	if len(edges) < 1 {
		t.Errorf("cloned graph edges = %d, want ≥1", len(edges))
	}

	// Verify node IDs are all from the new universe (not template)
	var cloneEntityCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM entities WHERE universe_id = $1`, newID).Scan(&cloneEntityCount); err != nil {
		t.Fatalf("count clone entities: %v", err)
	}
	if cloneEntityCount < 20 {
		t.Errorf("clone entities = %d, want ≥20", cloneEntityCount)
	}
}

func TestMigrationIdempotency(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "014")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()

	// Count template entities after first migration run
	var count1 int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM entities WHERE universe_id = $1`, templateUniverseID).Scan(&count1); err != nil {
		t.Fatalf("count entities run 1: %v", err)
	}

	// Re-run migration 014 seed data directly
	sql, err := readMigrationFile("../migrations/014_seed_demo_saga.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("re-run migration 014: %v", err)
	}

	// Count again — should be same (idempotent)
	var count2 int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM entities WHERE universe_id = $1`, templateUniverseID).Scan(&count2); err != nil {
		t.Fatalf("count entities run 2: %v", err)
	}
	if count1 != count2 {
		t.Errorf("migration not idempotent: run1=%d run2=%d", count1, count2)
	}
}

func readMigrationFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}
