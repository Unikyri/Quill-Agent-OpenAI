import styles from './EmptyState.module.css'

interface EmptyStateProps {
  icon?: string
  title: string
  message: string
  cta?: string
  onCta?: () => void
}

export default function EmptyState({ icon = '◇', title, message, cta, onCta }: EmptyStateProps) {
  return (
    <div className={styles.emptyCard}>
      <div className={`glyph ${styles.icon}`}>{icon}</div>
      <h3 className={styles.title}>{title}</h3>
      <p className={styles.message}>{message}</p>
      {cta && onCta && (
        <button className={styles.cta} onClick={onCta}>
          {cta}
        </button>
      )}
    </div>
  )
}
