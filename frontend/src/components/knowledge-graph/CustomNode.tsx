import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import styles from './GraphCanvas.module.css'
import { NODE_TYPE_META } from './nodeTypeMeta'

function CustomNode({ data }: NodeProps) {
  const nodeType = (data.type as string) || 'character'
  const meta = NODE_TYPE_META[nodeType] || NODE_TYPE_META.character

  return (
    <div className={styles.customNode} style={{ borderColor: meta.color }}>
      <Handle type="target" position={Position.Top} className={styles.handle} />
      <div className={styles.nodeContent}>
        <span className={`${styles.nodeIcon} glyph`}>{meta.icon}</span>
        <span className={styles.nodeLabel}>{(data.label as string) || 'Untitled'}</span>
      </div>
      <Handle type="source" position={Position.Bottom} className={styles.handle} />
    </div>
  )
}

export default memo(CustomNode)
