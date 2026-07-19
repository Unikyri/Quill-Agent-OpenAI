import { useEffect, useMemo, useState } from 'react'
import { api, type TimelineEventDTO } from '../../lib/api'
import { useGraphStore } from '../../stores/graphStore'
import styles from './TimelineSlider.module.css'

interface TimelineSliderProps {
  universeId: string
}

function eventLabel(event: TimelineEventDTO): string {
  return event.timeline_label || event.title
}

export default function TimelineSlider({ universeId }: TimelineSliderProps) {
  const [events, setEvents] = useState<TimelineEventDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [index, setIndex] = useState(0)
  const nodes = useGraphStore((state) => state.nodes)
  const focusNode = useGraphStore((state) => state.focusNode)
  const setEventHighlight = useGraphStore((state) => state.setEventHighlight)

  useEffect(() => {
    let live = true
    setLoading(true)
    setError(null)
    api.getTimeline(universeId)
      .then(({ events: raw }) => {
        if (!live) return
        const sorted = [...(raw || [])].sort((a, b) => (a.timeline_position ?? 0) - (b.timeline_position ?? 0))
        setEvents(sorted)
        setIndex(0)
        setLoading(false)
      })
      .catch((requestError: Error) => {
        if (!live) return
        setError(requestError.message || 'The timeline could not be loaded.')
        setLoading(false)
      })
    return () => { live = false }
  }, [universeId])

  const nameByID = useMemo(() => new Map(nodes.map((node) => [node.id, node.data.label])), [nodes])
  const selected = events[index]

  // Selecting a position on the timeline highlights that event's key
  // entities on the map immediately — no extra click needed to "filter" by
  // event. Clearing on unmount keeps a stale highlight from surviving a
  // navigation away from the map.
  useEffect(() => {
    setEventHighlight(selected?.participants && selected.participants.length > 0 ? selected.participants : null)
    return () => setEventHighlight(null)
  }, [selected, setEventHighlight])

  if (loading) return <div className={styles.wrap}><div className={`skeleton ${styles.skeleton}`} /></div>
  if (error) return <p className={styles.error} role="alert">Timeline unavailable: {error}</p>
  if (events.length === 0) return null

  return (
    <section className={styles.wrap} aria-labelledby="timeline-slider-heading">
      <div className={styles.header}>
        <span className={styles.kicker} id="timeline-slider-heading">Timeline</span>
        <span className={styles.position}>{index + 1} / {events.length}</span>
      </div>

      <div className={styles.scrubber}>
        <button
          type="button"
          className={styles.step}
          onClick={() => setIndex((i) => Math.max(0, i - 1))}
          disabled={index === 0}
          aria-label="Previous event"
        >‹</button>
        {/* Discrete, directly-clickable positions — precise even with many
            events, unlike dragging a continuous range to an exact step. */}
        <div className={styles.track} role="group" aria-label="Timeline events">
          {events.map((event, i) => (
            <button
              key={event.id}
              type="button"
              className={`${styles.tick} ${i === index ? styles.tickActive : ''}`}
              onClick={() => setIndex(i)}
              aria-current={i === index}
              title={eventLabel(event)}
            >
              <span className={styles.tickDot} aria-hidden="true" />
            </button>
          ))}
        </div>
        <button
          type="button"
          className={styles.step}
          onClick={() => setIndex((i) => Math.min(events.length - 1, i + 1))}
          disabled={index === events.length - 1}
          aria-label="Next event"
        >›</button>
      </div>

      {selected && (
        <div className={styles.eventCard}>
          <p className={styles.eventLabel}>{eventLabel(selected)}</p>
          {selected.description && <p className={styles.eventDescription}>{selected.description}</p>}
          {selected.participants && selected.participants.length > 0 && (
            <div className={styles.participants} aria-label="Key entities in this event">
              {selected.participants.map((entityID) => (
                <button
                  key={entityID}
                  type="button"
                  className={styles.participantChip}
                  onClick={() => void focusNode(entityID)}
                  title="Center the map on this entity"
                >
                  {nameByID.get(entityID) || `${entityID.slice(0, 8)}…`}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </section>
  )
}
