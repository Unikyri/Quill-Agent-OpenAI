package repositories

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/testutil"
)

func setupGraphTest(t *testing.T, pool *pgxpool.Pool) (string, string, string) {
	t.Helper()
	ctx := context.Background()
	repo := NewGraphRepo(pool)

	graphName := "universe_" + uuid.NewString()
	if err := repo.CreateGraph(ctx, graphName); err != nil {
		t.Fatalf("CreateGraph: %v", err)
	}

	e1 := uuid.NewString()
	e2 := uuid.NewString()

	// create nodes
	if err := repo.CreateNode(ctx, graphName, "Character", map[string]interface{}{
		"entity_id":       e1,
		"name":            "Alice",
		"status":          "active",
		"relevance_score": 0.8,
	}); err != nil {
		t.Fatalf("create node 1: %v", err)
	}
	if err := repo.CreateNode(ctx, graphName, "Character", map[string]interface{}{
		"entity_id":       e2,
		"name":            "Bob",
		"status":          "active",
		"relevance_score": 0.5,
	}); err != nil {
		t.Fatalf("create node 2: %v", err)
	}

	// create edge
	if err := repo.CreateEdge(ctx, graphName, e1, e2, "ALLY_OF", nil); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	return graphName, e1, e2
}

// TestGraphRepoNHopTraversal verifies traversal up to N hops.
func TestGraphRepoNHopTraversal(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	graphName, e1, _ := setupGraphTest(t, pool)

	ctx := context.Background()
	repo := NewGraphRepo(pool)

	nodes, edges, err := repo.NHopTraversal(ctx, graphName, e1, 2)
	if err != nil {
		t.Fatalf("NHopTraversal: %v", err)
	}

	if len(nodes) < 2 {
		t.Errorf("NHopTraversal should return at least 2 nodes (start + neighbor), got %d", len(nodes))
	}
	if len(edges) < 1 {
		t.Errorf("NHopTraversal should return at least 1 edge, got %d", len(edges))
	}
}

// TestGraphRepoDeleteEdge removes an edge between two nodes.
func TestGraphRepoDeleteEdge(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	graphName, e1, e2 := setupGraphTest(t, pool)

	ctx := context.Background()
	repo := NewGraphRepo(pool)

	if err := repo.DeleteEdge(ctx, graphName, e1, e2, "ALLY_OF"); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}

	// Traverse again — edge should be gone
	_, edges, err := repo.NHopTraversal(ctx, graphName, e1, 1)
	if err != nil {
		t.Fatalf("NHopTraversal after delete: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges after DeleteEdge, got %d", len(edges))
	}
}

// TestGraphRepoUpdateEdge modifies an edge's relationship type.
func TestGraphRepoUpdateEdge(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	graphName, e1, e2 := setupGraphTest(t, pool)

	ctx := context.Background()
	repo := NewGraphRepo(pool)

	// The existing CreateEdge created ALLY_OF. Delete it and recreate as ENEMY_OF.
	// ponytail: update an edge = delete old + create new; AGE doesn't have SET on edges easily
	if err := repo.DeleteEdge(ctx, graphName, e1, e2, "ALLY_OF"); err != nil {
		t.Fatalf("delete old edge: %v", err)
	}
	if err := repo.CreateEdge(ctx, graphName, e1, e2, "ENEMY_OF", nil); err != nil {
		t.Fatalf("create new edge: %v", err)
	}

	_, edges, err := repo.NHopTraversal(ctx, graphName, e1, 1)
	if err != nil {
		t.Fatalf("traverse after update: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge after update, got %d", len(edges))
	}
}

// TestGraphRepoFullQuery returns structured graph data.
func TestGraphRepoFullQuery(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	graphName, _, _ := setupGraphTest(t, pool)

	ctx := context.Background()
	repo := NewGraphRepo(pool)

	nodes, edges, err := repo.FullQuery(ctx, graphName)
	if err != nil {
		t.Fatalf("FullQuery: %v", err)
	}

	if len(nodes) < 2 {
		t.Errorf("FullQuery should return at least 2 nodes, got %d", len(nodes))
	}
	if len(edges) < 1 {
		t.Errorf("FullQuery should return at least 1 edge, got %d", len(edges))
	}

	// Verify nodes have data
	for _, n := range nodes {
		if n.ID == "" {
			t.Error("node.ID should not be empty")
		}
	}

	// Verify edges have data
	for _, e := range edges {
		if e.Source == "" || e.Target == "" {
			t.Error("edge Source and Target should not be empty")
		}
	}
}

// TestGraphRepoCompose tests creating two edges between the same pair to verify compose works.
func TestGraphRepoEdgeCRUD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "011")
	if !testutil.CheckAGE(t, pool) {
		t.Skip("Apache AGE extension not available; skipping graph-dependent test")
	}
	ctx := context.Background()
	repo := NewGraphRepo(pool)
	graphName := "universe_" + uuid.NewString()
	repo.CreateGraph(ctx, graphName)

	e1 := uuid.NewString()
	e2 := uuid.NewString()

	// Create nodes
	repo.CreateNode(ctx, graphName, "Character", map[string]interface{}{"entity_id": e1, "name": "X", "status": "active", "relevance_score": 0.5})
	repo.CreateNode(ctx, graphName, "Character", map[string]interface{}{"entity_id": e2, "name": "Y", "status": "active", "relevance_score": 0.5})

	// Create edge
	if err := repo.CreateEdge(ctx, graphName, e1, e2, "KNOWS", map[string]interface{}{"since": "ch1"}); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	// Delete it
	if err := repo.DeleteEdge(ctx, graphName, e1, e2, "KNOWS"); err != nil {
		t.Fatalf("delete edge: %v", err)
	}

	// Verify deletion
	_, edges, _ := repo.NHopTraversal(ctx, graphName, e1, 1)
	if len(edges) != 0 {
		t.Errorf("expected 0 edges after delete, got %d", len(edges))
	}

	// Recreate with different type
	if err := repo.CreateEdge(ctx, graphName, e1, e2, "ENEMY_OF", nil); err != nil {
		t.Fatalf("recreate edge: %v", err)
	}

	_, edges2, _ := repo.NHopTraversal(ctx, graphName, e1, 1)
	if len(edges2) != 1 {
		t.Errorf("expected 1 edge after recreate, got %d", len(edges2))
	}
}

// TestEscapeCypherString verifies that the escapeCypherString helper
// correctly escapes single quotes and backslashes for safe Cypher interpolation.
//
// RED: escapeCypherString does not exist yet — compilation will fail until
// the production code is added.
func TestEscapeCypherString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single_quote", "O'Brien", "O\\'Brien"},
		{"backslash", "path\\to\\file", "path\\\\to\\\\file"},
		{"normal", "Alice", "Alice"},
		{"empty", "", ""},
		{"mixed", "O'Brien\\Jr", "O\\'Brien\\\\Jr"},
		{"only_quote", "'", "\\'"},
		{"only_backslash", "\\", "\\\\"},
		{"already_escaped", "already\\'safe", "already\\\\\\'safe"}, // ponytail: double-escape is harmless — better safe than injection
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeCypherString(tt.input)
			if got != tt.want {
				t.Errorf("escapeCypherString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
