import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate, useLocation, Outlet, NavLink } from 'react-router-dom'
import { UniverseContext, type UniverseContextValue } from '../contexts/UniverseContext'
import { useAuthStore } from '../stores/authStore'
import { api } from '../lib/api'
import styles from './UniverseLayout.module.css'

// Glyph mapping per ADR-2 (inline Unicode icons, no icon library).
const TABS = [
  { to: 'panorama', label: 'Dashboard', glyph: '◇' },
  { to: 'works', label: 'Works', glyph: '❑' },
  { to: 'editor', label: 'Editor', glyph: '✎' },
  { to: 'entities', label: 'Entities', glyph: '○' },
  { to: 'graph', label: 'Graph', glyph: '✳' },
  { to: 'timeline', label: 'Timeline', glyph: '⌇' },
  { to: 'contradictions', label: 'Contradictions', glyph: '△' },
  { to: 'plot-holes', label: 'Plot Holes', glyph: '◠' },
  { to: 'ingest', label: 'Ingestion', glyph: '⇩' },
]

export default function UniverseLayout() {
  const { universeId } = useParams<{ universeId: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuthStore()
  const [ctx, setCtx] = useState<UniverseContextValue>({ universe: null, works: [], refetchWorks: async () => {} })
  const [entityCount, setEntityCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sidebarOpen, setSidebarOpen] = useState(true)

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
    // Entity count for the switcher card — supplementary, fails silently.
    api.listEntities(universeId, { limit: '1' })
      .then((res) => setEntityCount(res.pagination?.total ?? 0))
      .catch(() => {})
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

  const activeTab = TABS.find((tab) => location.pathname.includes(`/${tab.to}`))
  const universeInitial = (ctx.universe?.name || '?').charAt(0).toUpperCase()
  const userInitial = (user?.display_name || '?').charAt(0).toUpperCase()

  return (
    <UniverseContext.Provider value={ctx}>
      <div className={styles.wrap}>
        {sidebarOpen && (
          <aside className={styles.sidebar}>
            <div className={styles.brandRow}>
              <div className={styles.brandMark}>Q</div>
              <span className={styles.brandName}>Quill</span>
              <button
                className={styles.collapseBtn}
                onClick={() => setSidebarOpen(false)}
                aria-label="Hide panel"
                title="Hide panel"
              >
                «
              </button>
            </div>

            <div className={styles.switcherLabel}>Universe</div>
            <button className={styles.switcherCard} onClick={() => navigate('/dashboard')}>
              <span className={styles.switcherAvatar}>{universeInitial}</span>
              <span className={styles.switcherInfo}>
                <span className={styles.switcherName}>{ctx.universe?.name || 'Universe'}</span>
                <span className={styles.switcherMeta}>
                  {ctx.universe?.genre} · {entityCount} {entityCount === 1 ? 'entity' : 'entities'}
                </span>
              </span>
              <span className={`${styles.switcherCaret} glyph`}>▾</span>
            </button>
            <button className={styles.newUniverseLink} onClick={() => navigate('/dashboard')}>
              + New Universe
            </button>

            <div className={styles.navLabel}>Navigate</div>
            <nav className={styles.tabBar}>
              {TABS.map((tab) => (
                <NavLink
                  key={tab.to}
                  to={tab.to}
                  end
                  className={({ isActive }) => `${styles.tab} ${isActive ? styles.tabActive : ''}`}
                >
                  <span className="glyph">{tab.glyph}</span> {tab.label}
                </NavLink>
              ))}
            </nav>

            <div className={styles.userFooter}>
              <span className={styles.userAvatar}>{userInitial}</span>
              <span className={styles.userInfo}>
                <span className={styles.userName}>{user?.display_name}</span>
                <span className={styles.userEmail}>{user?.email}</span>
              </span>
              <button
                className={styles.logoutBtn}
                onClick={logout}
                aria-label="Sign out"
                title="Sign out"
              >
                <span className="glyph">⏻</span>
              </button>
            </div>
          </aside>
        )}

        <div className={styles.contentCol}>
          <header className={styles.topbar}>
            {!sidebarOpen && (
              <button
                className={styles.menuBtn}
                onClick={() => setSidebarOpen(true)}
                aria-label="Show panel"
                title="Show panel"
              >
                <span className="glyph">☰</span>
              </button>
            )}
            <div className={styles.crumbBlock}>
              <div className={styles.crumb}>Universe</div>
              <div className={styles.pageTitle}>{activeTab?.label || ctx.universe?.name}</div>
            </div>
            <div className={styles.searchStub}>
              <span className="glyph">⌕</span>
              <span>Recall from the universe…</span>
            </div>
          </header>

          <div className={styles.content}>
            <Outlet />
          </div>
        </div>
      </div>
    </UniverseContext.Provider>
  )
}
