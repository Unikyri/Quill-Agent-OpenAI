import { create } from 'zustand'
import { api } from '../lib/api'
import { createOpaqueDemoId, guidedDemoSessionId } from '../pages/guidedDemo'

interface User {
  id: string
  email: string
  display_name: string
}

interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, displayName: string) => Promise<void>
  demoLogin: () => Promise<string>
  logout: () => void
  init: () => Promise<void>
}

function demoRegistration() {
  const identity = createOpaqueDemoId()

  return {
    email: `demo-${identity}@example.invalid`,
    // A single UUID (36 chars) stays comfortably under bcrypt's 72-byte
    // password cap; concatenating two UUIDs here used to produce a
    // 73-character password that made every registration fail.
    password: identity,
    display_name: 'Demo Visitor',
  }
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  token: localStorage.getItem('token'),
  isAuthenticated: !!localStorage.getItem('token'),

  login: async (email, password) => {
    const { user, token } = await api.login({ email, password })
    localStorage.setItem('token', token)
    set({ user, token, isAuthenticated: true })
  },

  register: async (email, password, displayName) => {
    const { user, token } = await api.register({ email, password, display_name: displayName })
    localStorage.setItem('token', token)
    set({ user, token, isAuthenticated: true })
  },

  demoLogin: async () => {
    const sessionId = guidedDemoSessionId()
    if (get().isAuthenticated) {
      const { universe_id } = await api.demoClone(sessionId)
      return universe_id
    }

    const { user, token } = await api.register(demoRegistration())
    localStorage.setItem('token', token)
    set({ user, token, isAuthenticated: true })
    const { universe_id } = await api.demoClone(sessionId)
    return universe_id
  },

  logout: () => {
    localStorage.removeItem('token')
    set({ user: null, token: null, isAuthenticated: false })
  },

  init: async () => {
    const token = localStorage.getItem('token')
    if (!token) return
    try {
      const { user } = await api.me()
      set({ user, isAuthenticated: true })
    } catch {
      localStorage.removeItem('token')
      set({ user: null, token: null, isAuthenticated: false })
    }
  },
}))
