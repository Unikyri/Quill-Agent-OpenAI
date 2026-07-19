import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import GraphCanvas from '../GraphCanvas'
import { useGraphStore } from '../../../stores/graphStore'

const { mockCore, mockCytoscape } = vi.hoisted(() => {
  const mockCore = {
    add: vi.fn(),
    destroy: vi.fn(),
    elements: vi.fn(() => ({ remove: vi.fn(), unselect: vi.fn(), removeClass: vi.fn() })),
    nodes: vi.fn(() => ({ forEach: vi.fn(), map: vi.fn(() => []) })),
    edges: vi.fn(() => ({ forEach: vi.fn() })),
    fit: vi.fn(),
    layout: vi.fn(() => ({ run: vi.fn() })),
    on: vi.fn(),
    resize: vi.fn(),
    $id: vi.fn(() => ({ select: vi.fn() })),
  }
  const mockCytoscape = Object.assign(vi.fn(() => mockCore), { use: vi.fn() })

  return { mockCore, mockCytoscape }
})

const graphLimits = { hops: 2, max_hops: 2, node_limit: 96, edge_limit: 160, result_limit: 256 }

vi.mock('cytoscape', () => ({ default: mockCytoscape }))
vi.mock('cytoscape-fcose', () => ({ default: {} }))

function latestAddedNodeIds() {
  const calls = mockCore.add.mock.calls
  const latestElements = (calls[calls.length - 1]?.[0] ?? []) as Array<{
    group: string
    data: { id: string }
  }>

  return latestElements
    .filter((element) => element.group === 'nodes')
    .map((element) => element.data.id)
}

beforeEach(() => {
  vi.clearAllMocks()
  useGraphStore.setState({
    nodes: [
      { id: 'active', type: 'character', data: { label: 'Active', status: 'active' } },
      { id: 'archived', type: 'object', data: { label: 'Archived', status: 'archived' } },
    ],
    edges: [],
    nodeFilter: { character: true, place: true, object: true, faction: true, event: true, world_rule: true, plot_arc: true },
    showArchived: false,
    limits: graphLimits,
    eventHighlightIds: null,
  })
})

describe('GraphCanvas', () => {
  it('hides archived nodes until the archived toggle is enabled', async () => {
    render(<GraphCanvas />)
    expect(screen.getByRole('application', { name: /story relationship map/i })).toBeInTheDocument()

    await waitFor(() => {
      expect(latestAddedNodeIds()).toEqual(['active'])
    })

    act(() => {
      useGraphStore.setState({ showArchived: true })
    })

    await waitFor(() => {
      expect(latestAddedNodeIds()).toEqual(['active', 'archived'])
    })
  })

  it('still renders using the default render caps when graph data has no traversal bounds (full-graph view)', async () => {
    useGraphStore.setState({ limits: null })

    render(<GraphCanvas />)

    await waitFor(() => {
      expect(latestAddedNodeIds()).toEqual(['active'])
      expect(mockCore.layout).toHaveBeenCalled()
    })
  })

  it('dims nodes/edges not in eventHighlightIds without re-running layout', async () => {
    const activeNode = { id: () => 'active', addClass: vi.fn() }
    const otherNode = { id: () => 'other', addClass: vi.fn() }
    mockCore.nodes.mockReturnValue({
      forEach: (fn: (n: typeof activeNode) => void) => [activeNode, otherNode].forEach(fn),
      map: (fn: (n: typeof activeNode) => string) => [activeNode, otherNode].map(fn),
    } as unknown as ReturnType<typeof mockCore.nodes>)

    render(<GraphCanvas />)
    await waitFor(() => expect(mockCore.layout).toHaveBeenCalledTimes(1))

    act(() => {
      useGraphStore.setState({ eventHighlightIds: ['active'] })
    })

    await waitFor(() => {
      expect(otherNode.addClass).toHaveBeenCalledWith('dimmed')
    })
    expect(activeNode.addClass).not.toHaveBeenCalled()
    // Highlighting must not trigger a fresh layout/re-add of elements.
    expect(mockCore.layout).toHaveBeenCalledTimes(1)
  })
})
