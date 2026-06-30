import { create } from 'zustand'
import { api } from '../lib/api'

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
  logout: () => void
  init: () => Promise<void>
}

export const useAuthStore = create<AuthState>((set) => ({
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
