package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// QueryReport holds per-query recall metrics.
type QueryReport struct {
	ID           string
	Query        string
	RecallAt5    float64
	PrecisionAt5 float64
	MRR          float64
	NDCGAt5      float64
}

// RecallReport holds the full evaluation report.
type RecallReport struct {
	Timestamp string
	Queries   []QueryReport
}

// writeRecallReport writes a markdown table of recall metrics to the given path.
func writeRecallReport(t *testing.T, path string, report RecallReport) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir report dir: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	defer f.Close()

	if report.Timestamp == "" {
		report.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	fmt.Fprintln(f, "# Memory Recall Evaluation Report")
	fmt.Fprintf(f, "\nGenerated: %s\n\n", report.Timestamp)
	fmt.Fprintln(f, "| Query | recall@5 | precision@5 | MRR | nDCG@5 |")
	fmt.Fprintln(f, "|-------|----------|-------------|-----|--------|")
	for _, q := range report.Queries {
		fmt.Fprintf(f, "| %s | %.3f | %.3f | %.3f | %.3f |\n",
			q.Query, q.RecallAt5, q.PrecisionAt5, q.MRR, q.NDCGAt5)
	}
}
