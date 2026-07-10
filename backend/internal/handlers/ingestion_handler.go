package handlers

import (
	"context"
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/services"
)

// IngestionStarter is the minimal service interface for the ingestion handler.
type IngestionStarter interface {
	Start(ctx context.Context, universeID uuid.UUID, reader io.Reader, filename string) (jobID uuid.UUID, duplicate bool, err error)
	ListJobs(ctx context.Context, universeID uuid.UUID) ([]models.IngestionJob, error)
}

// IngestionHandler handles document upload for async ingestion.
type IngestionHandler struct {
	ingestionSvc IngestionStarter
}

// NewIngestionHandler creates an ingestion handler backed by the given service.
func NewIngestionHandler(svc IngestionStarter) *IngestionHandler {
	if svc == nil {
		panic("ingestionSvc required")
	}
	return &IngestionHandler{ingestionSvc: svc}
}

// Ingest handles POST /api/v1/universes/:id/ingest.
// Parses a multipart form file and kicks off async processing.
func (h *IngestionHandler) Ingest(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe ID"},
		})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "File is required"},
		})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": "Failed to open uploaded file"},
		})
	}
	defer f.Close()

	jobID, duplicate, err := h.ingestionSvc.Start(c.Context(), universeID, f, file.Filename)
	if err != nil {
		if errors.Is(err, services.ErrUnsupportedFileType) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fiber.Map{"code": "VALIDATION_ERROR", "message": err.Error()},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	if duplicate {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_id": jobID.String(),
			"status": "duplicate",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id": jobID.String(),
		"status": "accepted",
	})
}

// Jobs handles GET /api/v1/universes/:id/ingestions.
func (h *IngestionHandler) Jobs(c *fiber.Ctx) error {
	universeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid universe ID"},
		})
	}

	jobs, err := h.ingestionSvc.ListJobs(c.Context(), universeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.JSON(fiber.Map{"jobs": jobs})
}
