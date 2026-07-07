package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/middleware"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
	"github.com/quill/backend/internal/testutil"
	"github.com/quill/backend/internal/ws"
)

// TestE2EFullFlow validates the end-to-end across Phase 2a endpoints.
func TestE2EFullFlow(t *testing.T) {
	pool, app := setupE2EApp(t)
	defer pool.Close()

	// ── Register + Login ──
	registerBody := `{"email":"e2e@test.com","password":"password123","display_name":"E2E User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", toReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("register got %d: %s", resp.StatusCode, string(body))
	}

	loginBody := `{"email":"e2e@test.com","password":"password123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", toReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &loginResp)
	token := loginResp.Token
	if token == "" {
		t.Fatal("login returned empty token")
	}

	// ── Create Universe ──
	universeBody := `{"name":"E2E Universe","description":"Test","genre":"fantasy","format":"novel"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/universes", toReader(universeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("create universe: %v", err)
	}
	var uv struct {
		Universe struct{ ID string `json:"id"` } `json:"universe"`
	}
	body, _ = io.ReadAll(resp.Body)
	json.Unmarshal(body, &uv)
	universeID := uv.Universe.ID
	if universeID == "" {
		t.Fatal("universe ID is empty")
	}

	// ── Create Work ──
	workBody := `{"title":"E2E Work","type":"novel","synopsis":"Test"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/universes/"+universeID+"/works", toReader(workBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("create work: %v", err)
	}

	// ── Query Phase 2a endpoints ──

	checkEndpoint := func(label, method, url, bodyStr string) {
		t.Helper()
		var req *http.Request
		if bodyStr != "" {
			req = httptest.NewRequest(method, url, toReader(bodyStr))
		} else {
			req = httptest.NewRequest(method, url, nil)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		if err != nil {
			t.Errorf("%s: request error: %v", label, err)
			return
		}
		// Assert 2xx status
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Errorf("%s: expected 2xx status, got %d", label, resp.StatusCode)
			return
		}
		// Assert non-empty response body
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			t.Errorf("%s: expected non-empty response body", label)
		}
		t.Logf("%s: HTTP %d, body %d bytes", label, resp.StatusCode, len(body))
	}

	checkEndpoint("contradictions list", "GET", "/api/v1/contradictions?universe_id="+universeID, "")
	checkEndpoint("contradiction resolve", "PUT", "/api/v1/contradictions/00000000-0000-4000-8000-0000000000e2/resolve", "")
	checkEndpoint("timeline list", "GET", "/api/v1/timeline?universe_id="+universeID, "")
	checkEndpoint("plot holes", "GET", "/api/v1/plot-holes?universe_id="+universeID, "")
	checkEndpoint("graph", "GET", "/api/v1/graph?universe_id="+universeID, "")
	checkEndpoint("neighbors", "GET", "/api/v1/entities/00000000-0000-4000-8000-000000000001/neighbors?universe_id="+universeID, "")
	checkEndpoint("recall", "POST", "/api/v1/universes/"+universeID+"/recall", `{"query":"hero","k":5}`)
}

func setupE2EApp(t *testing.T) (*pgxpool.Pool, *fiber.App) {
	t.Helper()

	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "005")

	cfg := &config.Config{
		DatabaseURL:                "postgres://localhost:5432/test",
		QwenAPIKey:                 "test-key",
		QwenBaseURL:                "http://localhost:9999",
		QwenMaxModel:               "qwen-max-latest",
		QwenTurboModel:             "qwen-turbo-latest",
		QwenEmbeddingModel:         "text-embedding-v3",
		QwenEmbeddingDims:          1024,
		JWTSecret:                  "e2e-test-secret",
		JWTExpirationHours:         24,
		BCryptCost:                 4,
		Port:                       "0",
		AllowedOrigins:             "*",
		MaxUploadSizeMB:            10,
		DecayLambda:                0.1,
		ArchiveThreshold:           0.15,
		PlotHoleChapters:           8,
		MaxContradictionCandidates: 3,
		WSEnabled:                  false,
	}

	userRepo := repositories.NewUserRepo(pool)
	universeRepo := repositories.NewUniverseRepo(pool)
	workRepo := repositories.NewWorkRepo(pool)
	chapterRepo := repositories.NewChapterRepo(pool)
	entityRepo := repositories.NewEntityRepo(pool)
	vectorRepo := repositories.NewVectorRepo(pool)
	graphRepo := repositories.NewGraphRepo(pool)
	contradictionRepo := repositories.NewContradictionRepo(pool)
	timelineRepo := repositories.NewTimelineRepo(pool)
	plotHoleRepo := repositories.NewPlotHoleRepo(pool)

	qwenSvc := services.NewQwenService(cfg, nil)
	authSvc := services.NewAuthService(userRepo, cfg)
	universeSvc := services.NewUniverseService(pool, universeRepo, graphRepo)
	workSvc := services.NewWorkService(pool, workRepo)
	_ = services.NewChapterService(pool, chapterRepo, nil, nil)
	entitySvc := services.NewEntityService(pool, entityRepo, vectorRepo, qwenSvc)
	_ = entitySvc
	contraSvc := services.NewContradictionService(pool, contradictionRepo, entityRepo, qwenSvc, nil, cfg.MaxContradictionCandidates, nil)
	timelineSvc := services.NewTimelineService(pool, timelineRepo, nil, nil)
	plotHoleSvc := services.NewPlotHoleService(pool, plotHoleRepo, entityRepo, cfg.PlotHoleChapters, nil, nil)
	memorySvc := services.NewMemoryService(graphRepo, entityRepo, vectorRepo)
	_ = services.NewRelevanceService(pool, entityRepo, cfg.DecayLambda, cfg.ArchiveThreshold, nil)
	_ = ws.NewHub(authSvc, nil, nil, nil)

	authH := NewAuthHandler(authSvc)
	universeH := NewUniverseHandler(universeSvc)
	workH := NewWorkHandler(workSvc)
	contradictionH := NewContradictionHandler(contraSvc, contradictionRepo)
	timelineH := NewTimelineHandler(timelineSvc, timelineRepo)
	plotHoleH := NewPlotHoleHandler(plotHoleSvc).WithRepo(plotHoleRepo)
	graphH := NewGraphHandler(graphRepo, memorySvc, entityRepo)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		},
	})

	auth := app.Group("/api/v1/auth")
	auth.Post("/register", authH.Register)
	auth.Post("/login", authH.Login)

	api := app.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(authSvc))
	api.Post("/universes", universeH.Create)
	api.Post("/universes/:universe_id/works", workH.Create)
	api.Get("/contradictions", contradictionH.ListByUniverse)
	api.Put("/contradictions/:id/resolve", contradictionH.Resolve)
	api.Get("/timeline", timelineH.ListByUniverse)
	api.Post("/timeline", timelineH.Create)
	api.Get("/plot-holes", plotHoleH.ListByUniverse)
	api.Get("/graph", graphH.FullGraph)
	api.Get("/entities/:id/neighbors", graphH.Neighbors)
	api.Post("/universes/:id/recall", graphH.Recall)

	return pool, app
}

func toReader(s string) io.Reader {
	return strings.NewReader(s)
}
