import { useEffect, useRef, useCallback, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useEditorStore } from '../stores/editorStore'
import { useWSStore } from '../stores/wsStore'
import { useWS } from '../hooks/useWS'
import { api } from '../lib/api'
import TipTapEditor from '../components/editor/TipTapEditor'
import CraftReviewPanel from '../components/editor/CraftReviewPanel'
import ContextPanel from '../components/context-panel/ContextPanel'
import styles from './EditorPage.module.css'

interface Chapter {
  id: string; title: string; order_index: number; status: string; updated_at?: string
}

interface WorkInfo {
  id: string; title: string; universe_id: string
}

const PANEL_STORAGE_KEY = 'quill:editor-workspace-panels'
const MIN_RAIL_WIDTH = 220
const MIN_CONTEXT_WIDTH = 260
const MAX_PANEL_WIDTH = 420

type PanelSide = 'rail' | 'context'

interface StoredPanelState {
  railCollapsed?: boolean
  contextCollapsed?: boolean
  railWidth?: number
  contextWidth?: number
}

function readPanelState(): StoredPanelState {
  if (typeof window === 'undefined') return {}
  try {
    return JSON.parse(window.localStorage.getItem(PANEL_STORAGE_KEY) || '{}') as StoredPanelState
  } catch {
    return {}
  }
}

function clampPanelWidth(width: number, min: number) {
  return Math.min(Math.max(width, min), MAX_PANEL_WIDTH)
}

export default function EditorPage() {
  const { chapterId, universeId } = useParams<{ chapterId: string; universeId: string }>()
  const navigate = useNavigate()
  const { content, wordCount, isSaving, lastSavedAt, setContent, saveContent } = useEditorStore()
  const wsStatus = useWSStore((s) => s.status)
  const wsError = useWSStore((s) => s.lastError)
  const sendWS = useWSStore((s) => s.send)
  const craftReviews = useWSStore((s) => s.craftReviews) || []
  const saveTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const [workInfo, setWorkInfo] = useState<WorkInfo | null>(null)
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [chapterTitle, setChapterTitle] = useState('')
  const [showNewForm, setShowNewForm] = useState(false)
  const [newChapterTitle, setNewChapterTitle] = useState('')
  const [creatingChapter, setCreatingChapter] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [editorReady, setEditorReady] = useState(false)
  const [railCollapsed, setRailCollapsed] = useState(() => readPanelState().railCollapsed ?? false)
  const [contextCollapsed, setContextCollapsed] = useState(() => readPanelState().contextCollapsed ?? false)
  const [railWidth, setRailWidth] = useState(() => clampPanelWidth(readPanelState().railWidth ?? 240, MIN_RAIL_WIDTH))
  const [contextWidth, setContextWidth] = useState(() => clampPanelWidth(readPanelState().contextWidth ?? 280, MIN_CONTEXT_WIDTH))
  const [craftReviewing, setCraftReviewing] = useState(false)

  useWS()

  useEffect(() => {
    window.localStorage.setItem(PANEL_STORAGE_KEY, JSON.stringify({
      railCollapsed,
      contextCollapsed,
      railWidth,
      contextWidth,
    }))
  }, [railCollapsed, contextCollapsed, railWidth, contextWidth])

  // Load chapter data and determine workId
  useEffect(() => {
    if (!chapterId) return
    setEditorReady(false)
    api.getChapter(chapterId).then(({ chapter }) => {
      setContent(chapter.content || '', chapter.raw_text || '')
      setChapterTitle(chapter.title || '')
      if (chapter.work_id) {
        api.getWork(chapter.work_id).then(({ work }) => {
          setWorkInfo({ id: work.id, title: work.title, universe_id: work.universe_id })
          setEditorReady(true)
        }).catch(() => setEditorReady(true))
      } else {
        setEditorReady(true)
      }
    }).catch(() => setEditorReady(true))
  }, [chapterId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Load sibling chapters
  useEffect(() => {
    if (!workInfo?.id) return
    api.listChapters(workInfo.id)
      .then(({ chapters }) => setChapters(chapters || []))
      .catch(() => {})
  }, [workInfo?.id, chapterId])

  // A result arrives as a separate WS message. Keep the explicit review
  // button busy until that message is observed; live paragraph analysis never
  // changes this state.
  useEffect(() => {
    setCraftReviewing(false)
  }, [craftReviews.length, wsError])

  useEffect(() => {
    setCraftReviewing(false)
  }, [chapterId])

  const handleContentChange = useCallback((_html: string, text: string) => {
    setContent(_html, text)
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    saveTimerRef.current = setTimeout(() => {
      if (chapterId) saveContent(chapterId)
    }, 5000)
  }, [chapterId, setContent, saveContent])

  const handleCraftReview = useCallback(({ passage }: { passage: string; from: number; to: number }) => {
    if (!chapterId || !workInfo?.id || !universeId || !passage.trim() || typeof sendWS !== 'function') return
    setCraftReviewing(true)
    sendWS({
      type: 'craft_review_request',
      payload: {
        universe_id: workInfo.universe_id || universeId,
        work_id: workInfo.id,
        chapter_id: chapterId,
        passage: passage.trim(),
      },
    })
  }, [chapterId, universeId, workInfo, sendWS])

  const handleCreateChapter = async () => {
    if (!workInfo?.id || !newChapterTitle.trim() || creatingChapter) return
    setCreatingChapter(true); setSubmitError(null)
    try {
      const { chapter } = await api.createChapter(workInfo.id, { title: newChapterTitle.trim() })
      setShowNewForm(false); setNewChapterTitle('')
      navigate(`/universe/${universeId}/editor/${chapter.id}`)
    } catch (err) {
      setSubmitError((err as Error).message || 'Failed to create chapter')
    } finally { setCreatingChapter(false) }
  }

  const wsStatusClass =
    wsStatus === 'open' ? styles.statusOpen
    : wsStatus === 'reconnecting' ? styles.statusWarn
    : styles.statusClosed

  const sorted = [...chapters].sort((a, b) => a.order_index - b.order_index)
  const craftReview = [...craftReviews]
    .reverse()
    .find((review) => review.chapter_id === chapterId) || null
  const resizePanel = (side: PanelSide, clientX: number) => {
    if (side === 'rail') {
      setRailWidth(clampPanelWidth(clientX, MIN_RAIL_WIDTH))
      return
    }
    setContextWidth(clampPanelWidth(window.innerWidth - clientX, MIN_CONTEXT_WIDTH))
  }

  const startResize = (side: PanelSide, event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    const onPointerMove = (moveEvent: PointerEvent) => resizePanel(side, moveEvent.clientX)
    const stopResize = () => {
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerup', stopResize)
    }
    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', stopResize, { once: true })
  }

  const resizeWithKeyboard = (side: PanelSide, event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const delta = event.key === 'ArrowRight' ? 16 : -16
    if (side === 'rail') setRailWidth((width) => clampPanelWidth(width + delta, MIN_RAIL_WIDTH))
    else setContextWidth((width) => clampPanelWidth(width - delta, MIN_CONTEXT_WIDTH))
  }

  const workspaceStyle = {
    '--chapter-panel-width': railCollapsed ? '36px' : `${railWidth}px`,
    '--context-panel-width': contextCollapsed ? '36px' : `${contextWidth}px`,
  } as CSSProperties

  return (
    <div className={styles.wrap} style={workspaceStyle}>
      {/* Chapter rail */}
      <aside id="chapter-panel" className={`${styles.rail} ${railCollapsed ? styles.panelCollapsed : ''}`} aria-label="Chapter navigation">
        {!railCollapsed && <div className={`${styles.railContent} q-scroll`}>
          <div className={styles.railHeader}>
            {workInfo ? (
              <span
                className={styles.railWorkLink}
                role="button"
                tabIndex={0}
                onClick={() => navigate(`/universe/${universeId}/works`)}
              >
                {workInfo.title}
              </span>
            ) : (
              <span className={styles.railWorkLink} style={{ color: 'var(--muted-3)', cursor: 'default' }}>
                Works &amp; Chapters
              </span>
            )}
            <button
              className={styles.railAddBtn}
              onClick={() => setShowNewForm((v) => !v)}
              aria-label="New chapter"
              title="New chapter"
            >+</button>
          </div>

          {showNewForm && (
            <div className={styles.railForm}>
              <input
                className={styles.railFormInput}
                placeholder="Chapter title"
                value={newChapterTitle}
                autoFocus
                disabled={creatingChapter}
                onChange={(e) => setNewChapterTitle(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleCreateChapter()}
              />
              {submitError && <p className={styles.railFormError}>{submitError}</p>}
            </div>
          )}

          <div className={styles.railList}>
            {sorted.map((ch, i) => (
              <button
                key={ch.id}
                className={`${styles.railItem} ${ch.id === chapterId ? styles.railItemActive : ''}`}
                onClick={() => navigate(`/universe/${universeId}/editor/${ch.id}`)}
              >
                <span className={styles.railDot} data-status={ch.status} />
                <span className={styles.railItemTitle}>{i + 1} · {ch.title}</span>
              </button>
            ))}
          </div>
        </div>}
        <button
          className={`${styles.panelToggle} ${styles.railToggle}`}
          onClick={() => setRailCollapsed((collapsed) => !collapsed)}
          aria-controls="chapter-panel"
          aria-expanded={!railCollapsed}
          aria-label={railCollapsed ? 'Expand chapter panel' : 'Collapse chapter panel'}
          title={railCollapsed ? 'Expand chapter panel' : 'Collapse chapter panel'}
        >{railCollapsed ? '›' : '‹'}</button>
        {!railCollapsed && <div
          className={`${styles.resizeHandle} ${styles.railResizeHandle}`}
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize chapter panel"
          aria-valuemin={MIN_RAIL_WIDTH}
          aria-valuemax={MAX_PANEL_WIDTH}
          aria-valuenow={railWidth}
          tabIndex={0}
          onPointerDown={(event) => startResize('rail', event)}
          onKeyDown={(event) => resizeWithKeyboard('rail', event)}
        />}
      </aside>

      {/* Editor panel */}
      <div className={styles.editorPanel}>
        <div className={styles.headerBar}>
          <div className={styles.headerLeft}>
            <span className={styles.headerCrumb}>{workInfo?.title || 'Work'}</span>
            <span className={styles.headerTitle}>{chapterTitle || 'Editor'}</span>
          </div>
          <div className={styles.headerRight}>
            <span>
              {isSaving
                ? <><span className={styles.savingDot} />Saving…</>
                : lastSavedAt
                ? <><span className={styles.savedDot} />Saved</>
                : ''}
            </span>
            <span>{wordCount.toLocaleString()} words</span>
            <span className={`glyph ${styles.wsIndicator} ${wsStatusClass}`} title={`WS: ${wsStatus}`}>●</span>
          </div>
        </div>

        {!editorReady ? (
          <div className={styles.loading}>Loading editor…</div>
        ) : chapterId && workInfo ? (
          <TipTapEditor
            chapterId={chapterId}
            workId={workInfo.id}
            universeId={workInfo.universe_id || universeId || ''}
            initialContent={content}
            onContentChange={handleContentChange}
            onCraftReview={handleCraftReview}
            reviewing={craftReviewing}
          />
        ) : (
          <div className={styles.noChapterState}>
            <span className={`glyph ${styles.noChapterGlyph}`}>✎</span>
            <p className={styles.noChapterText}>Select a chapter from the list to start writing.</p>
          </div>
        )}
      </div>

      {/* Context panel */}
      <aside id="analysis-panel" className={`${styles.contextPanel} ${contextCollapsed ? styles.panelCollapsed : ''}`} aria-label="Live analysis">
        {!contextCollapsed && <div className={styles.contextContent}>
          <div className={styles.contextStack}>
            <CraftReviewPanel
              review={craftReview}
              loading={craftReviewing}
              universeId={workInfo?.universe_id || universeId || ''}
              workId={workInfo?.id || ''}
              chapterId={chapterId || ''}
            />
            <div className={styles.contextAnalysis}>
              <ContextPanel status={wsStatus} universeId={workInfo?.universe_id || universeId} />
            </div>
          </div>
        </div>}
        <button
          className={`${styles.panelToggle} ${styles.contextToggle}`}
          onClick={() => setContextCollapsed((collapsed) => !collapsed)}
          aria-controls="analysis-panel"
          aria-expanded={!contextCollapsed}
          aria-label={contextCollapsed ? 'Expand live analysis panel' : 'Collapse live analysis panel'}
          title={contextCollapsed ? 'Expand live analysis panel' : 'Collapse live analysis panel'}
        >{contextCollapsed ? '‹' : '›'}</button>
        {!contextCollapsed && <div
          className={`${styles.resizeHandle} ${styles.contextResizeHandle}`}
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize live analysis panel"
          aria-valuemin={MIN_CONTEXT_WIDTH}
          aria-valuemax={MAX_PANEL_WIDTH}
          aria-valuenow={contextWidth}
          tabIndex={0}
          onPointerDown={(event) => startResize('context', event)}
          onKeyDown={(event) => resizeWithKeyboard('context', event)}
        />}
      </aside>
    </div>
  )
}
