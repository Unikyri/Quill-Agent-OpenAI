import { create } from 'zustand'
import { api, type GraphTraversalLimitsDTO } from '../lib/api'
import {
  adaptEntityNeighborhood,
  type StoryGraphEdge,
  type StoryGraphNode,
} from '../lib/graphElements'
import { ENTITY_TYPES } from '../lib/entityTypes'

export type GraphNode = StoryGraphNode
export type GraphEdge = StoryGraphEdge

interface GraphState {
  nodes: GraphNode[]
  edges: GraphEdge[]
  selectedNodeId: string | null
  selectedEdgeId: string | null
  nodeFilter: Record<string, boolean>
  showArchived: boolean
  loading: boolean
  error: string | null
  truncated: boolean
  limits: GraphTraversalLimitsDTO | null
  requestVersion: number
  _universeId: string | null
  focalNodeId: string | null
  breadcrumb: string[]
  // Entity IDs to highlight (dimming everything else) — set by the timeline
  // slider when a story event is selected, so "filter by event" doesn't
  // require an extra click, only whichever entities are already on the map.
  eventHighlightIds: string[] | null
  fetchGraph: (universeId: string) => Promise<void>
  refresh: () => Promise<void>
  focusNode: (id: string) => Promise<void>
  resetFocus: () => Promise<void>
  goBack: () => Promise<void>
  selectNode: (id: string | null) => void
  selectEdge: (id: string | null) => void
  toggleFilter: (type: string) => void
  toggleArchived: () => void
  setEventHighlight: (ids: string[] | null) => void
}

const ALL_TYPES = ENTITY_TYPES as readonly string[]

function isCurrentRequest(
  get: () => GraphState,
  requestVersion: number,
  universeId: string,
  focalNodeId?: string,
) {
  const state = get()
  return state.requestVersion === requestVersion
    && state._universeId === universeId
    && (focalNodeId === undefined || state.focalNodeId === focalNodeId)
}

function neighborhoodState(response: Awaited<ReturnType<typeof api.getEntityNeighbors>>) {
  return adaptEntityNeighborhood(response)
}

export const useGraphStore = create<GraphState>((set, get) => ({
  nodes: [],
  edges: [],
  selectedNodeId: null,
  selectedEdgeId: null,
  nodeFilter: Object.fromEntries(ALL_TYPES.map((type) => [type, true])),
  showArchived: false,
  loading: false,
  error: null,
  truncated: false,
  limits: null,
  requestVersion: 0,
  _universeId: null,
  focalNodeId: null,
  breadcrumb: [],
  eventHighlightIds: null,

  fetchGraph: async (universeId) => {
    const requestVersion = get().requestVersion + 1
    set({
      loading: true,
      error: null,
      requestVersion,
      _universeId: universeId,
      focalNodeId: null,
      selectedNodeId: null,
      selectedEdgeId: null,
      breadcrumb: [],
      truncated: false,
      limits: null,
      eventHighlightIds: null,
    })

    try {
      // Land on an ego graph centered on the universe's most relevant
      // entity — a curated, ranked neighborhood reads as an actual graph.
      // An unranked full-universe dump (every node, no fan-out shaping)
      // renders as disconnected squares once there are more than a few
      // entities, so it is not used as the default view.
      let { entities } = await api.listEntities(universeId, { limit: '1', status: 'active' })
      if (!isCurrentRequest(get, requestVersion, universeId)) return

      if (entities.length === 0) {
        ({ entities } = await api.listEntities(universeId, { limit: '1', status: 'archived' }))
        if (!isCurrentRequest(get, requestVersion, universeId)) return
      }

      const focalNodeId = entities[0]?.id
      if (!focalNodeId) {
        if (isCurrentRequest(get, requestVersion, universeId)) {
          set({ nodes: [], edges: [], truncated: false, limits: null, focalNodeId: null, loading: false })
        }
        return
      }

      const response = await api.getEntityNeighbors(focalNodeId, universeId, 2)
      if (!isCurrentRequest(get, requestVersion, universeId)) return

      const { nodes, edges, truncated, limits } = neighborhoodState(response)
      set({ nodes, edges, truncated, limits, focalNodeId, selectedNodeId: focalNodeId, loading: false })
    } catch (error) {
      if (isCurrentRequest(get, requestVersion, universeId)) {
        set({ error: (error as Error).message, loading: false })
      }
    }
  },

  refresh: async () => {
    const { _universeId, focalNodeId } = get()
    if (!_universeId) return
    if (!focalNodeId) {
      // No focal entity yet: "refresh" means reload the full graph, the same
      // view fetchGraph landed on (e.g. after new entities were extracted).
      await get().fetchGraph(_universeId)
      return
    }

    const requestVersion = get().requestVersion + 1
    set({ requestVersion, loading: true, error: null })
    try {
      const response = await api.getEntityNeighbors(focalNodeId, _universeId, 2)
      if (!isCurrentRequest(get, requestVersion, _universeId, focalNodeId)) return

      const { nodes, edges, truncated, limits } = neighborhoodState(response)
      set({ nodes, edges, truncated, limits, error: null, loading: false })
    } catch (error) {
      if (isCurrentRequest(get, requestVersion, _universeId, focalNodeId)) {
        set({ error: (error as Error).message, loading: false })
      }
    }
  },

  focusNode: async (id) => {
    const { _universeId, focalNodeId, breadcrumb } = get()
    if (!_universeId) return
    if (id === focalNodeId) {
      set({ selectedNodeId: id, selectedEdgeId: null })
      return
    }

    const requestVersion = get().requestVersion + 1
    set({ loading: true, error: null, requestVersion, selectedEdgeId: null })
    try {
      const response = await api.getEntityNeighbors(id, _universeId, 2)
      if (!isCurrentRequest(get, requestVersion, _universeId)) return

      const { nodes, edges, truncated, limits } = neighborhoodState(response)
      set({
        nodes,
        edges,
        truncated,
        limits,
        focalNodeId: id,
        selectedNodeId: id,
        breadcrumb: focalNodeId ? [...breadcrumb, focalNodeId] : breadcrumb,
        loading: false,
      })
    } catch (error) {
      if (isCurrentRequest(get, requestVersion, _universeId)) {
        set({ error: (error as Error).message, loading: false })
      }
    }
  },

  resetFocus: async () => {
    const universeId = get()._universeId
    if (universeId) await get().fetchGraph(universeId)
  },

  goBack: async () => {
    const { breadcrumb } = get()
    const previous = breadcrumb[breadcrumb.length - 1]
    if (!previous) return

    set({ breadcrumb: breadcrumb.slice(0, -1) })
    await get().focusNode(previous)
    set({ breadcrumb: breadcrumb.slice(0, -1) })
  },

  selectNode: (id) => set({ selectedNodeId: id, selectedEdgeId: null }),
  selectEdge: (id) => set({ selectedEdgeId: id, selectedNodeId: null }),

  toggleFilter: (type) => {
    const current = get().nodeFilter
    set({ nodeFilter: { ...current, [type]: !current[type] } })
  },

  toggleArchived: () => set((state) => ({ showArchived: !state.showArchived })),

  setEventHighlight: (ids) => set({ eventHighlightIds: ids }),
}))
