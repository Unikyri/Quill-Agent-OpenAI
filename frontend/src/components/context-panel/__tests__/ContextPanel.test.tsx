import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ContextPanel from '../ContextPanel'
import { useWSStore } from '../../../stores/wsStore'
import { api } from '../../../lib/api'

vi.mock('../../../lib/api', () => ({
  api: {
    getMemoryStatus: vi.fn().mockResolvedValue({ consolidated_count: 0, entities: [] }),
  },
}))

beforeEach(() => {
  vi.clearAllMocks()
  ;(api.getMemoryStatus as ReturnType<typeof vi.fn>).mockResolvedValue({ consolidated_count: 0, entities: [] })
  // Reset store state before each test
  useWSStore.setState({
    status: 'idle',
    lastError: null,
    reconnectAttempt: 0,
    analysisResults: [],
    contradictions: [],
    arbiterNote: null,
    discoveredEntities: [],
    recallItems: [],
    graphPings: [],
    pipeline: null,
    budget: null,
  })
})

describe('ContextPanel', () => {
  it('toggles a collapsed disclosure section', async () => {
    const user = userEvent.setup()
    render(<ContextPanel status="idle" />)

    const summary = screen.getByText('Relevant memory')
    const disclosure = summary.closest('details')
    expect(disclosure).toHaveProperty('open', false)

    await user.click(summary)
    expect(disclosure).toHaveProperty('open', true)

    await user.click(summary)
    expect(disclosure).toHaveProperty('open', false)
  })

  it('shows empty state when no messages', () => {
    render(<ContextPanel status="idle" />)
    expect(screen.getByText('AI contradiction analysis will appear here')).toBeInTheDocument()
    expect(screen.getByText('Semantic memory appears as you write')).toBeInTheDocument()
  })

  it('shows red indicator when disconnected', () => {
    render(<ContextPanel status="closed" />)
    expect(screen.getByTitle('WS: closed')).toBeInTheDocument()
  })

  it('shows green indicator when connected', () => {
    render(<ContextPanel status="open" />)
    expect(screen.getByTitle('WS: open')).toBeInTheDocument()
  })

  it('shows yellow indicator when reconnecting', () => {
    render(<ContextPanel status="reconnecting" />)
    expect(screen.getByTitle('WS: reconnecting')).toBeInTheDocument()
  })

  it('renders contradiction cards from store', () => {
    useWSStore.setState({
      contradictions: [
        { id: 'c1', message: 'Character age mismatch', severity: 'high' },
        { id: 'c2', message: 'Timeline conflict', severity: 'medium' },
      ],
    })

    render(<ContextPanel status="open" />)
    expect(screen.getByText('Character age mismatch')).toBeInTheDocument()
    expect(screen.getByText('Timeline conflict')).toBeInTheDocument()
    expect(screen.getByText('HIGH')).toBeInTheDocument()
    expect(screen.getByText('MEDIUM')).toBeInTheDocument()
  })

  it('renders entity cards from store', () => {
    useWSStore.setState({
      discoveredEntities: [
        { id: 'e1', name: 'Alice', type: 'character' },
      ],
    })

    render(<ContextPanel status="open" />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
  })

  it('renders recall cards from store', () => {
    useWSStore.setState({
      recallItems: [
        { id: 'r1', fact: 'Alice is a detective', score: 0.95 },
      ],
    })

    render(<ContextPanel status="open" />)
    expect(screen.getByText(/Alice is a detective/)).toBeInTheDocument()
    expect(screen.getByText('95%')).toBeInTheDocument()
  })

  it('renders the Arbiter note only once one has actually been synthesized', () => {
    const { rerender } = render(<ContextPanel status="open" />)
    expect(screen.queryByText("🧭 Arbiter's note")).not.toBeInTheDocument()

    useWSStore.setState({ arbiterNote: 'The contradiction about Edric matters most here.' })
    rerender(<ContextPanel status="open" />)
    expect(screen.getByText("🧭 Arbiter's note")).toBeInTheDocument()
    expect(screen.getByText('The contradiction about Edric matters most here.')).toBeInTheDocument()
  })

  it('renders both contradiction and entity data simultaneously', () => {
    useWSStore.setState({
      contradictions: [{ id: 'c1', message: 'Conflict', severity: 'low' }],
      discoveredEntities: [{ id: 'e1', name: 'Bob', type: 'place' }],
    })

    render(<ContextPanel status="open" />)
    expect(screen.getByText('Conflict')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })
})

describe('ContextPanel — Context Budget section', () => {
  it('renders token breakdown and used_percent from budget slice', () => {
    useWSStore.setState({
      budget: {
        max_context_tokens: 8000,
        available: 4000,
        entities_tokens: 1400,
        vector_tokens: 1600,
        tools_tokens: 1000,
        used_percent: 50,
      },
    })
    render(<ContextPanel status="open" />)
    expect(screen.getByText('Context Budget')).toBeInTheDocument()
    expect(screen.getByText('50.0% used')).toBeInTheDocument()
  })
})

describe('ContextPanel — Retrieval Sources badges', () => {
  it('splits a comma-joined source field into distinct badges', () => {
    useWSStore.setState({
      recallItems: [{ id: 'r1', fact: 'Alice is a detective', score: 0.9, source: 'vector,graph' }],
    })
    render(<ContextPanel status="open" />)
    expect(screen.getByText('vector')).toBeInTheDocument()
    expect(screen.getByText('graph')).toBeInTheDocument()
  })
})

describe('ContextPanel — Live Pipeline stepper', () => {
  it('shows the active stage without a stale count when the count is omitted', () => {
    useWSStore.setState({ pipeline: { stage: 'checking_contradictions' } })
    render(<ContextPanel status="open" />)
    expect(screen.getByText('Live Pipeline')).toBeInTheDocument()
    expect(screen.queryByText('0', { exact: true })).not.toBeInTheDocument()
  })
})

describe('ContextPanel — Entity Lifecycle sparkline', () => {
  it('renders a lifecycle chip and degrades sparkline for single-point history', async () => {
    ;(api.getMemoryStatus as ReturnType<typeof vi.fn>).mockResolvedValue({
      consolidated_count: 0,
      entities: [
        {
          id: 'e1', name: 'Alice', type: 'character', relevance_score: 0.5, status: 'active',
          consolidated: false, lifecycle: 'active', history: [{ score: 0.5, recorded_at: '2026-07-07T12:00:00Z' }],
        },
      ],
    })
    render(<ContextPanel status="open" universeId="u1" />)
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument())
    expect(screen.getAllByText('active').length).toBeGreaterThan(0)
    expect(screen.queryByTestId('sparkline-path-e1')).not.toBeInTheDocument()
    expect(screen.getByTestId('sparkline-dot-e1')).toBeInTheDocument()
  })

  it('renders no path/dot for empty history without throwing', async () => {
    ;(api.getMemoryStatus as ReturnType<typeof vi.fn>).mockResolvedValue({
      consolidated_count: 0,
      entities: [
        {
          id: 'e2', name: 'Bob', type: 'character', relevance_score: 0.5, status: 'active',
          consolidated: false, lifecycle: 'active', history: [],
        },
      ],
    })
    render(<ContextPanel status="open" universeId="u1" />)
    await waitFor(() => expect(screen.getByText('Bob')).toBeInTheDocument())
    expect(screen.queryByTestId('sparkline-path-e2')).not.toBeInTheDocument()
    expect(screen.queryByTestId('sparkline-dot-e2')).not.toBeInTheDocument()
  })
})

describe('ContextPanel — lifecycle fetch failures', () => {
  it('shows an accessible retry when the lifecycle cannot be loaded', async () => {
    ;(api.getMemoryStatus as ReturnType<typeof vi.fn>)
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ consolidated_count: 0, entities: [] })

    render(<ContextPanel status="open" universeId="u1" />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the memory lifecycle.')
    screen.getByRole('button', { name: 'Retry' }).click()

    await waitFor(() => expect(api.getMemoryStatus).toHaveBeenCalledTimes(2))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('keeps the last-known lifecycle visible and labels it when refresh fails', async () => {
    ;(api.getMemoryStatus as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        consolidated_count: 0,
        entities: [{
          id: 'e1', name: 'Alice', type: 'character', relevance_score: 0.5, status: 'active',
          consolidated: false, lifecycle: 'active', history: [],
        }],
      })
      .mockRejectedValue(new Error('offline'))

    render(<ContextPanel status="open" universeId="u1" />)
    await screen.findByText('Alice')

    useWSStore.setState({ pipeline: { stage: 'entities_extracted' } })

    expect(await screen.findByText('Could not refresh the lifecycle. Showing last-known data.')).toBeInTheDocument()
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Retry')
  })
})
