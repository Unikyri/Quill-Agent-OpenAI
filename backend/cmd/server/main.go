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
	"github.com/quill/backend/internal/mcp"
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
	skillRegistry, err := services.NewSkillRegistry(cfg.SkillDir)
	if err != nil {
		log.Fatalf("Failed to load skill registry: %v", err)
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
	skillRepo := repositories.NewSkillRepo(pool)

	// Phase 2a repos
	contradictionRepo := repositories.NewContradictionRepo(pool)
	timelineRepo := repositories.NewTimelineRepo(pool)
	plotHoleRepo := repositories.NewPlotHoleRepo(pool)
	consolidationRepo := repositories.NewConsolidationRepo(pool)
	writerMemoryRepo := repositories.NewWriterMemoryRepo(pool)

	// ── Services ──

	tok := services.NewTokenizer()
	budgetMgr := services.NewContextBudgetManager(tok, cfg.MaxContextTokens, cfg.ResponseReserve)
	var llmSvc services.LLMService
	switch cfg.LLMProtocol {
	case "dashscope":
		llmSvc = services.NewDashScopeService(cfg, budgetMgr)
	default:
		// Keep OpenAI-compatible Qwen as the reversible fallback. An invalid
		// protocol is rejected by config.Load before the composition root.
		llmSvc = services.NewQwenService(cfg, budgetMgr)
	}
	authSvc := services.NewAuthService(userRepo, cfg)
	universeSvc := services.NewUniverseService(pool, universeRepo, graphRepo)
	universeSvc.SetSkillActivation(skillRepo, skillRegistry)
	workSvc := services.NewWorkService(pool, workRepo)
	entitySvc := services.NewEntityService(pool, entityRepo, vectorRepo, llmSvc)
	entitySvc.SetConfidenceThreshold(cfg.EntityConfidenceThreshold)
	demoSvc := services.NewDemoService(pool, universeRepo, graphRepo)

	// Phase 2a services
	consolidationSvc := services.NewConsolidationService(consolidationRepo, entityRepo, llmSvc)
	relevSvc := services.NewRelevanceService(pool, entityRepo, cfg.DecayLambda, cfg.ArchiveThreshold, consolidationSvc)
	chapterSvc := services.NewChapterService(pool, chapterRepo, workRepo, relevSvc)
	writerMemorySvc := services.NewWriterMemoryService(writerMemoryRepo, llmSvc, cfg.WriterPreferencePromotionThreshold, cfg.DecayLambda, cfg.ArchiveThreshold)
	stylometrySvc := services.NewStylometryService(writerMemoryRepo)
	chapterSvc.SetWriterMemory(universeRepo, stylometrySvc)
	chapterSvc.SetWriterMemoryDecayer(writerMemorySvc)
	memorySvc := services.NewMemoryService(graphRepo, entityRepo, vectorRepo)
	memorySvc.SetConsolidationRepo(consolidationRepo)
	memorySvc.SetWriterMemoryRepo(writerMemoryRepo)
	memorySvc.SetBudgetMgr(budgetMgr)
	if reranker, ok := llmSvc.(services.Reranker); ok {
		memorySvc.SetReranker(reranker)
	}
	memorySvc.SetHistoryRepo(repositories.NewEntityRelevanceHistoryRepo(pool))
	memorySvc.SetRelevanceDeltaEpsilon(cfg.RelevanceDeltaEpsilon)
	craftReviewSvc := services.NewCraftReviewService(skillRegistry, skillRepo, universeRepo, llmSvc, memorySvc, writerMemorySvc, cfg.QwenExtractionModel, cfg.QwenCraftModel)

	// QuillExecutor dispatches agent tool calls (vector search + graph queries)
	// to the appropriate repos. UniverseID is set per-call by the agent loop.
	executor := &services.QuillExecutor{
		VectorRepo: vectorRepo,
		GraphRepo:  graphRepo,
		EntityRepo: entityRepo,
		MemorySvc:  memorySvc,
		QwenSvc:    llmSvc,
	}

	timelineSvc := services.NewTimelineService(pool, timelineRepo, llmSvc, executor)
	plotHoleSvc := services.NewPlotHoleService(pool, plotHoleRepo, entityRepo, cfg.PlotHoleChapters, llmSvc, executor, cfg.PlotHoleAgentDepth)

	contraSvc := services.NewContradictionService(pool, contradictionRepo, entityRepo, llmSvc, executor, cfg.MaxContradictionCandidates, budgetMgr, cfg.ContradictionAgentDepth)

	// WebSocket Hub (created first with nil submitter/recaller — set later to avoid circular init)
	hub := ws.NewHub(authSvc, nil, memorySvc, llmSvc)
	hub.SetUniverseOwnerResolver(universeRepo)
	hub.SetParagraphOwnershipResolvers(workRepo, chapterRepo)
	hub.SetCraftReviewer(craftReviewSvc)

	// AnalysisService (depends on all other services and the hub)
	analysisSvc := services.NewAnalysisService(pool, entitySvc, contraSvc, relevSvc, timelineSvc, plotHoleSvc, llmSvc, hub, memorySvc)

	// Wire the analysis service into the hub (now both exist)
	hub.SetSubmitter(analysisSvc)

	// Ingestion service (async document upload pipeline)
	ingestionSvc := services.NewIngestionService(pool, entitySvc, vectorRepo, graphRepo, llmSvc, hub)
	ingestionSvc.SetPostIngestAnalysis(contraSvc, plotHoleSvc, budgetMgr, cfg.IngestAnalysisMaxChapters)
	ingestionSvc.SetStylometry(stylometrySvc)

	// ── Handlers ──

	authH := handlers.NewAuthHandler(authSvc)
	universeH := handlers.NewUniverseHandler(universeSvc)
	workH := handlers.NewWorkHandler(workSvc)
	chapterH := handlers.NewChapterHandler(chapterSvc)
	chapterH.SetOwnershipRepos(universeRepo, workRepo, chapterRepo)
	entityH := handlers.NewEntityHandler(entitySvc)
	candidateH := handlers.NewEntityCandidateHandler(entitySvc, universeRepo, writerMemorySvc)
	healthH := handlers.NewHealthHandler(pool, llmSvc, cfg)
	demoH := handlers.NewDemoHandler(demoSvc)
	exportH := handlers.NewExportHandler(chapterRepo, workRepo, universeRepo)

	// Phase 2a handlers
	contradictionH := handlers.NewContradictionHandler(contraSvc, contradictionRepo)
	timelineH := handlers.NewTimelineHandler(timelineSvc, timelineRepo)
	plotHoleH := handlers.NewPlotHoleHandler(plotHoleSvc).WithRepo(plotHoleRepo)
	graphH := handlers.NewGraphHandler(graphRepo, memorySvc, entityRepo, llmSvc)
	graphH.SetDecayer(relevSvc)
	graphH.SetWriterMemoryDecayer(writerMemorySvc)
	graphH.SetUniverseOwnerRepo(universeRepo)
	ingestionH := handlers.NewIngestionHandler(ingestionSvc)
	ingestionH.SetUniverseOwnerRepo(universeRepo)
	writerMemoryH := handlers.NewWriterMemoryHandler(writerMemorySvc)
	writerMemoryH.SetOwnershipRepos(universeRepo, chapterRepo)
	skillH := handlers.NewSkillHandler(skillRegistry, universeSvc)
	mcpServer := mcp.NewServer(authSvc, universeRepo, executor, memorySvc, llmSvc)

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

	// WebSocket route (public, self-authenticating via auth_init message)
	if cfg.WSEnabled {
		app.Get("/api/v1/ws", websocket.New(hub.Handle))
	}

	// Protected routes
	api := app.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(authSvc))

	// Auth (protected)
	api.Get("/auth/me", authH.Me)
	api.Get("/skills", skillH.Catalogue)
	app.Post("/api/v1/mcp", mcpServer.Handle)

	// Universes
	api.Post("/universes", universeH.Create)
	api.Get("/universes", universeH.List)
	api.Get("/universes/:id", universeH.GetByID)
	api.Put("/universes/:id", universeH.Update)
	api.Delete("/universes/:id", universeH.Delete)
	api.Get("/universes/:id/skills", skillH.ListUniverse)
	api.Put("/universes/:id/skills", skillH.ReplaceUniverse)

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
	api.Get("/chapters/:id/export.md", exportH.Chapter)
	api.Get("/works/:id/export.md", exportH.Work)

	// Entities
	api.Get("/universes/:universe_id/entities", entityH.ListByUniverse)
	api.Get("/entities/:id", entityH.GetByID)
	api.Put("/entities/:id", entityH.Update)
	api.Get("/universes/:universe_id/candidates", candidateH.List)
	api.Post("/candidates/:id/accept", candidateH.Accept)
	api.Post("/candidates/:id/dismiss", candidateH.Dismiss)
	api.Post("/candidates/:id/merge", candidateH.Merge)

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
	api.Post("/universes/:id/recall/explain", graphH.RecallExplain)
	api.Get("/universes/:id/memory-status", graphH.MemoryStatus)
	api.Post("/universes/:id/decay", graphH.RunDecay)
	api.Post("/universes/:id/ingest", ingestionH.Ingest)
	api.Get("/universes/:id/ingestions", ingestionH.Jobs)

	// Writer Memory (authenticated, user-scoped evidence and correction)
	api.Get("/users/me/preferences", writerMemoryH.ListPreferences)
	api.Get("/users/me/preferences/:id/evidence", writerMemoryH.Evidence)
	api.Patch("/users/me/preferences/:id", writerMemoryH.Correct)
	api.Put("/users/me/preferences/:id/correct", writerMemoryH.Correct)
	api.Delete("/users/me/preferences/:id", writerMemoryH.Deactivate)
	api.Post("/users/me/preferences/:id/deactivate", writerMemoryH.Deactivate)
	api.Post("/users/me/preferences/feedback", writerMemoryH.Feedback)

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
