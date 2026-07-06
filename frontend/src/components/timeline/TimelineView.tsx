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

function formatEraKicker(timestamp: string): string {
  const d = new Date(timestamp)
  if (Number.isNaN(d.getTime())) return timestamp.toUpperCase()
  return `YEAR ${d.getFullYear()}`
}

export default function TimelineView({ events }: TimelineViewProps) {
  return (
    <div className={styles.timeline}>
      <div className={styles.spine} />
      {events.map((event) => (
        <div key={event.id} className={styles.eventRow}>
          <div className={styles.marker} />
          <div className={styles.eventCard}>
            <span className={styles.eraKicker}>
              {formatEraKicker(event.timestamp)}
              {event.chapter ? ` · ${event.chapter}` : ''}
            </span>
            <h4 className={styles.eventLabel}>{event.label}</h4>
            <p className={styles.eventDesc}>{event.description}</p>
          </div>
        </div>
      ))}
    </div>
  )
}
