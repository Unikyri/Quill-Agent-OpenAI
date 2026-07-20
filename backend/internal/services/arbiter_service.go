package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/quill/backend/internal/models"
)

// ArbiterService is a fourth, consensus-forming agent that sits above the two
// specialist analysts already fanned out per paragraph — the Continuity
// Analyst (ContradictionService) and the Plot-Hole Evaluator
// (PlotHoleService). Each specialist reasons independently and never sees the
// other's findings, so two overlapping or conflicting-severity findings can
// reach the writer with no adjudication between them. ArbiterService reviews
// both sets together and writes one short, prioritized note.
type ArbiterService struct {
	qwenSvc LLMService
}

// NewArbiterService constructs an arbiter. qwenSvc may be nil for testing;
// Adjudicate becomes a no-op in that case.
func NewArbiterService(qwenSvc LLMService) *ArbiterService {
	return &ArbiterService{qwenSvc: qwenSvc}
}

// Adjudicate reviews one paragraph's raw findings and returns a short
// synthesis for the writer. Best-effort and nil-safe: no configured model or
// no findings at all returns ("", nil) rather than an error — this is a
// presentation enrichment on top of the two specialists' own results, never
// a gate on them being surfaced.
func (s *ArbiterService) Adjudicate(ctx context.Context, contradictions []models.Contradiction, plotHoles []models.PlotHole) (string, error) {
	if s.qwenSvc == nil || (len(contradictions) == 0 && len(plotHoles) == 0) {
		return "", nil
	}

	var findings strings.Builder
	for i, c := range contradictions {
		fmt.Fprintf(&findings, "Contradiction %d: %s\n", i+1, c.Description)
	}
	for i, p := range plotHoles {
		fmt.Fprintf(&findings, "Plot hole %d: %s\n", i+1, p.Description)
	}

	systemPrompt := `You are the adjudicator reviewing raw findings from two other narrative analysts: a continuity analyst (contradictions) and a plot-hole evaluator. You did not generate these findings yourself.

Given their findings for one paragraph, write ONE short note for the writer (2-3 sentences maximum):
- Say which finding matters most right now and why.
- If two findings actually describe the same underlying issue, say so instead of repeating both.
Do not restate every finding verbatim — synthesize a verdict, don't summarize a list.`

	messages := []QwenMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: findings.String()},
	}

	// No tools: adjudication reasons over findings already in hand, it does
	// not need to go search memory or the graph for more — RunAgentLoop's
	// empty-tools path is a single chat completion, so this stays a real,
	// tool-capable agent call without paying for a loop it doesn't need.
	result, err := s.qwenSvc.RunAgentLoop(ctx, messages, nil, nil, 1)
	if err != nil {
		return "", fmt.Errorf("adjudicate findings: %w", err)
	}
	return strings.TrimSpace(result), nil
}
