import { useEffect, useState } from 'react'
import { useParams, Navigate } from 'react-router-dom'
import { api } from '../lib/api'
import { buildNestedPath } from '../lib/routeRedirect'

// Legacy top-level deep link (`/editor/:chapterId`) → nested universe-scoped
// route (ADR-3, RISK-4). Fetches the chapter to learn its universe_id, then
// redirects; keeps old bookmarks/links working without duplicating EditorPage.
export default function EditorRedirect() {
  const { chapterId } = useParams<{ chapterId: string }>()
  const [target, setTarget] = useState<string | null>(null)

  const [failed, setFailed] = useState(false)

  useEffect(() => {
    if (!chapterId) return
    api
      .getChapter(chapterId)
      .then(({ chapter }) => {
        setTarget(buildNestedPath(chapter.universe_id, 'editor', chapterId))
      })
      .catch(() => setFailed(true))
  }, [chapterId])

  if (failed) return <Navigate to="/dashboard" replace />
  if (!target) return null
  return <Navigate to={target} replace />
}
