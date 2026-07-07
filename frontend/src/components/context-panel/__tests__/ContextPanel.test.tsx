import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
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
    discoveredEntities: [],
    recallItems: [],
    graphPings: [],
    pipeline: null,
    budget: null,
  })
})

describe('ContextPanel', () => {
  it('shows empty state when no messages', () => {
    render(<ContextPanel status="idle" />)
    expect(screen.getByText(/AI insights will appear here/)).toBeInTheDocument()
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
    expect(screen.getByText(/Alice/)).toBeInTheDocument()
    expect(screen.getByText(/Type: character/)).toBeInTheDocument()
    expect(screen.getByText('NEW')).toBeInTheDocument()
  })

  it('renders recall cards from store', () => {
    useWSStore.setState({
      recallItems: [
        { id: 'r1', fact: 'Alice is a detective', score: 0.95 },
      ],
    })

    render(<ContextPanel status="open" />)
    expect(screen.getByText(/Alice is a detective/)).toBeInTheDocument()
    expect(screen.getByText('Confidence: 95%')).toBeInTheDocument()
  })

  it('shows active card count', () => {
    useWSStore.setState({
      contradictions: [{ id: 'c1', message: 'Conflict', severity: 'low' }],
      discoveredEntities: [{ id: 'e1', name: 'Bob', type: 'place' }],
    })

    render(<ContextPanel status="open" />)
    expect(screen.getByText('2 active')).toBeInTheDocument()
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
    expect(screen.getByText('50%')).toBeInTheDocument()
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
