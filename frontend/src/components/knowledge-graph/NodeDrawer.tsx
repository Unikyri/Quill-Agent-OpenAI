import { useGraphStore } from '../../stores/graphStore'
import styles from './GraphCanvas.module.css'

const TYPE_LABELS: Record<string, string> = {
  character: 'Character',
  location: 'Location',
  item: 'Item',
  event: 'Event',
  concept: 'Concept',
}

export default function NodeDrawer() {
  const selectedNodeId = useGraphStore((s) => s.selectedNodeId)
  const nodes = useGraphStore((s) => s.nodes)
  const selectNode = useGraphStore((s) => s.selectNode)

  if (!selectedNodeId) return null

  const node = nodes.find((n) => n.id === selectedNodeId)
  if (!node) return null

  const typeLabel = TYPE_LABELS[node.type] || node.type
  const description = (node.data.description as string) || ''

  return (
    <div className={styles.drawer}>
      <div className={styles.drawerHeader}>
        <h3 className={styles.drawerTitle}>{node.data.label as string}</h3>
        <button className={styles.drawerClose} onClick={() => selectNode(null)}>
          ✕
        </button>
      </div>
      <span className={styles.drawerType}>{typeLabel}</span>
      {description && <p className={styles.drawerDesc}>{description}</p>}
      {Object.entries(node.data)
        .filter(([k]) => k !== 'label' && k !== 'description' && k !== 'type')
        .map(([key, value]) => (
          <div key={key} className={styles.drawerField}>
            <span className={styles.drawerFieldKey}>{key}</span>
            <span className={styles.drawerFieldValue}>{String(value)}</span>
          </div>
        ))}
    </div>
  )
}
