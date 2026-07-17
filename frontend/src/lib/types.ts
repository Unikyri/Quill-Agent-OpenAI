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
  pre_rerank_position?: number
  post_rerank_position?: number
  rerank_delta?: number
  rerank_score?: number
}

export interface RecallBudget {
  max_context_tokens: number
  available: number
  entities_tokens: number
  vector_tokens: number
  tools_tokens: number
  used_percent: number
  vector_tokens_used: number
}

// Mirrors backend models.IngestionJob JSON tags (models.go).
export interface IngestionJobDTO {
  id: string
  universe_id: string
  work_id: string
  filename?: string
  file_type?: string
  status: string
  total_chapters_detected: number
  chapters_processed: number
  entities_extracted: number
  content_hash?: string
  error_message?: string
  started_at?: string
  completed_at?: string
  created_at: string
}

export interface RecallExplanation {
  query: string
  pipeline_sizes: Record<string, number>
  items: ExplainedItem[]
  budget: RecallBudget
}

export interface WriterObservationDTO {
  id: string
  user_id: string
  universe_id?: string
  metric: string
  value: number
  sample_size: number
  computed_at: string
}

export interface WriterPreferenceDTO {
  id: string
  user_id: string
  statement: string
  scope: 'universal' | 'genre_bound'
  genre_tags: string[]
  confidence: number
  relevance_score: number
  lifecycle: 'active' | 'archived'
  last_reinforced_at: string
  observation_ids: string[]
  feedback_event_ids: string[]
  created_at: string
}

export interface WriterFeedbackEventDTO {
  id: string
  user_id: string
  universe_id?: string
  chapter_id?: string
  note_id?: string
  signal: 'accept' | 'reject' | 'behavioural_accept'
  preference_id?: string
  payload: Record<string, unknown>
  created_at: string
}

export interface WriterPreferenceHistoryDTO {
  id: string
  user_id: string
  preference_id?: string
  relevance_score: number
  confidence: number
  lifecycle: 'active' | 'archived'
  recorded_at: string
}

export interface WriterPreferenceEvidenceDTO {
  preference: WriterPreferenceDTO
  observations: WriterObservationDTO[]
  feedback_events: WriterFeedbackEventDTO[]
  history: WriterPreferenceHistoryDTO[]
}

export interface SkillCatalogueItem {
  name: string
  description: string
  genre_tags: string[]
  stage: string
}

export interface UniverseSkillDTO {
  universe_id: string
  skill_name: string
  activated_at: string
}

export interface CraftReviewSelection {
  skill: string
  rationale: string
}

export interface CraftReviewNote {
  id: string
  skill: string
  quote: string
  note: string
  severity: 'info' | 'suggestion' | 'warning' | string
  category?: string
}

export interface CraftReviewResult {
  universe_id: string
  work_id: string
  chapter_id: string
  selections: CraftReviewSelection[]
  notes: CraftReviewNote[]
}
