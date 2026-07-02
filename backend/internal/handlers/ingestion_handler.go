package handlers

import (
	"context"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// IngestionStarter is the minimal service interface for the ingestion handler.
type IngestionStarter interface {
	Start(ctx context.Context, universeID, workID uuid.UUID, reader io.Reader, filename string) (uuid.UUID, error)
}

// IngestionHandler handles document upload for async ingestion.
type IngestionHandler struct {
	ingestionSvc IngestionStarter
}

// NewIngestionHandler creates an ingestion handler backed by the given service.
func NewIngestionHandler(svc IngestionStarter) *IngestionHandler {
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

	if h.ingestionSvc == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": fiber.Map{"code": "SERVICE_UNAVAILABLE", "message": "Ingestion service not configured"},
		})
	}

	// ponytail: work_id follows universe_id for now. A future UI can let users
	// select which work to ingest into.
	const placeholderWorkID = "00000000-0000-0000-0000-000000000000"
	workID, _ := uuid.Parse(placeholderWorkID)

	jobID, err := h.ingestionSvc.Start(c.Context(), universeID, workID, f, file.Filename)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"job_id": jobID.String(),
		"status": "accepted",
	})
}
