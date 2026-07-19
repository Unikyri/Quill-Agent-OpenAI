import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import MemoryInspectorPage from '../MemoryInspectorPage'

vi.mock('../MemoryInspectorPage.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))

const mockRouteParams = vi.hoisted(() => ({ universeId: 'uni-1' }))
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useParams: () => mockRouteParams }
})

const mockGetMemoryStatus = vi.fn()
const mockRunDecay = vi.fn()
const mockRecallExplain = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    getMemoryStatus: (...args: unknown[]) => mockGetMemoryStatus(...args),
    runDecay: (...args: unknown[]) => mockRunDecay(...args),
    recallExplain: (...args: unknown[]) => mockRecallExplain(...args),
  },
}))

function pageTree() {
  return (
    <MemoryRouter initialEntries={[`/universe/${mockRouteParams.universeId}/memory`]}>
      <Routes><Route path="/universe/:universeId/memory" element={<MemoryInspectorPage />} /></Routes>
    </MemoryRouter>
  )
}

function renderPage(universeId = 'uni-1') {
  mockRouteParams.universeId = universeId
  return render(pageTree())
}

const recallResponse = {
  query: 'Where was the oath made?',
  pipeline_sizes: { vector: 1 },
  items: [{ id: 'r1', entity_id: 'e1', fact: 'The oath was made at the old gate.', rrf_score: .2, contributions: [], fit_in_budget: true }],
  budget: { max_context_tokens: 1000, available: 700, entities_tokens: 100, vector_tokens: 200, tools_tokens: 0, used_percent: 30, vector_tokens_used: 120 },
}

beforeEach(() => {
  vi.clearAllMocks()
  mockRouteParams.universeId = 'uni-1'
  mockGetMemoryStatus.mockResolvedValue({ consolidated_count: 2, entities: [] })
  mockRunDecay.mockResolvedValue({ ok: true })
  mockRecallExplain.mockResolvedValue(recallResponse)
})

describe('MemoryInspectorPage', () => {
  it('renders Recall, Forgetting, and Context Budget as always-visible stacked sections', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /what does quill remember/i })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /what fit in the prompt/i })).toBeInTheDocument()
    await waitFor(() => expect(mockGetMemoryStatus).toHaveBeenCalledWith('uni-1'))
    expect(await screen.findByRole('heading', { name: /decay, relevance, and consolidation/i })).toBeInTheDocument()
  })

  it('shows the context budget for a recall without any disclosure interaction', async () => {
    renderPage()
    expect(screen.getByText(/run a recall above/i)).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText(/ask about your story/i), { target: { value: 'Where was the oath made?' } })
    fireEvent.click(screen.getByRole('button', { name: /^recall$/i }))

    await waitFor(() => expect(screen.getByTestId('fused-item-r1')).toHaveTextContent(/old gate/i))
    expect(screen.getByTestId('budget-fitted-count')).toHaveTextContent('1')
  })

  it('loads lifecycle data as soon as the page mounts, with no click required', async () => {
    renderPage()
    await waitFor(() => expect(mockGetMemoryStatus).toHaveBeenCalledWith('uni-1'))
    expect(screen.getByText(/2 consolidated memories/i)).toBeInTheDocument()
  })

  it('no longer embeds the Writer Memory panel (moved to the account-scoped Writer Profile)', () => {
    renderPage()
    expect(screen.queryByText(/inspect what quill has learned about your writing/i)).not.toBeInTheDocument()
  })

  it('clears A recall evidence and its budget when the route changes to B', async () => {
    let resolveA!: (value: typeof recallResponse) => void
    mockRecallExplain.mockImplementation((universeId: string) => new Promise((resolve) => {
      if (universeId === 'uni-a') resolveA = resolve
    }))

    const view = renderPage('uni-a')
    fireEvent.change(screen.getByLabelText(/ask about your story/i), { target: { value: 'A question' } })
    fireEvent.click(screen.getByRole('button', { name: /^recall$/i }))
    await waitFor(() => expect(mockRecallExplain).toHaveBeenCalledWith('uni-a', 'A question', 10))

    mockRouteParams.universeId = 'uni-b'
    view.rerender(pageTree())
    expect(screen.getByText(/run a recall above/i)).toBeInTheDocument()

    resolveA({ ...recallResponse, query: 'A question', items: [{ ...recallResponse.items[0], fact: 'A-only evidence' }] })
    await waitFor(() => expect(screen.getByRole('heading', { name: /what does quill remember/i })).toBeInTheDocument())
    expect(screen.queryByText('A-only evidence')).not.toBeInTheDocument()
    expect(screen.getByText(/run a recall above/i)).toBeInTheDocument()
  })
})
