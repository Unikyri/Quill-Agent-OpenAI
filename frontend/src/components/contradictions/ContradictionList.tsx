import { useState, useMemo } from 'react'
import { api } from '../../lib/api'
import styles from './ContradictionList.module.css'

export interface Contradiction {
  id: string
  entity_id?: string
  severity: string
  description: string
  suggestion?: string
  evidence_a?: string
  evidence_a_chapter_id?: string
  evidence_b?: string
  evidence_b_chapter_id?: string
  status: string
}

interface ContradictionListProps {
  universeId: string
  contradictions: Contradiction[]
}

const SEVERITY_CLASS: Record<string, string> = {
  low: styles.severityLow,
  medium: styles.severityMedium,
  high: styles.severityHigh,
}

type LocalStatus = 'open' | 'resolved' | 'dismissed'

export default function ContradictionList({ universeId, contradictions }: ContradictionListProps) {
  const [filter, setFilter] = useState<string>('all')
  const [statusOverride, setStatusOverride] = useState<Record<string, LocalStatus>>({})
  const [actionError, setActionError] = useState<string | null>(null)

  const filtered = useMemo(() => {
    return contradictions.filter((c) => {
      if (filter !== 'all' && c.severity !== filter) return false
      return true
    })
  }, [contradictions, filter])

  const handleResolve = async (id: string) => {
    if (!window.confirm('Mark this contradiction as resolved?')) return
    setStatusOverride((prev) => ({ ...prev, [id]: 'resolved' }))
    setActionError(null)
    try {
      await api.resolveContradiction(universeId, id)
    } catch (err) {
      setStatusOverride((prev) => ({ ...prev, [id]: 'open' }))
      setActionError((err as Error).message)
    }
  }

  const handleDismiss = async (id: string) => {
    setStatusOverride((prev) => ({ ...prev, [id]: 'dismissed' }))
    setActionError(null)
    try {
      await api.dismissContradiction(universeId, id)
    } catch (err) {
      setStatusOverride((prev) => ({ ...prev, [id]: 'open' }))
      setActionError((err as Error).message)
    }
  }

  const severities = ['all', 'low', 'medium', 'high']

  return (
    <div>
      {actionError && <p className={styles.resolveError}>Action failed: {actionError}</p>}
      <div className={styles.filterBar}>
        {severities.map((s) => (
          <button
            key={s}
            className={`${styles.filterBtn} ${filter === s ? styles.filterBtnActive : ''}`}
            onClick={() => setFilter(s)}
          >
            {s === 'all' ? 'All' : s}
          </button>
        ))}
      </div>

      <div className={styles.listWrap}>
        {filtered.map((c) => {
          const status: LocalStatus = statusOverride[c.id] ?? (c.status as LocalStatus) ?? 'open'
          const isSettled = status !== 'open'
          return (
            <div key={c.id} className={`${styles.card} ${isSettled ? styles.cardResolved : ''}`}>
              <div className={styles.cardHeader}>
                <span className={`${styles.severity} ${SEVERITY_CLASS[c.severity] || styles.severityLow}`}>
                  {c.severity}
                </span>
                {status === 'resolved' && <span className={styles.resolvedLabel}>✓ Resolved</span>}
                {status === 'dismissed' && <span className={styles.dismissedLabel}>Dismissed — marked intentional.</span>}
                {status === 'open' && (
                  <div className={styles.actions}>
                    <button className={styles.resolveBtn} onClick={() => handleResolve(c.id)}>
                      Resolve
                    </button>
                    <button className={styles.dismissBtn} onClick={() => handleDismiss(c.id)}>
                      Dismiss
                    </button>
                  </div>
                )}
              </div>
              <p className={styles.cardMessage}>{c.description}</p>

              {(c.evidence_a || c.evidence_b) && (
                <div className={styles.evidenceGrid}>
                  {c.evidence_a && (
                    <div className={styles.evidencePanel}>
                      <p className={styles.evidenceQuote}>&ldquo;{c.evidence_a}&rdquo;</p>
                      {c.evidence_a_chapter_id && <span className={styles.evidenceTag}>Ch. {c.evidence_a_chapter_id.slice(0, 8)}</span>}
                    </div>
                  )}
                  {c.evidence_b && (
                    <div className={styles.evidencePanel}>
                      <p className={styles.evidenceQuote}>&ldquo;{c.evidence_b}&rdquo;</p>
                      {c.evidence_b_chapter_id && <span className={styles.evidenceTag}>Ch. {c.evidence_b_chapter_id.slice(0, 8)}</span>}
                    </div>
                  )}
                </div>
              )}

              {c.suggestion && (
                <div className={styles.suggestionBox}>
                  <div className={styles.suggestionKicker}>Suggestion</div>
                  <div className={styles.suggestionText}>{c.suggestion}</div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
