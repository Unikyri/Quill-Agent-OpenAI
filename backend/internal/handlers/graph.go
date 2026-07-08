package handlers

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

// graphQuerier is the subset of *repositories.GraphRepo used by GraphHandler.
// pontail: tiny interface for testability — no full repo abstraction needed.
type graphQuerier interface {
	FullQuery(ctx context.Context, graphName string) ([]repositories.GraphNode, []repositories.GraphEdge, error)
	NHopTraversal(ctx context.Context, graphName string, startNodeID string, hops int) ([]repositories.GraphNode, []repositories.GraphEdge, error)
}

// queryEmbedder is the subset of *services.QwenService used to embed a
// recall-explain query string. Local interface (mirrors the graphQuerier
// convention above) rather than importing ws.EmbeddingProvider, which would
// be a wrong-direction handlers→ws dependency.
type queryEmbedder interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// Decayer is the subset of *services.RelevanceService used to trigger a
// decay run. Local interface (mirrors the queryEmbedder convention above).
type Decayer interface {
	DecayAll(ctx context.Context, universeID uuid.UUID) error
}

// GraphHandler serves graph-related REST endpoints.
type GraphHandler struct {
	graphRepo  graphQuerier
	memorySvc  *services.MemoryService
	entityRepo *repositories.EntityRepo
	embedder   queryEmbedder
	decayer    Decayer
}

// NewGraphHandler creates a graph handler. embedder is nil-allowed: a nil
// embedder puts RecallExplain in degraded mode (query not embedded), the
// same nil-safe convention ws.Hub uses for its EmbeddingProvider — unlike
// graphRepo/memorySvc/entityRepo, it does not panic on nil.
func NewGraphHandler(graphRepo *repositories.GraphRepo, memorySvc *services.MemoryService, entityRepo *repositories.EntityRepo, embedder queryEmbedder) *GraphHandler {
	if graphRepo == nil {
		panic("graphRepo required")
	}
	if memorySvc == nil {
		panic("memorySvc required")
	}
	if entityRepo == nil {
		panic("entityRepo required")
	}
	return &GraphHandler{graphRepo: graphRepo, memorySvc: memorySvc, entityRepo: entityRepo, embedder: embedder}
}

// SetDecayer wires the decay trigger post-construction, mirroring the
// optional-setter convention (see queryEmbedder's nil-safe handling above)
// so the 4 positional NewGraphHandler call sites stay untouched.
func (h *GraphHandler) SetDecayer(d Decayer) {
	h.decayer = d
}

// FullGraph returns all nodes and edges for a universe's graph.
// GET /api/v1/universes/:universe_id/graph
func (h *GraphHandler) FullGraph(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("universe_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe_id"},
		})
	}

	graphName := "universe_" + universeID.String()
	nodes, edges, err := h.graphRepo.FullQuery(c.Context(), graphName)
	if err != nil {
		// ponytail: AGE throws "graph does not exist" for new universes; return empty 200
		if strings.Contains(err.Error(), "does not exist") {
			return c.JSON(fiber.Map{
				"nodes": []repositories.GraphNode{},
				"edges": []repositories.GraphEdge{},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	if nodes == nil {
		nodes = []repositories.GraphNode{}
	}
	if edges == nil {
		edges = []repositories.GraphEdge{}
	}

	return c.JSON(fiber.Map{
		"nodes": nodes,
		"edges": edges,
	})
}

// Neighbors returns the N-hop neighbors of a graph entity.
// GET /api/v1/entities/:id/neighbors?universe_id=X&hops=2
func (h *GraphHandler) Neighbors(c *fiber.Ctx) error {
	entityID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid entity ID"},
		})
	}

	universeID, err := uuid.Parse(c.Query("universe_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe_id"},
		})
	}

	hops := c.QueryInt("hops", 1)
	if hops < 1 {
		hops = 1
	}
	if hops > 5 {
		hops = 5 // ponytail: cap at 5 to avoid deep traversal
	}

	graphName := "universe_" + universeID.String()
	nodes, edges, err := h.graphRepo.NHopTraversal(c.Context(), graphName, entityID.String(), hops)
	if err != nil {
		// ponytail: AGE throws "graph does not exist" for new universes; return empty 200
		if strings.Contains(err.Error(), "does not exist") {
			return c.JSON(fiber.Map{
				"nodes": []repositories.GraphNode{},
				"edges": []repositories.GraphEdge{},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	if nodes == nil {
		nodes = []repositories.GraphNode{}
	}
	if edges == nil {
		edges = []repositories.GraphEdge{}
	}

	return c.JSON(fiber.Map{
		"nodes": nodes,
		"edges": edges,
	})
}

// Recall returns contextually-relevant entities via the memory service.
// POST /api/v1/universes/:id/recall
// Body: {"query": "text", "k": 5}
func (h *GraphHandler) Recall(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe ID"},
		})
	}

	var req struct {
		Query string `json:"query"`
		K     int    `json:"k"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
		})
	}

	if req.K <= 0 {
		req.K = 5
	}
	if req.K > 20 {
		req.K = 20
	}

	items, err := h.memorySvc.Recall(c.Context(), universeID, nil, req.K)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{
		"items": items,
	})
}

// RecallExplain returns the same fused recall results as Recall, but with a
// full per-pipeline RRF contribution ledger per item — unlike Recall, it
// embeds req.Query via the injected embedder before calling into the memory
// service, so the query is never silently ignored.
// POST /api/v1/universes/:id/recall/explain
// Body: {"query": "text", "k": 5}
func (h *GraphHandler) RecallExplain(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe ID"},
		})
	}

	var req struct {
		Query string `json:"query"`
		K     int    `json:"k"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
		})
	}

	if req.K <= 0 {
		req.K = 10
	}
	if req.K > 20 {
		req.K = 20
	}

	// Embed the query string before passing to RecallExplain (mirror
	// ws/hub.go's handleRecallRequest embedding step) — degraded mode only
	// when there's no embedder or an empty query, never silently on error.
	var embedding []float32
	if h.embedder != nil && req.Query != "" {
		embedding, err = h.embedder.GenerateEmbedding(c.Context(), req.Query)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "failed to embed query"},
			})
		}
	}

	explanation, err := h.memorySvc.RecallExplain(c.Context(), universeID, embedding, req.Query, req.K)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(explanation)
}

// MemoryStatus returns per-entity relevance history and derived lifecycle
// state for a universe, feeding the frontend's entity lifecycle sparkline.
// GET /api/v1/universes/:id/memory-status
func (h *GraphHandler) MemoryStatus(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe ID"},
		})
	}

	status, err := h.memorySvc.MemoryStatus(c.Context(), universeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(status)
}

// RunDecay triggers a decay run for a universe (normally fired on chapter
// advance via ChapterService; exposed here so the frontend can trigger it
// on demand for the memory-theater demo page).
// POST /api/v1/universes/:id/decay
func (h *GraphHandler) RunDecay(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe ID"},
		})
	}

	if h.decayer == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": fiber.Map{"code": "NOT_CONFIGURED", "message": "decay not available"},
		})
	}

	if err := h.decayer.DecayAll(c.Context(), universeID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{"ok": true})
}
