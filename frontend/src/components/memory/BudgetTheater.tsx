import type { ExplainedItem, RecallBudget } from '../../lib/types'
import styles from './BudgetTheater.module.css'

interface BudgetTheaterProps {
  budget: RecallBudget | null
  items: ExplainedItem[]
}

// Real BudgetReport fields (design obs #265) — no "system"/"user" categories
// exist on the backend, so bars map 1:1 to entities/vector/tools tokens.
const CATEGORIES: Array<{ key: keyof RecallBudget; label: string }> = [
  { key: 'entities_tokens', label: 'Entities' },
  { key: 'vector_tokens', label: 'Vector' },
  { key: 'tools_tokens', label: 'Tools' },
]

export default function BudgetTheater({ budget, items }: BudgetTheaterProps) {
  if (!budget) {
    return (
      <div className={styles.wrap}>
        <span className={styles.kicker}>Budget Theater</span>
        <p className={styles.emptyPlaceholder}>No budget data yet — run a Fusion Explorer query to populate this</p>
      </div>
    )
  }

  const fittedCount = items.filter((i) => i.fit_in_budget).length
  const droppedCount = items.length - fittedCount

  return (
    <div className={styles.wrap}>
      <div className={styles.header}>
        <span className={styles.kicker}>Budget Theater</span>
        <span className={styles.usedPercent}>{budget.used_percent}% of window used</span>
      </div>

      <div className={styles.bars}>
        {CATEGORIES.map(({ key, label }) => {
          const value = budget[key] as number
          const pct = budget.max_context_tokens > 0 ? Math.min(100, (value / budget.max_context_tokens) * 100) : 0
          return (
            <div key={key} className={styles.barRow} data-testid={`budget-bar-${key}`}>
              <span className={styles.barLabel}>{label}</span>
              <div className={styles.barTrack}>
                <div className={styles.barFill} style={{ width: `${pct}%` }} />
              </div>
              <span className={styles.barValue}>{value.toLocaleString()} tok</span>
            </div>
          )
        })}
      </div>

      <div className={styles.counts}>
        <span className={styles.fittedCount} data-testid="budget-fitted-count">Fitted: {fittedCount}</span>
        <span className={styles.droppedCount} data-testid="budget-dropped-count">Dropped: {droppedCount}</span>
      </div>
    </div>
  )
}
