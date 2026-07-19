import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, act, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import KnowledgeGraphPage from '../KnowledgeGraphPage'
import { UniverseContext } from '../../contexts/UniverseContext'
import { useGraphStore } from '../../stores/graphStore'

const { mockCytoscape } = vi.hoisted(() => {
  const core = {
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
  const mockCytoscape = Object.assign(vi.fn(() => core), { use: vi.fn() })

  return { mockCytoscape }
})

vi.mock('cytoscape', () => ({ default: mockCytoscape }))
vi.mock('cytoscape-fcose', () => ({ default: {} }))

// CSS module mock
vi.mock('../KnowledgeGraphPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

// Navigate spy for CTA assertion
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

// Mutable box for wsStore graphPings — reassign .value to simulate new pings
const { pingBox } = vi.hoisted(() => {
  const box: { value: Array<Record<string, unknown>> } = { value: [] }
  return { pingBox: box }
})

// Mock api. listEntities backs both the initial auto-focus lookup and the
// "Jump to entity" search box; getEntityNeighbors backs fetchGraph/focusNode;
// getTimeline backs the embedded TimelineSlider.
const mockListEntities = vi.fn()
const mockGetEntityNeighbors = vi.fn()
const mockGetTimeline = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    listEntities: (...args: unknown[]) => mockListEntities(...args),
    getEntityNeighbors: (...args: unknown[]) => mockGetEntityNeighbors(...args),
    getTimeline: (...args: unknown[]) => mockGetTimeline(...args),
  },
}))

// Mock wsStore — reads pingBox.value so reassigning it gives a new reference.
// Must apply the Zustand selector so `useWSStore(s => s.graphPings)` returns the array, not the wrapper object.
vi.mock('../../stores/wsStore', () => ({
  useWSStore: (selector: unknown) => {
    const state = { graphPings: pingBox.value }
    return typeof selector === 'function' ? (selector as (s: typeof state) => unknown)(state) : state
  },
}))

const defaultContext = {
  universe: { id: 'uni-1', name: 'Test Universe', genre: 'Fantasy', format: 'Novel' },
  works: [],
  refetchWorks: vi.fn(),
}

const graphLimits = { hops: 2, max_hops: 2, node_limit: 96, edge_limit: 160, result_limit: 256 }

function renderPage() {
  return render(
    <UniverseContext.Provider value={defaultContext}>
      <MemoryRouter initialEntries={['/universe/uni-1/explore/map']}>
        <Routes>
          <Route path="/universe/:universeId/explore/map" element={<KnowledgeGraphPage />} />
        </Routes>
      </MemoryRouter>
    </UniverseContext.Provider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockNavigate.mockClear()
  mockGetTimeline.mockResolvedValue({ events: [] })
  pingBox.value = [] // start each test with no pings
  useGraphStore.setState({
    nodes: [],
    edges: [],
    selectedNodeId: null,
    nodeFilter: { character: true, place: true, object: true, event: true, faction: true, world_rule: true, plot_arc: true },
    loading: false,
    error: null,
    truncated: false,
    limits: null,
    _universeId: null,
    focalNodeId: null,
    breadcrumb: [],
  })
})

describe('KnowledgeGraphPage', () => {
  it('shows loading state initially', () => {
    // Freeze the promise so loading state persists
    mockListEntities.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByTestId('loading-state')).toBeInTheDocument()
  })

  it('shows empty state when the universe has no entities', async () => {
    mockListEntities.mockResolvedValue({ entities: [] })
    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'No relationship map yet' })).toBeInTheDocument()
    })
  })

  it('renders graph controls and canvas when nodes exist', async () => {
    mockListEntities.mockResolvedValue({ entities: [{ id: 'n1' }] })
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [
        { id: 'n1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice"}}' } },
      ],
      edges: [],
      truncated: false,
      limits: graphLimits,
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('application', { name: /story relationship map/i })).toBeInTheDocument()
      expect(screen.getByRole('checkbox', { name: 'Toggle Character entities' })).toBeInTheDocument()
    })
  })

  it('warns when the server returns a truncated neighborhood', async () => {
    mockListEntities.mockResolvedValue({ entities: [{ id: 'n1' }] })
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [{ id: 'n1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice"}}' } }],
      edges: [],
      truncated: true,
      limits: graphLimits,
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Large neighborhood: showing a bounded partial map.')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Retry map' })).toBeInTheDocument()
    })
  })

  it('distinguishes a failed entity search from zero matching entities and offers retry', async () => {
    mockListEntities
      .mockResolvedValueOnce({ entities: [{ id: 'n1' }] })
      .mockRejectedValueOnce(new Error('Search connection lost'))
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [{ id: 'n1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice"}}' } }],
      edges: [],
      truncated: false,
      limits: graphLimits,
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('application', { name: /story relationship map/i })).toBeInTheDocument()
    })

    vi.useFakeTimers()
    fireEvent.change(screen.getByRole('textbox', { name: 'Jump to entity' }), { target: { value: 'alice' } })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(180)
    })
    vi.useRealTimers()

    expect(screen.getByRole('alert')).toHaveTextContent('Search unavailable. Search connection lost')
    expect(screen.getByRole('button', { name: 'Retry search' })).toBeInTheDocument()
    expect(screen.queryByText('No matching entities.')).not.toBeInTheDocument()
  })

  it('labels a successful empty entity search as zero matches', async () => {
    mockListEntities
      .mockResolvedValueOnce({ entities: [{ id: 'n1' }] })
      .mockResolvedValueOnce({ entities: [] })
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [{ id: 'n1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice"}}' } }],
      edges: [],
      truncated: false,
      limits: graphLimits,
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('application', { name: /story relationship map/i })).toBeInTheDocument()
    })

    vi.useFakeTimers()
    fireEvent.change(screen.getByRole('textbox', { name: 'Jump to entity' }), { target: { value: 'missing' } })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(180)
    })
    vi.useRealTimers()

    expect(screen.getByText('No matching entities.')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows error state on API failure', async () => {
    mockListEntities.mockRejectedValue(new Error('Fetch failed'))
    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('error-state')).toBeInTheDocument()
      expect(screen.getByText('Fetch failed')).toBeInTheDocument()
    })
  })

  it('shows retry button on error', async () => {
    mockListEntities.mockRejectedValue(new Error('Oops'))
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Retry')).toBeInTheDocument()
    })
  })

  it('calls refresh when WS graph_updated ping arrives via wsStore', async () => {
    mockListEntities.mockResolvedValue({ entities: [{ id: 'n1' }] })
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [{ id: 'n1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice"}}' } }],
      edges: [],
      truncated: false,
      limits: graphLimits,
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('application', { name: /story relationship map/i })).toBeInTheDocument()
    })
    // fetchGraph called once during load
    expect(mockGetEntityNeighbors).toHaveBeenCalledTimes(1)

    // Simulate WS ping: assign a new array so effect dependency reference changes
    pingBox.value = [{ type: 'graph_updated' }]

    // Trigger re-render without unmounting: produce a new nodes reference
    const { nodes } = useGraphStore.getState()
    act(() => {
      useGraphStore.setState({ nodes: [...nodes] })
    })

    // refresh() keeps the current focal neighborhood fresh.
    await waitFor(() => {
      expect(mockGetEntityNeighbors).toHaveBeenCalledTimes(2)
    })
  })

  it('renders CTA button in empty state that navigates to ingestion', async () => {
    mockListEntities.mockResolvedValue({ entities: [] })
    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'No relationship map yet' })).toBeInTheDocument()
    })

    const ctaButton = screen.getByRole('button', { name: 'Go to import' })
    expect(ctaButton).toBeInTheDocument()
    expect(ctaButton.tagName).toBe('BUTTON')

    fireEvent.click(ctaButton)
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/write?panel=import')
  })
})
