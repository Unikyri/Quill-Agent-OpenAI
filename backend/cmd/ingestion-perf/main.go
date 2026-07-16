// Command ingestion-perf creates deterministic manuscript fixtures and records
// repeatable ingestion-harness metadata. It deliberately does not call Qwen
// unless a future live runner is configured; this keeps local plumbing checks
// free of quota/cost while making external prerequisites visible in the report.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type report struct {
	GeneratedAt              string            `json:"generated_at"`
	Mode                     string            `json:"mode"`
	Pages                    int               `json:"pages"`
	Words                    int               `json:"words"`
	Chunks                   int               `json:"chunks"`
	Runs                     int               `json:"runs"`
	WallSeconds              []float64         `json:"wall_seconds"`
	P50Seconds               float64           `json:"p50_seconds"`
	P95Seconds               float64           `json:"p95_seconds"`
	ChunksPerSecond          float64           `json:"chunks_per_second"`
	Models                   map[string]string `json:"models"`
	Config                   map[string]string `json:"config"`
	DuplicateNaturalKeyQuery string            `json:"duplicate_natural_key_query"`
	ExternalChecks           map[string]string `json:"external_checks"`
}

func main() {
	pages := flag.Int("pages", 50, "fixture size: 50 or 400 pages")
	runs := flag.Int("runs", 3, "number of deterministic plumbing runs")
	output := flag.String("output", "", "report path (default: artifacts/ingestion-perf-<pages>.json)")
	fixture := flag.String("fixture", "", "optional path to write the generated Markdown fixture")
	flag.Parse()
	if *pages != 50 && *pages != 400 {
		panic("-pages must be 50 or 400")
	}
	if *runs < 1 {
		panic("-runs must be at least 1")
	}

	content := fixtureContent(*pages)
	if *fixture != "" {
		must(os.MkdirAll(filepath.Dir(*fixture), 0o755))
		must(os.WriteFile(*fixture, []byte(content), 0o644))
	}
	samples := make([]float64, 0, *runs)
	chunks := 0
	for range *runs {
		start := time.Now()
		chunks = strings.Count(content, "\n# Chapter ")
		if strings.HasPrefix(content, "# Chapter ") {
			chunks++
		}
		_ = strings.Fields(content)
		samples = append(samples, time.Since(start).Seconds())
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	median := percentile(sorted, .50)
	p95 := percentile(sorted, .95)
	words := len(strings.Fields(content))
	path := *output
	if path == "" {
		path = filepath.Join("artifacts", fmt.Sprintf("ingestion-perf-%d-page.json", *pages))
	}
	r := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Mode: "deterministic-plumbing", Pages: *pages, Words: words, Chunks: chunks, Runs: *runs,
		WallSeconds: samples, P50Seconds: median, P95Seconds: p95,
		ChunksPerSecond:          float64(chunks) / max(median, 0.000001),
		Models:                   map[string]string{"extraction": env("QWEN_EXTRACTION_MODEL", "qwen-turbo"), "reasoning": env("QWEN_REASONING_MODEL", "qwen-max"), "embedding": env("QWEN_EMBEDDING_MODEL", "text-embedding-v4")},
		Config:                   map[string]string{"llm_tpm_turbo": env("LLM_TPM_TURBO", "5000000"), "llm_tpm_max": env("LLM_TPM_MAX", "1000000"), "llm_rpm": env("LLM_RPM", "600"), "llm_interactive_reserve": env("LLM_INTERACTIVE_RESERVE", "0.30"), "llm_max_concurrency": env("LLM_MAX_CONCURRENCY", "5"), "llm_ramp_step": env("LLM_RAMP_STEP", "1"), "qwen_retry_max_attempts": env("QWEN_RETRY_MAX_ATTEMPTS", "3"), "qwen_embedding_dimensions": env("QWEN_EMBEDDING_DIMENSIONS", "1024")},
		DuplicateNaturalKeyQuery: "SELECT lower(name), type, COUNT(*) FROM entities WHERE universe_id = $1 GROUP BY lower(name), type HAVING COUNT(*) > 1;",
		ExternalChecks:           map[string]string{"live_qwen": "not run: requires a validated QWEN_API_KEY and a full ingestion invocation", "database_duplicate_check": "not run: execute duplicate_natural_key_query after a live ingest", "james_holden": "covered by TestReduceMentionsSeriallyResolvesDuplicatesAndPersistsMentions"},
	}
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	data, err := json.MarshalIndent(r, "", "  ")
	must(err)
	must(os.WriteFile(path, append(data, '\n'), 0o644))
	fmt.Println(path)
}

func fixtureContent(pages int) string {
	const wordsPerPage = 375
	var b strings.Builder
	for page := 1; page <= pages; page++ {
		fmt.Fprintf(&b, "# Chapter %d\n\n", page)
		for word := 0; word < wordsPerPage; word++ {
			if word%75 == 0 {
				b.WriteString("James Holden ")
			} else {
				b.WriteString("prose ")
			}
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	// Nearest-rank makes a small sample conservative: with three runs, p95 is
	// the slowest run instead of silently reporting the median.
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
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
