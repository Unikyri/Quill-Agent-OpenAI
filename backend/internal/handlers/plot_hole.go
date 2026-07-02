package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

// PlotHoleHandler serves plot-hole-related REST endpoints.
type PlotHoleHandler struct {
	plotHoleSvc  *services.PlotHoleService
	plotHoleRepo *repositories.PlotHoleRepo
}

// NewPlotHoleHandler creates a plot hole handler.
// If plotHoleRepo is nil, listing falls back to the service's internal repo.
func NewPlotHoleHandler(plotHoleSvc *services.PlotHoleService) *PlotHoleHandler {
	return &PlotHoleHandler{plotHoleSvc: plotHoleSvc}
}

// WithRepo sets the PlotHoleRepo for listing operations.
func (h *PlotHoleHandler) WithRepo(repo *repositories.PlotHoleRepo) *PlotHoleHandler {
	h.plotHoleRepo = repo
	return h
}

// ListByUniverse returns all plot holes for a universe.
// GET /api/v1/universes/:universe_id/plot-holes
func (h *PlotHoleHandler) ListByUniverse(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("universe_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe_id"},
		})
	}

	if h.plotHoleRepo == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "PlotHoleRepo not initialized"},
		})
	}

	holes, err := h.plotHoleRepo.ListByUniverse(c.Context(), universeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	if holes == nil {
		holes = []models.PlotHole{}
	}

	return c.JSON(fiber.Map{
		"plot_holes": holes,
	})
}
