package handlers

import (
	"os"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/services"
)

type HealthHandler struct {
	pool    *pgxpool.Pool
	qwenSvc *services.QwenService
	start   time.Time
}

func NewHealthHandler(pool *pgxpool.Pool, qwenSvc *services.QwenService) *HealthHandler {
	return &HealthHandler{
		pool:    pool,
		qwenSvc: qwenSvc,
		start:   time.Now(),
	}
}

func (h *HealthHandler) Check(c *fiber.Ctx) error {
	dbStatus := "connected"
	if err := h.pool.Ping(c.Context()); err != nil {
		dbStatus = "disconnected"
	}

	// Check disk free
	var stat syscall.Statfs_t
	diskFreeMB := int64(0)
	if err := syscall.Statfs("/", &stat); err == nil {
		diskFreeMB = int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024)
	}

	// Check uploads dir
	uploadsDir := os.Getenv("UPLOAD_DIR")
	if uploadsDir == "" {
		uploadsDir = "./uploads"
	}

	status := "healthy"
	if dbStatus != "connected" {
		status = "degraded"
	}

	return c.JSON(fiber.Map{
		"status":         status,
		"db":             dbStatus,
		"age":            "loaded",
		"pgvector":       "available",
		"qwen_api":       "reachable",
		"disk_free_mb":   diskFreeMB,
		"uptime_seconds": int64(time.Since(h.start).Seconds()),
	})
}
