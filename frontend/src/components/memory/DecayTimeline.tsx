import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../../lib/api'
import type { MemoryHistoryPoint, MemoryStatusEntity } from '../../lib/types'
import styles from './DecayTimeline.module.css'

interface DecayTimelineProps {
  universeId: string
}

const ARCHIVE_THRESHOLD = 0.15
const VIEW_W = 800
const VIEW_H = 300
const PAD = 30
const INNER_W = VIEW_W - PAD * 2
const INNER_H = VIEW_H - PAD * 2

const LIFECYCLE_META: Record<string, { color: string; label: string }> = {
  active: { color: 'var(--success-2)', label: 'active' },
  decaying: { color: 'var(--gold-ink)', label: 'decaying' },
  archived: { color: 'var(--muted-3)', label: 'archived' },
  consolidated: { color: 'var(--node-event)', label: 'consolidated' },
  reactivated: { color: 'var(--teal)', label: 'reactivated' },
}

function scoreY(score: number) {
  return PAD + (1 - score) * INNER_H
}

function pointX(index: number, length: number) {
  if (length <= 1) return PAD
  return PAD + (index / (length - 1)) * INNER_W
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message ? error.message : 'Could not load the memory lifecycle.'
}

interface Crossing {
  index: number
  kind: 'archive' | 'reactivate'
}

function findCrossings(history: MemoryHistoryPoint[]): Crossing[] {
  const crossings: Crossing[] = []
  for (let index = 1; index < history.length; index++) {
    const previous = history[index - 1].score
    const current = history[index].score
    if (previous > ARCHIVE_THRESHOLD && current <= ARCHIVE_THRESHOLD) crossings.push({ index, kind: 'archive' })
    if (previous <= ARCHIVE_THRESHOLD && current > ARCHIVE_THRESHOLD) crossings.push({ index, kind: 'reactivate' })
  }
  return crossings
}

function EntityLine({ entity }: { entity: MemoryStatusEntity }) {
  const meta = LIFECYCLE_META[entity.lifecycle] || LIFECYCLE_META.active
  if (entity.history.length === 0) return null
  if (entity.history.length === 1) {
    return <circle data-testid={`decay-dot-${entity.id}`} cx={pointX(0, 1)} cy={scoreY(entity.history[0].score)} r={4} fill={meta.color}><title>{`${entity.name} — ${entity.history[0].score.toFixed(2)} (${meta.label})`}</title></circle>
  }
  const points = entity.history.map((point, index) => `${pointX(index, entity.history.length)},${scoreY(point.score)}`).join(' ')
  return (
    <g>
      <polyline data-testid={`decay-polyline-${entity.id}`} points={points} fill="none" stroke={meta.color} strokeWidth={2}><title>{`${entity.name} (${meta.label})`}</title></polyline>
      {findCrossings(entity.history).map((crossing) => (
        <text key={`${crossing.kind}-${crossing.index}`} data-testid={`decay-marker-${entity.id}-${crossing.kind}-${crossing.index}`} x={pointX(crossing.index, entity.history.length)} y={scoreY(entity.history[crossing.index].score) + (crossing.kind === 'archive' ? 14 : -8)} textAnchor="middle" fontSize={12} fill={meta.color} aria-hidden="true">
          {crossing.kind === 'archive' ? '▼' : '▲'}
        </text>
      ))}
    </g>
  )
}

export default function DecayTimeline({ universeId }: DecayTimelineProps) {
  const [entities, setEntities] = useState<MemoryStatusEntity[]>([])
  const [consolidatedCount, setConsolidatedCount] = useState(0)
  const [loadedUniverseId, setLoadedUniverseId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [errorUniverseId, setErrorUniverseId] = useState<string | null>(null)
  const [running, setRunning] = useState(false)
  const [runError, setRunError] = useState<string | null>(null)
  const loadRequestId = useRef(0)
  const currentUniverseId = useRef(universeId)
  currentUniverseId.current = universeId

  const loadStatus = useCallback(async (): Promise<boolean> => {
    const requestId = ++loadRequestId.current
    setLoading(true)
    setError(null)
    setErrorUniverseId(null)
    try {
      const response = await api.getMemoryStatus(universeId)
      if (requestId !== loadRequestId.current || currentUniverseId.current !== universeId) return false
      setEntities(response.entities || [])
      setConsolidatedCount(response.consolidated_count || 0)
      setLoadedUniverseId(universeId)
      return true
    } catch (requestError) {
      if (requestId !== loadRequestId.current || currentUniverseId.current !== universeId) return false
      setError(errorMessage(requestError))
      setErrorUniverseId(universeId)
      return false
    } finally {
      if (requestId === loadRequestId.current && currentUniverseId.current === universeId) setLoading(false)
    }
  }, [universeId])

  useEffect(() => {
    setEntities([])
    setConsolidatedCount(0)
    setLoadedUniverseId(null)
    setRunning(false)
    setRunError(null)
    void loadStatus()
    return () => {
      loadRequestId.current += 1
    }
  }, [loadStatus])

  const handleRunDecay = async () => {
    setRunning(true)
    setRunError(null)
    try {
      await api.runDecay(universeId)
      if (currentUniverseId.current !== universeId) return
      const refreshed = await loadStatus()
      if (!refreshed && currentUniverseId.current === universeId) setRunError('The decay sweep ran, but Quill could not refresh the lifecycle data.')
    } catch (requestError) {
      if (currentUniverseId.current === universeId) setRunError(errorMessage(requestError))
    } finally {
      if (currentUniverseId.current === universeId) setRunning(false)
    }
  }

  const thresholdY = scoreY(ARCHIVE_THRESHOLD)
  const hasCurrentData = loadedUniverseId === universeId
  const hasCurrentError = errorUniverseId === universeId
  if (!hasCurrentData && (!hasCurrentError || loading)) return <section className={styles.wrap} role="status" aria-live="polite">Loading memory lifecycle…</section>
  if (hasCurrentError && !hasCurrentData) {
    return <section className={styles.wrap} role="alert"><p className={styles.error}>{error}</p><button className={styles.advanceBtn} type="button" onClick={() => void loadStatus()}>Retry</button></section>
  }

  return (
    <section className={styles.wrap} aria-labelledby="lifecycle-title">
      <div className={styles.header}>
        <div>
          <p className={styles.kicker}>Memory lifecycle</p>
          <h2 id="lifecycle-title">Decay, relevance, and consolidation</h2>
        </div>
        <button className={styles.advanceBtn} type="button" onClick={() => void handleRunDecay()} disabled={running}>{running ? 'Running sweep…' : 'Run a decay sweep'}</button>
      </div>
      <p className={styles.summary}>Each line is one entity’s relevance history. Higher means it has been mentioned more recently; the dashed line is the archive threshold (15%). ▼ marks a move into archive and ▲ a later reactivation. A sweep recomputes these scores from existing mentions; it does not analyze new prose.</p>
      <p className={styles.summary}>{consolidatedCount} {consolidatedCount === 1 ? 'consolidated memory is' : 'consolidated memories are'} currently available to recall.</p>
      {hasCurrentError && <div className={styles.degraded} role="status">Could not refresh the lifecycle. Showing the last available data. <button type="button" onClick={() => void loadStatus()}>Retry</button></div>}
      {runError && <div className={styles.degraded} role="alert">{runError} <button type="button" onClick={() => void handleRunDecay()}>Retry sweep</button></div>}

      {entities.length === 0 ? <p className={styles.emptyPlaceholder}>No entity lifecycle data yet. Quill shows this after it has tracked story entities.</p> : (
        <>
          <svg data-testid="decay-timeline-svg" className={styles.svg} viewBox={`0 0 ${VIEW_W} ${VIEW_H}`} preserveAspectRatio="xMidYMid meet" aria-label="Entity relevance over time">
            <line data-testid="decay-threshold-line" x1={PAD} y1={thresholdY} x2={VIEW_W - PAD} y2={thresholdY} stroke="var(--muted-3)" strokeWidth={1} strokeDasharray="4 3" />
            {entities.map((entity) => <EntityLine key={entity.id} entity={entity} />)}
          </svg>
          <ul className={styles.legend} aria-label="Entity lifecycle summary">
            {entities.map((entity) => {
              const meta = LIFECYCLE_META[entity.lifecycle] || LIFECYCLE_META.active
              return <li key={entity.id} className={styles.legendItem}><span className={styles.legendDot} style={{ background: meta.color }} />{entity.name}: {meta.label}, <span className={styles.relevanceFigure}>{Math.round(entity.relevance_score * 100)}% relevance</span></li>
            })}
          </ul>
        </>
      )}
    </section>
  )
}
