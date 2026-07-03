import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useGraphStore } from '../graphStore'

// Mock api
const mockGetGraph = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    getGraph: (...args: unknown[]) => mockGetGraph(...args),
  },
}))

function getStore() {
  return useGraphStore.getState()
}

// Backend returns {id, labels, properties} — store transforms to {id, type, position, data}
const mockBackendNodes = [
  { id: 'n1', labels: ['Character'], properties: { raw: "n1:Character {entity_id: 'n1', name: 'Alice', status: 'active'}", entity_id: 'n1' } },
  { id: 'n2', labels: ['Location'], properties: { raw: "n2:Location {entity_id: 'n2', name: 'Castle', status: 'active'}", entity_id: 'n2' } },
  { id: 'n3', labels: ['Concept'], properties: { raw: "n3:Concept {entity_id: 'n3', name: 'Magic', status: 'active'}", entity_id: 'n3' } },
]

const mockBackendEdges = [
  { id: 'e1', type: 'lives_in', properties: { raw: "{source: 'n1', target: 'n2'}" } },
]

beforeEach(() => {
  vi.clearAllMocks()
  useGraphStore.setState({
    nodes: [],
    edges: [],
    selectedNodeId: null,
    nodeFilter: { character: true, location: true, item: true, event: true, concept: true },
    loading: false,
    error: null,
    _universeId: null,
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
      expect(f.location).toBe(true)
      expect(f.item).toBe(true)
      expect(f.event).toBe(true)
      expect(f.concept).toBe(true)
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
    it('sets loading true and populates nodes/edges on success', async () => {
      mockGetGraph.mockResolvedValue({ nodes: mockBackendNodes, edges: mockBackendEdges })

      const promise = getStore().fetchGraph('uni-1')
      expect(getStore().loading).toBe(true)

      await promise
      const nodes = getStore().nodes
      expect(nodes).toHaveLength(3)
      expect(nodes[0].id).toBe('n1')
      expect(nodes[0].type).toBe('character')
      expect(nodes[0].data.label).toBe('Alice')
      expect(nodes[0].position).toHaveProperty('x')
      expect(nodes[0].position).toHaveProperty('y')
      expect(getStore().edges).toHaveLength(1)
      expect(getStore().edges[0].source).toBe('n1')
      expect(getStore().edges[0].target).toBe('n2')
      expect(getStore().loading).toBe(false)
      expect(getStore().error).toBeNull()
      expect(getStore()._universeId).toBe('uni-1')
    })

    it('sets error on failure', async () => {
      mockGetGraph.mockRejectedValue(new Error('Network error'))

      await getStore().fetchGraph('uni-1')
      expect(getStore().loading).toBe(false)
      expect(getStore().error).toBe('Network error')
      expect(getStore().nodes).toEqual([])
    })
  })

  describe('refresh', () => {
    it('refetches using stored universeId', async () => {
      mockGetGraph.mockResolvedValueOnce({ nodes: mockBackendNodes, edges: mockBackendEdges })
      await getStore().fetchGraph('uni-1')
      vi.clearAllMocks()

      const updatedNodes = [{ ...mockBackendNodes[0], properties: { ...mockBackendNodes[0].properties, raw: "n1:Character {entity_id: 'n1', name: 'Alice Updated', status: 'active'}" } }]
      mockGetGraph.mockResolvedValueOnce({ nodes: updatedNodes, edges: [] })

      await getStore().refresh()
      expect(mockGetGraph).toHaveBeenCalledWith('uni-1')
      expect(getStore().nodes).toHaveLength(1)
      expect(getStore().nodes[0].data.label).toBe('Alice Updated')
      expect(getStore().edges).toEqual([])
    })

    it('does nothing if no universeId was set', async () => {
      await getStore().refresh()
      expect(mockGetGraph).not.toHaveBeenCalled()
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
      expect(getStore().nodeFilter.location).toBe(true)
      expect(getStore().nodeFilter.concept).toBe(true)
    })
  })
})
