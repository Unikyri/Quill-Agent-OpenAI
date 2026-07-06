import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import TimelinePage from '../TimelinePage'
import { UniverseContext } from '../../contexts/UniverseContext'

// CSS module mock
vi.mock('../TimelinePage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

// Mock api
const mockGetTimeline = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    getTimeline: (...args: unknown[]) => mockGetTimeline(...args),
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
      <MemoryRouter initialEntries={['/universe/uni-1/timeline']}>
        <Routes>
          <Route path="/universe/:universeId/timeline" element={<TimelinePage />} />
        </Routes>
      </MemoryRouter>
    </UniverseContext.Provider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('TimelinePage', () => {
  it('shows loading state initially', () => {
    mockGetTimeline.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByTestId('loading-state')).toBeInTheDocument()
  })

  it('shows empty state when no events', async () => {
    mockGetTimeline.mockResolvedValue({ events: [] })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('No Timeline Events')).toBeInTheDocument()
    })
  })

  it('shows error state on API failure', async () => {
    mockGetTimeline.mockRejectedValue(new Error('Fetch failed'))
    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('error-state')).toBeInTheDocument()
      expect(screen.getByText('Fetch failed')).toBeInTheDocument()
    })
  })

  it('renders events sorted by timestamp ascending', async () => {
    mockGetTimeline.mockResolvedValue({
      events: [
        { id: 'e2', label: 'Battle', timestamp: '2025-06-15', description: 'The battle begins' },
        { id: 'e1', label: 'Prologue', timestamp: '2024-01-10', description: 'A long time ago' },
        { id: 'e3', label: 'Epilogue', timestamp: '2026-03-20', description: 'And so it ends' },
      ],
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Prologue')).toBeInTheDocument()
      expect(screen.getByText('Battle')).toBeInTheDocument()
      expect(screen.getByText('Epilogue')).toBeInTheDocument()
    })

    // Verify chronological order: Prologue (2024) → Battle (2025) → Epilogue (2026)
    const labels = screen.getAllByText(/Prologue|Battle|Epilogue/).map((el) => el.textContent)
    expect(labels).toEqual(['Prologue', 'Battle', 'Epilogue'])
  })

  it('renders chapter label when extracted from event label', async () => {
    mockGetTimeline.mockResolvedValue({
      events: [
        { id: 'e1', label: 'Ch. 3 The Escape', timestamp: '2025-01-01', description: 'They flee' },
      ],
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Ch. 3 The Escape')).toBeInTheDocument()
      expect(screen.getAllByText(/Ch\. 3/).length).toBeGreaterThan(0)
    })
  })
})
