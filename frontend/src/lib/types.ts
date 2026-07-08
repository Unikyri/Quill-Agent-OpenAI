// Shared types used by both api.ts and memory-related components. Extracted
// from a former duplicate in ContextPanel.tsx + api.ts (S3 Memory Theater
// design, obs #265) — ContextPanel keeps its own inline copy per spec
// non-goal, this file exists so no THIRD copy appears.

export type Lifecycle = 'active' | 'decaying' | 'archived' | 'consolidated' | 'reactivated'

export interface MemoryHistoryPoint {
  score: number
  recorded_at: string
}

export interface MemoryStatusEntity {
  id: string
  name: string
  type: string
  relevance_score: number
  status: string
  consolidated: boolean
  lifecycle: Lifecycle
  history: MemoryHistoryPoint[]
}

// Mirrors backend RRFContribution/ExplainedItem (fuse_rrf_explain.go) and the
// /recall/explain response shape. Centralized here (S3 Memory Theater Slice D,
// obs #265) so api.ts, FusionExplorer, and BudgetTheater share one definition.
export interface RRFContribution {
  pipeline: string
  rank: number
  delta: number
}

export interface ExplainedItem {
  id: string
  entity_id: string
  fact: string
  rrf_score: number
  contributions: RRFContribution[]
  fit_in_budget: boolean
}

export interface RecallBudget {
  max_context_tokens: number
  available: number
  entities_tokens: number
  vector_tokens: number
  tools_tokens: number
  used_percent: number
}

export interface RecallExplanation {
  query: string
  pipeline_sizes: Record<string, number>
  items: ExplainedItem[]
  budget: RecallBudget
}
