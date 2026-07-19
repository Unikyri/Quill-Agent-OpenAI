import { useGraphStore } from '../../stores/graphStore'
import styles from './GraphCanvas.module.css'
import { ENTITY_TYPE_META, ENTITY_TYPES } from '../../lib/entityTypes'

// Doubles as the node-type legend: each row is both a filter toggle and a
// color/icon key, so a separate static legend component would just duplicate this.
export default function GraphControls() {
  const nodeFilter = useGraphStore((s) => s.nodeFilter)
  const toggleFilter = useGraphStore((s) => s.toggleFilter)
  const showArchived = useGraphStore((s) => s.showArchived)
  const toggleArchived = useGraphStore((s) => s.toggleArchived)
  const focalNodeId = useGraphStore((s) => s.focalNodeId)

  return (
    <>
      <div className={styles.filterBar}>
        {ENTITY_TYPES.map((type) => {
          const meta = ENTITY_TYPE_META[type]
          return (
          <label key={type} className={styles.filterLabel}>
            <input
              type="checkbox"
              checked={nodeFilter[type] ?? true}
              onChange={() => toggleFilter(type)}
              className={styles.filterCheckbox}
              aria-label={`Toggle ${meta.label} entities`}
            />
            <span className={styles.filterBadge} style={{ background: meta.color }} aria-hidden="true" />
            <span className={styles.filterText}>{meta.label}</span>
          </label>
          )
        })}
        <label className={styles.filterLabel}>
          <input
            type="checkbox"
            checked={showArchived}
            onChange={toggleArchived}
            className={styles.filterCheckbox}
            aria-label="Show archived entities"
          />
          <span className={styles.filterText}>Show archived</span>
        </label>
      </div>

      {focalNodeId && (
        <div className={styles.edgeLegend} aria-label="What the relationship line styles mean">
          <span className={styles.edgeLegendItem}>
            <svg width="26" height="8" aria-hidden="true"><line x1="1" y1="4" x2="25" y2="4" stroke="#155e58" strokeWidth="2.5" /></svg>
            Direct to focal entity
          </span>
          <span className={styles.edgeLegendItem}>
            <svg width="26" height="8" aria-hidden="true"><line x1="1" y1="4" x2="25" y2="4" stroke="#7c8b85" strokeWidth="1.5" /></svg>
            Between direct neighbors
          </span>
          <span className={styles.edgeLegendItem}>
            <svg width="26" height="8" aria-hidden="true"><line x1="1" y1="4" x2="25" y2="4" stroke="#b7bdb8" strokeWidth="1" strokeDasharray="3,2" /></svg>
            Second-degree (fainter, dashed)
          </span>
        </div>
      )}
    </>
  )
}
