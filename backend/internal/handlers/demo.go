package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/services"
)

type DemoHandler struct {
	demoSvc *services.DemoService
}

func NewDemoHandler(demoSvc *services.DemoService) *DemoHandler {
	return &DemoHandler{demoSvc: demoSvc}
}

func (h *DemoHandler) Clone(c *fiber.Ctx) error {
	sessionID := c.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	universeID, err := h.demoSvc.CloneUniverse(c.Context(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"universe_id": universeID,
		"message":     "Demo universe cloned successfully",
	})
}

func (h *DemoHandler) Reset(c *fiber.Ctx) error {
	sessionID := c.Get("X-Session-ID")
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "X-Session-ID header required"},
		})
	}

	universeID, err := h.demoSvc.ResetUniverse(c.Context(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"universe_id": universeID,
		"message":     "Demo data reset successfully",
	})
}
