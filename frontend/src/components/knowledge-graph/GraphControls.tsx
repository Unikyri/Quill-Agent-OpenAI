import { useGraphStore } from '../../stores/graphStore'
import styles from './GraphCanvas.module.css'

const NODE_STYLES: Record<string, { color: string; icon: string }> = {
  character: { color: '#6c5ce7', icon: '👤' },
  location: { color: '#00b894', icon: '📍' },
  item: { color: '#fdcb6e', icon: '🔮' },
  event: { color: '#e17055', icon: '⚡' },
  concept: { color: '#74b9ff', icon: '💡' },
}

export default function GraphControls() {
  const nodeFilter = useGraphStore((s) => s.nodeFilter)
  const toggleFilter = useGraphStore((s) => s.toggleFilter)

  return (
    <div className={styles.filterBar}>
      {Object.entries(NODE_STYLES).map(([type, style]) => (
        <label key={type} className={styles.filterLabel}>
          <input
            type="checkbox"
            checked={nodeFilter[type] ?? true}
            onChange={() => toggleFilter(type)}
            className={styles.filterCheckbox}
          />
          <span
            className={styles.filterBadge}
            style={{ background: style.color }}
          >
            {style.icon}
          </span>
          <span className={styles.filterText}>{type}</span>
        </label>
      ))}
    </div>
  )
}
