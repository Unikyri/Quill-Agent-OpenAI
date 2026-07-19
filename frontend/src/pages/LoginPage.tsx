import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useDemoProvisioning } from '../hooks/useDemoProvisioning'
import styles from './LoginPage.module.css'

export default function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [isRegister, setIsRegister] = useState(false)
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { login, register } = useAuthStore()
  const navigate = useNavigate()
  const { startDemo, pending: demoPending, error: demoError } = useDemoProvisioning()

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

  return (
    <div className={styles.container}>
      {/* Left hero pane — animated relationship-graph SVG */}
      <div className={styles.hero}>
        <svg
          viewBox="0 0 560 760"
          preserveAspectRatio="xMidYMid slice"
          className={styles.heroSvg}
          aria-hidden
        >
          <g fill="none" stroke="rgba(233,225,207,.09)" strokeWidth="1.2">
            <path d="M-40 150 Q160 110 300 175 T620 150" />
            <path d="M-40 235 Q150 195 320 260 T620 235" />
            <path d="M-40 330 Q180 295 330 360 T640 335" />
            <path d="M-40 440 Q150 405 320 470 T620 450" />
            <path d="M-40 560 Q190 520 340 585 T640 560" />
            <path d="M-40 670 Q160 640 320 690 T620 675" />
          </g>
          <g stroke="rgba(233,225,207,.26)" strokeWidth="1.4">
            <line x1="175" y1="180" x2="370" y2="250" />
            <line x1="370" y1="250" x2="470" y2="175" />
            <line x1="370" y1="250" x2="420" y2="400" />
            <line x1="370" y1="250" x2="250" y2="370" />
            <line x1="175" y1="180" x2="470" y2="175" />
          </g>
          <line
            x1="250"
            y1="370"
            x2="470"
            y2="175"
            stroke="#d98b63"
            strokeWidth="1.6"
            strokeDasharray="6 6"
            className={styles.dash}
          />
          <g className={styles.float1}>
            <circle cx="370" cy="250" r="26" fill="#d9a441" />
            <circle
              cx="370"
              cy="250"
              r="26"
              fill="none"
              stroke="rgba(217,164,65,.35)"
              strokeWidth="8"
              className={styles.glow}
            />
          </g>
          <g className={styles.float2}>
            <circle cx="175" cy="180" r="19" fill="#e9e1cf" />
          </g>
          <g className={styles.float3}>
            <circle cx="470" cy="175" r="19" fill="#8fae86" />
          </g>
          <g className={styles.float4}>
            <circle cx="420" cy="400" r="17" fill="#c98a5c" />
          </g>
          <g className={styles.float5}>
            <circle cx="250" cy="370" r="17" fill="#e9e1cf" />
          </g>
        </svg>
        <div className={styles.heroGradient} />

        <div className={styles.heroBrand} onClick={() => navigate('/')} style={{ cursor: 'pointer' }}>
          <span className={styles.brandMark}>Q</span>
          <h1 className={styles.brandName}>Quill</h1>
        </div>

        <div className={styles.heroCopy}>
          <p className={styles.heroKicker}>Your second brain</p>
          <p className={styles.heroTitle}>
            Every character, every plot, every rule of your world — remembered.
          </p>
          <p className={styles.heroSubtitle}>
            Quill reads your work, extracts the lore, and watches for contradictions while you write.
          </p>
        </div>

        <div className={styles.heroLegend}>
          <span className={styles.legendItem}>
            <span className={`${styles.legendDot} ${styles.legendCharacter}`} />
            Character
          </span>
          <span className={styles.legendItem}>
            <span className={`${styles.legendDot} ${styles.legendPlace}`} />
            Place
          </span>
          <span className={styles.legendItem}>
            <span className={`${styles.legendDot} ${styles.legendItem2}`} />
            Item
          </span>
          <span className={styles.legendItem}>
            <span className={styles.legendDash} />
            Contradiction
          </span>
        </div>
      </div>

      {/* Right pane — auth form */}
      <div className={styles.formPane}>
        <div className={styles.formCard}>
          <h2 className={styles.heading}>
            {isRegister ? 'Create your author account' : 'Welcome back'}
          </h2>
          <p className={styles.subheading}>
            {isRegister
              ? 'Start building your narrative second brain.'
              : 'Pick up your universe right where you left it.'}
          </p>

          <div className={styles.tabs}>
            <button
              type="button"
              className={`${styles.tab} ${!isRegister ? styles.tabActive : ''}`}
              onClick={() => setIsRegister(false)}
            >
              Sign In
            </button>
            <button
              type="button"
              className={`${styles.tab} ${isRegister ? styles.tabActive : ''}`}
              onClick={() => setIsRegister(true)}
            >
              Register
            </button>
          </div>

          <form className={styles.form} onSubmit={handleSubmit}>
            {isRegister && (
              <div className={styles.field}>
                <label className={styles.label}>Author Name</label>
                <input
                  placeholder="Author Name"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  required
                />
              </div>
            )}
            <div className={styles.field}>
              <label className={styles.label}>Email</label>
              <input
                type="email"
                placeholder="Email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className={styles.field}>
              <label className={styles.label}>Password</label>
              <input
                type="password"
                placeholder="Password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
              {isRegister && <p className={styles.hint}>Minimum 8 characters.</p>}
            </div>
            {(error || demoError) && <p className={styles.errorMsg}>{error || demoError}</p>}
            <button type="submit" className={styles.submitBtn} disabled={loading}>
              {loading ? 'Loading…' : isRegister ? 'Create My Account' : 'Enter My Universe'}
            </button>
          </form>

          <div className={styles.divider}>
            <span className={styles.dividerLine} />
            <span className={styles.dividerText}>or</span>
            <span className={styles.dividerLine} />
          </div>

          <button className={styles.demoBtn} onClick={() => void startDemo()} disabled={demoPending}>
            Try the Demo
          </button>


        </div>
      </div>
    </div>
  )
}
