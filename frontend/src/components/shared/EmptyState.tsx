import type { ReactNode } from 'react'
import styles from './EmptyState.module.css'

interface EmptyStateProps {
  title: string
  detail: string
  icon?: ReactNode
  action?: ReactNode
}

// Shared "no data yet" presentation used across every page-level empty state
// (Dashboard, Review, Writer Profile, Story Graph, Memory Lab, etc.) so a
// fresh universe never reads as ad-hoc gray-text paragraphs.
export default function EmptyState({ title, detail, icon, action }: EmptyStateProps) {
  return (
    <section className={styles.wrap}>
      {icon && <div className={styles.iconSlot} aria-hidden="true">{icon}</div>}
      <h2 className={styles.title}>{title}</h2>
      <p className={styles.detail}>{detail}</p>
      {action && <div className={styles.action}>{action}</div>}
    </section>
  )
}
