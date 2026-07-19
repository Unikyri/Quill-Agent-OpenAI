import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AlertCircle, Brain, ChevronDown, Compass, Home, Loader2, LogOut, PenLine, User, Wifi, WifiOff } from 'lucide-react'
import { Link, NavLink, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'
import { useFeedback } from '../components/feedback'
import { GenreTagPicker } from '../components/genres'
import { UniverseContext, type UniverseContextValue } from '../contexts/UniverseContext'
import { explorePath, memoryPath, profileMemoryPath, reviewPath, writePath } from '../lib/canonicalRoutes'
import { api } from '../lib/api'
import { useWS } from '../hooks/useWS'
import { useAuthStore } from '../stores/authStore'
import { useUniverseStore } from '../stores/universeStore'
import { useWSStore, type WSStatus } from '../stores/wsStore'
import {
  guidedDemoSessionId,
  isGuidedDemoUniverse,
  readGuidedDemoProgress,
  recordGuidedDemoProgress,
  rememberGuidedDemoUniverse,
  type GuidedDemoProgress,
} from './guidedDemo'
import styles from './UniverseLayout.module.css'

type Destination = 'home' | 'write' | 'explore' | 'memory' | 'review'

interface NavigationItem {
  id: Destination
  label: string
  shortcut: string
  to: (universeId: string) => string
  Icon: typeof Home
}

const navigation: NavigationItem[] = [
  { id: 'home', label: 'Home', shortcut: 'Alt+1', to: () => '/dashboard', Icon: Home },
  { id: 'write', label: 'Write', shortcut: 'Alt+2', to: writePath, Icon: PenLine },
  { id: 'explore', label: 'Map', shortcut: 'Alt+3', to: (id) => explorePath(id, 'map'), Icon: Compass },
  { id: 'memory', label: 'Memory', shortcut: 'Alt+4', to: memoryPath, Icon: Brain },
  { id: 'review', label: 'Review', shortcut: 'Alt+5', to: (id) => reviewPath(id, 'issues'), Icon: AlertCircle },
]

function activeDestination(pathname: string, universeId?: string): Destination {
  if (pathname === '/dashboard') return 'home'
  if (!universeId) return 'home'

  const prefix = `/universe/${universeId}/`
  if (!pathname.startsWith(prefix)) return 'home'
  if (pathname.startsWith(`${prefix}write`)) return 'write'
  if (pathname.startsWith(`${prefix}explore`)) return 'explore'
  if (pathname.startsWith(`${prefix}memory`)) return 'memory'
  if (pathname.startsWith(`${prefix}review`)) return 'review'
  return 'write'
}

function isTypingTarget(target: EventTarget | null): boolean {
  const element = target instanceof HTMLElement ? target : null
  return Boolean(element?.closest('input, textarea, select, [contenteditable="true"]'))
}

function statusCopy(status: WSStatus, activeAnalyses: number, lastError: string | null) {
  if (lastError) {
    return { label: 'Live analysis needs attention', detail: lastError, Icon: AlertCircle, tone: 'error' }
  }
  if (activeAnalyses > 0) {
    return {
      label: `Analyzing ${activeAnalyses} ${activeAnalyses === 1 ? 'paragraph' : 'paragraphs'}`,
      detail: 'Live analysis is in progress.',
      Icon: Loader2,
      tone: 'working',
    }
  }
  if (status === 'open') return { label: 'Live analysis connected', detail: 'WebSocket connection is open.', Icon: Wifi, tone: 'ready' }
  if (status === 'connecting' || status === 'reconnecting') {
    return { label: 'Connecting live analysis', detail: 'WebSocket connection is establishing.', Icon: Loader2, tone: 'working' }
  }
  return { label: 'Live analysis offline', detail: 'WebSocket is not connected.', Icon: WifiOff, tone: 'offline' }
}

function requestErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback
}

interface GuidedDemoJourneyProps {
  universeId: string
  destination: Destination
  pathname: string
  observedAnalysis: boolean
}

function GuidedDemoJourney({ universeId, destination, pathname, observedAnalysis }: GuidedDemoJourneyProps) {
  const isDemo = isGuidedDemoUniverse(universeId)
  const navigate = useNavigate()
  const [progress, setProgress] = useState<GuidedDemoProgress>(() => readGuidedDemoProgress(universeId))
  const [resetting, setResetting] = useState(false)
  const [resetError, setResetError] = useState<string | null>(null)

  useEffect(() => {
    setProgress(readGuidedDemoProgress(universeId))
  }, [universeId])

  const handleReset = async () => {
    if (resetting) return
    setResetting(true)
    setResetError(null)
    try {
      const { universe_id: resetUniverseId } = await api.demoReset(guidedDemoSessionId())
      rememberGuidedDemoUniverse(resetUniverseId)
      setProgress(readGuidedDemoProgress(resetUniverseId))
      if (resetUniverseId !== universeId) navigate(writePath(resetUniverseId))
    } catch (error) {
      setResetError(error instanceof Error && error.message ? error.message : 'We could not reset the guided demo. Try again.')
    } finally {
      setResetting(false)
    }
  }

  useEffect(() => {
    if (!isDemo) return

    const updates: Partial<GuidedDemoProgress> = {}
    if (destination === 'write') updates.openedWriting = true
    if (pathname.startsWith(`/universe/${universeId}/explore/map`)) updates.openedMap = true
    if (observedAnalysis) updates.observedAnalysis = true

    if (Object.entries(updates).every(([key, value]) => !value || progress[key as keyof GuidedDemoProgress])) return
    setProgress(recordGuidedDemoProgress(universeId, updates))
  }, [destination, isDemo, observedAnalysis, pathname, progress, universeId])

  if (!isDemo) return null

  const steps = [
    {
      title: 'Clone or reset the real demo universe',
      detail: resetting
        ? 'Resetting the guided demo…'
        : 'Ready from a successful clone or reset for this signed-in account.',
      complete: true,
      to: undefined,
      action: 'Reset demo',
    },
    {
      title: 'Open writing',
      detail: progress.openedWriting ? 'Write has been opened for this demo.' : 'Open Write and choose a real chapter.',
      complete: progress.openedWriting,
      to: writePath(universeId),
      action: 'Open Write',
    },
    {
      title: 'Trigger and observe live analysis',
      detail: progress.observedAnalysis
        ? 'A completed analysis result has been observed for this demo.'
        : destination === 'write'
          ? 'Submit a paragraph and wait for Quill to return a result.'
          : 'Submit a paragraph from Write, then wait for a real result.',
      complete: progress.observedAnalysis,
      to: writePath(universeId),
      action: 'Go to Write',
    },
    {
      title: 'Explore the relationship map',
      detail: progress.openedMap ? 'The relationship map has been opened.' : 'Open the map when analysis has something to explore.',
      complete: progress.openedMap,
      to: explorePath(universeId, 'map'),
      action: 'Open map',
    },
    {
      title: 'Ask Memory a lore question',
      detail: destination === 'memory'
        ? 'Memory is open. Run a recall and inspect the evidence there.'
        : 'Ask from Memory; this guide stays honest rather than inferring a recall result.',
      complete: false,
      to: memoryPath(universeId),
      action: 'Open Memory',
    },
    {
      title: 'Review a real issue',
      detail: destination === 'review'
        ? 'Review is open. Inspect the available findings before making a decision.'
        : 'Open Review and inspect an issue if Quill has reported one.',
      complete: false,
      to: reviewPath(universeId, 'issues'),
      action: 'Open Review',
    },
  ]
  const observedCount = steps.filter((step) => step.complete).length

  return (
    <aside className={styles.demoJourney} aria-labelledby="demo-journey-title">
      <div className={styles.demoJourneyHeading}>
        <div>
          <p className={styles.demoJourneyEyebrow}>Guided demo</p>
          <h2 id="demo-journey-title">Six steps, only real progress</h2>
        </div>
        <p className={styles.demoJourneyProgress} role="status" aria-live="polite">
          {observedCount} verified {observedCount === 1 ? 'step' : 'steps'}
        </p>
      </div>
      <ol className={styles.demoJourneySteps}>
        {steps.map((step, index) => (
          <li className={styles.demoJourneyStep} data-complete={step.complete} key={step.title}>
            <span className={styles.demoJourneyNumber} aria-hidden="true">{index + 1}</span>
            <div className={styles.demoJourneyCopy}>
              <div className={styles.demoJourneyStepHeading}>
                <h3>{step.title}</h3>
                <span className={styles.demoJourneyState}>{step.complete ? 'Observed' : 'Pending'}</span>
              </div>
              <p>{step.detail}</p>
              {index === 0 ? (
                <>
                  <button
                    className={styles.demoJourneyResetBtn}
                    disabled={resetting}
                    onClick={() => void handleReset()}
                    type="button"
                  >
                    {resetting ? 'Resetting…' : step.action}
                  </button>
                  {resetError && <p className={styles.demoJourneyResetError} role="alert">{resetError}</p>}
                </>
              ) : (
                !step.complete && step.to && <Link className={styles.demoJourneyLink} to={step.to}>{step.action}</Link>
              )}
            </div>
          </li>
        ))}
      </ol>
    </aside>
  )
}

export default function UniverseLayout() {
  const { universeId } = useParams<{ universeId: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuthStore()
  const { publish, update } = useFeedback()
  const { status: wsStatus } = useWS()
  const lastError = useWSStore((state) => state.lastError)
  const setUniverseScope = useWSStore((state) => state.setUniverseScope)
  const activeAnalyses = useWSStore((state) => Object.values(state.submissions).filter((submission) =>
    submission.universeId === universeId && (submission.phase === 'submitted' || submission.phase === 'analyzing'),
  ).length)
  const observedDemoAnalysis = useWSStore((state) => Object.values(state.submissions).some((submission) =>
    submission.universeId === universeId && submission.phase === 'done',
  ))
  const fetchUniverses = useUniverseStore((state) => state.fetchUniverses)
  const universes = useUniverseStore((state) => state.universes)
  const [ctx, setCtx] = useState<UniverseContextValue>({ universe: null, works: [], refetchWorks: async () => {} })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [switcherOpen, setSwitcherOpen] = useState(false)
  const [accountOpen, setAccountOpen] = useState(false)
  const [editingGenres, setEditingGenres] = useState(false)
  const [editableGenres, setEditableGenres] = useState<string[]>([])
  const [genreSaveError, setGenreSaveError] = useState<string | null>(null)
  const [savingGenres, setSavingGenres] = useState(false)
  const switcherRef = useRef<HTMLDivElement>(null)
  const accountRef = useRef<HTMLDivElement>(null)
  const lastWsStatus = useRef<WSStatus | null>(null)
  const lastWsError = useRef<string | null>(null)
  const universeLoadRequestId = useRef(0)
  const currentUniverseId = useRef(universeId)
  currentUniverseId.current = universeId

  const refetchWorks = useCallback(async () => {
    if (!universeId) return
    const { works } = await api.listWorks(universeId)
    setCtx((current) => ({ ...current, works }))
  }, [universeId])

  const loadUniverse = useCallback(async (): Promise<boolean> => {
    if (!universeId || currentUniverseId.current !== universeId) return false

    const requestId = ++universeLoadRequestId.current
    const requestUniverseId = universeId
    const isCurrentRequest = () => (
      universeLoadRequestId.current === requestId && currentUniverseId.current === requestUniverseId
    )

    setLoading(true)
    setError(null)
    setCtx({ universe: null, works: [], refetchWorks })

    try {
      const [{ universe }, { works }] = await Promise.all([api.getUniverse(universeId), api.listWorks(universeId)])
      if (!isCurrentRequest()) return false
      setCtx({ universe, works, refetchWorks })
      return true
    } catch (loadError) {
      if (!isCurrentRequest()) return false
      setError(loadError instanceof Error ? loadError.message : 'Could not load this universe.')
      return false
    } finally {
      if (isCurrentRequest()) setLoading(false)
    }
  }, [refetchWorks, universeId])

  const retryUniverseLoad = useCallback(() => loadUniverse(), [loadUniverse])

  const openGenreEditor = useCallback(() => {
    const genres = ctx.universe?.genre_tags ?? (ctx.universe?.genre ? [ctx.universe.genre] : [])
    setEditableGenres(genres.filter(Boolean))
    setGenreSaveError(null)
    setSwitcherOpen(false)
    setEditingGenres(true)
  }, [ctx.universe])

  const closeGenreEditor = useCallback(() => {
    if (savingGenres) return
    setEditingGenres(false)
    setGenreSaveError(null)
  }, [savingGenres])

  const saveGenres = useCallback(async () => {
    if (!universeId || savingGenres) return

    const feedbackId = publish({ scope: 'home', status: 'running', message: 'Saving universe genres…' })
    setGenreSaveError(null)
    setSavingGenres(true)

    try {
      const { universe } = await api.updateUniverse(universeId, { genre_tags: editableGenres })
      setCtx((current) => ({ ...current, universe: universe ?? current.universe }))
      setEditingGenres(false)
      const message = 'Universe genres saved.'
      update(feedbackId, { status: 'completed', message })
    } catch (saveError) {
      const message = requestErrorMessage(saveError, 'We could not save universe genres. Please try again.')
      setGenreSaveError(message)
      update(feedbackId, { status: 'failed', message })
    } finally {
      setSavingGenres(false)
    }
  }, [editableGenres, publish, savingGenres, universeId, update])

  useEffect(() => {
    void fetchUniverses()
  }, [fetchUniverses])

  useEffect(() => {
    setUniverseScope(universeId ?? null)
  }, [setUniverseScope, universeId])

  useEffect(() => {
    void loadUniverse()
    return () => {
      universeLoadRequestId.current += 1
    }
  }, [loadUniverse])

  useEffect(() => {
    if (!error) return
    publish({
      scope: 'request',
      status: 'failed',
      message: `Could not load this universe: ${error}`,
      retry: retryUniverseLoad,
    })
  }, [error, publish, retryUniverseLoad])

  useEffect(() => {
    const previous = lastWsStatus.current
    lastWsStatus.current = wsStatus
    if (previous === null || previous === wsStatus) return
    if (wsStatus === 'open') {
      publish({ scope: 'connection', status: 'completed', message: 'Live analysis connected.' })
    } else if (previous === 'open' && (wsStatus === 'closed' || wsStatus === 'reconnecting')) {
      publish({ scope: 'connection', status: 'offline', message: 'Live analysis connection was interrupted.' })
    }
  }, [publish, wsStatus])

  useEffect(() => {
    if (!lastError || lastWsError.current === lastError) return
    lastWsError.current = lastError
    publish({ scope: 'connection', status: 'failed', message: `Live analysis error: ${lastError}` })
  }, [lastError, publish])

  useEffect(() => {
    const closeMenus = (event: MouseEvent) => {
      const target = event.target as Node
      if (switcherRef.current && !switcherRef.current.contains(target)) setSwitcherOpen(false)
      if (accountRef.current && !accountRef.current.contains(target)) setAccountOpen(false)
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setSwitcherOpen(false)
      setAccountOpen(false)
    }
    document.addEventListener('mousedown', closeMenus)
    document.addEventListener('keydown', closeOnEscape)
    return () => {
      document.removeEventListener('mousedown', closeMenus)
      document.removeEventListener('keydown', closeOnEscape)
    }
  }, [])

  useEffect(() => {
    const onShortcut = (event: KeyboardEvent) => {
      if (!event.altKey || isTypingTarget(event.target)) return
      const position = Number(event.key) - 1
      const item = navigation[position]
      if (!item || !universeId) return
      event.preventDefault()
      navigate(item.to(universeId))
    }
    window.addEventListener('keydown', onShortcut)
    return () => window.removeEventListener('keydown', onShortcut)
  }, [navigate, universeId])

  const destination = activeDestination(location.pathname, universeId)
  const headerStatus = useMemo(() => statusCopy(wsStatus, activeAnalyses, lastError), [activeAnalyses, lastError, wsStatus])
  const isFullBleed = /^\/universe\/[^/]+\/write\/[^/]+/.test(location.pathname) || location.pathname.endsWith('/explore/map')
  const universeInitial = (ctx.universe?.name || '?').charAt(0).toUpperCase()
  const userInitial = (user?.display_name || user?.email || '?').charAt(0).toUpperCase()

  if (loading) {
    return <div className={styles.stateWrap}><p className={styles.stateText}>Loading universe…</p></div>
  }

  if (error) {
    return (
      <div className={styles.stateWrap}>
        <p className={styles.stateText}>Failed to load universe: {error}</p>
        <div className={styles.stateActions}>
          <button className={styles.stateBtn} type="button" onClick={retryUniverseLoad}>Try again</button>
          <button className={styles.stateLink} type="button" onClick={() => navigate('/dashboard')}>Back to Home</button>
        </div>
      </div>
    )
  }

  return (
    <UniverseContext.Provider value={ctx}>
      <a className={styles.skipLink} href="#universe-main">Skip to content</a>
      <div className={styles.shell}>
        <header className={styles.appBar}>
          <NavLink className={styles.brand} to="/dashboard" aria-label="Quill Home">
            <span className={styles.brandMark}>Q</span>
            <span>Quill</span>
          </NavLink>

          <nav className={styles.primaryNav} aria-label="Primary navigation">
            {navigation.map(({ id, label, shortcut, to, Icon }) => {
              const active = destination === id
              return (
                <NavLink
                  key={id}
                  className={`${styles.navItem} ${active ? styles.navItemActive : ''}`}
                  to={to(universeId ?? '')}
                  aria-current={active ? 'page' : undefined}
                  aria-keyshortcuts={shortcut}
                  title={`${label} (${shortcut})`}
                >
                  <Icon aria-hidden="true" size={17} strokeWidth={1.8} />
                  <span>{label}</span>
                </NavLink>
              )
            })}
          </nav>

          <div className={styles.actions}>
            <div className={styles.statusWrap} title={headerStatus.detail}>
              <span className={`${styles.status} ${styles[`status${headerStatus.tone.charAt(0).toUpperCase()}${headerStatus.tone.slice(1)}` as keyof typeof styles]}`} role="status" aria-live="polite">
                <headerStatus.Icon aria-hidden="true" className={headerStatus.tone === 'working' ? styles.statusSpinner : undefined} size={15} />
                <span>{headerStatus.label}</span>
              </span>
            </div>

            <div className={styles.menuWrap} ref={switcherRef}>
              <button
                className={styles.universeButton}
                type="button"
                aria-expanded={switcherOpen}
                aria-haspopup="menu"
                aria-label={`Switch universe, current universe ${ctx.universe?.name || 'unknown'}`}
                onClick={() => setSwitcherOpen((open) => !open)}
              >
                <span className={styles.universeAvatar}>{universeInitial}</span>
                <span className={styles.universeName}>{ctx.universe?.name || 'Universe'}</span>
                <ChevronDown aria-hidden="true" size={15} />
              </button>
              {switcherOpen && (
                <div className={styles.menu} role="menu" aria-label="Universe switcher">
                  {universes.map((universe) => (
                    <button
                      className={styles.menuItem}
                      key={universe.id}
                      role="menuitem"
                      type="button"
                      aria-label={universe.name}
                      onClick={() => {
                        navigate(writePath(universe.id))
                        setSwitcherOpen(false)
                      }}
                    >
                      <span className={styles.menuAvatar}>{universe.name.charAt(0).toUpperCase()}</span>
                      <span>{universe.name}</span>
                    </button>
                  ))}
                  <button className={styles.menuItem} role="menuitem" type="button" onClick={openGenreEditor}>
                    Edit genres
                  </button>
                  <NavLink className={styles.menuItem} role="menuitem" to="/dashboard" onClick={() => setSwitcherOpen(false)}>
                    Manage universes
                  </NavLink>
                </div>
              )}
            </div>

            <div className={styles.menuWrap} ref={accountRef}>
              <button
                className={styles.accountButton}
                type="button"
                aria-expanded={accountOpen}
                aria-haspopup="menu"
                aria-label="Open account menu"
                onClick={() => setAccountOpen((open) => !open)}
              >
                <span className={styles.userAvatar}>{userInitial}</span>
                <User aria-hidden="true" size={15} />
              </button>
              {accountOpen && (
                <div className={`${styles.menu} ${styles.accountMenu}`} role="menu" aria-label="Account menu">
                  <span className={styles.accountName}>{user?.display_name || user?.email || 'Writer'}</span>
                  <NavLink className={styles.menuItem} role="menuitem" to={profileMemoryPath()} onClick={() => setAccountOpen(false)}>
                    <User aria-hidden="true" size={15} />
                    Writer profile
                  </NavLink>
                  <button className={styles.menuItem} role="menuitem" type="button" onClick={logout}>
                    <LogOut aria-hidden="true" size={15} />
                    Sign out
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>

        {universeId && (
          <GuidedDemoJourney
            destination={destination}
            observedAnalysis={observedDemoAnalysis}
            pathname={location.pathname}
            universeId={universeId}
          />
        )}

        <main id="universe-main" className={`${styles.content} ${isFullBleed ? styles.fullBleed : ''}`} tabIndex={-1}>
          <Outlet />
        </main>

        {editingGenres && (
          <div className={styles.genreDialogBackdrop} role="presentation">
            <section className={styles.genreDialog} aria-labelledby="edit-genres-title" aria-modal="true" role="dialog">
              <div className={styles.genreDialogHeading}>
                <div>
                  <p className={styles.genreDialogEyebrow}>Universe settings</p>
                  <h2 id="edit-genres-title">Edit genres for {ctx.universe?.name || 'this universe'}</h2>
                  <p>Genres are optional. Choose any combination from Quill’s existing list.</p>
                </div>
                <button className={styles.genreDialogClose} disabled={savingGenres} onClick={closeGenreEditor} type="button">
                  Close
                </button>
              </div>
              <GenreTagPicker
                disabled={savingGenres}
                id="edit-universe-genres"
                label="Genres"
                onChange={setEditableGenres}
                value={editableGenres}
              />
              {genreSaveError && <p className={styles.genreDialogError} role="alert">{genreSaveError}</p>}
              <div className={styles.genreDialogActions}>
                <button className={styles.genreDialogSave} disabled={savingGenres} onClick={() => void saveGenres()} type="button">
                  {savingGenres ? 'Saving genres…' : genreSaveError ? 'Try again' : 'Save genres'}
                </button>
                <button className={styles.genreDialogCancel} disabled={savingGenres} onClick={closeGenreEditor} type="button">
                  Cancel
                </button>
              </div>
            </section>
          </div>
        )}
      </div>
    </UniverseContext.Provider>
  )
}
