import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import PlotHoleList, { type PlotHole } from '../components/plot-holes/PlotHoleList'
import PageStatus from '../components/shared/PageStatus'
import styles from './PlotHolesPage.module.css'

export default function PlotHolesPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const [plotHoles, setPlotHoles] = useState<PlotHole[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  function fetchPlotHoles(id: string) {
    setLoading(true); setError(null)
    api.getPlotHoles(id)
      .then(({ plot_holes }) => { setPlotHoles(plot_holes || []); setLoading(false) })
      .catch((err: Error) => { setError(err.message); setLoading(false) })
  }

  useEffect(() => {
    if (universeId) fetchPlotHoles(universeId)
  }, [universeId])

  if (loading || error) return (
    <PageStatus
      loading={loading}
      error={error}
      onRetry={() => universeId && fetchPlotHoles(universeId)}
    />
  )

  if (plotHoles.length === 0) return (
    <div className={styles.wrap}>
      <div className={styles.emptyState}>
        <span className={`glyph ${styles.emptyGlyph}`}>◠</span>
        <p className={styles.emptyTitle}>No Plot Holes</p>
        <p className={styles.emptyText}>
          No plot holes detected. AI analysis scans your works for narrative gaps, inconsistencies, and unresolved threads.
        </p>
      </div>
    </div>
  )

  return (
    <div className={styles.wrap}>
      <PlotHoleList plotHoles={plotHoles} universeId={universeId!} />
    </div>
  )
}
