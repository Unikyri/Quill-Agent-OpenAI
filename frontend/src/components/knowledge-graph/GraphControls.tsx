import { useGraphStore } from '../../stores/graphStore'
import styles from './GraphCanvas.module.css'
import { NODE_TYPE_META } from './nodeTypeMeta'

// Doubles as the node-type legend: each row is both a filter toggle and a
// color/icon key, so a separate static legend component would just duplicate this.
export default function GraphControls() {
  const nodeFilter = useGraphStore((s) => s.nodeFilter)
  const toggleFilter = useGraphStore((s) => s.toggleFilter)

  return (
    <div className={styles.filterBar}>
      {Object.entries(NODE_TYPE_META).map(([type, meta]) => (
        <label key={type} className={styles.filterLabel}>
          <input
            type="checkbox"
            checked={nodeFilter[type] ?? true}
            onChange={() => toggleFilter(type)}
            className={styles.filterCheckbox}
          />
          <span className={`${styles.filterBadge} glyph`} style={{ background: meta.color }}>
            {meta.icon}
          </span>
          <span className={styles.filterText}>{meta.label}</span>
        </label>
      ))}
    </div>
  )
}
