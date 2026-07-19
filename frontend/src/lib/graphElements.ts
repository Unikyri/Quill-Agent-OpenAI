import type { EntityNeighborhoodDTO, GraphNeighborEdgeDTO, GraphNeighborNodeDTO, GraphTraversalLimitsDTO } from './api'
import { ENTITY_TYPES, type EntityType } from './entityTypes'
import { parseVertexRaw } from './graphParse'

export type StoryGraphNodeType = EntityType | 'unknown'

export interface StoryGraphNode {
  id: string
  type: StoryGraphNodeType
  data: {
    label: string
    relevanceScore?: number
    status?: string
    // Distance from the focal entity in the ego graph: 0 = focal, 1 = direct
    // neighbor, 2 = second-hop. Undefined for a full-graph fetch, which has
    // no focal entity.
    hop?: number
  }
}

export interface StoryGraphEdge {
  id: string
  source: string
  target: string
  relationshipType: string
}

export interface StoryGraphNeighborhood {
  nodes: StoryGraphNode[]
  edges: StoryGraphEdge[]
  truncated: boolean
  // null for a full-graph fetch, which has no bounded-traversal hop count to
  // report. toCytoscapeElements never reads this field.
  limits: GraphTraversalLimitsDTO | null
}

// These match the backend neighborhood budget. Keeping a renderer guard here
// means fCoSE never receives an oversized response even if an older or proxy
// server violates the API contract.
export const GRAPH_RENDER_NODE_LIMIT = 96
export const GRAPH_RENDER_EDGE_LIMIT = 160

export interface CytoscapeElementData {
  id: string
  label?: string
  source?: string
  target?: string
  relationshipType?: string
  entityType?: StoryGraphNodeType
  relevanceScore?: number
  status?: string
  focal?: boolean
  hop?: number
  // How peripheral this edge is: 0 touches the focal entity, 1 connects two
  // direct neighbors, 2 reaches a second-degree node. Drives the edge visual
  // language (see GraphCanvas.tsx). Undefined outside ego mode.
  edgeTier?: number
}

export interface CytoscapeElement {
  group: 'nodes' | 'edges'
  data: CytoscapeElementData
}

function rawText(node: GraphNeighborNodeDTO): string {
  return typeof node.properties.raw === 'string' ? node.properties.raw : ''
}

function graphNodeType(value: string): StoryGraphNodeType {
  return (ENTITY_TYPES as readonly string[]).includes(value) ? value as EntityType : 'unknown'
}

function mapNode(node: GraphNeighborNodeDTO): StoryGraphNode {
  const parsed = parseVertexRaw(rawText(node))
  const id = parsed.entityId || node.id
  // hop is a sibling of `raw` in properties (server-stamped, not part of the
  // AGE vertex itself — see GraphRepo.BoundedNHopTraversal), not parsed from raw.
  const hop = typeof node.properties.hop === 'number' ? node.properties.hop : undefined

  return {
    id,
    type: graphNodeType(parsed.type),
    data: {
      // An ID is an honest fallback when AGE does not expose a name; inventing a
      // display name would make the map look more complete than the source data.
      label: parsed.name || id,
      relevanceScore: parsed.relevanceScore,
      status: parsed.status,
      hop,
    },
  }
}

function mapEdge(edge: GraphNeighborEdgeDTO): StoryGraphEdge | null {
  if (!edge.id || !edge.source || !edge.target) return null

  return {
    id: edge.id,
    source: edge.source,
    target: edge.target,
    relationshipType: edge.type,
  }
}

function hasPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function traversalLimits(response: EntityNeighborhoodDTO): GraphTraversalLimitsDTO {
  const limits = response.limits
  if (!limits
    || !hasPositiveInteger(limits.hops)
    || !hasPositiveInteger(limits.max_hops)
    || !hasPositiveInteger(limits.node_limit)
    || !hasPositiveInteger(limits.edge_limit)
    || !hasPositiveInteger(limits.result_limit)) {
    throw new Error('Graph response is missing traversal limits. Retry the map.')
  }
  return limits
}

/**
 * Converts the API's AGE traversal DTO into renderer-neutral story elements.
 * Keeping this adapter out of the store means neither state nor API contracts
 * inherit the old React Flow node/edge shape.
 */
export function adaptEntityNeighborhood(response: EntityNeighborhoodDTO): StoryGraphNeighborhood {
  const limits = traversalLimits(response)
  const mappedNodes = response.nodes.map(mapNode)
  const nodeLimit = Math.min(limits.node_limit, GRAPH_RENDER_NODE_LIMIT)
  const nodes = mappedNodes.slice(0, nodeLimit)
  const nodeIDs = new Set(nodes.map((node) => node.id))
  const mappedEdges = response.edges
    .map(mapEdge)
    .filter((edge): edge is StoryGraphEdge => edge !== null)
    .filter((edge) => nodeIDs.has(edge.source) && nodeIDs.has(edge.target))
  const edgeLimit = Math.min(limits.edge_limit, GRAPH_RENDER_EDGE_LIMIT)
  const edges = mappedEdges.slice(0, edgeLimit)

  return {
    nodes,
    edges,
    truncated: response.truncated || nodes.length < mappedNodes.length || edges.length < mappedEdges.length,
    limits,
  }
}

/**
 * Cytoscape receives edge semantics for selection and the inspector, but never
 * a `label` property. Relationship prose belongs in the inspector and list,
 * not repeated across the map surface.
 */
export function toCytoscapeElements(
  neighborhood: StoryGraphNeighborhood,
  focalNodeId: string | null,
): CytoscapeElement[] {
  // The adapter already applies this bound. Repeating it at the renderer
  // boundary protects fCoSE if a future caller bypasses the store adapter.
  const nodes = neighborhood.nodes.slice(0, GRAPH_RENDER_NODE_LIMIT)
  const nodeIDs = new Set(nodes.map((node) => node.id))
  const hopByID = new Map(nodes.map((node) => [node.id, node.data.hop]))
  const edges = neighborhood.edges
    .filter((edge) => nodeIDs.has(edge.source) && nodeIDs.has(edge.target))
    .slice(0, GRAPH_RENDER_EDGE_LIMIT)

  return [
    ...nodes.map((node) => ({
      group: 'nodes' as const,
      data: {
        id: node.id,
        label: node.data.label,
        entityType: node.type,
        relevanceScore: node.data.relevanceScore,
        status: node.data.status,
        focal: node.id === focalNodeId,
        hop: node.data.hop,
      },
    })),
    ...edges.map((edge) => {
      const sourceHop = hopByID.get(edge.source)
      const targetHop = hopByID.get(edge.target)
      // An edge is as peripheral as its farthest endpoint (max), unless it
      // touches the focal entity directly — that always reads as the
      // strongest tier regardless of the other end. Every degree-2 node's
      // edge connects back to a degree-1 seed, i.e. hop 1↔2, so taking the
      // *lower* hop (as an earlier version of this did) put those edges in
      // the same visual tier as focal-to-degree-1 edges — the two most
      // common edge kinds in the graph ended up indistinguishable.
      const edgeTier = sourceHop === undefined || targetHop === undefined
        ? undefined
        : sourceHop === 0 || targetHop === 0 ? 0 : Math.max(sourceHop, targetHop)
      return {
        group: 'edges' as const,
        data: {
          id: edge.id,
          source: edge.source,
          target: edge.target,
          relationshipType: edge.relationshipType,
          edgeTier,
        },
      }
    }),
  ]
}

export function relationText(
  edge: StoryGraphEdge,
  nodes: readonly StoryGraphNode[],
): string {
  const names = new Map(nodes.map((node) => [node.id, node.data.label]))
  const source = names.get(edge.source) || edge.source
  const target = names.get(edge.target) || edge.target
  const type = edge.relationshipType.replace(/[_-]+/g, ' ').trim()

  return type
    ? `${source} ${type} ${target}.`
    : `${source} has a relationship with ${target}; the relationship type is unavailable.`
}
