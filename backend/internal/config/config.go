package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL      string
	DBMaxConnections int
	DBMaxIdleConns   int
	QwenAPIKey       string
	QwenBaseURL      string
	// LLMProtocol selects the wire client at composition time. "openai" keeps
	// the Sprint 2 compatible endpoint; "dashscope" opts into the native
	// DashScope HTTP protocol without changing callers.
	LLMProtocol       string
	QwenNativeBaseURL string
	// Role-based model configuration. These names are the supported public API.
	QwenExtractionModel string
	QwenReasoningModel  string
	QwenCraftModel      string
	QwenFallbackModel   string
	QwenFallbackOn429   bool
	QwenRerankModel     string
	// Deprecated compatibility aliases. Keep these until all deployments have
	// migrated from QWEN_MAX_MODEL/QWEN_TURBO_MODEL.
	QwenMaxModel                       string
	QwenTurboModel                     string
	QwenEmbeddingModel                 string
	QwenEmbeddingDims                  int
	SkillDir                           string
	JWTSecret                          string
	JWTExpirationHours                 int
	BCryptCost                         int
	Port                               string
	FrontendURL                        string
	AllowedOrigins                     string
	MaxUploadSizeMB                    int
	UploadDir                          string
	DebounceSeconds                    int
	QwenMaxConcurrency                 int
	QwenTurboConcurrency               int
	LLMTPMTurbo                        int
	LLMTPMMax                          int
	LLMRPM                             int
	LLMInteractiveReserve              float64
	LLMMaxConcurrency                  int
	LLMRampStep                        int
	QwenRetryMaxAttempts               int
	QwenHealthTimeout                  time.Duration
	QwenAPITimeout                     time.Duration
	DecayLambda                        float64
	ArchiveThreshold                   float64
	RelevanceDeltaEpsilon              float64
	PlotHoleChapters                   int
	MaxContradictionCandidates         int
	WSEnabled                          bool
	MaxContextTokens                   int
	ResponseReserve                    int
	ContradictionAgentDepth            int
	PlotHoleAgentDepth                 int
	IngestAnalysisMaxChapters          int
	WriterPreferencePromotionThreshold int
	EntityConfidenceThreshold          float64
}

func Load() (*Config, error) {
	protocol := strings.ToLower(strings.TrimSpace(getEnv("LLM_PROTOCOL", "openai")))
	if protocol == "" {
		protocol = "openai"
	}
	if protocol != "openai" && protocol != "dashscope" {
		return nil, fmt.Errorf("LLM_PROTOCOL must be openai or dashscope, got %q", protocol)
	}

	baseURL := getEnv("QWEN_BASE_URL", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1")
	nativeBaseURL := os.Getenv("QWEN_NATIVE_BASE_URL")
	if nativeBaseURL == "" {
		nativeBaseURL = deriveNativeBaseURL(baseURL)
	}

	cfg := &Config{
		DatabaseURL:                        getEnv("DATABASE_URL", "postgres://quill:quill_dev_password@localhost:5432/quill?sslmode=disable"),
		DBMaxConnections:                   getEnvInt("DB_MAX_CONNECTIONS", 8),
		DBMaxIdleConns:                     getEnvInt("DB_MAX_IDLE_CONNECTIONS", 5),
		QwenAPIKey:                         os.Getenv("QWEN_API_KEY"),
		QwenBaseURL:                        baseURL,
		LLMProtocol:                        protocol,
		QwenNativeBaseURL:                  nativeBaseURL,
		QwenExtractionModel:                getEnvWithLegacy("QWEN_EXTRACTION_MODEL", "QWEN_TURBO_MODEL", "qwen-turbo"),
		QwenReasoningModel:                 getEnvWithLegacy("QWEN_REASONING_MODEL", "QWEN_MAX_MODEL", "qwen-max"),
		QwenCraftModel:                     getEnv("QWEN_CRAFT_MODEL", getEnvWithLegacy("QWEN_REASONING_MODEL", "QWEN_MAX_MODEL", "qwen-max")),
		QwenFallbackModel:                  getEnv("QWEN_FALLBACK_MODEL", ""),
		QwenFallbackOn429:                  getEnvBool("QWEN_FALLBACK_ON_429", false),
		QwenRerankModel:                    getEnv("QWEN_RERANK_MODEL", "qwen3-rerank"),
		QwenEmbeddingModel:                 getEnv("QWEN_EMBEDDING_MODEL", "text-embedding-v4"),
		QwenEmbeddingDims:                  getEnvInt("QWEN_EMBEDDING_DIMENSIONS", 1024),
		SkillDir:                           getEnv("SKILL_DIR", "./skills"),
		JWTSecret:                          getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		JWTExpirationHours:                 getEnvInt("JWT_EXPIRATION_HOURS", 24),
		BCryptCost:                         getEnvInt("BCRYPT_COST", 12),
		Port:                               getEnv("PORT", "8080"),
		FrontendURL:                        getEnv("FRONTEND_URL", "http://localhost:3000"),
		AllowedOrigins:                     getEnv("ALLOWED_ORIGINS", "http://localhost:3000"),
		MaxUploadSizeMB:                    getEnvInt("MAX_UPLOAD_SIZE_MB", 10),
		UploadDir:                          getEnv("UPLOAD_DIR", "./uploads"),
		DebounceSeconds:                    getEnvInt("DEBOUNCE_SECONDS", 5),
		QwenMaxConcurrency:                 getEnvInt("QWEN_MAX_CONCURRENCY", 3),
		QwenTurboConcurrency:               getEnvInt("QWEN_TURBO_CONCURRENCY", 5),
		LLMTPMTurbo:                        getEnvInt("LLM_TPM_TURBO", 5_000_000),
		LLMTPMMax:                          getEnvInt("LLM_TPM_MAX", 1_000_000),
		LLMRPM:                             getEnvInt("LLM_RPM", 600),
		LLMInteractiveReserve:              getEnvFloat("LLM_INTERACTIVE_RESERVE", 0.30),
		LLMMaxConcurrency:                  getEnvInt("LLM_MAX_CONCURRENCY", 5),
		LLMRampStep:                        getEnvInt("LLM_RAMP_STEP", 1),
		QwenRetryMaxAttempts:               getEnvInt("QWEN_RETRY_MAX_ATTEMPTS", 3),
		QwenHealthTimeout:                  time.Duration(getEnvInt("QWEN_HEALTH_TIMEOUT_SECONDS", 5)) * time.Second,
		QwenAPITimeout:                     time.Duration(getEnvInt("QWEN_API_TIMEOUT_SECONDS", 120)) * time.Second,
		DecayLambda:                        getEnvFloat("DECAY_LAMBDA", 0.1),
		ArchiveThreshold:                   getEnvFloat("ARCHIVE_THRESHOLD", 0.15),
		RelevanceDeltaEpsilon:              getEnvFloat("RELEVANCE_DELTA_EPSILON", 0.01),
		PlotHoleChapters:                   getEnvInt("PLOT_HOLE_CHAPTERS", 8),
		MaxContradictionCandidates:         getEnvInt("MAX_CONTRADICTION_CANDIDATES", 3),
		WSEnabled:                          getEnvBool("QUILL_WS_ENABLED", true),
		MaxContextTokens:                   getEnvInt("QWEN_MAX_CONTEXT_TOKENS", 50000),
		ResponseReserve:                    getEnvInt("QWEN_RESPONSE_RESERVE", 2000),
		ContradictionAgentDepth:            getEnvInt("CONTRADICTION_AGENT_DEPTH", 3),
		PlotHoleAgentDepth:                 getEnvInt("PLOT_HOLE_AGENT_DEPTH", 2),
		IngestAnalysisMaxChapters:          getEnvInt("INGEST_ANALYSIS_MAX_CHAPTERS", 25),
		WriterPreferencePromotionThreshold: getEnvInt("WRITER_PREFERENCE_PROMOTION_THRESHOLD", 3),
		EntityConfidenceThreshold:          getEnvFloat("ENTITY_CONFIDENCE_THRESHOLD", 0.70),
	}
	// Populate compatibility aliases from the canonical role fields. Code that
	// still constructs Config with the old fields remains supported by the
	// service constructor, but loaded configuration has one source of truth.
	cfg.QwenMaxModel = cfg.QwenReasoningModel
	cfg.QwenTurboModel = cfg.QwenExtractionModel

	if cfg.QwenAPIKey == "" {
		return nil, fmt.Errorf("QWEN_API_KEY environment variable is required")
	}

	return cfg, nil
}

func deriveNativeBaseURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	base = strings.TrimSuffix(base, "/compatible-mode/v1")
	base = strings.TrimSuffix(base, "/api/v1")
	return base
}

func getEnvWithLegacy(key, legacyKey, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return getEnv(legacyKey, fallback)
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
