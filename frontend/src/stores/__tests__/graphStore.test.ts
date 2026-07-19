import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useGraphStore } from '../graphStore'

// Mock api
const mockListEntities = vi.fn()
const mockGetEntityNeighbors = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    listEntities: (...args: unknown[]) => mockListEntities(...args),
    getEntityNeighbors: (...args: unknown[]) => mockGetEntityNeighbors(...args),
  },
}))

function getStore() {
  return useGraphStore.getState()
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

function graphNode(id: string, name: string) {
  return {
    id,
    properties: {
      raw: `{"id":1,"label":"character","properties":{"entity_id":"${id}","name":"${name}","status":"active","relevance_score":0.7}}`,
    },
  }
}

// Backend returns {id, labels, properties: {raw}} where raw is the agtype vertex
// text AGE actually emits, e.g. {"id":..., "label":"character", "properties":{"entity_id":"n1","name":"Alice","status":"active","relevance_score":0.7}}::vertex
const mockBackendNodes = [
  { id: 'n1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice","status":"active","relevance_score":0.7}}' } },
  { id: 'n2', properties: { raw: '{"id":2,"label":"place","properties":{"entity_id":"n2","name":"Castle","status":"active","relevance_score":0.5}}' } },
  { id: 'n3', properties: { raw: '{"id":3,"label":"world_rule","properties":{"entity_id":"n3","name":"Magic","status":"active","relevance_score":0.3}}' } },
]

const mockBackendEdges = [
  { id: 'e1', source: 'n1', target: 'n2', type: 'lives_in' },
]

const graphLimits = { hops: 2, max_hops: 2, node_limit: 96, edge_limit: 160, result_limit: 256 }

beforeEach(() => {
  vi.clearAllMocks()
  useGraphStore.setState({
    nodes: [],
    edges: [],
    selectedNodeId: null,
    nodeFilter: { character: true, place: true, object: true, event: true, faction: true, world_rule: true, plot_arc: true },
    showArchived: false,
    loading: false,
    error: null,
    truncated: false,
    limits: null,
    requestVersion: 0,
    _universeId: null,
    focalNodeId: null,
    breadcrumb: [],
  })
})

describe('graphStore', () => {
  describe('initial state', () => {
    it('has empty nodes and edges', () => {
      expect(getStore().nodes).toEqual([])
      expect(getStore().edges).toEqual([])
    })

    it('has all type filters enabled', () => {
      const f = getStore().nodeFilter
      expect(f.character).toBe(true)
      expect(f.place).toBe(true)
      expect(f.event).toBe(true)
      expect(f.faction).toBe(true)
      expect(f.world_rule).toBe(true)
      expect(f.plot_arc).toBe(true)
    })

    it('hides archived entities by default', () => {
      expect(getStore().showArchived).toBe(false)
    })

    it('has null selectedNodeId', () => {
      expect(getStore().selectedNodeId).toBeNull()
    })

    it('is not loading and has no error', () => {
      expect(getStore().loading).toBe(false)
      expect(getStore().error).toBeNull()
    })
  })

  describe('fetchGraph', () => {
    it('auto-focuses the most relevant active entity and populates its ego neighborhood', async () => {
      mockListEntities.mockResolvedValue({ entities: [{ id: 'n1' }] })
      mockGetEntityNeighbors.mockResolvedValue({ nodes: mockBackendNodes, edges: mockBackendEdges, truncated: false, limits: graphLimits })

      const promise = getStore().fetchGraph('uni-1')
      expect(getStore().loading).toBe(true)

      await promise
      const nodes = getStore().nodes
      expect(nodes).toHaveLength(3)
      expect(nodes[0].id).toBe('n1')
      expect(nodes[0].type).toBe('character')
      expect(nodes[0].data.label).toBe('Alice')
      expect(nodes[0].data.relevanceScore).toBe(0.7)
      expect(nodes[0].data.status).toBe('active')
      expect(getStore().edges).toHaveLength(1)
      expect(getStore().edges[0].source).toBe('n1')
      expect(getStore().edges[0].target).toBe('n2')
      expect(getStore().loading).toBe(false)
      expect(getStore().error).toBeNull()
      expect(getStore()._universeId).toBe('uni-1')
      expect(getStore().focalNodeId).toBe('n1')
      expect(mockListEntities).toHaveBeenCalledWith('uni-1', { limit: '1', status: 'active' })
      expect(mockGetEntityNeighbors).toHaveBeenCalledWith('n1', 'uni-1', 2)
    })

    it('falls back to an archived entity when no active entity exists', async () => {
      mockListEntities
        .mockResolvedValueOnce({ entities: [] })
        .mockResolvedValueOnce({ entities: [{ id: 'n2' }] })
      mockGetEntityNeighbors.mockResolvedValueOnce({ nodes: mockBackendNodes, edges: mockBackendEdges, truncated: false, limits: graphLimits })

      await getStore().fetchGraph('uni-1')

      expect(mockListEntities).toHaveBeenNthCalledWith(1, 'uni-1', { limit: '1', status: 'active' })
      expect(mockListEntities).toHaveBeenNthCalledWith(2, 'uni-1', { limit: '1', status: 'archived' })
      expect(getStore().focalNodeId).toBe('n2')
    })

    it('shows an empty map when the universe has no entities at all', async () => {
      mockListEntities.mockResolvedValue({ entities: [] })

      await getStore().fetchGraph('uni-1')

      expect(getStore().nodes).toEqual([])
      expect(getStore().focalNodeId).toBeNull()
      expect(getStore().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      mockListEntities.mockRejectedValue(new Error('Network error'))

      await getStore().fetchGraph('uni-1')
      expect(getStore().loading).toBe(false)
      expect(getStore().error).toBe('Network error')
      expect(getStore().nodes).toEqual([])
    })

    it('preserves traversal truncation metadata for the map warning', async () => {
      mockListEntities.mockResolvedValue({ entities: [{ id: 'n1' }] })
      mockGetEntityNeighbors.mockResolvedValue({
        nodes: mockBackendNodes,
        edges: mockBackendEdges,
        truncated: true,
        limits: graphLimits,
      })

      await getStore().fetchGraph('uni-1')

      expect(getStore().truncated).toBe(true)
      expect(getStore().limits).toEqual(graphLimits)
    })

    it('ignores a stale universe response after a newer graph fetch finishes', async () => {
      const firstNeighborhood = deferred<{ nodes: ReturnType<typeof graphNode>[]; edges: never[]; truncated: boolean; limits: typeof graphLimits }>()
      const secondNeighborhood = deferred<{ nodes: ReturnType<typeof graphNode>[]; edges: never[]; truncated: boolean; limits: typeof graphLimits }>()
      mockListEntities
        .mockResolvedValueOnce({ entities: [{ id: 'n1' }] })
        .mockResolvedValueOnce({ entities: [{ id: 'n2' }] })
      mockGetEntityNeighbors.mockImplementation((id: string) => (
        id === 'n1' ? firstNeighborhood.promise : secondNeighborhood.promise
      ))

      const firstFetch = getStore().fetchGraph('uni-1')
      await Promise.resolve()
      const secondFetch = getStore().fetchGraph('uni-2')
      await Promise.resolve()

      secondNeighborhood.resolve({ nodes: [graphNode('n2', 'Second')], edges: [], truncated: false, limits: graphLimits })
      await secondFetch
      firstNeighborhood.resolve({ nodes: [graphNode('n1', 'First')], edges: [], truncated: false, limits: graphLimits })
      await firstFetch

      expect(getStore()._universeId).toBe('uni-2')
      expect(getStore().focalNodeId).toBe('n2')
      expect(getStore().nodes[0].data.label).toBe('Second')
    })
  })

  describe('refresh', () => {
    it('re-runs auto-focus when there is no focal entity', async () => {
      mockListEntities.mockResolvedValue({ entities: [] })
      await getStore().fetchGraph('uni-1')
      vi.clearAllMocks()

      mockListEntities.mockResolvedValueOnce({ entities: [{ id: 'n1' }] })
      mockGetEntityNeighbors.mockResolvedValueOnce({ nodes: mockBackendNodes, edges: mockBackendEdges, truncated: false, limits: graphLimits })

      await getStore().refresh()
      expect(mockListEntities).toHaveBeenCalledWith('uni-1', { limit: '1', status: 'active' })
      expect(getStore().focalNodeId).toBe('n1')
    })

    it('refetches the focal neighborhood when a focal entity is set', async () => {
      useGraphStore.setState({ _universeId: 'uni-1', focalNodeId: 'n1' })
      mockGetEntityNeighbors.mockResolvedValueOnce({ nodes: mockBackendNodes, edges: mockBackendEdges, truncated: false, limits: graphLimits })

      await getStore().refresh()
      expect(mockGetEntityNeighbors).toHaveBeenCalledWith('n1', 'uni-1', 2)
      expect(getStore().nodes).toHaveLength(3)
    })

    it('does nothing if no universeId was set', async () => {
      await getStore().refresh()
      expect(mockGetEntityNeighbors).not.toHaveBeenCalled()
      expect(mockListEntities).not.toHaveBeenCalled()
    })

    it('cannot overwrite a newer focal neighborhood when it resolves late', async () => {
      const staleRefresh = deferred<{ nodes: ReturnType<typeof graphNode>[]; edges: never[]; truncated: boolean; limits: typeof graphLimits }>()
      const focusedNeighborhood = deferred<{ nodes: ReturnType<typeof graphNode>[]; edges: never[]; truncated: boolean; limits: typeof graphLimits }>()
      useGraphStore.setState({ _universeId: 'uni-1', focalNodeId: 'n1', selectedNodeId: 'n1' })
      mockGetEntityNeighbors
        .mockReturnValueOnce(staleRefresh.promise)
        .mockReturnValueOnce(focusedNeighborhood.promise)

      const refresh = getStore().refresh()
      await Promise.resolve()
      const focus = getStore().focusNode('n2')
      await Promise.resolve()

      focusedNeighborhood.resolve({ nodes: [graphNode('n2', 'Focused')], edges: [], truncated: false, limits: graphLimits })
      await focus
      staleRefresh.resolve({ nodes: [graphNode('n1', 'Stale')], edges: [], truncated: false, limits: graphLimits })
      await refresh

      expect(getStore().focalNodeId).toBe('n2')
      expect(getStore().nodes[0].data.label).toBe('Focused')
    })
  })

  describe('focusNode', () => {
    it('keeps entity-type filters while loading a new focal neighborhood', async () => {
      useGraphStore.setState({
        _universeId: 'uni-1',
        focalNodeId: 'n1',
        nodeFilter: { character: false, place: true, object: true, event: true, faction: true, world_rule: true, plot_arc: true },
      })
      mockGetEntityNeighbors.mockResolvedValueOnce({ nodes: [graphNode('n2', 'Focused')], edges: [], truncated: false, limits: graphLimits })

      await getStore().focusNode('n2')

      expect(getStore().nodeFilter.character).toBe(false)
      expect(getStore().focalNodeId).toBe('n2')
    })

    it('keeps the newest focal selection when earlier clicks resolve late', async () => {
      const firstFocus = deferred<{ nodes: ReturnType<typeof graphNode>[]; edges: never[]; truncated: boolean; limits: typeof graphLimits }>()
      const secondFocus = deferred<{ nodes: ReturnType<typeof graphNode>[]; edges: never[]; truncated: boolean; limits: typeof graphLimits }>()
      useGraphStore.setState({ _universeId: 'uni-1', focalNodeId: 'n1', selectedNodeId: 'n1' })
      mockGetEntityNeighbors
        .mockReturnValueOnce(firstFocus.promise)
        .mockReturnValueOnce(secondFocus.promise)

      const focusSecond = getStore().focusNode('n2')
      await Promise.resolve()
      const focusThird = getStore().focusNode('n3')
      await Promise.resolve()

      secondFocus.resolve({ nodes: [graphNode('n3', 'Newest')], edges: [], truncated: false, limits: graphLimits })
      await focusThird
      firstFocus.resolve({ nodes: [graphNode('n2', 'Stale')], edges: [], truncated: false, limits: graphLimits })
      await focusSecond

      expect(getStore().focalNodeId).toBe('n3')
      expect(getStore().selectedNodeId).toBe('n3')
      expect(getStore().nodes[0].data.label).toBe('Newest')
    })
  })

  describe('selectNode', () => {
    it('sets selectedNodeId', () => {
      getStore().selectNode('n1')
      expect(getStore().selectedNodeId).toBe('n1')
    })

    it('clears selectedNodeId with null', () => {
      getStore().selectNode('n1')
      getStore().selectNode(null)
      expect(getStore().selectedNodeId).toBeNull()
    })
  })

  describe('toggleFilter', () => {
    it('toggles a single type filter off', () => {
      expect(getStore().nodeFilter.character).toBe(true)
      getStore().toggleFilter('character')
      expect(getStore().nodeFilter.character).toBe(false)
    })

    it('toggles back on', () => {
      getStore().toggleFilter('character') // off
      getStore().toggleFilter('character') // on
      expect(getStore().nodeFilter.character).toBe(true)
    })

    it('does not affect other filters', () => {
      getStore().toggleFilter('character')
      expect(getStore().nodeFilter.place).toBe(true)
      expect(getStore().nodeFilter.faction).toBe(true)
    })
  })

  describe('toggleArchived', () => {
    it('shows archived entities only when explicitly enabled', () => {
      getStore().toggleArchived()
      expect(getStore().showArchived).toBe(true)
      getStore().toggleArchived()
      expect(getStore().showArchived).toBe(false)
    })
  })
})
