import { useState } from 'react'
import { api } from '../../lib/api'
import type { RecallExplanation } from '../../lib/types'
import styles from './FusionExplorer.module.css'

interface FusionExplorerProps {
  universeId: string
  onResult?: (result: RecallExplanation) => void
}

// Same 5 RRF pipelines + colors as ContextPanel's SOURCE_META, re-declared
// here rather than imported (ContextPanel.tsx stays unmodified per spec).
const PIPELINES = ['vector', 'graph', 'recency', 'keyword', 'consolidated'] as const

const PIPELINE_META: Record<string, { color: string }> = {
  vector: { color: 'var(--teal)' },
  graph: { color: 'var(--node-worldrule)' },
  recency: { color: 'var(--gold-ink)' },
  keyword: { color: 'var(--muted)' },
  consolidated: { color: 'var(--node-event)' },
}

export default function FusionExplorer({ universeId, onResult }: FusionExplorerProps) {
  const [query, setQuery] = useState('')
  const [result, setResult] = useState<RecallExplanation | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleExplain = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.recallExplain(universeId, query, 10)
      setResult(res)
      onResult?.(res)
    } catch {
      setError('Failed to explain recall — try again')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.header}>
        <span className={styles.kicker}>Fusion Explorer</span>
        <div className={styles.searchRow}>
          <input
            className={styles.input}
            type="text"
            value={query}
            placeholder="Search memory…"
            onChange={(e) => setQuery(e.target.value)}
          />
          <button className={styles.explainBtn} onClick={handleExplain} disabled={loading}>
            Explain
          </button>
        </div>
      </div>

      {loading && <p className={styles.status}>Explaining…</p>}
      {error && <p className={styles.statusError}>{error}</p>}

      {result && !loading && !error && (
        <>
          <div className={styles.pipelineCols}>
            {PIPELINES.map((pipeline) => (
              <div key={pipeline} className={styles.pipelineCol} data-testid={`pipeline-column-${pipeline}`}>
                <span className={styles.pipelineDot} style={{ background: PIPELINE_META[pipeline].color }} />
                <span className={styles.pipelineLabel}>{pipeline}</span>
                <span className={styles.pipelineCount}>{result.pipeline_sizes[pipeline] ?? 0}</span>
              </div>
            ))}
          </div>

          {result.items.length === 0 ? (
            <p className={styles.emptyPlaceholder}>No results for this query</p>
          ) : (
            <ul className={styles.fusedList}>
              {result.items.map((item) => (
                <li key={item.id} className={styles.fusedItem} data-testid={`fused-item-${item.id}`}>
                  <div className={styles.fusedItemHeader}>
                    <span className={styles.fusedFact}>{item.fact}</span>
                    <span className={styles.fusedScore}>{item.rrf_score.toFixed(3)}</span>
                    <span
                      className={item.fit_in_budget ? styles.fitBadge : styles.dropBadge}
                      data-testid={`fit-in-budget-${item.id}`}
                    >
                      {item.fit_in_budget ? 'Fit' : 'Dropped'}
                    </span>
                  </div>
                  <div className={styles.contributions}>
                    {item.contributions.map((c) => (
                      <span
                        key={c.pipeline}
                        className={styles.contribution}
                        data-testid={`contribution-${item.id}-${c.pipeline}`}
                        style={{ borderColor: PIPELINE_META[c.pipeline]?.color ?? 'var(--muted)' }}
                      >
                        {c.pipeline} #{c.rank} (+{c.delta.toFixed(2)})
                      </span>
                    ))}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  )
}
