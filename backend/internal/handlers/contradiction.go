package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

// ContradictionHandler serves contradiction-related REST endpoints.
type ContradictionHandler struct {
	contraSvc    *services.ContradictionService
	contradictionRepo *repositories.ContradictionRepo
}

// NewContradictionHandler creates a contradiction handler.
func NewContradictionHandler(contraSvc *services.ContradictionService, contradictionRepo *repositories.ContradictionRepo) *ContradictionHandler {
	return &ContradictionHandler{contraSvc: contraSvc, contradictionRepo: contradictionRepo}
}

// ListByUniverse returns all contradictions for a universe.
// GET /api/v1/universes/:universe_id/contradictions
func (h *ContradictionHandler) ListByUniverse(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("universe_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe_id"},
		})
	}

	if h.contradictionRepo == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "ContradictionRepo not initialized"},
		})
	}

	contradictions, err := h.contradictionRepo.ListByUniverse(c.Context(), universeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{
		"contradictions": contradictions,
	})
}

// Dismiss marks a contradiction as dismissed without resolving.
// PUT /api/v1/universes/:universe_id/contradictions/:id/dismiss
func (h *ContradictionHandler) Dismiss(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid contradiction ID"},
		})
	}

	if h.contradictionRepo == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "ContradictionRepo not initialized"},
		})
	}

	if err := h.contradictionRepo.Dismiss(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{"status": "dismissed"})
}

// Resolve marks a contradiction as resolved.
// PUT /api/v1/universes/:universe_id/contradictions/:id/resolve
func (h *ContradictionHandler) Resolve(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid contradiction ID"},
		})
	}

	if h.contradictionRepo == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "ContradictionRepo not initialized"},
		})
	}

	now := time.Now()
	if err := h.contradictionRepo.Resolve(c.Context(), id, &now); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{"status": "resolved"})
}
