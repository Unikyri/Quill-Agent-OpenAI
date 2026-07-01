import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import styles from './LoginPage.module.css'

export default function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [isRegister, setIsRegister] = useState(false)
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { login, register, demoLogin } = useAuthStore()
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      if (isRegister) {
        await register(email, password, displayName)
      } else {
        await login(email, password)
      }
      navigate('/dashboard')
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleDemo = async () => {
    setError('')
    setLoading(true)
    try {
      const universeId = await demoLogin()
      navigate(`/universe/${universeId}`)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <h1 className={styles.heading}>Quill</h1>
        <p className={styles.subheading}>Write worlds into existence</p>

        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${!isRegister ? styles.tabActive : ''}`}
            onClick={() => setIsRegister(false)}
          >
            Sign In
          </button>
          <button
            className={`${styles.tab} ${isRegister ? styles.tabActive : ''}`}
            onClick={() => setIsRegister(true)}
          >
            Register
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          {isRegister && (
            <div className={styles.field}>
              <input
                placeholder="Display Name"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                required
              />
            </div>
          )}
          <div className={styles.field}>
            <input
              type="email"
              placeholder="Email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </div>
          <div className={styles.field}>
            <input
              type="password"
              placeholder="Password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          {error && <p className={styles.errorMsg}>{error}</p>}
          <button type="submit" className={styles.submitBtn} disabled={loading}>
            {loading ? 'Loading…' : isRegister ? 'Create Account' : 'Sign In'}
          </button>
        </form>

        <div className={styles.divider}>
          <span className={styles.dividerLine} />
          <span className={styles.dividerText}>or</span>
          <span className={styles.dividerLine} />
        </div>

        <button className={styles.demoBtn} onClick={handleDemo} disabled={loading}>
          Try the Demo
        </button>

        <p className={styles.quote}>
          "A writer is a world trapped in a person."<br />— Victor Hugo
        </p>
      </div>
    </div>
  )
}
