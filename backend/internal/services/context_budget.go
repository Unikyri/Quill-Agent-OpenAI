package services

import "sort"

// RankedItem is a piece of context text with a relevance score used to
// prioritize inclusion under a token budget.
type RankedItem struct {
	Text  string
	Score float64
}

// BudgetAllocation is the proportional token split computed by
// ContextBudgetManager.ComputeBudget.
type BudgetAllocation struct {
	EntitiesTokens int
	VectorTokens   int
	ToolsTokens    int
	Available      int
}

// BudgetReport is a serializable summary of a BudgetAllocation, suitable for
// sending over the WS progress channel (BudgetAllocation itself has no
// maxContext/percent-used view baked in).
type BudgetReport struct {
	MaxContextTokens int     `json:"max_context_tokens"`
	Available        int     `json:"available"`
	EntitiesTokens   int     `json:"entities_tokens"`
	VectorTokens     int     `json:"vector_tokens"`
	ToolsTokens      int     `json:"tools_tokens"`
	UsedPercent      float64 `json:"used_percent"`
}

// Report summarizes a into a BudgetReport against maxContext, the context
// window size the allocation was computed under. UsedPercent is the share of
// maxContext consumed by system/user/response overhead (maxContext -
// Available), floored at 0 to avoid division by zero when maxContext is 0.
func (a BudgetAllocation) Report(maxContext int) BudgetReport {
	var usedPercent float64
	if maxContext > 0 {
		used := maxContext - a.Available
		usedPercent = float64(used) / float64(maxContext) * 100
	}
	return BudgetReport{
		MaxContextTokens: maxContext,
		Available:        a.Available,
		EntitiesTokens:   a.EntitiesTokens,
		VectorTokens:     a.VectorTokens,
		ToolsTokens:      a.ToolsTokens,
		UsedPercent:      usedPercent,
	}
}

// ContextBudgetManager allocates a fixed context-window token budget across
// entities, vector memories, and tool results, and fits ranked items into a
// given budget.
type ContextBudgetManager struct {
	tok              *Tokenizer
	maxContextTokens int
	responseReserve  int
}

// NewContextBudgetManager builds a ContextBudgetManager bound to a tokenizer
// and the context window limits.
func NewContextBudgetManager(tok *Tokenizer, maxContextTokens, responseReserve int) *ContextBudgetManager {
	return &ContextBudgetManager{
		tok:              tok,
		maxContextTokens: maxContextTokens,
		responseReserve:  responseReserve,
	}
}

// ComputeBudget reserves systemTokens, userBaseTokens, and responseReserve
// from maxContextTokens, then splits the remainder 35% entities / 40%
// vector / 25% tools. Available and each split are floored at 0.
func (b *ContextBudgetManager) ComputeBudget(systemTokens, userBaseTokens int) BudgetAllocation {
	available := b.maxContextTokens - b.responseReserve - systemTokens - userBaseTokens
	if available < 0 {
		available = 0
	}

	return BudgetAllocation{
		EntitiesTokens: available * 35 / 100,
		VectorTokens:   available * 40 / 100,
		ToolsTokens:    available * 25 / 100,
		Available:      available,
	}
}

// FitToBudget sorts items by Score descending, then greedily includes items
// while staying within budget. It uses continue (not break) on overflow so
// a smaller later item can still fit after a larger one is skipped.
func (b *ContextBudgetManager) FitToBudget(items []RankedItem, budget int) (fitted []RankedItem, dropped, tokensUsed int) {
	if len(items) == 0 {
		return nil, 0, 0
	}

	sorted := make([]RankedItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })

	for _, item := range sorted {
		itemTokens := b.tok.CountTokens(item.Text)
		if tokensUsed+itemTokens > budget {
			dropped++
			continue
		}
		fitted = append(fitted, item)
		tokensUsed += itemTokens
	}
	return fitted, dropped, tokensUsed
}
