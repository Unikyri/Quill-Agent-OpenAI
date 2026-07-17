package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/middleware"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/services"
)

// EntityCandidateHandler is the authenticated review-tray surface. Candidate
// records are entities scoped by their owning universe; no client-provided
// universe ID is trusted for candidate mutations.
type EntityCandidateHandler struct {
	entities *services.EntityService
	owner    interface {
		FindByID(context.Context, uuid.UUID) (*models.Universe, error)
	}
	writer *services.WriterMemoryService
}

func NewEntityCandidateHandler(entities *services.EntityService, owner interface {
	FindByID(context.Context, uuid.UUID) (*models.Universe, error)
}, writer *services.WriterMemoryService) *EntityCandidateHandler {
	return &EntityCandidateHandler{entities: entities, owner: owner, writer: writer}
}

func (h *EntityCandidateHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return candidateUnauthorized(c)
	}
	universeID, err := uuid.Parse(c.Params("universe_id"))
	if err != nil {
		return candidateBadRequest(c, "invalid universe id")
	}
	if err := h.authorizeUniverse(c.Context(), userID, universeID); err != nil {
		return candidateDomainError(c, err)
	}
	items, err := h.entities.ListCandidates(c.Context(), universeID)
	if err != nil {
		return candidateInternal(c, err)
	}
	return c.JSON(fiber.Map{"candidates": items})
}

func (h *EntityCandidateHandler) Accept(c *fiber.Ctx) error {
	return h.decide(c, "accept", "active", func(ctx context.Context, id uuid.UUID) (*models.Entity, error) {
		return h.entities.AcceptCandidate(ctx, id)
	})
}

func (h *EntityCandidateHandler) Dismiss(c *fiber.Ctx) error {
	return h.decide(c, "dismiss", "dismissed", func(ctx context.Context, id uuid.UUID) (*models.Entity, error) {
		return h.entities.DismissCandidate(ctx, id)
	})
}

func (h *EntityCandidateHandler) Merge(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return candidateUnauthorized(c)
	}
	candidateID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return candidateBadRequest(c, "invalid candidate id")
	}
	var req models.MergeEntityCandidateRequest
	if err := c.BodyParser(&req); err != nil || req.TargetEntityID == uuid.Nil {
		return candidateBadRequest(c, "target_entity_id is required")
	}
	candidate, err := h.entities.GetByID(c.Context(), candidateID)
	if err != nil {
		return candidateNotFound(c)
	}
	if err := h.authorizeUniverse(c.Context(), userID, candidate.UniverseID); err != nil {
		return candidateDomainError(c, err)
	}
	target, err := h.entities.GetByID(c.Context(), req.TargetEntityID)
	if err != nil || target.UniverseID != candidate.UniverseID {
		return candidateDomainError(c, fiber.ErrForbidden)
	}
	merged, err := h.entities.MergeCandidate(c.Context(), candidateID, target.ID)
	if err != nil {
		return candidateDomainError(c, err)
	}
	h.logDecisionFailure(c.Context(), userID, candidate, "merge", map[string]interface{}{"candidate_id": candidateID, "target_entity_id": target.ID})
	return c.JSON(fiber.Map{"entity": merged})
}

func (h *EntityCandidateHandler) decide(c *fiber.Ctx, action, status string, mutate func(context.Context, uuid.UUID) (*models.Entity, error)) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return candidateUnauthorized(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return candidateBadRequest(c, "invalid candidate id")
	}
	candidate, err := h.entities.GetByID(c.Context(), id)
	if err != nil {
		return candidateNotFound(c)
	}
	if err := h.authorizeUniverse(c.Context(), userID, candidate.UniverseID); err != nil {
		return candidateDomainError(c, err)
	}
	entity, err := mutate(c.Context(), id)
	if err != nil {
		return candidateDomainError(c, err)
	}
	h.logDecisionFailure(c.Context(), userID, candidate, action, map[string]interface{}{"candidate_id": id, "status": status})
	return c.JSON(fiber.Map{"entity": entity})
}

func (h *EntityCandidateHandler) authorizeUniverse(ctx context.Context, userID, universeID uuid.UUID) error {
	if h.owner == nil {
		return nil
	}
	universe, err := h.owner.FindByID(ctx, universeID)
	if err != nil || universe == nil {
		return fiber.ErrNotFound
	}
	if universe.UserID != userID {
		return fiber.ErrForbidden
	}
	return nil
}

func (h *EntityCandidateHandler) recordDecision(ctx context.Context, userID uuid.UUID, candidate *models.Entity, action string, payload map[string]interface{}) error {
	if h.writer == nil {
		return nil
	}
	signal := "reject"
	if action == "accept" || action == "merge" {
		signal = "accept"
	}
	_, err := h.writer.RecordFeedback(ctx, services.WriterFeedbackInput{
		UserID: userID, UniverseID: &candidate.UniverseID, Signal: signal, Payload: payload,
	})
	return err
}

func (h *EntityCandidateHandler) logDecisionFailure(ctx context.Context, userID uuid.UUID, candidate *models.Entity, action string, payload map[string]interface{}) {
	if err := h.recordDecision(ctx, userID, candidate, action, payload); err != nil {
		// The entity decision is already committed. Feedback is telemetry and
		// must not turn a successful accept/merge/dismiss into an HTTP 500.
		log.Printf("[entity-candidate] record %s feedback for %s: %v", action, candidate.ID, err)
	}
}

func candidateUnauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": fiber.Map{"code": "UNAUTHORIZED", "message": "authentication required"}})
}

func candidateBadRequest(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "VALIDATION_ERROR", "message": message}})
}

func candidateNotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_FOUND", "message": "entity candidate not found"}})
}

func candidateInternal(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()}})
}

func candidateDomainError(c *fiber.Ctx, err error) error {
	if errors.Is(err, fiber.ErrForbidden) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": fiber.Map{"code": "FORBIDDEN", "message": "candidate access denied"}})
	}
	if errors.Is(err, fiber.ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "not found") {
		return candidateNotFound(c)
	}
	if strings.Contains(strings.ToLower(err.Error()), "required") || strings.Contains(strings.ToLower(err.Error()), "candidate") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "VALIDATION_ERROR", "message": err.Error()}})
	}
	return candidateInternal(c, fmt.Errorf("candidate operation: %w", err))
}
