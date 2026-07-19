import { useEffect, useState } from 'react'
import { useParams, Navigate } from 'react-router-dom'
import { api } from '../lib/api'
import { explorePath } from '../lib/canonicalRoutes'
import PageStatus from '../components/shared/PageStatus'

// Legacy top-level deep link (`/entity/:entityId`) → nested universe-scoped
// Explore route (ADR-3, RISK-4). See EditorRedirect for the same pattern.
export default function EntityRedirect() {
  const { entityId } = useParams<{ entityId: string }>()
  const [target, setTarget] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [retryAttempt, setRetryAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    if (!entityId) {
      setLoading(false)
      setError('This entity link is missing an entity. Return to Explore and choose an entity.')
      return () => { cancelled = true }
    }
    setLoading(true)
    setError(null)
    setTarget(null)
    api
      .getEntity(entityId)
      .then(({ entity }) => {
        if (cancelled) return
        setTarget(explorePath(entity.universe_id, 'map'))
      })
      .catch(() => {
        if (!cancelled) setError('Could not open this entity. It may no longer exist. Retry to try again.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [entityId, retryAttempt])

  if (!target) {
    return <PageStatus loading={loading} error={error} onRetry={entityId ? () => setRetryAttempt((attempt) => attempt + 1) : undefined} />
  }
  return <Navigate to={target} replace />
}
