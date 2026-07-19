import { useEffect, useMemo, useRef } from 'react'
import cytoscape, {
  type Core,
  type ElementDefinition,
  type LayoutOptions,
  type StylesheetJson,
} from 'cytoscape'
import fcose from 'cytoscape-fcose'
import { useGraphStore } from '../../stores/graphStore'
import { GRAPH_RENDER_EDGE_LIMIT, GRAPH_RENDER_NODE_LIMIT, toCytoscapeElements } from '../../lib/graphElements'
import styles from './GraphCanvas.module.css'

cytoscape.use(fcose)

const style: StylesheetJson = [
  {
    selector: 'node',
    style: {
      label: 'data(label)',
      shape: 'round-rectangle',
      width: 'label',
      height: 'label',
      padding: '14px',
      'background-color': '#fffefa',
      'border-width': 2,
      'border-color': '#52605b',
      color: '#192321',
      'font-family': 'Spline Sans, system-ui, sans-serif',
      'font-size': 12,
      'font-weight': 600,
      'text-wrap': 'wrap',
      'text-max-width': '150px',
      'text-valign': 'center',
      'text-halign': 'center',
      'overlay-opacity': 0,
    },
  },
  {
    selector: 'node[entityType = "character"]',
    style: { 'border-color': '#155e58' },
  },
  {
    selector: 'node[entityType = "place"]',
    style: { 'border-color': '#4d8069' },
  },
  {
    selector: 'node[entityType = "object"]',
    style: { 'border-color': '#8a5d00' },
  },
  {
    selector: 'node[entityType = "faction"]',
    style: { 'border-color': '#52605b' },
  },
  {
    selector: 'node[entityType = "event"]',
    style: { 'border-color': '#8a5d00' },
  },
  {
    selector: 'node[entityType = "world_rule"]',
    style: { 'border-color': '#337c73' },
  },
  {
    selector: 'node[entityType = "plot_arc"]',
    style: { 'border-color': '#a23d33' },
  },
  {
    selector: 'node[focal]',
    style: {
      'border-width': 3,
      'background-color': '#f1f3ee',
    },
  },
  // Ego-graph hop styling: degree-2 nodes read as peripheral context — same
  // shape and color language as degree-1, just quieter — so the eye lands on
  // the focal entity and its direct neighbors first.
  {
    selector: 'node[hop = 2]',
    style: {
      'font-size': 10.5,
      'border-width': 1.5,
      opacity: 0.75,
    },
  },
  {
    selector: 'node:selected',
    style: {
      'border-color': '#0c6a85',
      'background-color': '#e7f0ee',
    },
  },
  {
    selector: 'edge',
    style: {
      width: 1.5,
      'line-color': '#9ca7a2',
      'target-arrow-color': '#9ca7a2',
      'target-arrow-shape': 'triangle',
      'curve-style': 'bezier',
      label: '',
      'overlay-opacity': 0,
    },
  },
  // Edge visual language for the ego graph: a relationship's weight in the
  // line mirrors how peripheral it is, so the map reads center-out instead
  // of as an undifferentiated tangle. edgeTier: 0 touches the focal entity,
  // 1 connects two direct neighbors, 2 reaches a second-degree node (see
  // toCytoscapeElements) — matches the legend in GraphControls.tsx.
  {
    selector: 'edge[edgeTier = 0]',
    style: {
      width: 2.5,
      'line-color': '#155e58',
      'target-arrow-color': '#155e58',
    },
  },
  {
    selector: 'edge[edgeTier = 1]',
    style: {
      width: 1.5,
      'line-color': '#7c8b85',
      'target-arrow-color': '#7c8b85',
    },
  },
  {
    selector: 'edge[edgeTier = 2]',
    style: {
      width: 1,
      'line-color': '#b7bdb8',
      'target-arrow-color': '#b7bdb8',
      'line-style': 'dashed',
    },
  },
  {
    selector: 'edge:selected',
    style: {
      width: 3,
      'line-color': '#155e58',
      'target-arrow-color': '#155e58',
      'line-style': 'solid',
    },
  },
  // Applied/removed imperatively by the eventHighlightIds effect below, not
  // part of element data — dimming must not trigger a re-layout (see that
  // effect for why it's driven by classes instead of a data field).
  {
    selector: 'node.dimmed',
    style: { opacity: 0.25 },
  },
  {
    selector: 'edge.dimmed',
    style: { opacity: 0.12 },
  },
]

function prefersReducedMotion() {
  return typeof window !== 'undefined'
    && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}

export default function GraphCanvas() {
  const nodes = useGraphStore((state) => state.nodes)
  const edges = useGraphStore((state) => state.edges)
  const nodeFilter = useGraphStore((state) => state.nodeFilter)
  const showArchived = useGraphStore((state) => state.showArchived)
  const limits = useGraphStore((state) => state.limits)
  const focalNodeId = useGraphStore((state) => state.focalNodeId)
  const selectedNodeId = useGraphStore((state) => state.selectedNodeId)
  const selectedEdgeId = useGraphStore((state) => state.selectedEdgeId)
  const selectNode = useGraphStore((state) => state.selectNode)
  const selectEdge = useGraphStore((state) => state.selectEdge)
  const focusNode = useGraphStore((state) => state.focusNode)
  const eventHighlightIds = useGraphStore((state) => state.eventHighlightIds)
  const containerRef = useRef<HTMLDivElement>(null)
  const cyRef = useRef<Core | null>(null)
  const callbacksRef = useRef({ selectNode, selectEdge, focusNode })

  callbacksRef.current = { selectNode, selectEdge, focusNode }

  const visibleNeighborhood = useMemo(() => {
    // limits is only present for a bounded focal-entity traversal; a
    // full-graph fetch has no hop count to report, so fall back to the same
    // render caps the store adapters already applied to nodes/edges.
    const nodeCap = limits?.node_limit ?? GRAPH_RENDER_NODE_LIMIT
    const edgeCap = limits?.edge_limit ?? GRAPH_RENDER_EDGE_LIMIT

    const visibleNodes = nodes.filter((node) => (
      nodeFilter[node.type] !== false && (showArchived || node.data.status !== 'archived')
    )).slice(0, nodeCap)
    const visibleNodeIds = new Set(visibleNodes.map((node) => node.id))
    const visibleEdges = edges.filter((edge) => (
      visibleNodeIds.has(edge.source) && visibleNodeIds.has(edge.target)
    )).slice(0, edgeCap)

    return { nodes: visibleNodes, edges: visibleEdges, truncated: false, limits }
  }, [edges, limits, nodeFilter, nodes, showArchived])

  const elements = useMemo(
    () => toCytoscapeElements(visibleNeighborhood, focalNodeId),
    [focalNodeId, visibleNeighborhood],
  )

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const cy = cytoscape({
      container,
      style,
      elements: [],
      boxSelectionEnabled: false,
    })
    cyRef.current = cy

    cy.on('tap', 'node', (event) => {
      const id = event.target.id()
      callbacksRef.current.selectNode(id)
      void callbacksRef.current.focusNode(id)
    })
    cy.on('tap', 'edge', (event) => {
      callbacksRef.current.selectEdge(event.target.id())
    })
    cy.on('tap', (event) => {
      if (event.target === cy) {
        callbacksRef.current.selectNode(null)
        callbacksRef.current.selectEdge(null)
      }
    })

    const resizeObserver = new ResizeObserver(() => cy.resize())
    resizeObserver.observe(container)

    return () => {
      resizeObserver.disconnect()
      cy.destroy()
      cyRef.current = null
    }
  }, [])

  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return

    cy.elements().remove()
    if (elements.length === 0) return

    cy.add(elements as unknown as ElementDefinition[])

    // Ego mode (a focal entity was picked) carries a hop distance on every
    // node — lay those out in rings around the center instead of letting
    // fCoSE's force simulation scatter them. The full, unfocused graph has
    // no hop data and keeps the force-directed layout.
    const isEgoGraph = elements.some((el) => el.group === 'nodes' && typeof el.data.hop === 'number')
    const layout: LayoutOptions = isEgoGraph
      ? {
          name: 'concentric',
          animate: !prefersReducedMotion(),
          animationDuration: 220,
          concentric: (node) => 3 - (Number(node.data('hop')) || 0),
          levelWidth: () => 1,
          minNodeSpacing: 60,
          padding: 48,
        } as LayoutOptions
      : {
          name: 'fcose',
          quality: 'default',
          randomize: false,
          animate: !prefersReducedMotion(),
          animationDuration: 220,
          nodeSeparation: 90,
          idealEdgeLength: 150,
          nodeRepulsion: 8_000,
          gravity: 0.25,
          padding: 48,
        } as LayoutOptions
    cy.layout(layout).run()
    cy.fit(cy.elements(), 48)
  }, [elements])

  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return

    cy.elements().unselect()
    const selectedId = selectedNodeId || selectedEdgeId
    if (selectedId) cy.$id(selectedId).select()
  }, [selectedEdgeId, selectedNodeId])

  useEffect(() => {
    const cy = cyRef.current
    if (!cy) return

    cy.elements().removeClass('dimmed')
    if (!eventHighlightIds || eventHighlightIds.length === 0) return

    const highlighted = new Set(eventHighlightIds)
    const visibleNodeIds = new Set(cy.nodes().map((node) => node.id()))
    // Dimming everything because none of the event's participants happen to
    // be on the current map would read as a broken graph, not a filter —
    // leave it alone and let the timeline's own participant chips be the way
    // to jump to an off-screen entity instead.
    const anyVisible = eventHighlightIds.some((id) => visibleNodeIds.has(id))
    if (!anyVisible) return

    cy.nodes().forEach((node) => {
      if (!highlighted.has(node.id())) node.addClass('dimmed')
    })
    cy.edges().forEach((edge) => {
      if (!highlighted.has(edge.source().id()) || !highlighted.has(edge.target().id())) edge.addClass('dimmed')
    })
  }, [elements, eventHighlightIds])

  return (
    <div className={styles.canvasWrap}>
      <div
        ref={containerRef}
        className={styles.canvas}
        tabIndex={0}
        role="application"
        aria-label="Story relationship map. Use the entity and relationship lists for keyboard navigation."
      />
    </div>
  )
}
