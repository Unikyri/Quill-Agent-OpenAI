package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// ── QuillExecutor dependency interfaces ──────────────────────────────
// ponytail: interfaces per method the executor actually needs so tests
// can mock without real DB connections.

// vectorSearcher abstracts the vector-store operations QuillExecutor needs.
type vectorSearcher interface {
	FindSimilarParagraphs(ctx context.Context, universeID uuid.UUID, embedding []float32, excludeChapterID uuid.UUID, limit int) ([]repositories.SimilarParagraph, error)
}

// graphQuerier abstracts the graph-store operations QuillExecutor needs.
type graphQuerier interface {
	GetNeighbors(ctx context.Context, graphName, entityID string) ([]models.GraphNeighbor, error)
}

// entityLister abstracts entity-listing operations QuillExecutor needs.
type entityLister interface {
	ListByUniverseActive(ctx context.Context, universeID uuid.UUID) ([]models.Entity, error)
}

// QuillExecutor implements ToolExecutor with vector memory and entity graph
// tools. It dispatches tool calls by name to the appropriate handler.
//
// ponytail: switch dispatch — two tools, no registry overhead.
type QuillExecutor struct {
	VectorRepo   vectorSearcher
	GraphRepo    graphQuerier
	EntityRepo   entityLister
	MemorySvc    *MemoryService
	QwenSvc      *QwenService
	UniverseID   uuid.UUID
}

// ExecuteTool dispatches by tool name to the registered handler.
// Unknown tool names return an error.
func (e *QuillExecutor) ExecuteTool(name string, argsJSON string) (string, error) {
	switch name {
	case "search_vector_memory":
		return e.searchVectorMemory(argsJSON)
	case "query_entity_graph":
		return e.queryEntityGraph(argsJSON)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// searchVectorMemory generates an embedding for the query, finds similar
// paragraphs in the vector store, and returns formatted results.
func (e *QuillExecutor) searchVectorMemory(argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("search_vector_memory: invalid args: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("search_vector_memory: query is required")
	}

	ctx := context.Background()

	embedding, err := e.QwenSvc.GenerateEmbedding(ctx, args.Query)
	if err != nil {
		return "", fmt.Errorf("search_vector_memory: embed: %w", err)
	}

	paragraphs, err := e.VectorRepo.FindSimilarParagraphs(ctx, e.UniverseID, embedding, uuid.Nil, 5)
	if err != nil {
		return "", fmt.Errorf("search_vector_memory: find: %w", err)
	}

	if len(paragraphs) == 0 {
		return "No matching paragraphs found.", nil
	}

	var result string
	for i, p := range paragraphs {
		result += fmt.Sprintf("%d. [%s] %s (score: %.2f)\n", i+1, p.ChapterTitle, p.Content, 1.0-p.Distance)
	}
	return result, nil
}

// queryEntityGraph resolves an entity by name, fetches its graph neighbors,
// and returns a formatted summary.
//
// ponytail: entity resolution happens via EntityRepo.ListByUniverseActive
// then name matching. Graph traversal via GetNeighbors.
func (e *QuillExecutor) queryEntityGraph(argsJSON string) (string, error) {
	var args struct {
		EntityName string `json:"entity_name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("query_entity_graph: invalid args: %w", err)
	}
	if args.EntityName == "" {
		return "", fmt.Errorf("query_entity_graph: entity_name is required")
	}

	ctx := context.Background()

	// Resolve entity name to ID
	entities, err := e.EntityRepo.ListByUniverseActive(ctx, e.UniverseID)
	if err != nil {
		return "", fmt.Errorf("query_entity_graph: list entities: %w", err)
	}

	var entityID string
	for _, ent := range entities {
		if ent.Name == args.EntityName {
			entityID = ent.ID.String()
			break
		}
	}
	if entityID == "" {
		return fmt.Sprintf("Entity '%s' not found.", args.EntityName), nil
	}

	graphName := "universe_" + e.UniverseID.String()
	neighbors, err := e.GraphRepo.GetNeighbors(ctx, graphName, entityID)
	if err != nil {
		return "", fmt.Errorf("query_entity_graph: get neighbors: %w", err)
	}

	if len(neighbors) == 0 {
		return fmt.Sprintf("Entity '%s' has no graph connections.", args.EntityName), nil
	}

	var result string
	result += fmt.Sprintf("Neighbors of '%s':\n", args.EntityName)
	for _, n := range neighbors {
		result += fmt.Sprintf("- %s (relation: %s)\n", fmt.Sprintf("%v", n.Node), n.RelType)
	}
	return result, nil
}