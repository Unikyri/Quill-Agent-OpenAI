package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quill/backend/internal/config"
)

func TestValidateLiveConfigRejectsUnsafeOrIncompleteSetup(t *testing.T) {
	valid := &config.Config{
		DatabaseURL:        "postgres://quill:password@localhost:5432/quill?sslmode=disable",
		QwenAPIKey:         "test-key",
		QwenEmbeddingModel: "text-embedding-v4",
		QwenEmbeddingDims:  1024,
	}
	if err := validateLiveConfig(valid); err != nil {
		t.Fatalf("valid config: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*config.Config)
	}{
		{"missing key", func(cfg *config.Config) { cfg.QwenAPIKey = "" }},
		{"missing embedding model", func(cfg *config.Config) { cfg.QwenEmbeddingModel = "" }},
		{"invalid dimensions", func(cfg *config.Config) { cfg.QwenEmbeddingDims = 0 }},
		{"non-postgres database", func(cfg *config.Config) { cfg.DatabaseURL = "https://example.com" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *valid
			tc.mutate(&cfg)
			if err := validateLiveConfig(&cfg); err == nil {
				t.Fatal("validateLiveConfig returned nil")
			}
		})
	}
}

func TestLivePercentileUsesNearestRank(t *testing.T) {
	values := []float64{1, 2, 3}
	if got := percentile(values, .50); got != 2 {
		t.Fatalf("p50 = %v, want 2", got)
	}
	if got := percentile(values, .95); got != 3 {
		t.Fatalf("p95 = %v, want 3", got)
	}
}

func TestModelMetadataRecordsEmbeddingV4(t *testing.T) {
	models, values := modelMetadata(&config.Config{
		QwenExtractionModel: "qwen-turbo",
		QwenReasoningModel:  "qwen-max",
		QwenEmbeddingModel:  "text-embedding-v4",
		QwenEmbeddingDims:   1024,
	})
	if models["embedding"] != "text-embedding-v4" {
		t.Fatalf("embedding metadata = %q", models["embedding"])
	}
	if values["qwen_embedding_dimensions"] != "1024" {
		t.Fatalf("embedding dimensions metadata = %q", values["qwen_embedding_dimensions"])
	}
}

func TestWriteReportReturnsErrorForUnwritableOutputParent(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("file"), 0o644); err != nil {
		t.Fatalf("create output parent file: %v", err)
	}
	path := filepath.Join(parentFile, "report.json")
	if err := writeReport(path, &liveReport{Mode: "live-ingestion"}); err == nil {
		t.Fatal("writeReport returned nil for an unwritable output parent")
	}
}
