import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import MemoryInspectorPage from '../MemoryInspectorPage'

vi.mock('../MemoryInspectorPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockGetMemoryStatus = vi.fn()
const mockRunDecay = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    getMemoryStatus: (...args: unknown[]) => mockGetMemoryStatus(...args),
    runDecay: (...args: unknown[]) => mockRunDecay(...args),
    recallExplain: vi.fn(),
  },
}))

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/universe/uni-1/memory']}>
      <Routes>
        <Route path="/universe/:universeId/memory" element={<MemoryInspectorPage />} />
      </Routes>
    </MemoryRouter>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockGetMemoryStatus.mockResolvedValue({ consolidated_count: 0, entities: [] })
  mockRunDecay.mockResolvedValue({ ok: true })
})

describe('MemoryInspectorPage', () => {
  it('composes DecayTimeline, FusionExplorer, and BudgetTheater', async () => {
    renderPage()

    await waitFor(() => expect(mockGetMemoryStatus).toHaveBeenCalledWith('uni-1'))

    expect(screen.getByText(/decay timeline/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /explain/i })).toBeInTheDocument()
    expect(screen.getByText(/budget theater/i)).toBeInTheDocument()
  })

  it('renders empty states for all acts without crashing when there is no data', async () => {
    renderPage()

    await waitFor(() => expect(screen.getByText(/no memory data yet/i)).toBeInTheDocument())
    expect(screen.getByText(/no budget data yet/i)).toBeInTheDocument()
  })

  it('disables the decay control while a request is in flight', async () => {
    renderPage()
    await waitFor(() => expect(mockGetMemoryStatus).toHaveBeenCalledTimes(1))

    const btn = screen.getByRole('button', { name: /advance chapter/i })
    fireEvent.click(btn)
    expect(btn).toBeDisabled()

    await waitFor(() => expect(btn).not.toBeDisabled())
  })
})
