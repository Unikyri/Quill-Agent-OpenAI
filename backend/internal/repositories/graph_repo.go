package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
)

type GraphRepo struct {
	pool *pgxpool.Pool
}

func NewGraphRepo(pool *pgxpool.Pool) *GraphRepo {
	return &GraphRepo{pool: pool}
}

func (r *GraphRepo) CreateGraph(ctx context.Context, universeID string) error {
	graphName := "universe_" + universeID
	query := `SELECT * FROM cypher($1, $$ CREATE (g:Graph {name: $2}) RETURN g $$) AS (g agtype)`
	_, err := r.pool.Exec(ctx, query, graphName, graphName)
	if err != nil {
		return fmt.Errorf("create graph: %w", err)
	}
	return nil
}

func (r *GraphRepo) CreateNode(ctx context.Context, graphName, label string, properties map[string]interface{}) error {
	query := fmt.Sprintf(`SELECT * FROM cypher($1, $$ CREATE (n:%s {entity_id: '%s', name: '%s', status: '%s', relevance_score: %v}) RETURN n $$) AS (n agtype)`,
		label,
		properties["entity_id"],
		properties["name"],
		properties["status"],
		properties["relevance_score"],
	)
	_, err := r.pool.Exec(ctx, query, graphName)
	if err != nil {
		return fmt.Errorf("create graph node: %w", err)
	}
	return nil
}

func (r *GraphRepo) CreateEdge(ctx context.Context, graphName, sourceEntityID, targetEntityID, relType string, properties map[string]interface{}) error {
	query := fmt.Sprintf(`SELECT * FROM cypher($1, $$ MATCH (x {entity_id: '%s'}), (y {entity_id: '%s'}) CREATE (x)-[:%s]->(y) $$) AS (r agtype)`,
		sourceEntityID, targetEntityID, relType)
	_, err := r.pool.Exec(ctx, query, graphName)
	if err != nil {
		return fmt.Errorf("create graph edge: %w", err)
	}
	return nil
}

func (r *GraphRepo) UpdateNodeRelevance(ctx context.Context, graphName, entityID string, score float64) error {
	query := fmt.Sprintf(`SELECT * FROM cypher($1, $$ MATCH (n {entity_id: '%s'}) SET n.relevance_score = %v RETURN n $$) AS (n agtype)`,
		entityID, score)
	_, err := r.pool.Exec(ctx, query, graphName)
	if err != nil {
		return fmt.Errorf("update node relevance: %w", err)
	}
	return nil
}

func (r *GraphRepo) GetNeighbors(ctx context.Context, graphName, entityID string) ([]models.GraphNeighbor, error) {
	query := fmt.Sprintf(`SELECT * FROM cypher($1, $$ MATCH (n {entity_id: '%s'})-[r]-(m) RETURN type(r) AS rel_type, properties(r) AS rel_props, m $$) AS (rel_type agtype, rel_props agtype, m agtype)`,
		entityID)
	rows, err := r.pool.Query(ctx, query, graphName)
	if err != nil {
		return nil, fmt.Errorf("get neighbors: %w", err)
	}
	defer rows.Close()

	var neighbors []models.GraphNeighbor
	for rows.Next() {
		var n models.GraphNeighbor
		if err := rows.Scan(&n.RelType, &n.RelProps, &n.Node); err != nil {
			return nil, fmt.Errorf("scan neighbor: %w", err)
		}
		neighbors = append(neighbors, n)
	}
	return neighbors, nil
}

func (r *GraphRepo) GetFullGraph(ctx context.Context, graphName string) (string, error) {
	query := fmt.Sprintf(`SELECT * FROM cypher($1, $$ MATCH (n) OPTIONAL MATCH (n)-[r]->(m) RETURN n, r, m $$) AS (n agtype, r agtype, m agtype)`)
	rows, err := r.pool.Query(ctx, query, graphName)
	if err != nil {
		return "", fmt.Errorf("get full graph: %w", err)
	}
	defer rows.Close()
	return "graph data", nil
}

func (r *GraphRepo) DropGraph(ctx context.Context, graphName string) error {
	query := `SELECT * FROM cypher($1, $$ MATCH (n) DETACH DELETE n $$) AS (a agtype)`
	_, err := r.pool.Exec(ctx, query, graphName)
	if err != nil {
		return fmt.Errorf("drop graph: %w", err)
	}
	return nil
}
