package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/middleware"
	"github.com/quill/backend/internal/services"
)

type SkillHandler struct {
	registry    *services.SkillRegistry
	universeSvc *services.UniverseService
}

func NewSkillHandler(registry *services.SkillRegistry, universeSvc *services.UniverseService) *SkillHandler {
	return &SkillHandler{registry: registry, universeSvc: universeSvc}
}

func (h *SkillHandler) Catalogue(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"skills": h.registry.Catalogue()})
}

func (h *SkillHandler) ListUniverse(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return skillError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Invalid universe ID")
	}
	activations, err := h.universeSvc.ListSkills(c.Context(), middleware.GetUserID(c), universeID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(fiber.Map{"skills": activations})
}

func (h *SkillHandler) ReplaceUniverse(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return skillError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Invalid universe ID")
	}
	var body struct {
		SkillNames   []string `json:"skill_names"`
		ActiveSkills []string `json:"active_skills"`
		Skills       []string `json:"skills"`
	}
	if err := c.BodyParser(&body); err != nil {
		return skillError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
	}
	names := body.SkillNames
	if names == nil {
		names = body.ActiveSkills
	}
	if names == nil {
		names = body.Skills
	}
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	activations, err := h.universeSvc.ReplaceSkills(c.Context(), middleware.GetUserID(c), universeID, names)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(fiber.Map{"skills": activations})
}

func (h *SkillHandler) handleServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrUniverseAccessDenied):
		return skillError(c, fiber.StatusForbidden, "FORBIDDEN", "Universe access denied")
	case errors.Is(err, services.ErrUnknownSkill):
		return skillError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case strings.Contains(err.Error(), "universe not found"):
		return skillError(c, fiber.StatusNotFound, "NOT_FOUND", "Universe not found")
	default:
		return skillError(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func skillError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}
