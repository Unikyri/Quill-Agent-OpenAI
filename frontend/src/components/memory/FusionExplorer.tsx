import { useEffect, useRef, useState } from 'react'
import { api } from '../../lib/api'
import type { RecallExplanation } from '../../lib/types'
import styles from './FusionExplorer.module.css'

interface FusionExplorerProps {
  universeId: string
  onResult?: (result: RecallExplanation | null) => void
}

const PIPELINES = ['vector', 'graph', 'recency', 'keyword', 'consolidated', 'preference'] as const

const PIPELINE_META: Record<string, { color: string; label: string }> = {
  vector: { color: 'var(--teal)', label: 'Semantic matches' },
  graph: { color: 'var(--node-worldrule)', label: 'Story relationships' },
  recency: { color: 'var(--gold-ink)', label: 'Recent mentions' },
  keyword: { color: 'var(--muted)', label: 'Exact terms' },
  consolidated: { color: 'var(--node-event)', label: 'Consolidated lore' },
  preference: { color: 'var(--gold)', label: 'Writer preferences' },
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message ? error.message : 'Quill could not explain this recall.'
}

export default function FusionExplorer({ universeId, onResult }: FusionExplorerProps) {
  const [query, setQuery] = useState('')
  const [result, setResult] = useState<RecallExplanation | null>(null)
  const [loadedUniverseId, setLoadedUniverseId] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [loadingUniverseId, setLoadingUniverseId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [errorUniverseId, setErrorUniverseId] = useState<string | null>(null)
  const recallRequestId = useRef(0)
  const currentUniverseId = useRef(universeId)
  const onResultRef = useRef(onResult)
  currentUniverseId.current = universeId
  onResultRef.current = onResult

  useEffect(() => {
    recallRequestId.current += 1
    setQuery('')
    setResult(null)
    setLoadedUniverseId(null)
    setLoading(false)
    setLoadingUniverseId(null)
    setError(null)
    setErrorUniverseId(null)
    onResultRef.current?.(null)
  }, [universeId])

  useEffect(() => () => {
    recallRequestId.current += 1
  }, [])

  const explain = async () => {
    const normalizedQuery = query.trim()
    if (!normalizedQuery) {
      setError('Ask a specific question about your story first.')
      setErrorUniverseId(universeId)
      return
    }

    const requestId = ++recallRequestId.current
    const requestUniverseId = universeId
    const isCurrentRequest = () => (
      recallRequestId.current === requestId && currentUniverseId.current === requestUniverseId
    )
    setLoading(true)
    setLoadingUniverseId(requestUniverseId)
    setError(null)
    setErrorUniverseId(null)
    setResult(null)
    setLoadedUniverseId(null)
    onResultRef.current?.(null)
    try {
      const response = await api.recallExplain(universeId, normalizedQuery, 10)
      if (!isCurrentRequest()) return
      setResult(response)
      setLoadedUniverseId(requestUniverseId)
      onResultRef.current?.(response)
    } catch (requestError) {
      if (!isCurrentRequest()) return
      setError(errorMessage(requestError))
      setErrorUniverseId(requestUniverseId)
    } finally {
      if (isCurrentRequest()) setLoading(false)
    }
  }

  const hasCurrentResult = loadedUniverseId === universeId
  const hasCurrentError = errorUniverseId === universeId
  const isRecalling = loading && loadingUniverseId === universeId
  const currentResult = hasCurrentResult ? result : null
  const totalContributions = currentResult?.items.reduce((count, item) => count + item.contributions.length, 0) || 0

  return (
    <section className={styles.wrap} aria-labelledby="memory-question-title">
      <div className={styles.header}>
        <p className={styles.kicker}>Story recall</p>
        <h1 id="memory-question-title">What does Quill remember?</h1>
        <p className={styles.intro}>Ask about your story. Quill retrieves source facts instead of inventing prose. Semantic matches, relationships, recent mentions, exact terms, and consolidated lore are independently ranked, then combined; a source with zero results simply did not contribute.</p>
      </div>

      <form className={styles.searchRow} onSubmit={(event) => { event.preventDefault(); void explain() }}>
        <label className={styles.questionLabel} htmlFor="memory-query">Ask about your story</label>
        <div className={styles.searchControls}>
          <input
            id="memory-query"
            className={styles.input}
            type="search"
            value={query}
            placeholder="Where was the oath made?"
            onChange={(event) => setQuery(event.target.value)}
            disabled={isRecalling}
          />
          <button className={styles.explainBtn} type="submit" disabled={isRecalling}>
            {isRecalling ? 'Recalling…' : 'Recall'}
          </button>
        </div>
      </form>

      {isRecalling && <p className={styles.status} role="status" aria-live="polite">Searching Quill’s available memory evidence…</p>}
      {hasCurrentError && error && (
        <div className={styles.errorState} role="alert">
          <p>{error}</p>
          <button type="button" onClick={() => void explain()} disabled={isRecalling}>Retry</button>
        </div>
      )}

      {!currentResult && !isRecalling && !hasCurrentError && (
        <p className={styles.hint}>Start with one question. The retrieved answer and its evidence appear here first.</p>
      )}

      {currentResult && !isRecalling && !hasCurrentError && (
        <div className={styles.answer}>
          <div className={styles.answerHeader}>
            <div>
              <p className={styles.answerKicker}>Retrieved answer</p>
              <h2>What Quill found about “{currentResult.query}”</h2>
            </div>
            <span className={styles.answerCount}>{currentResult.items.length} {currentResult.items.length === 1 ? 'memory' : 'memories'}</span>
          </div>

          {currentResult.items.length === 0 ? (
            <p className={styles.emptyPlaceholder}>Quill found no matching memory. Try a different name or add/import more story material first.</p>
          ) : (
            <ol className={styles.evidenceList} aria-label="Retrieved memory evidence">
              {currentResult.items.map((item) => (
                <li key={item.id} className={styles.evidenceItem} data-testid={`fused-item-${item.id}`}>
                  <span>{item.fact}</span>
                  <span className={item.fit_in_budget ? styles.fitBadge : styles.dropBadge} data-testid={`fit-in-budget-${item.id}`}>
                    {item.fit_in_budget ? 'Included in context' : 'Held back by budget'}
                  </span>
                </li>
              ))}
            </ol>
          )}

          <details className={styles.explanation}>
            <summary>See how this recall was assembled ({totalContributions} pipeline contributions)</summary>
            <div className={styles.explanationBody}>
              <div className={styles.pipelineCols} aria-label="Recall pipeline availability">
                {PIPELINES.map((pipeline) => (
                  <div key={pipeline} className={styles.pipelineCol} data-testid={`pipeline-column-${pipeline}`}>
                    <span className={styles.pipelineDot} style={{ background: PIPELINE_META[pipeline].color }} />
                    <span className={styles.pipelineLabel}>{PIPELINE_META[pipeline].label}</span>
                    <span className={styles.pipelineCount}>{currentResult.pipeline_sizes[pipeline] ?? 0}</span>
                  </div>
                ))}
              </div>
              {currentResult.items.length > 0 && (
                <ul className={styles.contributionList} aria-label="Per-memory pipeline contribution">
                  {currentResult.items.map((item) => (
                    <li key={`${item.id}-contributions`}>
                      <span className={styles.contributionFact}>{item.fact}</span>
                      <span className={styles.contributions}>
                        {item.contributions.length === 0 ? 'No pipeline contribution details were returned.' : item.contributions.map((contribution) => (
                          <span
                            key={contribution.pipeline}
                            className={styles.contribution}
                            data-testid={`contribution-${item.id}-${contribution.pipeline}`}
                          >
                            {PIPELINE_META[contribution.pipeline]?.label ?? contribution.pipeline} #{contribution.rank}
                          </span>
                        ))}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </details>
        </div>
      )}
    </section>
  )
}
