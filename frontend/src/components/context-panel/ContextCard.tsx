import { useEffect, useState, useRef } from 'react'
import styles from './ContextPanel.module.css'

type CardType = 'recall' | 'contradiction' | 'entity'
type Severity = 'low' | 'medium' | 'high'

interface ContextCardProps {
  id: string
  type: CardType
  title: string
  detail: string
  severity?: Severity
  isNew?: boolean
  onDismiss: (id: string) => void
}

export default function ContextCard({
  id,
  type,
  title,
  detail,
  severity,
  isNew,
  onDismiss,
}: ContextCardProps) {
  const [fading, setFading] = useState(false)
  const [removed, setRemoved] = useState(false)
  const fadeTimerRef = useRef<ReturnType<typeof setTimeout>>()

  // Auto-fade after 10 seconds
  useEffect(() => {
    fadeTimerRef.current = setTimeout(() => {
      setFading(true)
      setTimeout(() => {
        setRemoved(true)
        onDismiss(id)
      }, 400) // match CSS transition
    }, 10000)

    return () => {
      if (fadeTimerRef.current) clearTimeout(fadeTimerRef.current)
    }
  }, [id, onDismiss])

  const handleDismiss = () => {
    if (fadeTimerRef.current) clearTimeout(fadeTimerRef.current)
    setFading(true)
    setTimeout(() => {
      setRemoved(true)
      onDismiss(id)
    }, 400)
  }

  if (removed) return null

  const severityLabel = severity ? severity.toUpperCase() : null
  const severityClass = severity ? styles[`severity${severity.charAt(0).toUpperCase() + severity.slice(1)}`] : ''

  const cardClass = [
    styles.card,
    fading ? styles.cardFading : '',
    type === 'contradiction' ? styles.cardDanger : '',
    type === 'entity' ? styles.cardInfo : '',
  ].filter(Boolean).join(' ')

  return (
    <div className={cardClass}>
      <div className={styles.cardHeader}>
        <span className={styles.cardTitle}>
          {type === 'contradiction' && <span className={`glyph ${styles.cardGlyph}`}>△ </span>}
          {type === 'entity' && <span className={`glyph ${styles.cardGlyph}`}>○ </span>}
          {type === 'recall' && <span className={`glyph ${styles.cardGlyph}`}>⌕ </span>}
          {title}
        </span>
        <button className={`glyph ${styles.dismissBtn}`} onClick={handleDismiss} title="Dismiss">
          ✗
        </button>
      </div>
      <p className={styles.cardDetail}>{detail}</p>
      <div className={styles.cardFooter}>
        {severityLabel && (
          <span className={`${styles.badge} ${severityClass}`}>{severityLabel}</span>
        )}
        {isNew && <span className={`${styles.badge} ${styles.badgeNew}`}>NEW</span>}
      </div>
    </div>
  )
}
