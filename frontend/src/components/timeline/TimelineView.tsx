import styles from './TimelineView.module.css'

export interface TimelineEvent {
  id: string
  label: string
  timestamp: string
  description: string
  chapter?: string
}

interface TimelineViewProps {
  events: TimelineEvent[]
}

function formatAxisDate(timestamp: string): string {
  const d = new Date(timestamp)
  if (Number.isNaN(d.getTime())) return timestamp
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

export default function TimelineView({ events }: TimelineViewProps) {
  return (
    <div className={styles.timeline}>
      {events.map((event) => (
        <div key={event.id} className={styles.eventCard}>
          <span className={styles.eventDate}>{formatAxisDate(event.timestamp)}</span>
          <div className={styles.eventDot} />
          {event.chapter && <p className={styles.eventChapter}>{event.chapter}</p>}
          <h4 className={styles.eventLabel}>{event.label}</h4>
          <p className={styles.eventDesc}>{event.description}</p>
        </div>
      ))}
    </div>
  )
}
