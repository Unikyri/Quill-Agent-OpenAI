import { useEffect, useState } from 'react'
import { useWSStore, type WSStatus } from '../../stores/wsStore'
import { NODE_TYPE_META } from '../knowledge-graph/nodeTypeMeta'
import { api } from '../../lib/api'
import styles from './ContextPanel.module.css'

interface ContextPanelProps {
  status: WSStatus
  universeId?: string
}

const PIPELINE_STAGES = [
  { key: 'entities_extracted', label: 'Entities extracted', countKey: 'entity_count' },
  { key: 'checking_contradictions', label: 'Checking contradictions', countKey: null },
  { key: 'contradictions_checked', label: 'Contradictions checked', countKey: 'contradiction_count' },
  { key: 'plot_holes_scanned', label: 'Plot holes scanned', countKey: 'plot_hole_count' },
  { key: 'context_budget', label: 'Context budget', countKey: null },
] as const

const SOURCE_META: Record<string, { color: string }> = {
  vector: { color: 'var(--teal)' },
  graph: { color: 'var(--node-worldrule)' },
  recency: { color: 'var(--gold-ink)' },
  keyword: { color: 'var(--muted)' },
  consolidated: { color: 'var(--node-event)' },
}

const LIFECYCLE_META: Record<string, { color: string; label: string }> = {
  active: { color: 'var(--success-2)', label: 'active' },
  decaying: { color: 'var(--gold-ink)', label: 'decaying' },
  archived: { color: 'var(--muted-3)', label: 'archived' },
  consolidated: { color: 'var(--node-event)', label: 'consolidated' },
  reactivated: { color: 'var(--teal)', label: 'reactivated' },
}

interface MemoryStatusEntity {
  id: string
  name: string
  type: string
  relevance_score: number
  status: string
  consolidated: boolean
  lifecycle: 'active' | 'decaying' | 'archived' | 'consolidated' | 'reactivated'
  history: Array<{ score: number; recorded_at: string }>
}

// Inline SVG sparkline — no charting library. Degrades to a single dot for
// 1-point history and renders nothing for empty history (no polyline crash).
function Sparkline({ entityId, history, dotColor }: {
  entityId: string
  history: Array<{ score: number; recorded_at: string }>
  dotColor: string
}) {
  const width = 60
  const height = 18
  if (history.length === 0) return null
  if (history.length === 1) {
    return (
      <svg width={width} height={height} data-testid={`sparkline-dot-${entityId}`}>
        <circle cx={width / 2} cy={height / 2} r={2.5} fill={dotColor} />
      </svg>
    )
  }
  const points = history.map((h, i) => {
    const x = (i / (history.length - 1)) * width
    const y = height - h.score * height
    return `${x},${y}`
  })
  return (
    <svg width={width} height={height} data-testid={`sparkline-path-${entityId}`}>
      <polyline points={points.join(' ')} fill="none" stroke="var(--teal)" strokeWidth={1.5} />
      <circle cx={width} cy={height - history[history.length - 1].score * height} r={2} fill={dotColor} />
    </svg>
  )
}

export default function ContextPanel({ status, universeId }: ContextPanelProps) {
  const contradictions = useWSStore((s) => s.contradictions)
  const discoveredEntities = useWSStore((s) => s.discoveredEntities)
  const recallItems = useWSStore((s) => s.recallItems)
  const graphPings = useWSStore((s) => s.graphPings)
  const pipeline = useWSStore((s) => s.pipeline)
  const budget = useWSStore((s) => s.budget)

  const [memoryEntities, setMemoryEntities] = useState<MemoryStatusEntity[]>([])

  const dismissContradiction = (id: string) => {
    useWSStore.setState((s) => ({ contradictions: s.contradictions.filter((c) => c.id !== id) }))
  }

  // Fetch memory-status on mount and refetch as new pipeline/graph signals
  // arrive (per design: mount + on analysis_progress / graph_updated).
  useEffect(() => {
    if (!universeId) return
    api.getMemoryStatus(universeId)
      .then((res) => setMemoryEntities(res.entities))
      .catch(() => { /* keep last-known entities on transient fetch failure */ })
  }, [universeId, pipeline?.stage, graphPings.length])

  const statusClass =
    status === 'open' ? styles.statusOpen
    : status === 'reconnecting' ? styles.statusReconnecting
    : styles.statusClosed

  const isConnected = status === 'open'
  const currentStageIdx = pipeline ? PIPELINE_STAGES.findIndex((s) => s.key === pipeline.stage) : -1

  return (
    <div className={styles.panel}>
      <div className={styles.panelHeader}>
        <h3 className={styles.panelTitle}>
          Live Analysis
          {isConnected && <span className={styles.liveIndicator}>● live</span>}
        </h3>
        <span className={`glyph ${styles.statusIndicator} ${statusClass}`} title={`WS: ${status}`}>●</span>
      </div>

      <div className={`${styles.panelBody} q-scroll`}>

        {/* Graph pings */}
        {graphPings.map((_g, i) => (
          <div key={`graph-${i}`} className={styles.graphPing}>
            <span className={`glyph ${styles.graphPingIcon}`}>✳</span>
            <span className={styles.graphPingText}>Knowledge graph updated</span>
            <button
              className={styles.graphPingDismiss}
              onClick={() => useWSStore.setState((s) => ({
                graphPings: s.graphPings.filter((_, idx) => idx !== i),
              }))}
            >✕</button>
          </div>
        ))}

        {/* ── Live Pipeline stepper ─────────────────────────────────── */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionKicker}>Live Pipeline</span>
          </div>
          <div className={styles.sectionBody}>
            <div className={styles.stepper}>
              {PIPELINE_STAGES.map((s, i) => {
                const state = currentStageIdx === -1 ? 'pending' : i < currentStageIdx ? 'done' : i === currentStageIdx ? 'active' : 'pending'
                const count = s.countKey && pipeline?.stage === s.key ? (pipeline as unknown as Record<string, unknown>)[s.countKey] : undefined
                return (
                  <div key={s.key} className={`${styles.step} ${styles[`step_${state}`]}`}>
                    <span className={styles.stepDot} />
                    <span className={styles.stepLabel}>{s.label}</span>
                    {typeof count === 'number' && <span className={styles.stepCount}>{count}</span>}
                  </div>
                )
              })}
            </div>
          </div>
        </div>

        {/* ── Context Budget ────────────────────────────────────────── */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionKicker}>Context Budget</span>
          </div>
          <div className={styles.sectionBody}>
            {!budget ? (
              <p className={styles.emptyPlaceholder}>Budget appears once analysis runs</p>
            ) : (
              <>
                <div className={styles.budgetBarTrack}>
                  <div className={styles.budgetBarFill} style={{ width: `${Math.min(budget.used_percent, 100)}%` }} />
                </div>
                <div className={styles.budgetLegend}>
                  <span>{budget.used_percent}%</span>
                  <span>{budget.available}/{budget.max_context_tokens} tokens available</span>
                </div>
                <div className={styles.budgetBreakdown}>
                  <span>entities {budget.entities_tokens}</span>
                  <span>vector {budget.vector_tokens}</span>
                  <span>tools {budget.tools_tokens}</span>
                </div>
              </>
            )}
          </div>
        </div>

        {/* ── Entities in this paragraph ─ always rendered ─────────── */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionKicker}>Entities in this paragraph</span>
          </div>
          <div className={styles.sectionBody}>
            {discoveredEntities.length === 0 ? (
              <div className={styles.entityChips}>
                {/* Skeleton chips when idle/loading */}
                <div className={`skeleton ${styles.chipSkeleton}`} />
                <div className={`skeleton ${styles.chipSkeleton}`} style={{ width: 52 }} />
                <div className={`skeleton ${styles.chipSkeleton}`} style={{ width: 78 }} />
              </div>
            ) : (
              <div className={styles.entityChips}>
                {discoveredEntities.map((e) => {
                  const meta = NODE_TYPE_META[e.type || ''] || NODE_TYPE_META.character
                  return (
                    <span
                      key={e.id || e.name}
                      className={styles.entityChip}
                      style={{
                        background: `${meta.color}18`,
                        borderColor: `${meta.color}30`,
                        color: meta.color,
                      }}
                    >
                      <span className={styles.entityChipDot} style={{ background: meta.color }} />
                      {e.name || 'Entity'}
                    </span>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        {/* ── Contradiction detected ─ always rendered ─────────────── */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionKicker}>⚠ Contradiction detected</span>
          </div>
          <div className={styles.sectionBody}>
            {contradictions.length === 0 ? (
              <>
                <div className={`skeleton ${styles.skRow}`} style={{ width: '90%', height: 40, marginBottom: 8 }} />
                <div className={`skeleton ${styles.skRow}`} style={{ width: '75%', height: 32 }} />
                <p className={styles.emptyPlaceholder} style={{ marginTop: 8 }}>
                  AI contradiction analysis will appear here
                </p>
              </>
            ) : (
              contradictions.map((c) => (
                <div key={c.id || String(Math.random())} className={styles.contradictionCard} style={{ marginBottom: 8 }}>
                  <div className={styles.contradictionKicker}>
                    Contradiction
                    {c.severity && (
                      <span className={styles.severityBadge}>{c.severity.toUpperCase()}</span>
                    )}
                  </div>
                  <p className={styles.contradictionText}>{c.message || String(c)}</p>
                  {(c as any).suggestion && (
                    <div className={styles.suggestionBox}>
                      <div className={styles.suggestionKicker}>Suggestion</div>
                      <div className={styles.suggestionText}>{(c as any).suggestion}</div>
                    </div>
                  )}
                  <div className={styles.contradictionActions}>
                    <button className={styles.resolveBtn}>Resolve</button>
                    <button className={styles.dismissBtn} onClick={() => dismissContradiction(c.id || '')}>
                      Dismiss
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* ── Relevant memory ─ always rendered, with retrieval-source badges ── */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionKicker}>Relevant memory</span>
          </div>
          <div className={styles.sectionBody}>
            {recallItems.length === 0 ? (
              <>
                <div className={`skeleton ${styles.skRow}`} style={{ height: 48, marginBottom: 6 }} />
                <p className={styles.emptyPlaceholder}>Semantic memory appears as you write</p>
              </>
            ) : (
              recallItems.map((r) => (
                <div key={r.id || String(Math.random())} style={{ marginBottom: 8 }}>
                  <p className={styles.memoryQuote}>"{r.fact}"</p>
                  <div className={styles.memorySource}>
                    <div className={styles.sourceBadges}>
                      {(r.source || '').split(',').filter(Boolean).map((src) => {
                        const meta = SOURCE_META[src] || { color: 'var(--muted)' }
                        return (
                          <span key={src} className={styles.sourceBadge} style={{ color: meta.color, borderColor: meta.color }}>
                            {src}
                          </span>
                        )
                      })}
                    </div>
                    {r.score && (
                      <span className={styles.memoryScore}>
                        {(r.score * 100).toFixed(0)}%
                      </span>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* ── Entity Lifecycle ──────────────────────────────────────── */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionKicker}>Entity Lifecycle</span>
          </div>
          <div className={styles.sectionBody}>
            {memoryEntities.length === 0 ? (
              <p className={styles.emptyPlaceholder}>Memory lifecycle appears once entities are tracked</p>
            ) : (
              memoryEntities.map((e) => {
                const lifecycleMeta = LIFECYCLE_META[e.lifecycle] || LIFECYCLE_META.active
                return (
                  <div key={e.id} className={styles.lifecycleRow}>
                    <span className={styles.lifecycleName}>{e.name}</span>
                    <Sparkline entityId={e.id} history={e.history} dotColor={lifecycleMeta.color} />
                    <span className={styles.lifecycleChip} style={{ color: lifecycleMeta.color, borderColor: lifecycleMeta.color }}>
                      {lifecycleMeta.label}
                    </span>
                  </div>
                )
              })
            )}
          </div>
        </div>

      </div>
    </div>
  )
}
