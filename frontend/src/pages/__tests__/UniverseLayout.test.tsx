import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import UniverseLayout from '../UniverseLayout'

// CSS module mock
vi.mock('../UniverseLayout.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

// Mock api
const mockGetUniverse = vi.fn()
const mockListWorks = vi.fn()
const mockListEntities = vi.fn()

vi.mock('../../lib/api', () => ({
  api: {
    getUniverse: (...args: unknown[]) => mockGetUniverse(...args),
    listWorks: (...args: unknown[]) => mockListWorks(...args),
    listEntities: (...args: unknown[]) => mockListEntities(...args),
  },
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

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

// Simple child tab for testing
function WorksTab() {
  return <div>Works Content</div>
}
function GraphTab() {
  return <div>Graph Content</div>
}

function renderLayout(initialRoute = '/universe/uni-1/works') {
  return render(
    <MemoryRouter initialEntries={[initialRoute]}>
      <Routes>
        <Route path="/universe/:universeId" element={<UniverseLayout />}>
          <Route path="works" element={<WorksTab />} />
          <Route path="graph" element={<GraphTab />} />
        </Route>
      </Routes>
    </MemoryRouter>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockGetUniverse.mockResolvedValue({
    universe: { id: 'uni-1', name: 'Middle Earth', genre: 'Fantasy', format: 'Novel Series' },
  })
  mockListWorks.mockResolvedValue({
    works: [{ id: 'w1', title: 'The Hobbit', type: 'novel', order_index: 1 }],
  })
  mockListEntities.mockResolvedValue({
    entities: [],
    pagination: { page: 1, limit: 1, total: 12, total_pages: 12 },
  })
})

describe('UniverseLayout', () => {
  it('shows loading state while fetching', () => {
    mockGetUniverse.mockReturnValue(new Promise(() => {}))
    mockListWorks.mockReturnValue(new Promise(() => {}))
    renderLayout()
    expect(screen.getByText('Loading universe…')).toBeInTheDocument()
  })

  it('renders universe switcher card and all 9 nested shell nav items after load', async () => {
    renderLayout()

    // Universe name appears in the switcher card
    await waitFor(() => {
      expect(screen.getAllByText('Middle Earth').length).toBeGreaterThanOrEqual(1)
    })

    expect(screen.getByText('Fantasy · 12 entities')).toBeInTheDocument()
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Works/ })).toBeInTheDocument()
    expect(screen.getByText('Editor')).toBeInTheDocument()
    expect(screen.getByText('Entities')).toBeInTheDocument()
    expect(screen.getByText('Graph')).toBeInTheDocument()
    expect(screen.getByText('Timeline')).toBeInTheDocument()
    expect(screen.getByText('Contradictions')).toBeInTheDocument()
    expect(screen.getByText('Plot Holes')).toBeInTheDocument()
    expect(screen.getByText('Ingestion')).toBeInTheDocument()
  })

  it('renders default Works tab content', async () => {
    renderLayout('/universe/uni-1/works')

    await waitFor(() => {
      expect(screen.getByText('Works Content')).toBeInTheDocument()
    })
  })

  it('navigates to Graph tab on click', async () => {
    const user = userEvent.setup()
    renderLayout('/universe/uni-1/works')

    await waitFor(() => {
      expect(screen.getByText('Graph')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Graph'))

    // Graph tab content should render
    await waitFor(() => {
      expect(screen.getByText('Graph Content')).toBeInTheDocument()
    })
    // Works tab content should be gone
    expect(screen.queryByText('Works Content')).not.toBeInTheDocument()
  })

  it('shows error state when API fails', async () => {
    mockGetUniverse.mockRejectedValue(new Error('Not found'))
    mockListWorks.mockRejectedValue(new Error('Not found'))
    renderLayout()

    await waitFor(() => {
      expect(screen.getByText(/Failed to load universe/)).toBeInTheDocument()
      expect(screen.getByText(/Not found/)).toBeInTheDocument()
    })
  })

  it('clicking the universe switcher card navigates back to the dashboard', async () => {
    const user = userEvent.setup()
    renderLayout()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Middle Earth/ })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /Middle Earth/ }))
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard')
  })

  it('shows the current user in the sidebar footer and signs out on click', async () => {
    const user = userEvent.setup()
    renderLayout()

    await waitFor(() => {
      expect(screen.getByText('Author Name')).toBeInTheDocument()
    })
    expect(screen.getByText('writer@example.com')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Sign out/i }))
    expect(mockLogout).toHaveBeenCalled()
  })

  it('collapses the sidebar and reveals a menu toggle in the header', async () => {
    const user = userEvent.setup()
    renderLayout()

    await waitFor(() => {
      expect(screen.getByRole('link', { name: /Works/ })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /Hide panel/i }))

    expect(screen.queryByRole('link', { name: /Works/ })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Show panel/i })).toBeInTheDocument()
  })

  it('shows the active tab title and a recall search stub in the header', async () => {
    renderLayout('/universe/uni-1/works')

    await waitFor(() => {
      expect(screen.getByText('Recall from the universe…')).toBeInTheDocument()
    })
  })
})
