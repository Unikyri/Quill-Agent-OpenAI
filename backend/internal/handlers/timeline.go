package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

// TimelineHandler serves timeline-related REST endpoints.
type TimelineHandler struct {
	timelineSvc  *services.TimelineService
	timelineRepo *repositories.TimelineRepo
	ownerRepo    universeOwnerResolver
}

// SetUniverseOwnerRepo enables production ownership checks without changing
// the existing constructor contract.
func (h *TimelineHandler) SetUniverseOwnerRepo(repo universeOwnerResolver) {
	h.ownerRepo = repo
}

// NewTimelineHandler creates a timeline handler.
func NewTimelineHandler(timelineSvc *services.TimelineService, timelineRepo *repositories.TimelineRepo) *TimelineHandler {
	if timelineRepo == nil {
		panic("timelineRepo required")
	}
	return &TimelineHandler{timelineSvc: timelineSvc, timelineRepo: timelineRepo}
}

// ListByUniverse returns all timeline events for a universe.
// GET /api/v1/universes/:universe_id/timeline
func (h *TimelineHandler) ListByUniverse(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("universe_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe_id"},
		})
	}
	if err := authorizeUniverse(c, h.ownerRepo, universeID); err != nil {
		return universeAccessError(c, err)
	}

	events, err := h.timelineRepo.ListByUniverse(c.Context(), universeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{
		"events": events,
	})
}

// Create creates a new timeline event.
// POST /api/v1/universes/:universe_id/timeline
func (h *TimelineHandler) Create(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("universe_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe_id"},
		})
	}
	if err := authorizeUniverse(c, h.ownerRepo, universeID); err != nil {
		return universeAccessError(c, err)
	}

	var req models.TimelineEvent
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
		})
	}

	req.UniverseID = universeID

	req.ID = uuid.New()

	// Advisory only — a chronologically-suspicious event is still the
	// writer's call to make, so a validation finding is surfaced alongside
	// the created event rather than blocking creation (matching how
	// contradictions/plot-holes are shown as alerts, not hard errors,
	// elsewhere in the app).
	var timelineWarning string
	if h.timelineSvc != nil {
		if err := h.timelineSvc.ValidateNewEvent(c.Context(), req); err != nil {
			timelineWarning = err.Error()
		}
	}

	if err := h.timelineRepo.Create(c.Context(), &req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	response := fiber.Map{"event": req}
	if timelineWarning != "" {
		response["timeline_warning"] = timelineWarning
	}
	return c.Status(fiber.StatusCreated).JSON(response)
}
