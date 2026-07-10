import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAuthStore } from '../authStore'

const mockMe = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    me: (...args: unknown[]) => mockMe(...args),
  },
}))

describe('authStore.init', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    useAuthStore.setState({ user: null, token: null, isAuthenticated: false })
  })

  it('does nothing when there is no stored token', async () => {
    await useAuthStore.getState().init()
    expect(mockMe).not.toHaveBeenCalled()
    expect(useAuthStore.getState().isAuthenticated).toBe(false)
  })

  it('hydrates the user when a stored token resolves via api.me', async () => {
    localStorage.setItem('token', 'valid-token')
    const user = { id: 'u1', email: 'a@b.com', display_name: 'Alice' }
    mockMe.mockResolvedValue({ user })

    await useAuthStore.getState().init()

    expect(mockMe).toHaveBeenCalled()
    expect(useAuthStore.getState().user).toEqual(user)
    expect(useAuthStore.getState().isAuthenticated).toBe(true)
  })

  it('clears the token when api.me rejects (expired/invalid token)', async () => {
    localStorage.setItem('token', 'stale-token')
    mockMe.mockRejectedValue(new Error('401 unauthorized'))

    await useAuthStore.getState().init()

    expect(localStorage.getItem('token')).toBeNull()
    expect(useAuthStore.getState().user).toBeNull()
    expect(useAuthStore.getState().isAuthenticated).toBe(false)
  })
})
