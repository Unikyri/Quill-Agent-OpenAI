package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvector "github.com/pgvector/pgvector-go/pgx"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/handlers"
	"github.com/quill/backend/internal/middleware"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
	"github.com/quill/backend/internal/ws"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to parse database config: %v", err)
	}
	poolCfg.MaxConns = int32(cfg.DBMaxConnections)
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "LOAD 'age'")
		if err != nil {
			return fmt.Errorf("load age: %w", err)
		}
		_, err = conn.Exec(ctx, "SET search_path = ag_catalog, \"$user\", public")
		if err != nil {
			return fmt.Errorf("set search_path: %w", err)
		}
		if err := pgxvector.RegisterTypes(ctx, conn); err != nil {
			return fmt.Errorf("register pgvector types: %w", err)
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Wait for DB to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		log.Printf("Waiting for database... (%d/30)", i+1)
		time.Sleep(time.Second)
	}

	// ── Repositories ──

	userRepo := repositories.NewUserRepo(pool)
	universeRepo := repositories.NewUniverseRepo(pool)
	workRepo := repositories.NewWorkRepo(pool)
	chapterRepo := repositories.NewChapterRepo(pool)
	entityRepo := repositories.NewEntityRepo(pool)
	vectorRepo := repositories.NewVectorRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)

	// Phase 2a repos
	contradictionRepo := repositories.NewContradictionRepo(pool)
	timelineRepo := repositories.NewTimelineRepo(pool)
	plotHoleRepo := repositories.NewPlotHoleRepo(pool)
	consolidationRepo := repositories.NewConsolidationRepo(pool)

	// ── Services ──

	tok := services.NewTokenizer()
	budgetMgr := services.NewContextBudgetManager(tok, cfg.MaxContextTokens, cfg.ResponseReserve)
	qwenSvc := services.NewQwenService(cfg, budgetMgr)
	authSvc := services.NewAuthService(userRepo, cfg)
	universeSvc := services.NewUniverseService(pool, universeRepo, graphRepo)
	workSvc := services.NewWorkService(pool, workRepo)
	entitySvc := services.NewEntityService(pool, entityRepo, vectorRepo, qwenSvc)
	demoSvc := services.NewDemoService(pool, universeRepo, graphRepo)

	// Phase 2a services
	consolidationSvc := services.NewConsolidationService(consolidationRepo, entityRepo, qwenSvc)
	relevSvc := services.NewRelevanceService(pool, entityRepo, cfg.DecayLambda, cfg.ArchiveThreshold, consolidationSvc)
	chapterSvc := services.NewChapterService(pool, chapterRepo, workRepo, relevSvc)
	memorySvc := services.NewMemoryService(graphRepo, entityRepo, vectorRepo)
	memorySvc.SetConsolidationRepo(consolidationRepo)
	memorySvc.SetBudgetMgr(budgetMgr)

	// QuillExecutor dispatches agent tool calls (vector search + graph queries)
	// to the appropriate repos. UniverseID is set per-call by the agent loop.
	executor := &services.QuillExecutor{
		VectorRepo: vectorRepo,
		GraphRepo:  graphRepo,
		EntityRepo: entityRepo,
		MemorySvc:  memorySvc,
		QwenSvc:    qwenSvc,
	}

	timelineSvc := services.NewTimelineService(pool, timelineRepo, qwenSvc, executor)
	plotHoleSvc := services.NewPlotHoleService(pool, plotHoleRepo, entityRepo, cfg.PlotHoleChapters, qwenSvc, executor)

	contraSvc := services.NewContradictionService(pool, contradictionRepo, entityRepo, qwenSvc, executor, cfg.MaxContradictionCandidates, budgetMgr)

	// WebSocket Hub (created first with nil submitter/recaller — set later to avoid circular init)
	hub := ws.NewHub(authSvc, nil, memorySvc, qwenSvc)

	// AnalysisService (depends on all other services and the hub)
	analysisSvc := services.NewAnalysisService(pool, entitySvc, contraSvc, relevSvc, timelineSvc, plotHoleSvc, qwenSvc, hub, memorySvc)

	// Wire the analysis service into the hub (now both exist)
	hub.SetSubmitter(analysisSvc)

	// Ingestion service (async document upload pipeline)
	ingestionSvc := services.NewIngestionService(pool, entitySvc, vectorRepo, graphRepo, qwenSvc, hub)

	// ── Handlers ──

	authH := handlers.NewAuthHandler(authSvc)
	universeH := handlers.NewUniverseHandler(universeSvc)
	workH := handlers.NewWorkHandler(workSvc)
	chapterH := handlers.NewChapterHandler(chapterSvc)
	entityH := handlers.NewEntityHandler(entitySvc)
	healthH := handlers.NewHealthHandler(pool, qwenSvc, cfg)
	demoH := handlers.NewDemoHandler(demoSvc)

	// Phase 2a handlers
	contradictionH := handlers.NewContradictionHandler(contraSvc, contradictionRepo)
	timelineH := handlers.NewTimelineHandler(timelineSvc, timelineRepo)
	plotHoleH := handlers.NewPlotHoleHandler(plotHoleSvc).WithRepo(plotHoleRepo)
	graphH := handlers.NewGraphHandler(graphRepo, memorySvc, entityRepo)
	ingestionH := handlers.NewIngestionHandler(ingestionSvc)

	// ── Fiber App ──

	app := fiber.New(fiber.Config{
		BodyLimit: cfg.MaxUploadSizeMB * 1024 * 1024,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(middleware.CORSMiddleware(cfg.AllowedOrigins))

	// Health (public)
	app.Get("/api/v1/health", healthH.Check)

	// Auth (public)
	auth := app.Group("/api/v1/auth")
	auth.Post("/register", authH.Register)
	auth.Post("/login", authH.Login)

	// Protected routes
	api := app.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(authSvc))

	// Auth (protected)
	api.Get("/auth/me", authH.Me)

	// Universes
	api.Post("/universes", universeH.Create)
	api.Get("/universes", universeH.List)
	api.Get("/universes/:id", universeH.GetByID)
	api.Put("/universes/:id", universeH.Update)
	api.Delete("/universes/:id", universeH.Delete)

	// Works
	api.Post("/universes/:universe_id/works", workH.Create)
	api.Get("/universes/:universe_id/works", workH.ListByUniverse)
	api.Get("/works/:id", workH.GetByID)
	api.Put("/works/:id", workH.Update)
	api.Delete("/works/:id", workH.Delete)

	// Chapters
	api.Post("/works/:work_id/chapters", chapterH.Create)
	api.Get("/works/:work_id/chapters", chapterH.ListByWork)
	api.Get("/chapters/:id", chapterH.GetByID)
	api.Put("/chapters/:id", chapterH.Update)
	api.Delete("/chapters/:id", chapterH.Delete)

	// Entities
	api.Get("/universes/:universe_id/entities", entityH.ListByUniverse)
	api.Get("/entities/:id", entityH.GetByID)
	api.Put("/entities/:id", entityH.Update)

	// Phase 2a REST routes
	api.Get("/universes/:universe_id/contradictions", contradictionH.ListByUniverse)
	api.Put("/universes/:universe_id/contradictions/:id/resolve", contradictionH.Resolve)
	api.Put("/universes/:universe_id/contradictions/:id/dismiss", contradictionH.Dismiss)
	api.Get("/universes/:universe_id/timeline", timelineH.ListByUniverse)
	api.Post("/universes/:universe_id/timeline", timelineH.Create)
	api.Get("/universes/:universe_id/plot-holes", plotHoleH.ListByUniverse)
	api.Get("/universes/:universe_id/graph", graphH.FullGraph)
	api.Get("/entities/:id/neighbors", graphH.Neighbors)
	api.Post("/universes/:id/recall", graphH.Recall)
	api.Post("/universes/:id/ingest", ingestionH.Ingest)

	// WebSocket route (gated by config)
	if cfg.WSEnabled {
		app.Get("/api/v1/ws", websocket.New(hub.Handle))
	}

	// Demo (public)
	app.Post("/api/v1/demo/clone", demoH.Clone)
	app.Post("/api/v1/demo/reset", demoH.Reset)

	// ── Graceful Shutdown Setup ──

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.Port)
		log.Printf("Quill backend starting on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down server...")

	// 1. Stop accepting new analysis jobs
	analysisSvc.Shutdown()
	log.Println("Analysis service stopped")

	// 2. Shut down Fiber (stops accepting new connections)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Quill backend stopped")
}
