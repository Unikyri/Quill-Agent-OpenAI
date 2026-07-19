import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, act, fireEvent, within } from '@testing-library/react'
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

// Each tab component has its own dedicated test suite — stub them here so
// this suite only asserts the shell wires the right entityId/universeId in,
// not their internals.
vi.mock('../../components/knowledge-graph/EntityOverviewTab', () => ({
  default: ({ entityId }: { entityId: string }) => (
    <div data-testid="entity-overview-tab">Overview for {entityId}</div>
  ),
}))
vi.mock('../../components/knowledge-graph/RelationshipsTab', () => ({
  default: ({ entityId, universeId }: { entityId: string; universeId: string }) => (
    <div data-testid="relationships-tab">Relationships for {entityId} in {universeId}</div>
  ),
}))
vi.mock('../../components/knowledge-graph/MentionsTab', () => ({
  default: ({ entityId, universeId }: { entityId: string; universeId: string }) => (
    <div data-testid="mentions-tab">Mentions for {entityId} in {universeId}</div>
  ),
}))
vi.mock('../../components/knowledge-graph/RelevanceHistoryTab', () => ({
  default: ({ entityId, universeId }: { entityId: string; universeId: string }) => (
    <div data-testid="relevance-history-tab">Relevance history for {entityId} in {universeId}</div>
  ),
}))

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

// Mock api. listEntities backs both the initial auto-focus lookup (via the
// store) and the left-pane entity search/filter rail; getEntityNeighbors
// backs fetchGraph/focusNode; getTimeline backs the embedded TimelineSlider.
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
const emptyCounts = { character: 0, place: 0, object: 0, faction: 0, event: 0, world_rule: 0, plot_arc: 0 }

function aliceNode() {
  return { id: 'n1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice"}}' } }
}

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
    mockListEntities.mockResolvedValue({ entities: [{ id: 'n1', name: 'Alice', type: 'character' }], counts_by_type: { ...emptyCounts, character: 1 }, pagination: { total: 1 } })
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [aliceNode()],
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
    mockListEntities.mockResolvedValue({ entities: [{ id: 'n1', name: 'Alice', type: 'character' }], counts_by_type: { ...emptyCounts, character: 1 }, pagination: { total: 1 } })
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [aliceNode()],
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
    mockListEntities.mockResolvedValue({ entities: [{ id: 'n1', name: 'Alice', type: 'character' }], counts_by_type: { ...emptyCounts, character: 1 }, pagination: { total: 1 } })
    mockGetEntityNeighbors.mockResolvedValue({
      nodes: [aliceNode()],
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

  // ── Left pane: entity search/filter carried over from EntitiesPage ────────
  describe('left pane entity filters', () => {
    function twoEntityResponse() {
      return {
        entities: [
          { id: 'n1', name: 'Alice', type: 'character' },
          { id: 'n2', name: 'Bob', type: 'character' },
        ],
        counts_by_type: { ...emptyCounts, character: 2 },
        pagination: { total: 2 },
      }
    }

    it('renders the entity list with search input and type-filter chips', async () => {
      mockListEntities.mockResolvedValue(twoEntityResponse())
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      const nav = await screen.findByRole('navigation', { name: 'Browse entities' })
      expect(within(nav).getByPlaceholderText('Search entity or alias…')).toBeInTheDocument()
      expect(within(nav).getByRole('button', { name: 'Characters (2)' })).toBeInTheDocument()
      expect(within(nav).getByText('Alice')).toBeInTheDocument()
      expect(within(nav).getByText('Bob')).toBeInTheDocument()
    })

    it('requests the selected type filter from the server', async () => {
      mockListEntities.mockResolvedValue(twoEntityResponse())
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      const nav = await screen.findByRole('navigation', { name: 'Browse entities' })
      mockListEntities.mockClear()
      fireEvent.click(within(nav).getByRole('button', { name: 'Characters (2)' }))

      await waitFor(() => {
        expect(mockListEntities).toHaveBeenCalledWith('uni-1', { limit: '100', page: '1', type: 'character' })
      })
    })

    it('requests search terms from the server as the user types', async () => {
      mockListEntities.mockResolvedValue(twoEntityResponse())
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      const nav = await screen.findByRole('navigation', { name: 'Browse entities' })
      mockListEntities.mockClear()
      fireEvent.change(within(nav).getByPlaceholderText('Search entity or alias…'), { target: { value: 'Ali' } })

      await waitFor(() => {
        expect(mockListEntities).toHaveBeenCalledWith('uni-1', { limit: '100', page: '1', search: 'Ali' })
      })
    })

    it('selecting an entity re-centers the map on it', async () => {
      mockListEntities.mockResolvedValue(twoEntityResponse())
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      const nav = await screen.findByRole('navigation', { name: 'Browse entities' })
      await waitFor(() => expect(mockGetEntityNeighbors).toHaveBeenCalledTimes(1))

      fireEvent.click(within(nav).getByText('Bob'))

      await waitFor(() => {
        expect(mockGetEntityNeighbors).toHaveBeenCalledWith('n2', 'uni-1', 2)
      })
    })

    it('marks the selected entity button as pressed for screen-reader users', async () => {
      mockListEntities.mockResolvedValue(twoEntityResponse())
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      const nav = await screen.findByRole('navigation', { name: 'Browse entities' })
      await waitFor(() => expect(mockGetEntityNeighbors).toHaveBeenCalledTimes(1))

      expect(within(nav).getByText('Bob').closest('button')).toHaveAttribute('aria-pressed', 'false')

      fireEvent.click(within(nav).getByText('Bob'))

      await waitFor(() => {
        expect(within(nav).getByText('Bob').closest('button')).toHaveAttribute('aria-pressed', 'true')
      })
    })
  })

  // ── Right pane: tabbed detail panel ────────────────────────────────────────
  describe('tabbed detail panel', () => {
    it('defaults to the Overview tab for the focal entity', async () => {
      mockListEntities.mockResolvedValue({ entities: [{ id: 'n1', name: 'Alice', type: 'character' }], counts_by_type: { ...emptyCounts, character: 1 }, pagination: { total: 1 } })
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      await waitFor(() => {
        expect(screen.getByTestId('entity-overview-tab')).toHaveTextContent('Overview for n1')
      })
      expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true')
    })

    it('switches between tabs, wiring entityId/universeId into each, and back to Overview', async () => {
      mockListEntities.mockResolvedValue({ entities: [{ id: 'n1', name: 'Alice', type: 'character' }], counts_by_type: { ...emptyCounts, character: 1 }, pagination: { total: 1 } })
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      await waitFor(() => screen.getByTestId('entity-overview-tab'))

      fireEvent.click(screen.getByRole('tab', { name: 'Relationships' }))
      expect(screen.queryByTestId('entity-overview-tab')).not.toBeInTheDocument()
      expect(screen.getByRole('tab', { name: 'Relationships' })).toHaveAttribute('aria-selected', 'true')
      expect(screen.getByTestId('relationships-tab')).toHaveTextContent('Relationships for n1 in uni-1')

      fireEvent.click(screen.getByRole('tab', { name: 'Mentions' }))
      expect(screen.getByTestId('mentions-tab')).toHaveTextContent('Mentions for n1 in uni-1')

      fireEvent.click(screen.getByRole('tab', { name: 'Relevance history' }))
      expect(screen.getByTestId('relevance-history-tab')).toHaveTextContent('Relevance history for n1 in uni-1')

      fireEvent.click(screen.getByRole('tab', { name: 'Overview' }))
      expect(screen.getByTestId('entity-overview-tab')).toBeInTheDocument()
    })

    it('resets to the Overview tab when a new entity is focused', async () => {
      mockListEntities.mockResolvedValue({
        entities: [
          { id: 'n1', name: 'Alice', type: 'character' },
          { id: 'n2', name: 'Bob', type: 'character' },
        ],
        counts_by_type: { ...emptyCounts, character: 2 },
        pagination: { total: 2 },
      })
      mockGetEntityNeighbors.mockResolvedValue({ nodes: [aliceNode()], edges: [], truncated: false, limits: graphLimits })
      renderPage()

      const nav = await screen.findByRole('navigation', { name: 'Browse entities' })
      await waitFor(() => screen.getByTestId('entity-overview-tab'))

      fireEvent.click(screen.getByRole('tab', { name: 'Relationships' }))
      expect(screen.getByTestId('relationships-tab')).toBeInTheDocument()

      fireEvent.click(within(nav).getByText('Bob'))

      await waitFor(() => {
        expect(screen.getByTestId('entity-overview-tab')).toHaveTextContent('Overview for n2')
      })
      expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true')
    })
  })

  // ── The old "Keyboard map" a11y fallback list (entities-only after T14 moved
  // Relationships into its own tab) is now fully redundant with the left-pane
  // entity list (a proper `nav[aria-label]` + accessible button list that
  // already performs the exact same `focusNode` selection) and was removed. ──
  describe('removed a11y fallback list', () => {
    it('no longer renders the duplicate "Keyboard map" entities/relationships summary', async () => {
      mockListEntities.mockResolvedValue({ entities: [{ id: 'n1', name: 'Alice', type: 'character' }], counts_by_type: { ...emptyCounts, character: 1 }, pagination: { total: 1 } })
      mockGetEntityNeighbors.mockResolvedValue({
        nodes: [aliceNode()],
        edges: [],
        truncated: false,
        limits: graphLimits,
      })
      renderPage()

      await screen.findByRole('navigation', { name: 'Browse entities' })
      expect(screen.queryByText('Keyboard map')).not.toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'Entities' })).not.toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'Relationships' })).not.toBeInTheDocument()
    })
  })
})
