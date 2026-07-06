import { useState, useEffect, useContext } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { UniverseContext } from '../contexts/UniverseContext'
import TimelineView, { type TimelineEvent } from '../components/timeline/TimelineView'
import PageStatus from '../components/shared/PageStatus'
import EmptyState from '../components/shared/EmptyState'
import styles from './TimelinePage.module.css'

export default function TimelinePage() {
  const { universeId } = useParams<{ universeId: string }>()
  const { universe } = useContext(UniverseContext)
  const [events, setEvents] = useState<TimelineEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchTimeline = () => {
    if (!universeId) return
    setLoading(true)
    setError(null)
    api.getTimeline(universeId)
      .then(({ events: raw }) => {
        const sorted = (raw || [])
          .map((e) => {
            const chapterMatch = e.label?.match(/(?:Ch\.?\s*)(\d+)/i)
            return {
              ...e,
              chapter: chapterMatch ? `Ch. ${chapterMatch[1]}` : undefined,
              timestamp: e.timestamp || '',
            }
          })
          .sort((a, b) => (a.timestamp || '').localeCompare(b.timestamp || ''))
        setEvents(sorted)
        setLoading(false)
      })
      .catch((err) => {
        setError((err as Error).message)
        setLoading(false)
      })
  }

  useEffect(() => {
    fetchTimeline()
  }, [universeId]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <PageStatus loading />
  if (error) return <PageStatus error={error} onRetry={fetchTimeline} />

  if (events.length === 0) {
    return (
      <EmptyState
        icon="⌇"
        title="No Timeline Events"
        message="Generate a timeline from your work's chapters using AI analysis to visualize the chronological flow of events."
        cta={universe ? `Analyze "${universe.name}"` : undefined}
      />
    )
  }

  return (
    <div className={styles.wrap}>
      <TimelineView events={events} />
    </div>
  )
}
