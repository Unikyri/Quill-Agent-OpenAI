import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import ContextPanel from '../ContextPanel'
import { useWSStore } from '../../../stores/wsStore'

beforeEach(() => {
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
