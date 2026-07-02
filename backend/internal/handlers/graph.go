package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

// GraphHandler serves graph-related REST endpoints.
type GraphHandler struct {
	graphRepo   *repositories.GraphRepo
	memorySvc   *services.MemoryService
	entityRepo  *repositories.EntityRepo
}

// NewGraphHandler creates a graph handler.
func NewGraphHandler(graphRepo *repositories.GraphRepo, memorySvc *services.MemoryService, entityRepo *repositories.EntityRepo) *GraphHandler {
	return &GraphHandler{graphRepo: graphRepo, memorySvc: memorySvc, entityRepo: entityRepo}
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

	if h.graphRepo == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "GraphRepo not initialized"},
		})
	}

	graphName := "universe_" + universeID.String()
	nodes, edges, err := h.graphRepo.FullQuery(c.Context(), graphName)
	if err != nil {
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

	if h.graphRepo == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "GraphRepo not initialized"},
		})
	}

	graphName := "universe_" + universeID.String()
	nodes, edges, err := h.graphRepo.NHopTraversal(c.Context(), graphName, entityID.String(), hops)
	if err != nil {
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

	if h.memorySvc == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "MemoryService not initialized"},
		})
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


