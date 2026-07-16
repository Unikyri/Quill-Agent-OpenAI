package services

import "testing"

func TestComputeBudgetSplit(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	// systemTokens + userBaseTokens chosen so available = 10000.
	alloc := mgr.ComputeBudget(9000, 9000)

	if alloc.Available != 10000 {
		t.Fatalf("Available = %d, want 10000", alloc.Available)
	}
	if alloc.EntitiesTokens != 3500 {
		t.Errorf("EntitiesTokens = %d, want 3500", alloc.EntitiesTokens)
	}
	if alloc.VectorTokens != 4000 {
		t.Errorf("VectorTokens = %d, want 4000", alloc.VectorTokens)
	}
	if alloc.ToolsTokens != 2500 {
		t.Errorf("ToolsTokens = %d, want 2500", alloc.ToolsTokens)
	}

	sum := alloc.EntitiesTokens + alloc.VectorTokens + alloc.ToolsTokens
	if sum > alloc.Available {
		t.Errorf("sum of splits %d exceeds Available %d", sum, alloc.Available)
	}
}

func TestComputeBudgetClampsToZero(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	// system + user + reserve far exceeds max context window.
	alloc := mgr.ComputeBudget(20000, 20000)

	if alloc.Available != 0 {
		t.Errorf("Available = %d, want 0", alloc.Available)
	}
	if alloc.EntitiesTokens != 0 || alloc.VectorTokens != 0 || alloc.ToolsTokens != 0 {
		t.Errorf("splits should be 0 when Available is 0, got entities=%d vector=%d tools=%d",
			alloc.EntitiesTokens, alloc.VectorTokens, alloc.ToolsTokens)
	}
}

func TestFitToBudgetGreedyDropsOverflow(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	items := []RankedItem{
		{Text: "a very long entity description that takes up a lot of tokens indeed", Score: 0.9},
		{Text: "short", Score: 0.8},
		{Text: "another very long entity description that also takes many tokens up", Score: 0.5},
		{Text: "tiny", Score: 0.1},
	}

	budgetForShortAndTiny := mgr.tok.CountTokens("short") + mgr.tok.CountTokens("tiny") + 1

	fitted, dropped, tokensUsed := mgr.FitToBudget(items, budgetForShortAndTiny)

	if tokensUsed > budgetForShortAndTiny {
		t.Errorf("tokensUsed = %d, want <= budget %d", tokensUsed, budgetForShortAndTiny)
	}
	if dropped != 2 {
		t.Errorf("dropped = %d, want 2 (both long items)", dropped)
	}
	if len(fitted) != 2 {
		t.Fatalf("len(fitted) = %d, want 2", len(fitted))
	}
	for _, f := range fitted {
		if f.Text != "short" && f.Text != "tiny" {
			t.Errorf("unexpected item retained: %q", f.Text)
		}
	}
}

func TestFitToBudgetAllFit(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	items := []RankedItem{
		{Text: "alpha", Score: 0.9},
		{Text: "beta", Score: 0.5},
		{Text: "gamma", Score: 0.1},
	}

	fitted, dropped, tokensUsed := mgr.FitToBudget(items, 10000)

	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
	if len(fitted) != len(items) {
		t.Errorf("len(fitted) = %d, want %d", len(fitted), len(items))
	}

	var expected int
	for _, it := range items {
		expected += tok.CountTokens(it.Text)
	}
	if tokensUsed != expected {
		t.Errorf("tokensUsed = %d, want %d", tokensUsed, expected)
	}
}

// TestBudgetAllocationReportPercentMath verifies BudgetAllocation.Report's
// percentage math: UsedPercent reflects how much of maxContext was consumed
// by system/user/reserve overhead (i.e. maxContext - Available), not the
// splits themselves.
func TestBudgetAllocationReportPercentMath(t *testing.T) {
	alloc := BudgetAllocation{
		EntitiesTokens: 3500,
		VectorTokens:   4000,
		ToolsTokens:    2500,
		Available:      10000,
	}

	report := alloc.Report(30000)

	if report.MaxContextTokens != 30000 {
		t.Errorf("MaxContextTokens = %d, want 30000", report.MaxContextTokens)
	}
	if report.Available != 10000 {
		t.Errorf("Available = %d, want 10000", report.Available)
	}
	if report.EntitiesTokens != 3500 || report.VectorTokens != 4000 || report.ToolsTokens != 2500 {
		t.Errorf("splits not carried through: %+v", report)
	}
	// used = 30000 - 10000 = 20000; usedPercent = 20000/30000*100 = 66.666...
	wantPercent := float64(30000-10000) / float64(30000) * 100
	if report.UsedPercent != wantPercent {
		t.Errorf("UsedPercent = %v, want %v", report.UsedPercent, wantPercent)
	}
}

// TestBudgetAllocationReportZeroMaxContext guards against division by zero.
func TestBudgetAllocationReportZeroMaxContext(t *testing.T) {
	alloc := BudgetAllocation{Available: 0}
	report := alloc.Report(0)
	if report.UsedPercent != 0 {
		t.Errorf("UsedPercent = %v, want 0 when maxContext is 0", report.UsedPercent)
	}
}

func TestFitToBudgetEmpty(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	fitted, dropped, tokensUsed := mgr.FitToBudget(nil, 1000)

	if fitted != nil {
		t.Errorf("fitted = %v, want nil", fitted)
	}
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
	if tokensUsed != 0 {
		t.Errorf("tokensUsed = %d, want 0", tokensUsed)
	}
}

func TestTruncateToTokensEmpty(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	if got := mgr.TruncateToTokens("", 100); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
	if got := mgr.TruncateToTokens("some text", 0); got != "" {
		t.Errorf("budget=0: got %q, want empty string", got)
	}
}

func TestTruncateToTokensFitsExactly(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	text := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	budget := tok.CountTokens(text) + 10 // comfortably fits all of it

	got := mgr.TruncateToTokens(text, budget)
	if got != text {
		t.Errorf("expected all paragraphs preserved in order, got %q", got)
	}
}

func TestTruncateToTokensDropsTrailingParagraphs(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	p1 := "Alpha paragraph with some words."
	p2 := "Beta paragraph with some more words."
	p3 := "Gamma paragraph, the last one, with even more words than the others."
	text := p1 + "\n\n" + p2 + "\n\n" + p3

	budget := tok.CountTokens(p1) + tok.CountTokens(p2) + 1 // fits p1+p2, not p3

	got := mgr.TruncateToTokens(text, budget)
	if got != p1+"\n\n"+p2 {
		t.Errorf("got %q, want %q", got, p1+"\n\n"+p2)
	}
}

// TestTruncateToTokensSingleParagraphOverBudget pins the decided behavior for
// the previously-unpinned edge case: a single paragraph alone larger than the
// budget is truncated at the token level rather than dropped to empty text.
func TestTruncateToTokensSingleParagraphOverBudget(t *testing.T) {
	tok := NewTokenizer()
	mgr := NewContextBudgetManager(tok, 30000, 2000)

	bigParagraph := ""
	for i := 0; i < 500; i++ {
		bigParagraph += "word "
	}

	budget := 10
	got := mgr.TruncateToTokens(bigParagraph, budget)

	if got == "" {
		t.Fatal("expected a non-empty truncated result, got empty string")
	}
	if tok.CountTokens(got) > budget {
		t.Errorf("truncated result has %d tokens, want <= %d", tok.CountTokens(got), budget)
	}
	if len(got) >= len(bigParagraph) {
		t.Errorf("expected truncation to actually shorten the text")
	}
}
