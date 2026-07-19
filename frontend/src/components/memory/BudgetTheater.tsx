import EmptyState from '../shared/EmptyState'
import type { ExplainedItem, RecallBudget } from '../../lib/types'
import styles from './BudgetTheater.module.css'

interface BudgetTheaterProps {
  budget: RecallBudget | null
  items: ExplainedItem[]
}

const CATEGORIES: Array<{ key: keyof RecallBudget; label: string }> = [
  { key: 'entities_tokens', label: 'Entities' },
  { key: 'vector_tokens', label: 'Vector' },
  { key: 'tools_tokens', label: 'Tools' },
]

function EvidenceList({ title, items, empty }: { title: string; items: ExplainedItem[]; empty: string }) {
  return (
    <section className={styles.evidenceGroup}>
      <h3>{title}</h3>
      {items.length === 0 ? <p>{empty}</p> : <ul>{items.map((item) => <li key={item.id}>{item.fact}</li>)}</ul>}
    </section>
  )
}

export default function BudgetTheater({ budget, items }: BudgetTheaterProps) {
  if (!budget) {
    return (
      <section className={styles.wrap} aria-label="Context budget">
        <p className={styles.kicker}>Context budget</p>
        <EmptyState
          title="What fit in the prompt"
          detail="Run a recall above to see which real memories fit in the context window and which were held back."
        />
      </section>
    )
  }

  const fitted = items.filter((item) => item.fit_in_budget)
  const dropped = items.filter((item) => !item.fit_in_budget)

  return (
    <section className={styles.wrap} aria-labelledby="budget-title">
      <div className={styles.header}>
        <div>
          <p className={styles.kicker}>Context budget</p>
          <h2 id="budget-title">What fit in the prompt</h2>
        </div>
        <span className={styles.usedPercent}>{budget.used_percent}% used</span>
      </div>
      <p className={styles.hint}>A token is a small piece of text, not a word. Quill reserves response room first; entity facts, vector matches, and tool output then compete for the remaining context. “Held back” means an item was retrieved but omitted from this request, not deleted from your story.</p>

      <div className={styles.bars}>
        {CATEGORIES.map(({ key, label }) => {
          const value = budget[key] as number
          const percentage = budget.max_context_tokens > 0 ? Math.min(100, (value / budget.max_context_tokens) * 100) : 0
          return (
            <div key={key} className={styles.barRow} data-testid={`budget-bar-${key}`}>
              <span className={styles.barLabel}>{label}</span>
              <div className={styles.barTrack} aria-hidden="true"><div className={styles.barFill} style={{ width: `${percentage}%` }} /></div>
              <span className={styles.barValue}>{key === 'vector_tokens' ? `${(budget.vector_tokens_used ?? 0).toLocaleString()} / ${value.toLocaleString()} tok` : `${value.toLocaleString()} tok`}</span>
            </div>
          )
        })}
      </div>

      <div className={styles.counts}>
        <span className={styles.fittedCount} data-testid="budget-fitted-count">Included: {fitted.length}</span>
        <span className={styles.droppedCount} data-testid="budget-dropped-count">Held back: {dropped.length}</span>
      </div>
      <div className={styles.evidenceGrid}>
        <EvidenceList title="Included in context" items={fitted} empty="No retrieved items fit the available context budget." />
        <EvidenceList title="Held back" items={dropped} empty="Every retrieved item fit the available context budget." />
      </div>
    </section>
  )
}
