import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate, Outlet, NavLink } from 'react-router-dom'
import { UniverseContext, type UniverseContextValue } from '../contexts/UniverseContext'
import { api } from '../lib/api'
import styles from './UniverseLayout.module.css'

export default function UniverseLayout() {
  const { universeId } = useParams<{ universeId: string }>()
  const navigate = useNavigate()
  const [ctx, setCtx] = useState<UniverseContextValue>({ universe: null, works: [], refetchWorks: async () => {} })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refetchWorks = useCallback(async () => {
    if (!universeId) return
    try {
      const { works } = await api.listWorks(universeId)
      setCtx((prev) => ({ ...prev, works }))
    } catch {
      // silent — works list is supplementary
    }
  }, [universeId])

  useEffect(() => {
    if (!universeId) return
    setLoading(true)
    setError(null)
    Promise.all([api.getUniverse(universeId), api.listWorks(universeId)])
      .then(([{ universe }, { works }]) => {
        setCtx({ universe, works, refetchWorks })
        setLoading(false)
      })
      .catch((err) => {
        setError((err as Error).message)
        setLoading(false)
      })
  }, [universeId, refetchWorks])

  if (loading) {
    return (
      <div className={styles.stateWrap}>
        <p className={styles.stateText}>Loading universe…</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className={styles.stateWrap}>
        <p className={styles.stateText}>Failed to load universe: {error}</p>
        <button className={styles.stateBtn} onClick={() => navigate('/dashboard')}>
          Back to Dashboard
        </button>
      </div>
    )
  }

  const tabs = [
    { to: 'works', label: 'Works' },
    { to: 'graph', label: 'Graph' },
    { to: 'timeline', label: 'Timeline' },
    { to: 'contradictions', label: 'Contradictions' },
    { to: 'plot-holes', label: 'Plot-holes' },
  ]

  return (
    <UniverseContext.Provider value={ctx}>
      <div className={styles.wrap}>
        <aside className={styles.sidebar}>
          <button className={styles.backBtn} onClick={() => navigate('/dashboard')}>
            ← Back to Dashboard
          </button>

          <div className={styles.header}>
            <h1 className={styles.heading}>{ctx.universe?.name || 'Universe'}</h1>
            <p className={styles.meta}>
              {ctx.universe?.genre} · {ctx.universe?.format}
            </p>
          </div>

          <nav className={styles.tabBar}>
            {tabs.map((tab) => (
              <NavLink
                key={tab.to}
                to={tab.to}
                end
                className={({ isActive }) =>
                  `${styles.tab} ${isActive ? styles.tabActive : ''}`
                }
              >
                {tab.label}
              </NavLink>
            ))}
          </nav>
        </aside>

        <div className={styles.content}>
          <Outlet />
        </div>
      </div>
    </UniverseContext.Provider>
  )
}
