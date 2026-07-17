import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { EntityCandidateDTO } from '../../lib/types'
import styles from './EntityCandidateTray.module.css'

interface EntityCandidateTrayProps {
  candidates: EntityCandidateDTO[]
  universeId: string
  error?: string | null
  onChanged: () => Promise<void> | void
  onDecision?: (candidateId: string) => void
}

interface ActiveEntity {
  id: string
  name: string
  type?: string
}

export default function EntityCandidateTray({ candidates, universeId, error, onChanged, onDecision }: EntityCandidateTrayProps) {
  const [pendingId, setPendingId] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [activeEntities, setActiveEntities] = useState<ActiveEntity[]>([])
  const [mergeTargets, setMergeTargets] = useState<Record<string, string>>({})

  useEffect(() => {
    let cancelled = false
    if (!universeId || typeof api.listEntities !== 'function') return () => undefined
    api.listEntities(universeId, { status: 'active', limit: '500', page: '1' })
      .then(({ entities }) => {
        if (!cancelled) setActiveEntities((entities || []).map((entity: ActiveEntity) => ({ id: entity.id, name: entity.name, type: entity.type })))
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [universeId])

  const decide = async (candidate: EntityCandidateDTO, action: 'accept' | 'dismiss' | 'merge') => {
    if (!candidate.entity_id || pendingId) return
    const targetEntityId = mergeTargets[candidate.entity_id]
    if (action === 'merge' && !targetEntityId) {
      setMessage('Choose an active entity before merging.')
      return
    }
    setPendingId(candidate.entity_id)
    setMessage(null)
    try {
      if (action === 'accept') await api.acceptEntityCandidate(candidate.entity_id)
      else if (action === 'dismiss') await api.dismissEntityCandidate(candidate.entity_id)
      else await api.mergeEntityCandidate(candidate.entity_id, targetEntityId)
      await onChanged()
      onDecision?.(candidate.entity_id)
      setMessage(action === 'accept' ? 'Candidate accepted.' : action === 'merge' ? 'Candidate merged.' : 'Candidate dismissed.')
    } catch (decisionError) {
      setMessage((decisionError as Error).message || 'Could not save candidate decision')
    } finally {
      setPendingId(null)
    }
  }

  return (
    <section className={styles.tray} aria-label="Entity candidate review tray">
      <div className={styles.header}>
        <div>
          <span className={styles.kicker}>Human confirmation</span>
          <h3 className={styles.title}>Entity candidates</h3>
        </div>
        <span className={styles.count}>{candidates.length}</span>
      </div>
      {error && <p className={styles.error} role="alert">{error}</p>}
      {message && <p className={styles.message} role="status">{message}</p>}
      {candidates.length === 0 ? (
        <p className={styles.empty}>No low-confidence candidates are waiting.</p>
      ) : (
        <div className={styles.list}>
          {candidates.map((candidate) => {
            const id = candidate.entity_id
            return (
              <article className={styles.candidate} key={id}>
                <div className={styles.candidateHeader}>
                  <strong>{candidate.name}</strong>
                  <span className={styles.type}>{candidate.type}</span>
                  <span className={styles.confidence}>{Math.round(candidate.confidence * 100)}%</span>
                </div>
                {candidate.evidence_quote && <blockquote>{candidate.evidence_quote}</blockquote>}
                {candidate.description && <p className={styles.description}>{candidate.description}</p>}
                <div className={styles.actions}>
                  <button type="button" onClick={() => void decide(candidate, 'accept')} disabled={pendingId !== null}>
                    {pendingId === id ? 'Saving…' : 'Accept'}
                  </button>
                  <button type="button" className={styles.dismiss} onClick={() => void decide(candidate, 'dismiss')} disabled={pendingId !== null}>
                    Dismiss
                  </button>
                  <select
                    aria-label={`Merge ${candidate.name} into active entity`}
                    value={mergeTargets[id] || ''}
                    onChange={(event) => setMergeTargets((current) => ({ ...current, [id]: event.target.value }))}
                    disabled={pendingId !== null || activeEntities.length === 0}
                  >
                    <option value="">Merge into…</option>
                    {activeEntities.filter((entity) => entity.id !== id).map((entity) => (
                      <option key={entity.id} value={entity.id}>{entity.name}</option>
                    ))}
                  </select>
                  <button type="button" onClick={() => void decide(candidate, 'merge')} disabled={pendingId !== null || !mergeTargets[id]}>
                    Merge
                  </button>
                </div>
              </article>
            )
          })}
        </div>
      )}
      {universeId && <span className={styles.srOnly}>Universe {universeId}</span>}
    </section>
  )
}
