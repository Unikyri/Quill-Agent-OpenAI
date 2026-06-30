import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import styles from './GraphCanvas.module.css'

// ponytail: inline map replaces a ConfigLoader or theme system for 5 types
const NODE_STYLES: Record<string, { color: string; icon: string }> = {
  character: { color: '#6c5ce7', icon: '👤' },
  location: { color: '#00b894', icon: '📍' },
  item: { color: '#fdcb6e', icon: '🔮' },
  event: { color: '#e17055', icon: '⚡' },
  concept: { color: '#74b9ff', icon: '💡' },
}

function CustomNode({ data }: NodeProps) {
  const nodeType = (data.type as string) || 'concept'
  const style = NODE_STYLES[nodeType] || NODE_STYLES.concept
  const label = (data.label as string) || 'Untitled'

  return (
    <div
      className={styles.customNode}
      style={{ borderColor: style.color }}
    >
      <Handle type="target" position={Position.Top} className={styles.handle} />
      <div className={styles.nodeContent}>
        <span className={styles.nodeIcon}>{style.icon}</span>
        <span className={styles.nodeLabel}>{label}</span>
      </div>
      <Handle type="source" position={Position.Bottom} className={styles.handle} />
    </div>
  )
}

export default memo(CustomNode)
