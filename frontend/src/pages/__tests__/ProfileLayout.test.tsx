import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import ProfileLayout from '../ProfileLayout'

vi.mock('../ProfileLayout.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))

const mockLogout = vi.fn()
vi.mock('../../stores/authStore', () => ({
  useAuthStore: vi.fn((selector?: (state: { user: { display_name: string; email: string }; logout: () => void }) => unknown) => {
    const state = { user: { display_name: 'Author Name', email: 'writer@example.com' }, logout: mockLogout }
    return selector ? selector(state) : state
  }),
}))

function Content() {
  return <div>Writer profile content</div>
}

function renderLayout() {
  return render(
    <MemoryRouter initialEntries={['/profile/memory']}>
      <Routes>
        <Route path="/profile" element={<ProfileLayout />}>
          <Route path="memory" element={<Content />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('ProfileLayout', () => {
  it('renders the brand, a way back to Home, and the routed content', () => {
    renderLayout()
    expect(screen.getByRole('link', { name: /quill home/i })).toHaveAttribute('href', '/dashboard')
    expect(screen.getByRole('link', { name: /back to home/i })).toHaveAttribute('href', '/dashboard')
    expect(screen.getByText('Writer profile content')).toBeInTheDocument()
  })

  it('signs the writer out from the account menu', async () => {
    const user = userEvent.setup()
    renderLayout()

    await user.click(screen.getByRole('button', { name: /open account menu/i }))
    await user.click(screen.getByRole('menuitem', { name: /sign out/i }))

    expect(mockLogout).toHaveBeenCalledTimes(1)
  })
})
