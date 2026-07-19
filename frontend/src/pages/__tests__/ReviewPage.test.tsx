import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import ReviewPage from '../ReviewPage'

vi.mock('../ReviewPage.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))

const mockRouteParams = vi.hoisted(() => ({ universeId: 'uni-1', view: 'issues' }))
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useParams: () => mockRouteParams }
})

const publish = vi.fn()
vi.mock('../../components/feedback', () => ({ useFeedback: () => ({ publish }) }))

const getContradictions = vi.fn()
const getPlotHoles = vi.fn()
const resolveContradiction = vi.fn()
const dismissContradiction = vi.fn()
const resolvePlotHole = vi.fn()
const dismissPlotHole = vi.fn()
const listEntityCandidates = vi.fn()
const acceptEntityCandidate = vi.fn()
const dismissEntityCandidate = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    getContradictions: (...args: unknown[]) => getContradictions(...args),
    getPlotHoles: (...args: unknown[]) => getPlotHoles(...args),
    resolveContradiction: (...args: unknown[]) => resolveContradiction(...args),
    dismissContradiction: (...args: unknown[]) => dismissContradiction(...args),
    resolvePlotHole: (...args: unknown[]) => resolvePlotHole(...args),
    dismissPlotHole: (...args: unknown[]) => dismissPlotHole(...args),
    listEntityCandidates: (...args: unknown[]) => listEntityCandidates(...args),
    acceptEntityCandidate: (...args: unknown[]) => acceptEntityCandidate(...args),
    dismissEntityCandidate: (...args: unknown[]) => dismissEntityCandidate(...args),
  },
}))

function pageTree(view = 'issues') {
  return (
    <MemoryRouter initialEntries={[`/universe/${mockRouteParams.universeId}/review/${view}`]}>
      <Routes><Route path="/universe/:universeId/review/:view" element={<ReviewPage />} /></Routes>
    </MemoryRouter>
  )
}

function renderPage(view = 'issues', universeId = 'uni-1') {
  mockRouteParams.universeId = universeId
  mockRouteParams.view = view
  return render(pageTree(view))
}

beforeEach(() => {
  vi.clearAllMocks()
  mockRouteParams.universeId = 'uni-1'
  mockRouteParams.view = 'issues'
  getContradictions.mockResolvedValue({
    contradictions: [
      { id: 'c-low', description: 'A minor date conflict', severity: 'low', status: 'open' },
      { id: 'c-high', description: 'The oath conflicts with chapter one', severity: 'high', evidence_a: 'The old gate was sealed.', status: 'open' },
    ],
  })
  getPlotHoles.mockResolvedValue({ plot_holes: [{ id: 'p-1', title: 'The missing messenger', description: 'No delivery is shown.', first_mentioned_chapter_id: 'chapter-7', status: 'open' }] })
  listEntityCandidates.mockResolvedValue({ candidates: [] })
})

describe('ReviewPage', () => {
  it('uses a single prioritized issues inbox with honest evidence limits', async () => {
    renderPage()
    await waitFor(() => expect(screen.getByTestId('review-contradiction-c-high')).toBeInTheDocument())

    const cards = [...document.querySelectorAll('[data-testid^="review-"]')]
    expect(cards[0]).toHaveAttribute('data-testid', 'review-contradiction-c-high')
    expect(screen.getByText(/model severity: high/i)).toBeInTheDocument()
    expect(screen.getByText(/API did not supply a source excerpt/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'New entities' })).toHaveAttribute('href', '/universe/uni-1/review/candidates')
  })

  it('renders exactly two tabs — Conflicts and New entities — with no Craft tab', async () => {
    renderPage()
    await waitFor(() => expect(screen.getByTestId('review-contradiction-c-high')).toBeInTheDocument())

    const tabs = screen.getByRole('navigation', { name: 'Review views' })
    expect(within(tabs).getAllByRole('link')).toHaveLength(2)
    expect(within(tabs).getByRole('link', { name: 'Conflicts' })).toBeInTheDocument()
    expect(within(tabs).getByRole('link', { name: 'New entities' })).toBeInTheDocument()
    expect(within(tabs).queryByRole('link', { name: /craft/i })).not.toBeInTheDocument()
  })

  it('uses the real plot-hole resolve endpoint only after inline confirmation', async () => {
    resolvePlotHole.mockResolvedValue(undefined)
    renderPage()
    await waitFor(() => expect(screen.getByTestId('review-plot-hole-p-1')).toBeInTheDocument())

    const plotHole = screen.getByTestId('review-plot-hole-p-1')
    fireEvent.click(within(plotHole).getByRole('button', { name: 'Resolve' }))
    expect(within(plotHole).getByText(/mark this finding resolved/i)).toBeInTheDocument()
    expect(resolvePlotHole).not.toHaveBeenCalled()

    fireEvent.click(within(plotHole).getByRole('button', { name: 'Confirm' }))
    await waitFor(() => expect(resolvePlotHole).toHaveBeenCalledWith('uni-1', 'p-1'))
    expect(within(plotHole).getByText('Resolved')).toBeInTheDocument()
  })

  it('shows live candidates as provisional and saves an explicit accept decision', async () => {
    listEntityCandidates.mockResolvedValue({ candidates: [{ entity_id: 'candidate-1', universe_id: 'uni-1', name: 'Orla', type: 'character', confidence: .84, status: 'candidate', evidence_quote: 'Orla took the key.' }] })
    acceptEntityCandidate.mockResolvedValue({ entity: { id: 'e-1' } })
    renderPage('candidates')

    await waitFor(() => expect(screen.getByTestId('candidate-candidate-1')).toBeInTheDocument())
    expect(screen.getByText(/not part of the story until you accept it/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /accept candidate/i }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))
    await waitFor(() => expect(acceptEntityCandidate).toHaveBeenCalledWith('candidate-1'))
    expect(screen.getByText('accepted')).toBeInTheDocument()
  })

  it('treats an unknown view (e.g. the removed craft tab) as unmapped, rendering no tab content', () => {
    renderPage('craft')
    expect(screen.queryByText(/does not provide a persisted craft-notes inbox/i)).not.toBeInTheDocument()
    expect(screen.queryByTestId(/^review-/)).not.toBeInTheDocument()
    expect(screen.queryByTestId(/^candidate-/)).not.toBeInTheDocument()
    expect(getContradictions).not.toHaveBeenCalled()
  })

  it('ignores a deferred A review inbox after the route changes to B', async () => {
    let resolveContradictionsA!: (value: { contradictions: Array<{ id: string; description: string; severity: string; status: string }> }) => void
    let resolveContradictionsB!: (value: { contradictions: Array<{ id: string; description: string; severity: string; status: string }> }) => void
    let resolvePlotHolesA!: (value: { plot_holes: unknown[] }) => void
    let resolvePlotHolesB!: (value: { plot_holes: unknown[] }) => void
    getContradictions.mockImplementation((universeId: string) => new Promise((resolve) => {
      if (universeId === 'uni-a') resolveContradictionsA = resolve
      else resolveContradictionsB = resolve
    }))
    getPlotHoles.mockImplementation((universeId: string) => new Promise((resolve) => {
      if (universeId === 'uni-a') resolvePlotHolesA = resolve
      else resolvePlotHolesB = resolve
    }))

    const view = renderPage('issues', 'uni-a')
    await waitFor(() => expect(getContradictions).toHaveBeenCalledWith('uni-a'))

    mockRouteParams.universeId = 'uni-b'
    view.rerender(pageTree('issues'))
    await waitFor(() => expect(getContradictions).toHaveBeenCalledWith('uni-b'))

    resolveContradictionsB({ contradictions: [{ id: 'b-issue', description: 'B-only conflict', severity: 'high', status: 'open' }] })
    resolvePlotHolesB({ plot_holes: [] })
    await waitFor(() => expect(screen.getByTestId('review-contradiction-b-issue')).toBeInTheDocument())

    resolveContradictionsA({ contradictions: [{ id: 'a-issue', description: 'A-only conflict', severity: 'high', status: 'open' }] })
    resolvePlotHolesA({ plot_holes: [] })
    await waitFor(() => expect(screen.getByTestId('review-contradiction-b-issue')).toBeInTheDocument())
    expect(screen.queryByText('A-only conflict')).not.toBeInTheDocument()
  })
})
