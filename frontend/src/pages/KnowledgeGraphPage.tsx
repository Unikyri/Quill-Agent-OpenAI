import { useEffect, useContext, useRef } from 'react'
import { useParams } from 'react-router-dom'
import { useGraphStore } from '../stores/graphStore'
import { useWSStore } from '../stores/wsStore'
import { UniverseContext } from '../contexts/UniverseContext'
import GraphCanvas from '../components/knowledge-graph/GraphCanvas'
import GraphControls from '../components/knowledge-graph/GraphControls'
import NodeDrawer from '../components/knowledge-graph/NodeDrawer'
import PageStatus from '../components/shared/PageStatus'
import EmptyState from '../components/shared/EmptyState'

export default function KnowledgeGraphPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const { universe } = useContext(UniverseContext)
  const fetchGraph = useGraphStore((s) => s.fetchGraph)
  const refresh = useGraphStore((s) => s.refresh)
  const loading = useGraphStore((s) => s.loading)
  const error = useGraphStore((s) => s.error)
  const nodes = useGraphStore((s) => s.nodes)
  const graphPings = useWSStore((s) => s.graphPings)
  const prevPingCount = useRef(graphPings.length)

  // Initial fetch
  useEffect(() => {
    if (universeId) {
      fetchGraph(universeId)
    }
  }, [universeId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Silent refetch on wsStore graph_updated ping
  useEffect(() => {
    if (graphPings.length > prevPingCount.current) {
      prevPingCount.current = graphPings.length
      refresh()
    }
  }, [graphPings, refresh])

  if (loading) return <PageStatus loading />
  if (error) return <PageStatus error={error} onRetry={() => universeId && fetchGraph(universeId)} />

  if (nodes.length === 0) {
    return (
      <EmptyState
        icon="🕸️"
        title="No Knowledge Graph"
        message="Generate a knowledge graph from your universe entities using AI analysis to visualize character, location, and event relationships."
        cta={universe ? `Analyze "${universe.name}"` : undefined}
      />
    )
  }

  return (
    <div>
      <GraphControls />
      <div style={{ position: 'relative' }}>
        <GraphCanvas />
        <NodeDrawer />
      </div>
    </div>
  )
}
