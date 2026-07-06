import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import PlotHolesPage from '../PlotHolesPage'
import { UniverseContext } from '../../contexts/UniverseContext'

// CSS module mock
vi.mock('../PlotHolesPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

// Mock api
const mockGetPlotHoles = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    getPlotHoles: (...args: unknown[]) => mockGetPlotHoles(...args),
  },
}))

const defaultContext = {
  universe: { id: 'uni-1', name: 'Test Universe', genre: 'Fantasy', format: 'Novel' },
  works: [],
  refetchWorks: vi.fn(),
}

function renderPage() {
  return render(
    <UniverseContext.Provider value={defaultContext}>
      <MemoryRouter initialEntries={['/universe/uni-1/plot-holes']}>
        <Routes>
          <Route path="/universe/:universeId/plot-holes" element={<PlotHolesPage />} />
        </Routes>
      </MemoryRouter>
    </UniverseContext.Provider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('PlotHolesPage', () => {
  it('shows loading state initially', () => {
    mockGetPlotHoles.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByTestId('loading-state')).toBeInTheDocument()
  })

  it('shows empty state when no plot holes', async () => {
    mockGetPlotHoles.mockResolvedValue({ plot_holes: [] })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('No Plot Holes')).toBeInTheDocument()
    })
  })

  it('shows error state on API failure', async () => {
    mockGetPlotHoles.mockRejectedValue(new Error('Server error'))
    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('error-state')).toBeInTheDocument()
      expect(screen.getByText('Server error')).toBeInTheDocument()
    })
  })

  it('renders each plot hole as an open-thread card with title and description', async () => {
    mockGetPlotHoles.mockResolvedValue({
      plot_holes: [
        { id: 'ph1', title: 'Low priority thread', description: 'Low priority issue', status: 'open' },
        { id: 'ph2', title: 'Critical gap thread', description: 'Critical gap', status: 'open' },
      ],
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Critical gap thread')).toBeInTheDocument()
      expect(screen.getByText('Low priority thread')).toBeInTheDocument()
    })

    expect(screen.getAllByText('OPEN THREAD').length).toBe(2)
  })

  it('shows "Go to chapter" only when a first-mentioned chapter is known', async () => {
    mockGetPlotHoles.mockResolvedValue({
      plot_holes: [
        { id: 'ph1', title: 'A problem', description: 'desc', status: 'open', first_mentioned_chapter_id: 'ch-9' },
      ],
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/Go to chapter/)).toBeInTheDocument()
    })
  })
})
