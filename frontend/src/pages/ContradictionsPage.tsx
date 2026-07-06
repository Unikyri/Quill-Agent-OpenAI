import { useState, useEffect, useContext } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { UniverseContext } from '../contexts/UniverseContext'
import ContradictionList, { type Contradiction } from '../components/contradictions/ContradictionList'
import PageStatus from '../components/shared/PageStatus'
import EmptyState from '../components/shared/EmptyState'
import styles from './ContradictionsPage.module.css'

export default function ContradictionsPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const { universe } = useContext(UniverseContext)
  const [contradictions, setContradictions] = useState<Contradiction[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchContradictions = () => {
    if (!universeId) return
    setLoading(true)
    setError(null)
    api.getContradictions(universeId)
      .then(({ contradictions: raw }) => {
        setContradictions(raw || [])
        setLoading(false)
      })
      .catch((err) => {
        setError((err as Error).message)
        setLoading(false)
      })
  }

  useEffect(() => {
    fetchContradictions()
  }, [universeId]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <PageStatus loading />
  if (error) return <PageStatus error={error} onRetry={fetchContradictions} />

  if (contradictions.length === 0) {
    return (
      <EmptyState
        icon="△"
        title="No Contradictions"
        message="No contradictions detected in your universe. AI analysis scans entities and plot events for inconsistencies — run an analysis to check for plot holes."
        cta={universe ? `Analyze "${universe.name}"` : undefined}
      />
    )
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.listArea}>
        <ContradictionList universeId={universeId!} contradictions={contradictions} />
      </div>
    </div>
  )
}
