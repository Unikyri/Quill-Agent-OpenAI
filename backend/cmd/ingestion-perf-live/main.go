// Command ingestion-perf-live measures the real asynchronous ingestion path.
// It creates isolated benchmark data, validates Qwen and PostgreSQL before any
// write, and never turns a missing or failed measurement into an SLA claim.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvector "github.com/pgvector/pgvector-go/pgx"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
)

const duplicateNaturalKeyQuery = `SELECT lower(name), type, COUNT(*) FROM entities WHERE universe_id = $1 GROUP BY lower(name), type HAVING COUNT(*) > 1;`

type naturalKeyDuplicate struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type liveRun struct {
	Run              int                   `json:"run"`
	Status           string                `json:"status"`
	WallSeconds      float64               `json:"wall_seconds"`
	Chunks           int                   `json:"chunks"`
	ChunksPerSecond  float64               `json:"chunks_per_second"`
	Entities         int                   `json:"entities_extracted"`
	DuplicateRows    []naturalKeyDuplicate `json:"duplicate_natural_key_rows"`
	Error            string                `json:"error,omitempty"`
	RetainedUniverse string                `json:"retained_universe,omitempty"`
}

type liveReport struct {
	GeneratedAt              string            `json:"generated_at"`
	Mode                     string            `json:"mode"`
	Pages                    int               `json:"pages"`
	Fixture                  string            `json:"fixture"`
	Words                    int               `json:"words"`
	Chunks                   int               `json:"chunks"`
	Runs                     int               `json:"runs"`
	CompletedRuns            int               `json:"completed_runs"`
	WallSeconds              []float64         `json:"wall_seconds"`
	P50Seconds               float64           `json:"p50_seconds"`
	P95Seconds               float64           `json:"p95_seconds"`
	ChunksPerSecond          float64           `json:"chunks_per_second"`
	Models                   map[string]string `json:"models,omitempty"`
	Config                   map[string]string `json:"config,omitempty"`
	DuplicateNaturalKeyQuery string            `json:"duplicate_natural_key_query"`
	RunDetails               []liveRun         `json:"run_details"`
	SLAAssessment            string            `json:"sla_assessment"`
	Error                    string            `json:"error,omitempty"`
}

type benchmarkScope struct {
	userID     uuid.UUID
	universeID uuid.UUID
	graphName  string
}

func main() {
	pages := flag.Int("pages", 50, "fixture size: 50 or 400 pages")
	runs := flag.Int("runs", 1, "number of live ingestion runs (each consumes Qwen quota)")
	fixture := flag.String("fixture", "", "fixture Markdown path (default: ../artifacts/fixtures/ingestion-<pages>-page.md)")
	output := flag.String("output", "", "report path (default: ../artifacts/reports/ingestion-live-<pages>-page.json)")
	timeout := flag.Duration("timeout", 20*time.Minute, "maximum wait for each asynchronous ingestion job")
	keep := flag.Bool("keep", false, "keep completed benchmark users, universes, and graphs for inspection")
	flag.Parse()

	path := *output
	if path == "" {
		path = filepath.Join("..", "artifacts", "reports", fmt.Sprintf("ingestion-live-%d-page.json", *pages))
	}
	report := liveReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Mode: "live-ingestion", Pages: *pages, Runs: *runs,
		DuplicateNaturalKeyQuery: duplicateNaturalKeyQuery,
		SLAAssessment:            "not evaluated: a completed live measurement is required",
	}

	if *pages != 50 && *pages != 400 {
		fail(path, &report, errors.New("-pages must be 50 or 400"))
	}
	if *runs < 1 {
		fail(path, &report, errors.New("-runs must be at least 1"))
	}
	if *timeout <= 0 {
		fail(path, &report, errors.New("-timeout must be positive"))
	}
	if *fixture == "" {
		*fixture = filepath.Join("..", "artifacts", "fixtures", fmt.Sprintf("ingestion-%d-page.md", *pages))
	}
	report.Fixture = *fixture
	content, err := os.ReadFile(*fixture)
	if err != nil {
		fail(path, &report, fmt.Errorf("read fixture: %w (run make ingestion-perf first)", err))
	}
	report.Words = len(strings.Fields(string(content)))
	report.Chunks = strings.Count(string(content), "# Chapter ")
	if report.Chunks != *pages {
		fail(path, &report, fmt.Errorf("fixture has %d chapters, want %d", report.Chunks, *pages))
	}

	cfg, err := config.Load()
	if err != nil {
		fail(path, &report, err)
	}
	if err := validateLiveConfig(cfg); err != nil {
		fail(path, &report, err)
	}
	report.Models, report.Config = modelMetadata(cfg)

	ctx := context.Background()
	pool, err := newPool(ctx, cfg)
	if err != nil {
		fail(path, &report, err)
	}
	defer pool.Close()

	qwen := services.NewQwenService(cfg, nil)
	preflightCtx, cancel := context.WithTimeout(ctx, cfg.QwenHealthTimeout)
	err = preflight(preflightCtx, pool, qwen)
	cancel()
	if err != nil {
		fail(path, &report, err)
	}

	for run := 1; run <= *runs; run++ {
		result := runOnce(ctx, pool, qwen, content, *timeout, run, *keep)
		report.RunDetails = append(report.RunDetails, result)
		if result.Status == "completed" && len(result.DuplicateRows) == 0 && result.Error == "" {
			report.CompletedRuns++
			report.WallSeconds = append(report.WallSeconds, result.WallSeconds)
		} else if report.Error == "" {
			report.Error = result.Error
			if report.Error == "" {
				report.Error = fmt.Sprintf("run %d ended with status %s or duplicate natural keys", run, result.Status)
			}
		}
	}

	if report.CompletedRuns == report.Runs {
		totalWall := 0.0
		for _, sample := range report.WallSeconds {
			totalWall += sample
		}
		report.ChunksPerSecond = float64(report.Chunks*report.CompletedRuns) / max(totalWall, 0.000001)
		if report.CompletedRuns > 1 {
			sorted := append([]float64(nil), report.WallSeconds...)
			sort.Float64s(sorted)
			report.P50Seconds = percentile(sorted, .50)
			report.P95Seconds = percentile(sorted, .95)
			report.SLAAssessment = "measured: compare p95_seconds to PF-1/PF-2 thresholds; this report does not infer a pass"
		} else {
			report.SLAAssessment = "single live measurement: p50/p95 are not calculated; run with -runs 2 or greater before comparing an SLA"
		}
	} else if report.Error == "" {
		report.Error = "one or more runs did not complete cleanly"
	}

	if err := writeReport(path, &report); err != nil {
		log.Fatalf("write live ingestion performance report: %v", err)
	}
	if report.Error != "" {
		log.Fatal(report.Error)
	}
	fmt.Println(path)
}

func validateLiveConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("application config is not available")
	}
	if strings.TrimSpace(cfg.QwenAPIKey) == "" {
		return errors.New("QWEN_API_KEY is required for live ingestion performance runs")
	}
	if strings.TrimSpace(cfg.QwenEmbeddingModel) == "" {
		return errors.New("QWEN_EMBEDDING_MODEL must be configured")
	}
	if cfg.QwenEmbeddingDims <= 0 {
		return errors.New("QWEN_EMBEDDING_DIMENSIONS must be positive")
	}
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return errors.New("DATABASE_URL must be a PostgreSQL connection URL")
	}
	return nil
}

func modelMetadata(cfg *config.Config) (map[string]string, map[string]string) {
	return map[string]string{
			"extraction": cfg.QwenExtractionModel,
			"reasoning":  cfg.QwenReasoningModel,
			"embedding":  cfg.QwenEmbeddingModel,
		}, map[string]string{
			"qwen_embedding_dimensions": fmt.Sprint(cfg.QwenEmbeddingDims),
			"llm_tpm_turbo":             fmt.Sprint(cfg.LLMTPMTurbo),
			"llm_tpm_max":               fmt.Sprint(cfg.LLMTPMMax),
			"llm_rpm":                   fmt.Sprint(cfg.LLMRPM),
			"llm_interactive_reserve":   fmt.Sprint(cfg.LLMInteractiveReserve),
			"llm_max_concurrency":       fmt.Sprint(cfg.LLMMaxConcurrency),
			"llm_ramp_step":             fmt.Sprint(cfg.LLMRampStep),
			"qwen_retry_max_attempts":   fmt.Sprint(cfg.QwenRetryMaxAttempts),
		}
}

func newPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	poolCfg.MaxConns = int32(cfg.DBMaxConnections)
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvector.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect PostgreSQL: %w", err)
	}
	return pool, nil
}

func preflight(ctx context.Context, pool *pgxpool.Pool, qwen *services.QwenService) error {
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("PostgreSQL preflight: %w", err)
	}
	var vector, age bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector'), EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'age')`).Scan(&vector, &age); err != nil {
		return fmt.Errorf("inspect PostgreSQL extensions: %w", err)
	}
	if !vector || !age {
		return fmt.Errorf("PostgreSQL preflight requires vector=%t and age=%t", vector, age)
	}
	if err := qwen.HealthCheck(ctx); err != nil {
		return fmt.Errorf("Qwen preflight: %w", err)
	}
	return nil
}

func runOnce(ctx context.Context, pool *pgxpool.Pool, qwen *services.QwenService, content []byte, timeout time.Duration, run int, keep bool) liveRun {
	result := liveRun{Run: run, Status: "failed"}
	scope, err := createScope(ctx, pool)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	terminal := false
	if !keep {
		defer func() {
			if !terminal {
				result.RetainedUniverse = scope.universeID.String()
				return
			}
			if err := cleanupScope(ctx, pool, scope); err != nil && result.Error == "" {
				result.Error = fmt.Sprintf("cleanup benchmark scope: %v", err)
			}
		}()
	} else {
		result.RetainedUniverse = scope.universeID.String()
	}

	entitySvc := services.NewEntityService(pool, repositories.NewEntityRepo(pool), repositories.NewVectorRepo(pool), qwen)
	ingestion := services.NewIngestionService(pool, entitySvc, repositories.NewVectorRepo(pool), repositories.NewGraphRepo(pool), qwen, nil)
	started := time.Now()
	jobID, duplicate, err := ingestion.Start(ctx, scope.universeID, bytes.NewReader(content), fmt.Sprintf("ingestion-%d-run-%d.md", len(content), run))
	if err != nil {
		terminal = true // Start failed before the background worker was created.
		result.Error = fmt.Sprintf("start ingestion: %v", err)
		return result
	}
	if duplicate {
		terminal = true // A duplicate path never starts a new background worker.
		result.Error = "benchmark scope unexpectedly received a duplicate ingestion job"
		return result
	}
	job, err := waitForTerminalJob(ctx, repositories.NewIngestionRepo(pool), jobID, timeout)
	result.WallSeconds = time.Since(started).Seconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	terminal = true
	result.Status = job.Status
	result.Chunks = job.TotalChaptersDetected
	result.Entities = job.EntitiesExtracted
	result.ChunksPerSecond = float64(result.Chunks) / max(result.WallSeconds, 0.000001)
	if job.Status != "completed" {
		result.Error = job.ErrorMessage
		if result.Error == "" {
			result.Error = "ingestion did not complete"
		}
		return result
	}
	result.DuplicateRows, err = duplicateRows(ctx, pool, scope.universeID)
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func createScope(ctx context.Context, pool *pgxpool.Pool) (benchmarkScope, error) {
	scope := benchmarkScope{userID: uuid.New(), universeID: uuid.New()}
	scope.graphName = "universe_" + scope.universeID.String()
	user := &models.User{ID: scope.userID, Email: fmt.Sprintf("ingestion-perf-%s@invalid", scope.userID), DisplayName: "Ingestion performance benchmark"}
	if err := repositories.NewUserRepo(pool).Create(ctx, user, "benchmark-only"); err != nil {
		return scope, fmt.Errorf("create benchmark user: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		_ = deleteUser(ctx, pool, scope.userID)
		return scope, fmt.Errorf("begin benchmark universe: %w", err)
	}
	universe := &models.Universe{ID: scope.universeID, UserID: scope.userID, Name: "Ingestion performance benchmark", GenreTags: []string{"science-fiction"}}
	if err := repositories.NewUniverseRepo(pool).Create(ctx, tx, universe); err != nil {
		_ = tx.Rollback(ctx)
		_ = deleteUser(ctx, pool, scope.userID)
		return scope, fmt.Errorf("create benchmark universe: %w", err)
	}
	if err := repositories.NewGraphRepo(pool).CreateGraphTx(ctx, tx, scope.universeID.String()); err != nil {
		_ = tx.Rollback(ctx)
		_ = deleteUser(ctx, pool, scope.userID)
		return scope, fmt.Errorf("create benchmark graph: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		_ = deleteUser(ctx, pool, scope.userID)
		return scope, fmt.Errorf("commit benchmark universe: %w", err)
	}
	return scope, nil
}

func cleanupScope(ctx context.Context, pool *pgxpool.Pool, scope benchmarkScope) error {
	graphErr := repositories.NewGraphRepo(pool).DropGraph(ctx, scope.graphName)
	userErr := deleteUser(ctx, pool, scope.userID)
	return errors.Join(graphErr, userErr)
}

func deleteUser(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) error {
	_, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

func waitForTerminalJob(ctx context.Context, repo *repositories.IngestionRepo, jobID uuid.UUID, timeout time.Duration) (*models.IngestionJob, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		job, err := repo.FindByID(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("read ingestion job: %w", err)
		}
		if job.Status == "completed" || job.Status == "failed" {
			return job, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("ingestion job %s did not reach a terminal status within %s", jobID, timeout)
		case <-ticker.C:
		}
	}
}

func duplicateRows(ctx context.Context, pool *pgxpool.Pool, universeID uuid.UUID) ([]naturalKeyDuplicate, error) {
	rows, err := pool.Query(ctx, duplicateNaturalKeyQuery, universeID)
	if err != nil {
		return nil, fmt.Errorf("query duplicate natural keys: %w", err)
	}
	defer rows.Close()
	duplicates := []naturalKeyDuplicate{}
	for rows.Next() {
		var row naturalKeyDuplicate
		if err := rows.Scan(&row.Name, &row.Type, &row.Count); err != nil {
			return nil, fmt.Errorf("scan duplicate natural key: %w", err)
		}
		duplicates = append(duplicates, row)
	}
	return duplicates, rows.Err()
}

func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(math.Ceil(q*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func writeReport(path string, report *liveReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func fail(path string, report *liveReport, err error) {
	report.Error = err.Error()
	if writeErr := writeReport(path, report); writeErr != nil {
		log.Fatalf("%v; additionally failed to write live ingestion performance report: %v", err, writeErr)
	}
	log.Fatal(err)
}
