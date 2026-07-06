import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ContradictionList, { type Contradiction } from '../ContradictionList'

vi.mock('../ContradictionList.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockResolve = vi.fn()
const mockDismiss = vi.fn()

vi.mock('../../../lib/api', () => ({
  api: {
    resolveContradiction: (...args: unknown[]) => mockResolve(...args),
    dismissContradiction: (...args: unknown[]) => mockDismiss(...args),
  },
}))

const contradictions: Contradiction[] = [
  {
    id: 'c1',
    severity: 'high',
    description: 'Kaelen is at the lighthouse, but Ch. 2 left him imprisoned in Veridia.',
    suggestion: 'Clarify how he escaped Veridia, or adjust his location.',
    evidence_a: 'Kaelen stood at the lighthouse door.',
    evidence_a_chapter_id: 'ch-3',
    evidence_b: 'They threw Kaelen into the Veridia dungeons.',
    evidence_b_chapter_id: 'ch-2',
    status: 'open',
  },
]

describe('ContradictionList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.confirm = vi.fn(() => true)
  })

  it('renders severity pill, dual evidence panels and suggestion', () => {
render(<ContradictionList universeId="uni-1" contradictions={contradictions} />)
    expect(screen.getAllByText('high').length).toBeGreaterThan(0)
    expect(screen.getByText(/imprisoned in Veridia/)).toBeInTheDocument()
    expect(screen.getByText(/Kaelen stood at the lighthouse door/)).toBeInTheDocument()
    expect(screen.getByText(/threw Kaelen into the Veridia dungeons/)).toBeInTheDocument()
    expect(screen.getByText(/Clarify how he escaped/)).toBeInTheDocument()
  })

  it('resolves a contradiction and dims the card', async () => {
    render(<ContradictionList universeId="uni-1" contradictions={contradictions} />)
    fireEvent.click(screen.getByRole('button', { name: /resolve/i }))
    await waitFor(() => expect(mockResolve).toHaveBeenCalledWith('uni-1', 'c1'))
    expect(screen.getByText(/Resolved/)).toBeInTheDocument()
  })

  it('dismisses a contradiction and shows dismissed state', async () => {
    render(<ContradictionList universeId="uni-1" contradictions={contradictions} />)
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))
    await waitFor(() => expect(mockDismiss).toHaveBeenCalledWith('uni-1', 'c1'))
    expect(screen.getByText(/Dismissed/)).toBeInTheDocument()
  })
})
