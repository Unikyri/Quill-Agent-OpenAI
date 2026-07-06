import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import ContradictionsPage from '../ContradictionsPage'
import { UniverseContext } from '../../contexts/UniverseContext'

// CSS module mock
vi.mock('../ContradictionsPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

// Mock api
const mockGetContradictions = vi.fn()
const mockResolveContradiction = vi.fn()

vi.mock('../../lib/api', () => ({
  api: {
    getContradictions: (...args: unknown[]) => mockGetContradictions(...args),
    resolveContradiction: (...args: unknown[]) => mockResolveContradiction(...args),
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
      <MemoryRouter initialEntries={['/universe/uni-1/contradictions']}>
        <Routes>
          <Route path="/universe/:universeId/contradictions" element={<ContradictionsPage />} />
        </Routes>
      </MemoryRouter>
    </UniverseContext.Provider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  // Mock confirm to always return true
  vi.spyOn(window, 'confirm').mockReturnValue(true)
})

describe('ContradictionsPage', () => {
  it('shows loading state initially', () => {
    mockGetContradictions.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByTestId('loading-state')).toBeInTheDocument()
  })

  it('renders contradiction cards on load', async () => {
    mockGetContradictions.mockResolvedValue({
      contradictions: [
        { id: 'c1', description: 'Character age mismatch', severity: 'high', status: 'open' },
        { id: 'c2', description: 'Timeline conflict', severity: 'low', status: 'open' },
      ],
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Character age mismatch')).toBeInTheDocument()
      expect(screen.getByText('Timeline conflict')).toBeInTheDocument()
    })
    // "high" and "low" appear in both filter buttons and severity badges
    expect(screen.getAllByText('high').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('low').length).toBeGreaterThanOrEqual(1)
  })

  it('shows empty state when no contradictions', async () => {
    mockGetContradictions.mockResolvedValue({ contradictions: [] })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('No Contradictions')).toBeInTheDocument()
    })
  })

  it('shows error state on failure', async () => {
    mockGetContradictions.mockRejectedValue(new Error('Server down'))
    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('error-state')).toBeInTheDocument()
      expect(screen.getByText('Server down')).toBeInTheDocument()
    })
  })

  it('filters contradictions by severity', async () => {
    mockGetContradictions.mockResolvedValue({
      contradictions: [
        { id: 'c1', description: 'High issue', severity: 'high', status: 'open' },
        { id: 'c2', description: 'Low issue', severity: 'low', status: 'open' },
      ],
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('High issue')).toBeInTheDocument()
      expect(screen.getByText('Low issue')).toBeInTheDocument()
    })

    const user = userEvent.setup()
    // Click the "high" filter button (the button, not the severity badge)
    const highButtons = screen.getAllByText('high')
    const filterBtn = highButtons.find((el) => el.tagName === 'BUTTON')!
    await user.click(filterBtn)

    // "High issue" still visible, "Low issue" filtered out
    expect(screen.getByText('High issue')).toBeInTheDocument()
    expect(screen.queryByText('Low issue')).not.toBeInTheDocument()
  })

  it('resolves contradiction optimistically on confirm', async () => {
    mockGetContradictions.mockResolvedValue({
      contradictions: [
        { id: 'c1', description: 'Fix me', severity: 'medium', status: 'open' },
      ],
    })
    mockResolveContradiction.mockResolvedValue(true)
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Fix me')).toBeInTheDocument()
    })

    const user = userEvent.setup()
    await user.click(screen.getByText('Resolve'))

    // Confirm dialog was shown
    expect(window.confirm).toHaveBeenCalledWith('Mark this contradiction as resolved?')

    await waitFor(() => {
      expect(screen.getByText('✓ Resolved')).toBeInTheDocument()
      expect(mockResolveContradiction).toHaveBeenCalledWith('uni-1', 'c1')
    })
  })
})
