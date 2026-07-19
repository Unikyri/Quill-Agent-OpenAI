import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import WriterMemoryPanel from '../WriterMemoryPanel'

vi.mock('../WriterMemoryPanel.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))
vi.mock('../../genres/GenreTagPicker.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))
vi.mock('../../../lib/genres', () => ({
  GENRE_OPTIONS: [
    { value: 'fantasy', label: 'Fantasy' },
    { value: 'horror', label: 'Horror' },
  ],
}))

const mockGetWriterPreferences = vi.fn()
const mockListUniverses = vi.fn()
const mockCorrectWriterPreference = vi.fn()
const mockDeactivateWriterPreference = vi.fn()
const mockGetWriterPreferenceEvidence = vi.fn()
vi.mock('../../../lib/api', () => ({
  api: {
    getWriterPreferences: (...args: unknown[]) => mockGetWriterPreferences(...args),
    listUniverses: (...args: unknown[]) => mockListUniverses(...args),
    correctWriterPreference: (...args: unknown[]) => mockCorrectWriterPreference(...args),
    deactivateWriterPreference: (...args: unknown[]) => mockDeactivateWriterPreference(...args),
    getWriterPreferenceEvidence: (...args: unknown[]) => mockGetWriterPreferenceEvidence(...args),
  },
}))

const observations = [
  { id: 'obs-1', user_id: 'u1', universe_id: 'uni-a', metric: 'adverb_density', value: 3.1, sample_size: 400, computed_at: '2026-01-01' },
  { id: 'obs-2', user_id: 'u1', metric: 'dialogue_ratio', value: 40, sample_size: 900, computed_at: '2026-01-02' },
]

const preferences = [
  {
    id: 'pref-universal', user_id: 'u1', statement: 'The writer values long sentence lengths.',
    scope: 'universal', genre_tags: [], confidence: 0.8, relevance_score: 0.9, lifecycle: 'active',
    last_reinforced_at: '2026-01-01', observation_ids: [], feedback_event_ids: [], created_at: '2026-01-01',
  },
  {
    id: 'pref-genre', user_id: 'u1', statement: 'The writer keeps a dialogue-forward balance.',
    scope: 'genre_bound', genre_tags: ['horror'], confidence: 0.6, relevance_score: 0.7, lifecycle: 'active',
    last_reinforced_at: '2026-01-01', observation_ids: [], feedback_event_ids: [], created_at: '2026-01-01',
  },
]

beforeEach(() => {
  vi.clearAllMocks()
  mockGetWriterPreferences.mockResolvedValue({ preferences, observations })
  mockListUniverses.mockResolvedValue({ universes: [{ id: 'uni-a', name: 'Middle Earth' }, { id: 'uni-b', name: 'Second World' }] })
  mockCorrectWriterPreference.mockResolvedValue({ preference: { ...preferences[0], scope: 'genre_bound', genre_tags: ['horror'] } })
  mockDeactivateWriterPreference.mockResolvedValue(undefined)
})

describe('WriterMemoryPanel (account-scoped)', () => {
  it('takes no universeId and never fetches a single universe', async () => {
    render(<WriterMemoryPanel />)
    await waitFor(() => expect(mockGetWriterPreferences).toHaveBeenCalled())
    expect(screen.queryByText(/in this universe/i)).not.toBeInTheDocument()
  })

  it('shows observations across every universe the writer has worked in, each labeled with its scope', async () => {
    render(<WriterMemoryPanel />)

    await waitFor(() => expect(screen.getByText(/middle earth/i)).toBeInTheDocument())
    expect(screen.getByText(/all universes/i)).toBeInTheDocument()
  })

  it('makes a genre-bound preference universal in a single click', async () => {
    mockCorrectWriterPreference.mockResolvedValue({ preference: { ...preferences[1], scope: 'universal', genre_tags: [] } })
    const user = userEvent.setup()
    render(<WriterMemoryPanel />)

    await waitFor(() => expect(screen.getByText(preferences[1].statement)).toBeInTheDocument())
    const genreCard = screen.getByText(preferences[1].statement).closest('article') as HTMLElement
    await user.click(within(genreCard).getByRole('button', { name: 'Make universal' }))

    await waitFor(() => expect(mockCorrectWriterPreference).toHaveBeenCalledWith('pref-genre', { scope: 'universal', genre_tags: [] }))
  })

  it('requires picking genres from the global list before binding a universal preference to a genre', async () => {
    const user = userEvent.setup()
    render(<WriterMemoryPanel />)

    await waitFor(() => expect(screen.getByText(preferences[0].statement)).toBeInTheDocument())
    const universalCard = screen.getByText(preferences[0].statement).closest('article') as HTMLElement
    await user.click(within(universalCard).getByRole('button', { name: 'Make genre-bound' }))

    // No universe context to infer a genre from — the picker must appear instead of an immediate API call.
    expect(mockCorrectWriterPreference).not.toHaveBeenCalled()
    await user.click(within(universalCard).getByRole('checkbox', { name: 'Horror' }))
    await user.click(within(universalCard).getByRole('button', { name: 'Save scope' }))

    await waitFor(() => expect(mockCorrectWriterPreference).toHaveBeenCalledWith('pref-universal', { scope: 'genre_bound', genre_tags: ['horror'] }))
  })
})
