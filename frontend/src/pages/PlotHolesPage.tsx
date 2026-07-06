import { useState, useEffect, useContext } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { UniverseContext } from '../contexts/UniverseContext'
import PlotHoleList, { type PlotHole } from '../components/plot-holes/PlotHoleList'
import PageStatus from '../components/shared/PageStatus'
import EmptyState from '../components/shared/EmptyState'
import styles from './PlotHolesPage.module.css'

export default function PlotHolesPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const { universe } = useContext(UniverseContext)
  const [plotHoles, setPlotHoles] = useState<PlotHole[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchPlotHoles = () => {
    if (!universeId) return
    setLoading(true)
    setError(null)
    api.getPlotHoles(universeId)
      .then(({ plot_holes }) => {
        setPlotHoles(plot_holes || [])
        setLoading(false)
      })
      .catch((err) => {
        setError((err as Error).message)
        setLoading(false)
      })
  }

  useEffect(() => {
    fetchPlotHoles()
  }, [universeId]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <PageStatus loading />
  if (error) return <PageStatus error={error} onRetry={fetchPlotHoles} />

  if (plotHoles.length === 0) {
    return (
      <EmptyState
        icon="◠"
        title="No Plot Holes"
        message="No plot holes detected. AI analysis scans your works for narrative gaps, inconsistencies, and unresolved threads."
        cta={universe ? `Analyze "${universe.name}"` : undefined}
      />
    )
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.listArea}>
        <PlotHoleList plotHoles={plotHoles} universeId={universeId!} />
      </div>
    </div>
  )
}
