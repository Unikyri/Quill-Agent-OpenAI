package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/middleware"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

// ExportHandler serves the recoverability surface in Markdown. It performs
// ownership checks before reading either a chapter or a work's chapters.
type ExportHandler struct {
	chapters *repositories.ChapterRepo
	works    *repositories.WorkRepo
	owner    interface {
		FindByID(context.Context, uuid.UUID) (*models.Universe, error)
	}
}

func NewExportHandler(chapters *repositories.ChapterRepo, works *repositories.WorkRepo, owner interface {
	FindByID(context.Context, uuid.UUID) (*models.Universe, error)
}) *ExportHandler {
	return &ExportHandler{chapters: chapters, works: works, owner: owner}
}

func (h *ExportHandler) Chapter(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return exportUnauthorized(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exportBadRequest(c, "invalid chapter id")
	}
	chapter, err := h.chapters.FindByID(c.Context(), id)
	if err != nil {
		return exportNotFound(c)
	}
	if err := h.authorize(c.Context(), userID, chapter.UniverseID); err != nil {
		return exportDomainError(c, err)
	}
	body := services.ExportChapterMarkdown(chapter.Title, chapter.Content)
	return sendMarkdown(c, safeFilename(chapter.Title, "chapter"), body)
}

func (h *ExportHandler) Work(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return exportUnauthorized(c)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exportBadRequest(c, "invalid work id")
	}
	work, err := h.works.FindByID(c.Context(), id)
	if err != nil {
		return exportNotFound(c)
	}
	if err := h.authorize(c.Context(), userID, work.UniverseID); err != nil {
		return exportDomainError(c, err)
	}
	chapters, err := h.chapters.ListByWork(c.Context(), work.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()}})
	}
	body := services.ExportWorkMarkdown(work.Title, chapters)
	return sendMarkdown(c, safeFilename(work.Title, "work"), body)
}

func (h *ExportHandler) authorize(ctx context.Context, userID, universeID uuid.UUID) error {
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

func sendMarkdown(c *fiber.Ctx, filename, body string) error {
	c.Set(fiber.HeaderContentType, "text/markdown; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.SendString(body)
}

func safeFilename(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	value = strings.ReplaceAll(filepath.Base(value), `"`, "")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value + ".md"
}

func exportUnauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": fiber.Map{"code": "UNAUTHORIZED", "message": "authentication required"}})
}

func exportBadRequest(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "VALIDATION_ERROR", "message": message}})
}

func exportNotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_FOUND", "message": "export source not found"}})
}

func exportDomainError(c *fiber.Ctx, err error) error {
	if err == fiber.ErrForbidden {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": fiber.Map{"code": "FORBIDDEN", "message": "export access denied"}})
	}
	if err == fiber.ErrNotFound {
		return exportNotFound(c)
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()}})
}
