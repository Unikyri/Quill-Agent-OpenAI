import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import DashboardPage from '../DashboardPage'

// CSS module mock
vi.mock('../DashboardPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockFetchUniverses = vi.fn()
const universeStoreState = {
  universes: [
    { id: 'uni-1', name: 'World One', genre: 'fantasy', format: 'novel' },
  ],
  fetchUniverses: mockFetchUniverses,
}
vi.mock('../../stores/universeStore', () => ({
  useUniverseStore: vi.fn((selector?: (state: typeof universeStoreState) => unknown) =>
    selector ? selector(universeStoreState) : universeStoreState
  ),
}))

const mockLogout = vi.fn()
const authStoreState = {
  user: { id: 'u1', email: 'writer@example.com', display_name: 'Author Name' },
  logout: mockLogout,
}
vi.mock('../../stores/authStore', () => ({
  useAuthStore: vi.fn((selector?: (state: typeof authStoreState) => unknown) =>
    selector ? selector(authStoreState) : authStoreState
  ),
}))

const mockUpdateUniverse = vi.fn()
const mockDeleteUniverse = vi.fn()
const mockCreateUniverse = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    updateUniverse: (...args: unknown[]) => mockUpdateUniverse(...args),
    deleteUniverse: (...args: unknown[]) => mockDeleteUniverse(...args),
    createUniverse: (...args: unknown[]) => mockCreateUniverse(...args),
  },
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

function renderPage() {
  return render(
    <MemoryRouter>
      <DashboardPage />
    </MemoryRouter>
  )
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('prompt', vi.fn(() => null))
    vi.stubGlobal('confirm', vi.fn(() => false))
  })

  it('navigates to the universe when the card itself receives Enter', async () => {
    renderPage()
    const card = screen.getByText('World One').closest('[role="button"]') as HTMLElement
    card.focus()
    await userEvent.keyboard('{Enter}')
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1')
  })

  it('navigates when clicking a non-button descendant of the card (e.g. the title)', async () => {
    renderPage()
    const user = userEvent.setup()

    await user.click(screen.getByText('World One'))
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1')
  })

  it('does not navigate when Enter/Space is pressed on the nested Edit or Delete button', async () => {
    renderPage()
    const user = userEvent.setup()

    screen.getByLabelText('Edit universe').focus()
    await user.keyboard('{Enter}')
    expect(mockNavigate).not.toHaveBeenCalled()

    screen.getByLabelText('Delete universe').focus()
    await user.keyboard(' ')
    expect(mockNavigate).not.toHaveBeenCalled()
  })
})
