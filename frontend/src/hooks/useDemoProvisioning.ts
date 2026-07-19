import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { writePath } from '../lib/canonicalRoutes'

interface UseDemoProvisioning {
  startDemo: () => Promise<void>
  pending: boolean
  error: string
}

/**
 * Register (or reuse the current session) → clone the seeded demo universe →
 * land the visitor on its Write screen. Shared by LoginPage and LandingPage
 * so both demo entry points provision identically.
 */
export function useDemoProvisioning(): UseDemoProvisioning {
  const demoLogin = useAuthStore((s) => s.demoLogin)
  const navigate = useNavigate()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  const startDemo = async () => {
    setError('')
    setPending(true)
    try {
      const universeId = await demoLogin()
      navigate(writePath(universeId))
    } catch (err: any) {
      setError(err.message)
    } finally {
      setPending(false)
    }
  }

  return { startDemo, pending, error }
}
