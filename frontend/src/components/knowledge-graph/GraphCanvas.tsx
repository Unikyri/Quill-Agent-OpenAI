import { useMemo, useCallback } from 'react'
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  type Node,
  type Edge,
  type NodeTypes,
} from 'reactflow'
import 'reactflow/dist/style.css'
import { useGraphStore } from '../../stores/graphStore'
import CustomNode from './CustomNode'
import { NODE_TYPE_META } from './nodeTypeMeta'
import styles from './GraphCanvas.module.css'

const nodeTypes: NodeTypes = { custom: CustomNode }

export default function GraphCanvas() {
  const nodes = useGraphStore((s) => s.nodes)
  const edges = useGraphStore((s) => s.edges)
  const nodeFilter = useGraphStore((s) => s.nodeFilter)
  const selectNode = useGraphStore((s) => s.selectNode)

  const visibleNodes: Node[] = useMemo(() => {
    return nodes
      .filter((n) => nodeFilter[n.type] !== false)
      .map((n) => ({
        id: n.id,
        type: 'custom',
        position: n.position,
        data: { ...n.data, type: n.type },
      }))
  }, [nodes, nodeFilter])

  const visibleEdges: Edge[] = useMemo(() => {
    const visibleIds = new Set(visibleNodes.map((n) => n.id))
    return edges
      .filter((e) => visibleIds.has(e.source) && visibleIds.has(e.target))
      .map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        label: e.label,
      }))
  }, [edges, visibleNodes])

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      selectNode(node.id)
    },
    [selectNode]
  )

  const onPaneClick = useCallback(() => {
    selectNode(null)
  }, [selectNode])

  return (
    <div className={styles.canvasWrap}>
      <ReactFlow
        nodes={visibleNodes}
        edges={visibleEdges}
        nodeTypes={nodeTypes}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        fitView
        className={styles.flowCanvas}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#c9bfa5" gap={20} />
        <Controls className={styles.controls} />
        <MiniMap
          nodeColor={(n) => {
            const type = (n.data as { type?: string })?.type
            return NODE_TYPE_META[type ?? '']?.color || '#6f6656'
          }}
          maskColor="rgba(43,38,32,0.35)"
          className={styles.minimap}
        />
      </ReactFlow>
    </div>
  )
}
