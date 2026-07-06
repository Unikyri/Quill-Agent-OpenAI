package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL          string
	DBMaxConnections     int
	DBMaxIdleConns       int
	QwenAPIKey           string
	QwenBaseURL          string
	QwenMaxModel         string
	QwenTurboModel       string
	QwenEmbeddingModel   string
	QwenEmbeddingDims    int
	JWTSecret            string
	JWTExpirationHours   int
	BCryptCost           int
	Port                 string
	FrontendURL          string
	AllowedOrigins       string
	MaxUploadSizeMB      int
	UploadDir            string
	DebounceSeconds      int
	QwenMaxConcurrency   int
	QwenTurboConcurrency int
	QwenHealthTimeout    time.Duration
	DecayLambda              float64
	ArchiveThreshold         float64
	PlotHoleChapters         int
	MaxContradictionCandidates int
	WSEnabled                bool
	MaxContextTokens         int
	ResponseReserve          int
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://quill:quill_dev_password@localhost:5432/quill?sslmode=disable"),
		DBMaxConnections:     getEnvInt("DB_MAX_CONNECTIONS", 25),
		DBMaxIdleConns:       getEnvInt("DB_MAX_IDLE_CONNECTIONS", 5),
		QwenAPIKey:           os.Getenv("QWEN_API_KEY"),
		QwenBaseURL:          getEnv("QWEN_BASE_URL", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"),
		QwenMaxModel:         getEnv("QWEN_MAX_MODEL", "qwen-max"),
		QwenTurboModel:       getEnv("QWEN_TURBO_MODEL", "qwen-turbo"),
		QwenEmbeddingModel:   getEnv("QWEN_EMBEDDING_MODEL", "text-embedding-v3"),
		QwenEmbeddingDims:    getEnvInt("QWEN_EMBEDDING_DIMENSIONS", 1024),
		JWTSecret:            getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		JWTExpirationHours:   getEnvInt("JWT_EXPIRATION_HOURS", 24),
		BCryptCost:           getEnvInt("BCRYPT_COST", 12),
		Port:                 getEnv("PORT", "8080"),
		FrontendURL:          getEnv("FRONTEND_URL", "http://localhost:3000"),
		AllowedOrigins:       getEnv("ALLOWED_ORIGINS", "http://localhost:3000"),
		MaxUploadSizeMB:      getEnvInt("MAX_UPLOAD_SIZE_MB", 10),
		UploadDir:            getEnv("UPLOAD_DIR", "./uploads"),
		DebounceSeconds:      getEnvInt("DEBOUNCE_SECONDS", 5),
		QwenMaxConcurrency:   getEnvInt("QWEN_MAX_CONCURRENCY", 3),
		QwenTurboConcurrency: getEnvInt("QWEN_TURBO_CONCURRENCY", 5),
		QwenHealthTimeout:         time.Duration(getEnvInt("QWEN_HEALTH_TIMEOUT_SECONDS", 5)) * time.Second,
		DecayLambda:               getEnvFloat("DECAY_LAMBDA", 0.1),
		ArchiveThreshold:          getEnvFloat("ARCHIVE_THRESHOLD", 0.15),
		PlotHoleChapters:          getEnvInt("PLOT_HOLE_CHAPTERS", 8),
		MaxContradictionCandidates: getEnvInt("MAX_CONTRADICTION_CANDIDATES", 3),
		WSEnabled:                 getEnvBool("QUILL_WS_ENABLED", true),
		MaxContextTokens:          getEnvInt("QWEN_MAX_CONTEXT_TOKENS", 30000),
		ResponseReserve:           getEnvInt("QWEN_RESPONSE_RESERVE", 2000),
	}

	if cfg.QwenAPIKey == "" {
		return nil, fmt.Errorf("QWEN_API_KEY environment variable is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}
