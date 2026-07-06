import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import PlotHoleList, { type PlotHole } from '../PlotHoleList'

vi.mock('../PlotHoleList.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

const plotHoles: PlotHole[] = [
  {
    id: 'p1',
    title: 'Missing letter thread',
    description: 'The letter to the Order is never followed up on.',
    status: 'open',
    related_entity_ids: ['e1'],
    first_mentioned_chapter_id: 'ch-4',
  },
  {
    id: 'p2',
    title: 'Unresolved oath',
    description: 'Kaelen swears an oath that is never paid off.',
    status: 'open',
  },
]

describe('PlotHoleList', () => {
  it('renders OPEN THREAD kicker, title and description for each hole', () => {
    render(<PlotHoleList plotHoles={plotHoles} universeId="uni-1" />)
    expect(screen.getAllByText('OPEN THREAD').length).toBe(2)
    expect(screen.getByText('Missing letter thread')).toBeInTheDocument()
    expect(screen.getByText(/never followed up on/)).toBeInTheDocument()
  })

  it('shows "Go to chapter →" only when a first-mentioned chapter exists, and navigates', () => {
    render(<PlotHoleList plotHoles={plotHoles} universeId="uni-1" />)
    const links = screen.getAllByText(/Go to chapter/)
    expect(links.length).toBe(1)
    fireEvent.click(links[0])
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/editor/ch-4')
  })
})
