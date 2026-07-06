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
