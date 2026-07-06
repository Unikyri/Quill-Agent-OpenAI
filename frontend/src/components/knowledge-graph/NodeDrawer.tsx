import { useGraphStore } from '../../stores/graphStore'
import styles from './GraphCanvas.module.css'
import { NODE_TYPE_META } from './nodeTypeMeta'

export default function NodeDrawer() {
  const selectedNodeId = useGraphStore((s) => s.selectedNodeId)
  const nodes = useGraphStore((s) => s.nodes)
  const selectNode = useGraphStore((s) => s.selectNode)

  if (!selectedNodeId) return null

  const node = nodes.find((n) => n.id === selectedNodeId)
  if (!node) return null

  const meta = NODE_TYPE_META[node.type] || NODE_TYPE_META.character
  const description = (node.data.description as string) || ''
  const relevanceScore = node.data.relevanceScore as number | undefined
  const status = node.data.status as string | undefined

  return (
    <div className={styles.drawer}>
      <div className={styles.drawerHeader}>
        <h3 className={styles.drawerTitle}>{node.data.label as string}</h3>
        <button className={`glyph ${styles.drawerClose}`} onClick={() => selectNode(null)}>
          ✕
        </button>
      </div>
      <span className={styles.drawerType} style={{ borderLeft: `3px solid ${meta.color}` }}>
        <span className="glyph">{meta.icon}</span> {meta.label}
      </span>
      {status && (
        <div className={styles.drawerField}>
          <span className={styles.drawerFieldKey}>Status</span>
          <span className={styles.drawerFieldValue}>{status}</span>
        </div>
      )}
      {typeof relevanceScore === 'number' && (
        <div className={styles.drawerField}>
          <span className={styles.drawerFieldKey}>Relevance</span>
          <span className={styles.drawerFieldValue}>{Math.round(relevanceScore * 100)}%</span>
        </div>
      )}
      {description && <p className={styles.drawerDesc}>{description}</p>}
      {Object.entries(node.data)
        .filter(([k]) => !['label', 'description', 'type', 'relevanceScore', 'status'].includes(k))
        .map(([key, value]) => (
          <div key={key} className={styles.drawerField}>
            <span className={styles.drawerFieldKey}>{key}</span>
            <span className={styles.drawerFieldValue}>{String(value)}</span>
          </div>
        ))}
    </div>
  )
}
