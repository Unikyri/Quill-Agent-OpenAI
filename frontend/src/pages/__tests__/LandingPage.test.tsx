import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import LandingPage from '../LandingPage'

vi.mock('../LandingPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

const mockDemoLogin = vi.fn()
vi.mock('../../stores/authStore', () => ({
  useAuthStore: vi.fn((selector?: (state: { demoLogin: typeof mockDemoLogin }) => unknown) => {
    const state = { demoLogin: mockDemoLogin }
    return selector ? selector(state) : state
  }),
}))

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="/" element={<LandingPage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('LandingPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('registers a disposable account, clones the demo universe, and redirects to its Write screen', async () => {
    mockDemoLogin.mockResolvedValueOnce('demo-universe-7')
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: /try the live demo/i }))

    await waitFor(() => {
      expect(mockDemoLogin).toHaveBeenCalledTimes(1)
      expect(mockNavigate).toHaveBeenCalledWith('/universe/demo-universe-7/write')
    })
  })

  it('still offers a plain login link for visitors who do not want the demo', () => {
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: /^log in$/i }))
    expect(mockNavigate).toHaveBeenCalledWith('/login')
  })
})
