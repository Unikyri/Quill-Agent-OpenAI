package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

// GraphNode represents a node returned from graph queries.
type GraphNode struct {
	ID         string                 `json:"id"`
	Labels     []string               `json:"labels"`
	Properties map[string]interface{} `json:"properties"`
}

// GraphEdge represents an edge returned from graph queries.
type GraphEdge struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

type GraphRepo struct {
	pool *pgxpool.Pool
}

func NewGraphRepo(pool *pgxpool.Pool) *GraphRepo {
	return &GraphRepo{pool: pool}
}

// quoteGraph quotes a graph name for inline interpolation in cypher() calls.
// AGE's cypher() expects `name` type arg; pgx `$1` sends `text` → overload miss.
// Graph names are always "universe_" + UUID (internal), no injection risk.
func quoteGraph(name string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(name, "'", "''"))
}

// escapeCypherString escapes single quotes and backslashes for safe
// interpolation into AGE Cypher query strings. AGE's cypher() function
// doesn't support parameterized queries inside $$ blocks, so string
// escaping is the only option.
//
// ponytail: backslash first, then quote — avoids double-escaping.
func escapeCypherString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

// withAgeTx loads AGE + sets search_path on a transaction's connection,
// then runs fn. Avoids pool.Acquire inside a transaction — prevents deadlock
// when pool is saturated by concurrent requests.
func (r *GraphRepo) withAgeTx(tx pgx.Tx, fn func(conn *pgx.Conn) error) error {
	c := tx.Conn()
	if _, err := c.Exec(context.Background(), "LOAD 'age'"); err != nil {
		return fmt.Errorf("load age: %w", err)
	}
	if _, err := c.Exec(context.Background(), `SET search_path = ag_catalog, "$user", public`); err != nil {
		return fmt.Errorf("set search_path: %w", err)
	}
	return fn(c)
}

// withAgeConn acquires a dedicated connection, loads AGE + sets search_path,
// runs fn, then releases. This ensures AGE is available regardless of pool state.
// AfterConnect in pgxpool doesn't reliably persist LOAD across all connections.
func (r *GraphRepo) withAgeConn(ctx context.Context, fn func(conn *pgx.Conn) error) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()
	c := conn.Conn()
	if _, err := c.Exec(ctx, "LOAD 'age'"); err != nil {
		return fmt.Errorf("load age: %w", err)
	}
	if _, err := c.Exec(ctx, `SET search_path = ag_catalog, "$user", public`); err != nil {
		return fmt.Errorf("set search_path: %w", err)
	}
	return fn(c)
}

func (r *GraphRepo) CreateGraph(ctx context.Context, universeID string) error {
	graphName := "universe_" + universeID
	return r.withAgeConn(ctx, func(c *pgx.Conn) error {
		// AGE requires create_graph() before running Cypher against the graph.
		_, err := c.Exec(ctx, fmt.Sprintf(`SELECT create_graph('%s')`, graphName))
		return err
	})
}

func (r *GraphRepo) CreateNode(ctx context.Context, graphName, label string, properties map[string]interface{}) error {
	return r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ CREATE (n:%s {entity_id: '%s', name: '%s', status: '%s', relevance_score: %v}) RETURN n $$) AS (n agtype)`,
			quoteGraph(graphName), label,
			escapeCypherString(fmt.Sprint(properties["entity_id"])),
			escapeCypherString(fmt.Sprint(properties["name"])),
			escapeCypherString(fmt.Sprint(properties["status"])),
			properties["relevance_score"])
		_, err := c.Exec(ctx, query)
		return err
	})
}

func (r *GraphRepo) CreateEdge(ctx context.Context, graphName, sourceEntityID, targetEntityID, relType string, properties map[string]interface{}) error {
	return r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (x {entity_id: '%s'}), (y {entity_id: '%s'}) CREATE (x)-[:%s]->(y) $$) AS (r agtype)`,
			quoteGraph(graphName),
			escapeCypherString(sourceEntityID),
			escapeCypherString(targetEntityID),
			relType)
		_, err := c.Exec(ctx, query)
		return err
	})
}

// Tx variants: use the transaction's connection instead of acquiring from pool.
// ponytail: identical cypher bodies to non-Tx originals, just different conn source.

func (r *GraphRepo) CreateGraphTx(ctx context.Context, tx pgx.Tx, universeID string) error {
	graphName := "universe_" + universeID
	return r.withAgeTx(tx, func(c *pgx.Conn) error {
		// AGE requires create_graph() before running Cypher against the graph
		// (same requirement as the non-Tx CreateGraph above).
		if _, err := c.Exec(ctx, fmt.Sprintf(`SELECT create_graph('%s')`, graphName)); err != nil {
			return err
		}
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ CREATE (g:Graph {name: '%s'}) RETURN g $$) AS (g agtype)`,
			quoteGraph(graphName), graphName)
		_, err := c.Exec(ctx, query)
		return err
	})
}

func (r *GraphRepo) CreateNodeTx(ctx context.Context, tx pgx.Tx, graphName, label string, properties map[string]interface{}) error {
	return r.withAgeTx(tx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ CREATE (n:%s {entity_id: '%s', name: '%s', status: '%s', relevance_score: %v}) RETURN n $$) AS (n agtype)`,
			quoteGraph(graphName), label,
			escapeCypherString(fmt.Sprint(properties["entity_id"])),
			escapeCypherString(fmt.Sprint(properties["name"])),
			escapeCypherString(fmt.Sprint(properties["status"])),
			properties["relevance_score"])
		_, err := c.Exec(ctx, query)
		return err
	})
}

func (r *GraphRepo) CreateEdgeTx(ctx context.Context, tx pgx.Tx, graphName, sourceEntityID, targetEntityID, relType string, properties map[string]interface{}) error {
	return r.withAgeTx(tx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (x {entity_id: '%s'}), (y {entity_id: '%s'}) CREATE (x)-[:%s]->(y) $$) AS (r agtype)`,
			quoteGraph(graphName),
			escapeCypherString(sourceEntityID),
			escapeCypherString(targetEntityID),
			relType)
		_, err := c.Exec(ctx, query)
		return err
	})
}

func (r *GraphRepo) UpdateNodeRelevance(ctx context.Context, graphName, entityID string, score float64) error {
	return r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (n {entity_id: '%s'}) SET n.relevance_score = %v RETURN n $$) AS (n agtype)`,
			quoteGraph(graphName), escapeCypherString(entityID), score)
		_, err := c.Exec(ctx, query)
		return err
	})
}

func (r *GraphRepo) GetNeighbors(ctx context.Context, graphName, entityID string) ([]models.GraphNeighbor, error) {
	var neighbors []models.GraphNeighbor
	err := r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (n {entity_id: '%s'})-[r]-(m) RETURN type(r) AS rel_type, properties(r) AS rel_props, m $$) AS (rel_type agtype, rel_props agtype, m agtype)`,
			quoteGraph(graphName), escapeCypherString(entityID))
		rows, err := c.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("get neighbors: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var n models.GraphNeighbor
			if err := rows.Scan(&n.RelType, &n.RelProps, &n.Node); err != nil {
				return fmt.Errorf("scan neighbor: %w", err)
			}
			neighbors = append(neighbors, n)
		}
		return nil
	})
	return neighbors, err
}

// GetNeighborsBatch resolves 1-hop neighbors for ALL given seed entity IDs in
// a single Cypher call (spec: "Graph Pipeline Uses Batched Neighbor Lookup"),
// instead of issuing one GetNeighbors call per seed. Matches n.entity_id
// against the seed list via a Cypher IN clause, keeping the seed's entity_id
// in the RETURN so rows can be grouped back into a per-seed map.
func (r *GraphRepo) GetNeighborsBatch(ctx context.Context, graphName string, entityIDs []string) (map[string][]models.GraphNeighbor, error) {
	result := make(map[string][]models.GraphNeighbor)
	if len(entityIDs) == 0 {
		return result, nil
	}

	quoted := make([]string, len(entityIDs))
	for i, id := range entityIDs {
		quoted[i] = fmt.Sprintf("'%s'", escapeCypherString(id))
	}
	idList := strings.Join(quoted, ", ")

	err := r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (n)-[r]-(m) WHERE n.entity_id IN [%s] RETURN n.entity_id AS seed_id, type(r) AS rel_type, properties(r) AS rel_props, m $$) AS (seed_id agtype, rel_type agtype, rel_props agtype, m agtype)`,
			quoteGraph(graphName), idList)
		rows, err := c.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("get neighbors batch: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var seedID string
			var n models.GraphNeighbor
			if err := rows.Scan(&seedID, &n.RelType, &n.RelProps, &n.Node); err != nil {
				return fmt.Errorf("scan neighbor batch: %w", err)
			}
			seedID = strings.Trim(seedID, `"`)
			result[seedID] = append(result[seedID], n)
		}
		return nil
	})
	return result, err
}

// FullQuery returns all nodes and edges for a universe's graph.
func (r *GraphRepo) FullQuery(ctx context.Context, graphName string) ([]GraphNode, []GraphEdge, error) {
	var nodes []GraphNode
	var edges []GraphEdge
	err := r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (n) OPTIONAL MATCH (n)-[r]->(m) RETURN n, r, m $$) AS (n agtype, r agtype, m agtype)`,
			quoteGraph(graphName))
		rows, err := c.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("full query: %w", err)
		}
		defer rows.Close()
		nodes, edges, err = collectGraphRows(rows)
		return err
	})
	return nodes, edges, err
}

// DeleteEdge removes a relationship between two nodes in the graph.
func (r *GraphRepo) DeleteEdge(ctx context.Context, graphName, sourceEntityID, targetEntityID, relType string) error {
	return r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (x {entity_id: '%s'})-[r:%s]->(y {entity_id: '%s'}) DELETE r $$) AS (a agtype)`,
			quoteGraph(graphName), escapeCypherString(sourceEntityID), relType, escapeCypherString(targetEntityID))
		_, err := c.Exec(ctx, query)
		return err
	})
}

// NHopTraversal performs a BFS traversal from a start node up to `hops` depth.
func (r *GraphRepo) NHopTraversal(ctx context.Context, graphName, startEntityID string, hops int) ([]GraphNode, []GraphEdge, error) {
	var nodes []GraphNode
	var edges []GraphEdge
	err := r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (n {entity_id: '%s'})-[r*1..%d]-(m) RETURN n, r, m $$) AS (n agtype, r agtype, m agtype)`,
			quoteGraph(graphName), escapeCypherString(startEntityID), hops)
		rows, err := c.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("n-hop traversal: %w", err)
		}
		defer rows.Close()
		nodes, edges, err = collectGraphRows(rows)
		return err
	})
	return nodes, edges, err
}

// DropGraph deletes all nodes and edges in a graph.
func (r *GraphRepo) DropGraph(ctx context.Context, graphName string) error {
	return r.withAgeConn(ctx, func(c *pgx.Conn) error {
		query := fmt.Sprintf(`SELECT * FROM cypher(%s, $$ MATCH (n) DETACH DELETE n $$) AS (a agtype)`,
			quoteGraph(graphName))
		_, err := c.Exec(ctx, query)
		return err
	})
}

// collectGraphRows extracts nodes and edges from AGE cypher result rows.
func collectGraphRows(rows pgx.Rows) ([]GraphNode, []GraphEdge, error) {
	nodeMap := make(map[string]GraphNode)
	edgeMap := make(map[string]GraphEdge)

	for rows.Next() {
		var nStr, rStr, mStr *string
		if err := rows.Scan(&nStr, &rStr, &mStr); err != nil {
			return nil, nil, fmt.Errorf("scan row: %w", err)
		}
		if nStr != nil {
			id := extractProp(*nStr, "entity_id")
			if id != "" {
				if _, exists := nodeMap[id]; !exists {
					nodeMap[id] = GraphNode{ID: id, Properties: map[string]interface{}{"raw": *nStr}}
				}
			}
		}
		if mStr != nil {
			id := extractProp(*mStr, "entity_id")
			if id != "" {
				if _, exists := nodeMap[id]; !exists {
					nodeMap[id] = GraphNode{ID: id, Properties: map[string]interface{}{"raw": *mStr}}
				}
			}
		}
		if rStr != nil {
			key := *rStr
			if _, exists := edgeMap[key]; !exists {
				edgeMap[key] = GraphEdge{ID: key, Type: "relationship", Properties: map[string]interface{}{"raw": *rStr}}
			}
		}
	}

	nodes := make([]GraphNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}
	edges := make([]GraphEdge, 0, len(edgeMap))
	for _, e := range edgeMap {
		edges = append(edges, e)
	}
	return nodes, edges, nil
}

// extractProp pulls a value from a raw agtype string.
func extractProp(agtypeStr, key string) string {
	search := fmt.Sprintf(`"%s": "`, key)
	idx := strings.Index(agtypeStr, search)
	if idx < 0 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(agtypeStr[start:], `"`)
	if end < 0 {
		return ""
	}
	return agtypeStr[start : start+end]
}
