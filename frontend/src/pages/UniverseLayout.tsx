import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate, useLocation, Outlet, NavLink } from 'react-router-dom'
import { useUniverseStore } from '../stores/universeStore'
import { UniverseContext, type UniverseContextValue } from '../contexts/UniverseContext'
import { useAuthStore } from '../stores/authStore'
import { api } from '../lib/api'
import styles from './UniverseLayout.module.css'

const GENRES = [
  { value: 'fantasy', label: 'Fantasy' },
  { value: 'sci-fi', label: 'Sci-Fi' },
  { value: 'mystery', label: 'Mystery' },
  { value: 'romance', label: 'Romance' },
  { value: 'horror', label: 'Horror' },
  { value: 'non-fiction', label: 'Non-Fiction' },
  { value: 'thriller', label: 'Thriller' },
  { value: 'historical', label: 'Historical' },
  { value: 'adventure', label: 'Adventure' },
  { value: 'comedy', label: 'Comedy' },
  { value: 'drama', label: 'Drama' },
]

const FORMATS = [
  { value: 'novel', label: 'Novel' },
  { value: 'short-story', label: 'Short Story' },
  { value: 'screenplay', label: 'Screenplay' },
  { value: 'poetry', label: 'Poetry' },
  { value: 'essay', label: 'Essay' },
  { value: 'article', label: 'Article' },
  { value: 'graphic-novel', label: 'Graphic Novel' },
]

const TABS = [
  { to: 'panorama', label: 'Panorama', glyph: '◇' },
  { to: 'works', label: 'Works & Chapters', glyph: '❑' },
  { to: 'editor', label: 'Editor', glyph: '✎' },
  { to: 'entities', label: 'Entities', glyph: '○' },
  { to: 'graph', label: 'Graph', glyph: '✳' },
  { to: 'timeline', label: 'Timeline', glyph: '⌇' },
  { to: 'contradictions', label: 'Contradictions', glyph: '△', badge: 'contradictions' },
  { to: 'plot-holes', label: 'Plot Holes', glyph: '◠', badge: 'plotHoles' },
  { to: 'ingest', label: 'Ingestion', glyph: '⇩' },
  { to: 'memory', label: 'Memory', glyph: '⧉' },
]

// Pages that need the content area to have no scroll (full-bleed: Editor, Graph)
const FULL_BLEED_ROUTES = ['editor', 'graph']

export default function UniverseLayout() {
  const { universeId } = useParams<{ universeId: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuthStore()
  const [ctx, setCtx] = useState<UniverseContextValue>({ universe: null, works: [], refetchWorks: async () => {} })
  const [entityCount, setEntityCount] = useState(0)
  const [contradictionCount, setContradictionCount] = useState(0)
  const [plotHoleCount, setPlotHoleCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const fetchUniverses = useUniverseStore((s) => s.fetchUniverses)
  const universes = useUniverseStore((s) => s.universes)

  // Switcher dropdown
  const [showSwitcher, setShowSwitcher] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Create Modal
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [createName, setCreateName] = useState('')
  const [createDesc, setCreateDesc] = useState('')
  const [createGenre, setCreateGenre] = useState('fantasy')
  const [createFormat, setCreateFormat] = useState('novel')

  // Edit Modal
  const [showEditModal, setShowEditModal] = useState(false)
  const [editingUniverse, setEditingUniverse] = useState<any | null>(null)
  const [editName, setEditName] = useState('')
  const [editDesc, setEditDesc] = useState('')
  const [editGenre, setEditGenre] = useState('fantasy')
  const [editFormat, setEditFormat] = useState('novel')

  useEffect(() => {
    fetchUniverses()
  }, [fetchUniverses])

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowSwitcher(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const openEditModalFor = (u: any) => {
    setEditingUniverse(u)
    setEditName(u.name)
    setEditDesc(u.description || '')
    setEditGenre(u.genre || 'fantasy')
    setEditFormat(u.format || 'novel')
    setShowEditModal(true)
    setShowSwitcher(false)
  }

  const handleEditUniverse = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingUniverse || !editName.trim()) return
    try {
      const { universe } = await api.updateUniverse(editingUniverse.id, {
        name: editName.trim(),
        description: editDesc,
        genre: editGenre,
        format: editFormat
      })
      if (ctx.universe?.id === editingUniverse.id) {
        setCtx((prev) => ({ ...prev, universe }))
      }
      await fetchUniverses()
      setShowEditModal(false)
      setEditingUniverse(null)
    } catch (err) {
      alert((err as Error).message)
    }
  }

  const handleDeleteUniverseFor = async (u: any) => {
    if (!window.confirm(`Delete "${u.name}"? This cannot be undone.`)) return
    try {
      await api.deleteUniverse(u.id)
      await fetchUniverses()
      setShowSwitcher(false)
      if (ctx.universe?.id === u.id) {
        navigate('/dashboard')
      }
    } catch (err) {
      alert((err as Error).message)
    }
  }

  const handleCreateUniverse = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!createName.trim()) return
    try {
      const { universe } = await api.createUniverse({
        name: createName.trim(),
        description: createDesc,
        genre: createGenre,
        format: createFormat
      })
      await fetchUniverses()
      setShowCreateModal(false)
      setCreateName('')
      setCreateDesc('')
      setCreateGenre('fantasy')
      setCreateFormat('novel')
      navigate(`/universe/${universe.id}`)
    } catch (err) {
      alert((err as Error).message)
    }
  }

  const refetchWorks = useCallback(async () => {
    if (!universeId) return
    try {
      const { works } = await api.listWorks(universeId)
      setCtx((prev) => ({ ...prev, works }))
    } catch {
      // silent
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
    api.listEntities(universeId, { limit: '1' })
      .then((res) => setEntityCount(res.pagination?.total ?? 0))
      .catch(() => {})
    api.getContradictions(universeId)
      .then((res) => setContradictionCount(res.contradictions?.length ?? 0))
      .catch(() => {})
    api.getPlotHoles(universeId)
      .then((res) => setPlotHoleCount(res.plot_holes?.length ?? 0))
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

  const badgeCounts: Record<string, number> = {
    contradictions: contradictionCount,
    plotHoles: plotHoleCount,
  }

  const activeTabKey = TABS.find((tab) => location.pathname.includes(`/${tab.to}`))
  const universeInitial = (ctx.universe?.name || '?').charAt(0).toUpperCase()
  const userInitial = (user?.display_name || '?').charAt(0).toUpperCase()

  const isFullBleed = FULL_BLEED_ROUTES.some((r) => location.pathname.includes(`/${r}`))

  return (
    <UniverseContext.Provider value={ctx}>
      <div className={styles.wrap}>
        {sidebarOpen && (
          <aside className={styles.sidebar}>
            {/* Brand */}
            <div className={styles.brandRow}>
              <div className={styles.brandMark}>Q</div>
              <span className={styles.brandName}>Quill</span>
              <button
                className={styles.collapseBtn}
                onClick={() => setSidebarOpen(false)}
                aria-label="Hide sidebar"
                title="Hide sidebar"
              >
                «
              </button>
            </div>

            {/* Universe switcher */}
            <div className={styles.switcherSection} style={{ position: 'relative' }} ref={dropdownRef}>
              <div className={styles.switcherLabel}>Universe</div>
              <button className={styles.switcherCard} onClick={() => setShowSwitcher(!showSwitcher)} style={{ width: '100%' }}>
                <span className={styles.switcherAvatar}>{universeInitial}</span>
                <span className={styles.switcherInfo}>
                  <span className={styles.switcherName}>{ctx.universe?.name || 'Universe'}</span>
                  <span className={styles.switcherMeta}>
                    {ctx.universe?.genre} · {entityCount} {entityCount === 1 ? 'entity' : 'entities'}
                  </span>
                </span>
                <span className={`${styles.switcherCaret} glyph`}>▾</span>
              </button>

              {showSwitcher && (
                <div className={styles.dropdownPopover}>
                  <div className={styles.dropdownList}>
                    {universes.map((u) => (
                      <div key={u.id} className={styles.dropdownRow}>
                        <div
                          className={styles.dropdownItemContent}
                          onClick={() => {
                            navigate(`/universe/${u.id}`)
                            setShowSwitcher(false)
                          }}
                        >
                          <span className={styles.dropdownAvatar}>{u.name.charAt(0).toUpperCase()}</span>
                          <span className={styles.dropdownInfo}>
                            <span className={styles.dropdownName}>{u.name}</span>
                            <span className={styles.dropdownMeta}>{u.genre}</span>
                          </span>
                        </div>
                        <div className={styles.dropdownActions}>
                          <button
                            onClick={(e) => {
                              e.stopPropagation()
                              openEditModalFor(u)
                            }}
                            title="Edit Universe"
                            className={styles.dropdownRowActionBtn}
                          >
                            ✏️
                          </button>
                          <button
                            onClick={(e) => {
                              e.stopPropagation()
                              handleDeleteUniverseFor(u)
                            }}
                            title="Delete Universe"
                            className={`${styles.dropdownRowActionBtn} ${styles.delete}`}
                          >
                            🗑️
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                  <div className={styles.dropdownDivider} />
                  <button
                    className={styles.dropdownNewBtn}
                    onClick={() => {
                      setShowCreateModal(true)
                      setShowSwitcher(false)
                    }}
                  >
                    <span>+</span> Create New Universe
                  </button>
                </div>
              )}

              <button className={styles.newUniverseLink} onClick={() => setShowCreateModal(true)}>
                + New universe
              </button>
            </div>

            {/* Navigation */}
            <nav className={styles.navSection}>
              <div className={styles.navLabel}>Navigate</div>
              <div className={styles.tabBar}>
                {TABS.map((tab) => {
                  const count = tab.badge ? badgeCounts[tab.badge] ?? 0 : 0
                  return (
                    <NavLink
                      key={tab.to}
                      to={tab.to}
                      end={tab.to === 'editor'}
                      className={({ isActive }) =>
                        `${styles.tab} ${isActive ? styles.tabActive : ''}`
                      }
                    >
                      <span className={`${styles.tabGlyph} glyph`}>{tab.glyph}</span>
                      {tab.label}
                      {count > 0 && (
                        <span className={styles.tabBadge}>{count}</span>
                      )}
                    </NavLink>
                  )
                })}
              </div>
            </nav>

            {/* User footer */}
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
          {/* Topbar */}
          <header className={styles.topbar}>
            {!sidebarOpen && (
              <button
                className={styles.menuBtn}
                onClick={() => setSidebarOpen(true)}
                aria-label="Show sidebar"
                title="Show sidebar"
              >
                <span className="glyph">☰</span>
              </button>
            )}
            <div className={styles.crumbBlock}>
              <div className={styles.crumb}>Universe</div>
              <div className={styles.pageTitle}>
                {activeTabKey?.label || ctx.universe?.name}
              </div>
            </div>
            <div className={styles.searchStub}>
              <span className={`${styles.searchGlyph} glyph`}>⌕</span>
              <span>Recall from the universe…</span>
              <span className={styles.searchShortcut}>⌘K</span>
            </div>
          </header>

          <div className={`${styles.content} ${isFullBleed ? styles.fullBleed : ''} q-scroll`}>
            <Outlet />
          </div>
        </div>
      </div>
      {/* Create Universe Modal */}
      {showCreateModal && (
        <div className={styles.modalOverlay}>
          <div className={styles.modalContent}>
            <div className={styles.modalHeader}>
              <h3 className={styles.modalTitle}>New Universe</h3>
              <button className={styles.modalCloseBtn} onClick={() => setShowCreateModal(false)}>×</button>
            </div>
            <form className={styles.modalForm} onSubmit={handleCreateUniverse}>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Name</label>
                <input
                  className={styles.modalInput}
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder="e.g. Cosmere"
                  autoFocus
                  required
                />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Description</label>
                <textarea
                  className={styles.modalInput}
                  value={createDesc}
                  onChange={(e) => setCreateDesc(e.target.value)}
                  placeholder="A brief description of this universe..."
                  rows={3}
                />
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                <div className={styles.formGroup}>
                  <label className={styles.formLabel}>Genre</label>
                  <select
                    className={styles.modalSelect}
                    value={createGenre}
                    onChange={(e) => setCreateGenre(e.target.value)}
                  >
                    {GENRES.map((g) => (
                      <option key={g.value} value={g.value}>{g.label}</option>
                    ))}
                  </select>
                </div>
                <div className={styles.formGroup}>
                  <label className={styles.formLabel}>Format</label>
                  <select
                    className={styles.modalSelect}
                    value={createFormat}
                    onChange={(e) => setCreateFormat(e.target.value)}
                  >
                    {FORMATS.map((f) => (
                      <option key={f.value} value={f.value}>{f.label}</option>
                    ))}
                  </select>
                </div>
              </div>
              <div className={styles.modalActions}>
                <button type="button" className={styles.btnCancel} onClick={() => setShowCreateModal(false)}>Cancel</button>
                <button type="submit" className={styles.btnSave} disabled={!createName.trim()}>Create</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Edit Universe Modal */}
      {showEditModal && (
        <div className={styles.modalOverlay}>
          <div className={styles.modalContent}>
            <div className={styles.modalHeader}>
              <h3 className={styles.modalTitle}>Edit Universe</h3>
              <button className={styles.modalCloseBtn} onClick={() => setShowEditModal(false)}>×</button>
            </div>
            <form className={styles.modalForm} onSubmit={handleEditUniverse}>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Name</label>
                <input
                  className={styles.modalInput}
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  placeholder="e.g. Cosmere"
                  autoFocus
                  required
                />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Description</label>
                <textarea
                  className={styles.modalInput}
                  value={editDesc}
                  onChange={(e) => setEditDesc(e.target.value)}
                  placeholder="A brief description..."
                  rows={3}
                />
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                <div className={styles.formGroup}>
                  <label className={styles.formLabel}>Genre</label>
                  <select
                    className={styles.modalSelect}
                    value={editGenre}
                    onChange={(e) => setEditGenre(e.target.value)}
                  >
                    {GENRES.map((g) => (
                      <option key={g.value} value={g.value}>{g.label}</option>
                    ))}
                  </select>
                </div>
                <div className={styles.formGroup}>
                  <label className={styles.formLabel}>Format</label>
                  <select
                    className={styles.modalSelect}
                    value={editFormat}
                    onChange={(e) => setEditFormat(e.target.value)}
                  >
                    {FORMATS.map((f) => (
                      <option key={f.value} value={f.value}>{f.label}</option>
                    ))}
                  </select>
                </div>
              </div>
              <div className={styles.modalActions}>
                <button type="button" className={styles.btnCancel} onClick={() => setShowEditModal(false)}>Cancel</button>
                <button type="submit" className={styles.btnSave} disabled={!editName.trim()}>Save Changes</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </UniverseContext.Provider>
  )
}
