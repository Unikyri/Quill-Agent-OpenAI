import { useEffect, useState } from 'react'
import { useParams, Navigate } from 'react-router-dom'
import { api } from '../lib/api'
import { buildNestedPath } from '../lib/routeRedirect'

// Legacy top-level deep link (`/entity/:entityId`) → nested universe-scoped
// route (ADR-3, RISK-4). See EditorRedirect for the same pattern.
export default function EntityRedirect() {
  const { entityId } = useParams<{ entityId: string }>()
  const [target, setTarget] = useState<string | null>(null)

  const [failed, setFailed] = useState(false)

  useEffect(() => {
    if (!entityId) return
    api
      .getEntity(entityId)
      .then(({ entity }) => {
        setTarget(buildNestedPath(entity.universe_id, 'entities', entityId))
      })
      .catch(() => setFailed(true))
  }, [entityId])

  if (failed) return <Navigate to="/dashboard" replace />
  if (!target) return null
  return <Navigate to={target} replace />
}
