import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import UniverseLayout from '../UniverseLayout'

vi.mock('../UniverseLayout.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))

const mockGetUniverse = vi.fn()
const mockListWorks = vi.fn()
const mockUpdateUniverse = vi.fn()
const mockDemoReset = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    getUniverse: (...args: unknown[]) => mockGetUniverse(...args),
    listWorks: (...args: unknown[]) => mockListWorks(...args),
    updateUniverse: (...args: unknown[]) => mockUpdateUniverse(...args),
    demoReset: (...args: unknown[]) => mockDemoReset(...args),
  },
}))

const mockFetchUniverses = vi.fn()
let universeStoreState = {
  universes: [
    { id: 'uni-1', name: 'Middle Earth' },
    { id: 'uni-2', name: 'Second World' },
  ],
  fetchUniverses: mockFetchUniverses,
}
vi.mock('../../stores/universeStore', () => ({
  useUniverseStore: vi.fn((selector?: (state: typeof universeStoreState) => unknown) =>
    selector ? selector(universeStoreState) : universeStoreState,
  ),
}))

const mockLogout = vi.fn()
vi.mock('../../stores/authStore', () => ({
  useAuthStore: vi.fn((selector?: (state: { user: { display_name: string; email: string }; logout: () => void }) => unknown) => {
    const state = { user: { display_name: 'Author Name', email: 'writer@example.com' }, logout: mockLogout }
    return selector ? selector(state) : state
  }),
}))

const mockSetUniverseScope = vi.fn()
let wsStoreState = {
  status: 'open',
  lastError: null as string | null,
  submissions: {},
  setUniverseScope: mockSetUniverseScope,
}
vi.mock('../../stores/wsStore', () => ({
  useWSStore: vi.fn((selector: (state: typeof wsStoreState) => unknown) => selector(wsStoreState)),
}))
vi.mock('../../hooks/useWS', () => ({ useWS: () => ({ status: wsStoreState.status }) }))
const mockFeedbackPublish = vi.fn(() => 'feedback-id')
const mockFeedbackUpdate = vi.fn()
vi.mock('../../components/feedback', () => ({ useFeedback: () => ({ publish: mockFeedbackPublish, update: mockFeedbackUpdate }) }))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

function Content({ label }: { label: string }) {
  return <div>{label}</div>
}

function renderLayout(initialRoute = '/universe/uni-1/write') {
  return render(
    <MemoryRouter initialEntries={[initialRoute]}>
      <Routes>
        <Route path="/universe/:universeId" element={<UniverseLayout />}>
          <Route path="write" element={<Content label="Write content" />} />
          <Route path="explore/entities" element={<Content label="Explore content" />} />
          <Route path="explore/map" element={<Content label="Map content" />} />
          <Route path="memory" element={<Content label="Memory content" />} />
          <Route path="review/issues" element={<Content label="Review content" />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  sessionStorage.clear()
  universeStoreState = {
    universes: [
      { id: 'uni-1', name: 'Middle Earth' },
      { id: 'uni-2', name: 'Second World' },
    ],
    fetchUniverses: mockFetchUniverses,
  }
  wsStoreState = { status: 'open', lastError: null, submissions: {}, setUniverseScope: mockSetUniverseScope }
  mockGetUniverse.mockResolvedValue({ universe: { id: 'uni-1', name: 'Middle Earth' } })
  mockListWorks.mockResolvedValue({ works: [{ id: 'work-1', title: 'A Work' }] })
  mockUpdateUniverse.mockResolvedValue({ universe: { id: 'uni-1', name: 'Middle Earth', genre_tags: [] } })
  mockDemoReset.mockResolvedValue({ universe_id: 'uni-1' })
})

describe('UniverseLayout', () => {
  it('renders a five-destination application bar and a skip-link target', async () => {
    renderLayout()

    await waitFor(() => expect(screen.getByText('Write content')).toBeInTheDocument())

    expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute('href', '/dashboard')
    expect(screen.getByRole('link', { name: 'Write' })).toHaveAttribute('href', '/universe/uni-1/write')
    expect(screen.getByRole('link', { name: 'Map' })).toHaveAttribute('href', '/universe/uni-1/explore/map')
    expect(screen.getByRole('link', { name: 'Memory' })).toHaveAttribute('href', '/universe/uni-1/memory')
    expect(screen.getByRole('link', { name: 'Review' })).toHaveAttribute('href', '/universe/uni-1/review/issues')
    expect(screen.queryByText('Works & Chapters')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Skip to content' })).toHaveAttribute('href', '#universe-main')
    expect(document.getElementById('universe-main')).toHaveAttribute('tabindex', '-1')
  })

  it('uses real WebSocket state for the persistent status', async () => {
    wsStoreState = {
      ...wsStoreState,
      submissions: {
        'submission-1': { submissionId: 'submission-1', paragraphRef: 'p-1', universeId: 'uni-1', phase: 'analyzing' },
      },
    }
    renderLayout()

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Analyzing 1 paragraph'))
    expect(screen.queryByText(/autosave/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Qwen health/i)).not.toBeInTheDocument()
  })

  it('switches universes into the canonical Write entry route', async () => {
    const user = userEvent.setup()
    renderLayout()

    await waitFor(() => expect(screen.getByRole('button', { name: /Switch universe, current universe Middle Earth/ })).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: /Switch universe, current universe Middle Earth/ }))
    await user.click(screen.getByRole('menuitem', { name: 'Second World' }))

    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-2/write')
    expect(mockSetUniverseScope).toHaveBeenCalledWith('uni-1')
  })

  it('edits universe genres with the exact optional genre_tags payload', async () => {
    mockGetUniverse.mockResolvedValue({ universe: { id: 'uni-1', name: 'Middle Earth', genre_tags: ['fantasy'] } })
    const user = userEvent.setup()
    renderLayout()

    await user.click(await screen.findByRole('button', { name: /Switch universe, current universe Middle Earth/ }))
    await user.click(screen.getByRole('menuitem', { name: 'Edit genres' }))

    const dialog = screen.getByRole('dialog', { name: 'Edit genres for Middle Earth' })
    await user.click(within(dialog).getByRole('button', { name: 'Remove Fantasy' }))
    expect(within(dialog).getByText('No genres selected. Genres are optional.')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Save genres' }))

    await waitFor(() => expect(mockUpdateUniverse).toHaveBeenCalledWith('uni-1', { genre_tags: [] }))
    expect(mockFeedbackUpdate).toHaveBeenCalledWith('feedback-id', expect.objectContaining({ status: 'completed' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('keeps a failed genre save visible and retryable', async () => {
    mockUpdateUniverse.mockRejectedValueOnce(new Error('forbidden'))
    const user = userEvent.setup()
    renderLayout()

    await user.click(await screen.findByRole('button', { name: /Switch universe, current universe Middle Earth/ }))
    await user.click(screen.getByRole('menuitem', { name: 'Edit genres' }))
    const dialog = screen.getByRole('dialog', { name: 'Edit genres for Middle Earth' })
    await user.click(within(dialog).getByRole('button', { name: 'Save genres' }))

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('forbidden')
    expect(mockFeedbackUpdate).toHaveBeenCalledWith('feedback-id', expect.objectContaining({ status: 'failed' }))
    await user.click(within(dialog).getByRole('button', { name: 'Try again' }))

    await waitFor(() => expect(mockUpdateUniverse).toHaveBeenCalledTimes(2))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('keeps the guided-demo journey pending until real activity is observed', async () => {
    localStorage.setItem('quill-guided-demo-universe-id', 'uni-1')
    wsStoreState = {
      ...wsStoreState,
      submissions: {
        'submission-1': { submissionId: 'submission-1', paragraphRef: 'p-1', universeId: 'uni-1', phase: 'done' },
      },
    }
    renderLayout()

    expect(await screen.findByRole('heading', { name: 'Six steps, only real progress' })).toBeInTheDocument()
    expect(await screen.findByText('3 verified steps')).toBeInTheDocument()
    expect(screen.getByText('A completed analysis result has been observed for this demo.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open map' })).toHaveAttribute('href', '/universe/uni-1/explore/map')
    expect(screen.getByRole('link', { name: 'Open Memory' })).toHaveAttribute('href', '/universe/uni-1/memory')
    expect(screen.getByRole('link', { name: 'Open Review' })).toHaveAttribute('href', '/universe/uni-1/review/issues')
    expect(screen.getByText('Ask Memory a lore question')).toBeInTheDocument()
    expect(screen.getByText('Review a real issue')).toBeInTheDocument()
  })

  it('resets the demo inline from step 1 now that Dashboard has no reset control', async () => {
    localStorage.setItem('quill-guided-demo-universe-id', 'uni-1')
    const user = userEvent.setup()
    renderLayout()

    expect(await screen.findByRole('heading', { name: 'Six steps, only real progress' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Reset on Home' })).not.toBeInTheDocument()

    const resetButton = screen.getByRole('button', { name: 'Reset demo' })
    await user.click(resetButton)

    await waitFor(() => expect(mockDemoReset).toHaveBeenCalledTimes(1))
    expect(mockDemoReset.mock.calls[0][0]).toEqual(expect.any(String))
  })

  it('records the map step only after the demo route is actually opened', async () => {
    localStorage.setItem('quill-guided-demo-universe-id', 'uni-1')
    renderLayout('/universe/uni-1/explore/map')

    expect(await screen.findByText('2 verified steps')).toBeInTheDocument()
    expect(screen.getByText('The relationship map has been opened.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open Write' })).toHaveAttribute('href', '/universe/uni-1/write')
    expect(screen.getByText('Ask Memory a lore question')).toBeInTheDocument()
  })

  it('supports Alt+number navigation without intercepting editor typing', async () => {
    renderLayout()
    await waitFor(() => expect(screen.getByText('Write content')).toBeInTheDocument())

    fireEvent.keyDown(window, { key: '3', altKey: true })
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/explore/map')
  })

  it('retries the actual universe and work requests after a load failure', async () => {
    const user = userEvent.setup()
    mockGetUniverse.mockRejectedValueOnce(new Error('Not found'))
    mockListWorks.mockRejectedValueOnce(new Error('Not found'))
    renderLayout()

    await waitFor(() => expect(screen.getByText(/Failed to load universe: Not found/)).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(mockGetUniverse).toHaveBeenCalledTimes(2))
    expect(mockListWorks).toHaveBeenCalledTimes(2)
    expect(await screen.findByText('Write content')).toBeInTheDocument()
  })
})
