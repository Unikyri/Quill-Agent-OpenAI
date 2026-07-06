import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import PanoramaPage from '../PanoramaPage'
import { UniverseContext } from '../../contexts/UniverseContext'

vi.mock('../PanoramaPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockListEntities = vi.fn()
const mockGetTimeline = vi.fn()
const mockGetContradictions = vi.fn()
const mockGetPlotHoles = vi.fn()
const mockListChapters = vi.fn()

vi.mock('../../lib/api', () => ({
  api: {
    listEntities: (...args: unknown[]) => mockListEntities(...args),
    getTimeline: (...args: unknown[]) => mockGetTimeline(...args),
    getContradictions: (...args: unknown[]) => mockGetContradictions(...args),
    getPlotHoles: (...args: unknown[]) => mockGetPlotHoles(...args),
    listChapters: (...args: unknown[]) => mockListChapters(...args),
  },
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

const ctxValue = {
  universe: { id: 'uni-1', name: 'Middle Earth', genre: 'Fantasy', format: 'Novel Series' },
  works: [{ id: 'w1', title: 'The Hobbit', type: 'novel', order_index: 1 }],
  refetchWorks: async () => {},
}

function renderPage(ctx = ctxValue) {
  return render(
    <MemoryRouter initialEntries={['/universe/uni-1/panorama']}>
      <UniverseContext.Provider value={ctx}>
        <Routes>
          <Route path="/universe/:universeId/panorama" element={<PanoramaPage />} />
        </Routes>
      </UniverseContext.Provider>
    </MemoryRouter>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockListEntities.mockResolvedValue({
    entities: [
      { id: 'e1', name: 'Kaelen', type: 'character' },
      { id: 'e2', name: 'The Lighthouse', type: 'place' },
    ],
    pagination: { page: 1, limit: 4, total: 47, total_pages: 12 },
  })
  mockGetTimeline.mockResolvedValue({
    events: [
      { id: 't1', label: 'Birth of Kaelen', timestamp: 'Year 820', description: '' },
      { id: 't2', label: 'Fall of Aethelgard', timestamp: 'Year 840', description: '' },
    ],
  })
  mockGetContradictions.mockResolvedValue({
    contradictions: [{ id: 'c1', description: 'Eye color mismatch', severity: 'high', status: 'open' }],
  })
  mockGetPlotHoles.mockResolvedValue({
    plot_holes: [
      { id: 'p1', description: 'Missing return', severity: 'low' },
      { id: 'p2', description: 'Unresolved arc', severity: 'low' },
      { id: 'p3', description: 'Dangling thread', severity: 'low' },
    ],
  })
  mockListChapters.mockResolvedValue({
    chapters: [{ id: 'ch1', title: 'Chapter 1', order_index: 1, word_count: 1200 }],
  })
})

describe('PanoramaPage', () => {
  it('renders the four stat cards with live counts', async () => {
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('47')).toBeInTheDocument()
    })
    expect(screen.getByText('Entities')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('Events')).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('Contradictions')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('Plot Holes')).toBeInTheDocument()
  })

  it('shows a continue-writing banner for the most recent chapter', async () => {
    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/Chapter 1/)).toBeInTheDocument()
    })
    expect(screen.getByText('Continue writing →')).toBeInTheDocument()
  })

  it('navigates to the editor when the continue-writing banner is clicked', async () => {
    const user = userEvent.setup()
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Continue writing →')).toBeInTheDocument()
    })
    await user.click(screen.getByText('Continue writing →'))
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/editor/ch1')
  })

  it('renders recent entities and the timeline preview', async () => {
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Kaelen')).toBeInTheDocument()
    })
    expect(screen.getByText('The Lighthouse')).toBeInTheDocument()
    expect(screen.getByText('Birth of Kaelen')).toBeInTheDocument()
  })

  it('shows the top AI-detected contradiction', async () => {
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Eye color mismatch')).toBeInTheDocument()
    })
    expect(screen.getByText('Detected by AI')).toBeInTheDocument()
  })

  it('omits the AI-detected card when there are no contradictions', async () => {
    mockGetContradictions.mockResolvedValue({ contradictions: [] })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Entities')).toBeInTheDocument()
    })
    expect(screen.queryByText('Detected by AI')).not.toBeInTheDocument()
  })

  it('shows an idle ingestion card when there are no works', async () => {
    mockListChapters.mockResolvedValue({ chapters: [] })
    renderPage({ ...ctxValue, works: [] })

    await waitFor(() => {
      expect(screen.getByText('Entities')).toBeInTheDocument()
    })
    expect(screen.queryByText('Continue writing →')).not.toBeInTheDocument()
    expect(screen.getByText(/No active ingestion/)).toBeInTheDocument()
  })
})
