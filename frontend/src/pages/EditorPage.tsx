import { useEffect, useRef, useCallback, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import { useFeedback } from '../components/feedback'
import { useEditorStore } from '../stores/editorStore'
import { useWSStore } from '../stores/wsStore'
import { useWS } from '../hooks/useWS'
import { api } from '../lib/api'
import type { CraftReviewResult, EntityCandidateDTO } from '../lib/types'
import TipTapEditor from '../components/editor/TipTapEditor'
import CraftReviewPanel from '../components/editor/CraftReviewPanel'
import EntityCandidateTray from '../components/editor/EntityCandidateTray'
import type { CandidateHighlightEntity } from '../components/editor/candidateHighlightExtension'
import ContextPanel from '../components/context-panel/ContextPanel'
import { IngestPanel } from '../components/editor/IngestPanel'
import { writeImportPath, writePath } from './writeRoutes'
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
const CRAFT_REVIEW_TIMEOUT_MS = 45_000

type PanelSide = 'rail' | 'context'

interface StoredPanelState {
  railCollapsed?: boolean
  contextCollapsed?: boolean
  railWidth?: number
  contextWidth?: number
  preferenceError?: boolean
}

function readPanelState(): StoredPanelState {
  if (typeof window === 'undefined') return {}
  try {
    return JSON.parse(window.localStorage.getItem(PANEL_STORAGE_KEY) || '{}') as StoredPanelState
  } catch (error) {
    return { preferenceError: Boolean(error) }
  }
}

function clampPanelWidth(width: number, min: number) {
  return Math.min(Math.max(width, min), MAX_PANEL_WIDTH)
}

export default function EditorPage() {
  const { chapterId, universeId } = useParams<{ chapterId: string; universeId: string }>()
  const routeKey = `${universeId || ''}:${chapterId || ''}`
  const activeRouteKeyRef = useRef(routeKey)
  const routeGenerationRef = useRef(0)
  if (activeRouteKeyRef.current !== routeKey) {
    activeRouteKeyRef.current = routeKey
    routeGenerationRef.current += 1
  }
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { publish, update } = useFeedback()
  const {
    content,
    wordCount,
    isSaving,
    saveStatus,
    saveError,
    lastSavedAt,
    setContent,
    saveContent,
    getLocalDraft,
    clearLocalDraft,
  } = useEditorStore()
  const wsStatus = useWSStore((s) => s.status)
  const wsError = useWSStore((s) => s.lastError)
  const wsErrorRequestId = useWSStore((s) => s.lastErrorRequestId)
  const clearWSError = useWSStore((s) => s.clearError)
  const sendWS = useWSStore((s) => s.send)
  const craftReviews = useWSStore((s) => s.craftReviews) || []
  const liveCandidates = useWSStore((s) => s.liveCandidates) || []
  const removeLiveCandidate = useWSStore((s) => s.removeLiveCandidate)
  const resetLiveAnalysis = useWSStore((s) => s.resetLiveAnalysis)
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const chapterLoadRequestRef = useRef(0)
  const knownEntitiesRequestRef = useRef(0)
  const candidatesRequestRef = useRef(0)
  const siblingChaptersRequestRef = useRef(0)
  const layoutStorageErrorNotified = useRef(false)
  const [initialPanelState] = useState(readPanelState)
  const [workInfo, setWorkInfo] = useState<WorkInfo | null>(null)
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [chapterTitle, setChapterTitle] = useState('')
  const [showNewForm, setShowNewForm] = useState(false)
  const [newChapterTitle, setNewChapterTitle] = useState('')
  const [creatingChapter, setCreatingChapter] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [editorReady, setEditorReady] = useState(false)
  const [railCollapsed, setRailCollapsed] = useState(() => initialPanelState.railCollapsed ?? false)
  const [contextCollapsed, setContextCollapsed] = useState(() => initialPanelState.contextCollapsed ?? false)
  const [railWidth, setRailWidth] = useState(() => clampPanelWidth(initialPanelState.railWidth ?? 240, MIN_RAIL_WIDTH))
  const [contextWidth, setContextWidth] = useState(() => clampPanelWidth(initialPanelState.contextWidth ?? 280, MIN_CONTEXT_WIDTH))
  const [craftReviewing, setCraftReviewing] = useState(false)
  const [craftReview, setCraftReview] = useState<CraftReviewResult | null>(null)
  const [requestedCraftSkills, setRequestedCraftSkills] = useState<string[] | null>(null)
  const [recoveryDraft, setRecoveryDraft] = useState<ReturnType<typeof getLocalDraft>>(null)
  const [editorKey, setEditorKey] = useState(0)
  const [knownEntities, setKnownEntities] = useState<Array<{ id: string; name: string; type?: string; aliases?: string[] }>>([])
  const [candidates, setCandidates] = useState<EntityCandidateDTO[]>([])
  const [candidateError, setCandidateError] = useState<string | null>(null)
  const [chapterLoadError, setChapterLoadError] = useState<string | null>(null)
  const [knownEntitiesError, setKnownEntitiesError] = useState<string | null>(null)
  const [chaptersError, setChaptersError] = useState<string | null>(null)
  const previousSaveStatus = useRef<string | null>(null)
  const craftRequestId = useRef<string | null>(null)
  const craftTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const terminalCraftRequestIds = useRef(new Set<string>())

  const rejectCraftRequest = useCallback((requestId: string | null) => {
    if (requestId) terminalCraftRequestIds.current.add(requestId)
    craftRequestId.current = null
    if (craftTimeoutRef.current) window.clearTimeout(craftTimeoutRef.current)
    craftTimeoutRef.current = null
    setCraftReviewing(false)
  }, [])

  useWS()

  useEffect(() => {
    if (!initialPanelState.preferenceError) return
    publish({ scope: 'write', status: 'failed', message: 'Workspace layout preferences could not be read. Default panel settings were restored.' })
  }, [initialPanelState.preferenceError, publish])

  useEffect(() => {
    try {
      window.localStorage.setItem(PANEL_STORAGE_KEY, JSON.stringify({
        railCollapsed,
        contextCollapsed,
        railWidth,
        contextWidth,
      }))
    } catch (error) {
      if (layoutStorageErrorNotified.current) return
      layoutStorageErrorNotified.current = true
      publish({
        scope: 'write',
        status: 'failed',
        message: error instanceof Error && error.message ? error.message : 'Workspace layout preferences could not be saved.',
      })
    }
  }, [contextCollapsed, contextWidth, publish, railCollapsed, railWidth])

  const loadChapter = useCallback(async () => {
    const requestId = ++chapterLoadRequestRef.current
    const requestRouteKey = routeKey
    const requestGeneration = routeGenerationRef.current
    const isCurrentRequest = () => (
      chapterLoadRequestRef.current === requestId
      && routeGenerationRef.current === requestGeneration
      && activeRouteKeyRef.current === requestRouteKey
    )

    if (!isCurrentRequest()) return
    if (saveTimerRef.current) {
      clearTimeout(saveTimerRef.current)
      saveTimerRef.current = null
    }
    setEditorReady(false)
    setChapterLoadError(null)
    setRecoveryDraft(null)
    setChapterTitle('')
    setKnownEntities([])
    setKnownEntitiesError(null)
    setCandidates([])
    setCandidateError(null)
    setWorkInfo(null)
    setChapters([])
    setChaptersError(null)
    setContent('', '')
    // The Live Analysis sidebar (pipeline/entities/contradictions/recall) is
    // keyed to whichever paragraph was last analyzed over the WS connection,
    // which stays open across chapter navigation — without this, switching
    // chapters kept showing the previous chapter's stale analysis.
    resetLiveAnalysis()

    if (!chapterId) {
      setEditorReady(true)
      return
    }

    try {
      const { chapter } = await api.getChapter(chapterId)
      if (!isCurrentRequest()) return
      setContent(chapter.content || '', chapter.raw_text || '')
      setChapterTitle(chapter.title || '')
      const local = typeof getLocalDraft === 'function' ? getLocalDraft(chapterId) : null
      const serverUpdatedAt = Date.parse(chapter.updated_at || '')
      const localIsNewer = Boolean(local && local.content !== (chapter.content || '') && local.updatedAt > (Number.isFinite(serverUpdatedAt) ? serverUpdatedAt : 0))
      if (localIsNewer) setRecoveryDraft(local)
      if (!chapter.work_id) throw new Error('This chapter is not attached to a manuscript.')

      const { work } = await api.getWork(chapter.work_id)
      if (!isCurrentRequest()) return
      setWorkInfo({ id: work.id, title: work.title, universe_id: work.universe_id })
    } catch (error) {
      if (!isCurrentRequest()) return
      const message = error instanceof Error && error.message ? error.message : 'We could not load this chapter.'
      setChapterLoadError(message)
      publish({ scope: 'write', status: 'failed', message })
    } finally {
      if (isCurrentRequest()) setEditorReady(true)
    }
  }, [chapterId, getLocalDraft, publish, resetLiveAnalysis, routeKey, setContent])

  useEffect(() => { void loadChapter() }, [loadChapter])

  const loadKnownEntities = useCallback(async () => {
    if (!universeId || typeof api.listEntities !== 'function') return
    const requestId = ++knownEntitiesRequestRef.current
    const requestRouteKey = routeKey
    const requestGeneration = routeGenerationRef.current
    const isCurrentRequest = () => (
      knownEntitiesRequestRef.current === requestId
      && routeGenerationRef.current === requestGeneration
      && activeRouteKeyRef.current === requestRouteKey
    )
    setKnownEntitiesError(null)
    try {
      const { entities } = await api.listEntities(universeId, { limit: '500', page: '1', status: 'active' })
      if (!isCurrentRequest()) return
      setKnownEntities((entities || []).filter((entity: { status?: string }) => !entity.status || entity.status === 'active').map((entity: { id: string; name: string; type?: string; aliases?: string[] }) => ({
        id: entity.id, name: entity.name, type: entity.type, aliases: entity.aliases,
      })))
      setEditorKey((key) => key + 1)
    } catch (error) {
      if (!isCurrentRequest()) return
      const message = error instanceof Error && error.message ? error.message : 'Known entity links are unavailable.'
      setKnownEntitiesError(message)
      publish({ scope: 'write', status: 'failed', message })
    }
  }, [publish, routeKey, universeId])

  useEffect(() => { void loadKnownEntities() }, [loadKnownEntities])

  const loadCandidates = useCallback(async () => {
    if (!universeId || typeof api.listEntityCandidates !== 'function') return
    const requestId = ++candidatesRequestRef.current
    const requestRouteKey = routeKey
    const requestGeneration = routeGenerationRef.current
    const isCurrentRequest = () => (
      candidatesRequestRef.current === requestId
      && routeGenerationRef.current === requestGeneration
      && activeRouteKeyRef.current === requestRouteKey
    )
    try {
      const response = await api.listEntityCandidates(universeId)
      if (!isCurrentRequest()) return
      setCandidates(response.candidates || [])
      setCandidateError(null)
    } catch (error) {
      if (!isCurrentRequest()) return
      const message = (error as Error).message || 'Could not load entity candidates'
      setCandidateError(message)
      publish({ scope: 'write', status: 'failed', message })
    }
  }, [publish, routeKey, universeId])

  useEffect(() => { void loadCandidates() }, [loadCandidates])

  const loadSiblingChapters = useCallback(async () => {
    const workId = workInfo?.id
    if (!workId) return
    const requestId = ++siblingChaptersRequestRef.current
    const requestRouteKey = routeKey
    const requestGeneration = routeGenerationRef.current
    const isCurrentRequest = () => (
      siblingChaptersRequestRef.current === requestId
      && routeGenerationRef.current === requestGeneration
      && activeRouteKeyRef.current === requestRouteKey
    )
    setChaptersError(null)
    try {
      const { chapters: nextChapters } = await api.listChapters(workId)
      if (!isCurrentRequest()) return
      setChapters(nextChapters || [])
    } catch (error) {
      if (!isCurrentRequest()) return
      const message = error instanceof Error && error.message ? error.message : 'Chapter navigation is unavailable.'
      setChaptersError(message)
      publish({ scope: 'write', status: 'failed', message })
    }
  }, [publish, routeKey, workInfo?.id])

  useEffect(() => { void loadSiblingChapters() }, [chapterId, loadSiblingChapters])

  useEffect(() => {
    const activeRequestId = craftRequestId.current
    const matchingReview = craftReviews.find((review) => review.request_id === activeRequestId)
    if (!matchingReview || !activeRequestId) return
    if (terminalCraftRequestIds.current.has(matchingReview.request_id)) return
    setCraftReview(matchingReview)
    rejectCraftRequest(activeRequestId)
  }, [craftReviews, rejectCraftRequest])

  useEffect(() => {
    // `lastError` also carries connection and unrelated analysis failures.
    // Only a server error explicitly tagged with this review's request ID can
    // end its lifecycle; everything else remains observable globally while
    // this request keeps waiting for its own result or timeout.
    if (!craftReviewing || !wsError || !craftRequestId.current || wsErrorRequestId !== craftRequestId.current) return
    rejectCraftRequest(craftRequestId.current)
    setSubmitError(`Craft review could not finish: ${wsError}`)
  }, [craftReviewing, rejectCraftRequest, wsError, wsErrorRequestId])

  useEffect(() => {
    rejectCraftRequest(craftRequestId.current)
    setCraftReview(null)
  }, [chapterId, rejectCraftRequest, universeId])

  useEffect(() => () => {
    rejectCraftRequest(craftRequestId.current)
  }, [rejectCraftRequest])

  useEffect(() => {
    if (!chapterId || saveStatus !== 'failed' || previousSaveStatus.current === 'failed') {
      previousSaveStatus.current = saveStatus || null
      return
    }
    previousSaveStatus.current = saveStatus
    publish({
      scope: 'autosave',
      status: 'failed',
      message: saveError || 'Autosave failed. Your local recovery copy is still available.',
      retry: () => saveContent(chapterId),
    })
  }, [chapterId, publish, saveContent, saveError, saveStatus])

  const handleContentChange = useCallback((_html: string, text: string) => {
    if (!chapterId) return
    const saveRouteKey = routeKey
    const saveGeneration = routeGenerationRef.current
    setContent(_html, text, chapterId)
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    saveTimerRef.current = setTimeout(() => {
      if (routeGenerationRef.current !== saveGeneration || activeRouteKeyRef.current !== saveRouteKey) return
      void saveContent(chapterId)
    }, 5000)
  }, [chapterId, routeKey, setContent, saveContent])

  useEffect(() => () => {
    if (saveTimerRef.current) {
      clearTimeout(saveTimerRef.current)
      saveTimerRef.current = null
    }
  }, [routeKey])

  const handleRestoreDraft = useCallback(() => {
    if (!chapterId || !recoveryDraft) return
    setContent(recoveryDraft.content, recoveryDraft.rawText, chapterId)
    setRecoveryDraft(null)
    setEditorKey((key) => key + 1)
    void saveContent(chapterId)
  }, [chapterId, recoveryDraft, saveContent, setContent])

  const handleDiscardDraft = useCallback(() => {
    if (chapterId && typeof clearLocalDraft === 'function') clearLocalDraft(chapterId)
    setRecoveryDraft(null)
  }, [chapterId, clearLocalDraft])

  const handleExport = useCallback(async (kind: 'chapter' | 'work') => {
    const id = kind === 'chapter' ? chapterId : workInfo?.id
    if (!id) return
    const feedbackId = publish({ scope: 'write', status: 'running', message: `Preparing ${kind} export…` })
    try {
      // Export only after the current editor snapshot reaches the server;
      // otherwise a just-typed paragraph is missing from the download.
      if (chapterId && !(await saveContent(chapterId))) {
        update(feedbackId, { status: 'failed', message: 'Export paused because the latest chapter could not be saved.' })
        return
      }
      const markdown = kind === 'chapter'
        ? await api.exportChapterMarkdown(id)
        : await api.exportWorkMarkdown(id)
      const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${(kind === 'chapter' ? chapterTitle : workInfo?.title) || 'quill-export'}.md`.replace(/[^a-z0-9._-]+/gi, '-').replace(/^-|-$/g, '')
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
      update(feedbackId, { status: 'completed', message: `${kind === 'chapter' ? 'Chapter' : 'Manuscript'} export is ready.` })
    } catch (error) {
      const message = (error as Error).message || 'Export failed'
      setSubmitError(message)
      update(feedbackId, { status: 'failed', message })
    }
  }, [chapterId, chapterTitle, publish, saveContent, update, workInfo])

  const handleEntityClick = useCallback((entityId: string) => {
    navigate(`/universe/${universeId}/entities/${entityId}`)
  }, [navigate, universeId])

  const currentUniverseLiveCandidates = liveCandidates.filter((candidate) => candidate.universe_id === (workInfo?.universe_id || universeId))
  const visibleCandidates = [...candidates]
  for (const candidate of currentUniverseLiveCandidates) {
    if (!visibleCandidates.some((item) => item.entity_id === candidate.entity_id)) visibleCandidates.push(candidate)
  }
  const visibleCandidateHighlights: CandidateHighlightEntity[] = visibleCandidates.map((candidate) => ({
    id: candidate.entity_id,
    name: candidate.name,
    type: candidate.type,
    aliases: candidate.aliases,
    confidence: candidate.confidence,
    evidence_quote: candidate.evidence_quote,
  }))

  const handleCandidateDecision = useCallback(async (candidateId: string, decision: 'accept' | 'dismiss') => {
    try {
      if (decision === 'accept') await api.acceptEntityCandidate(candidateId)
      else await api.dismissEntityCandidate(candidateId)
      setCandidates((current) => current.filter((candidate) => candidate.entity_id !== candidateId))
      removeLiveCandidate(candidateId)
      publish({ scope: 'review', status: 'completed', message: decision === 'accept' ? 'Entity candidate accepted.' : 'Entity candidate dismissed.' })
    } catch (error) {
      const message = (error as Error).message || 'Could not save candidate decision'
      setCandidateError(message)
      publish({ scope: 'review', status: 'failed', message })
    }
  }, [publish, removeLiveCandidate])

  const handleCraftReview = useCallback(({ passage }: { passage: string; from: number; to: number }) => {
    if (!chapterId || !workInfo?.id || !universeId || !passage.trim() || typeof sendWS !== 'function') return
    const requestId = crypto.randomUUID()
    // Do not let an old global transport/analysis error immediately fail a
    // newly submitted craft review.
    if (typeof clearWSError === 'function') clearWSError()
    if (craftRequestId.current) terminalCraftRequestIds.current.add(craftRequestId.current)
    craftRequestId.current = requestId
    if (craftTimeoutRef.current) window.clearTimeout(craftTimeoutRef.current)
    setCraftReviewing(true)
    setCraftReview(null)
    setSubmitError(null)
    sendWS({
      type: 'craft_review_request',
      payload: {
        universe_id: workInfo.universe_id || universeId,
        work_id: workInfo.id,
        chapter_id: chapterId,
        passage: passage.trim(),
        request_id: requestId,
        ...(requestedCraftSkills ? { requested_skill_names: requestedCraftSkills } : {}),
      },
    })
    craftTimeoutRef.current = window.setTimeout(() => {
      if (craftRequestId.current !== requestId) return
      rejectCraftRequest(requestId)
      setSubmitError('Craft review took too long. Your text was not changed; try again when live analysis is available.')
    }, CRAFT_REVIEW_TIMEOUT_MS)
  }, [chapterId, universeId, workInfo, sendWS, requestedCraftSkills, clearWSError, rejectCraftRequest])

  const handleCreateChapter = async () => {
    if (!workInfo?.id || !newChapterTitle.trim() || creatingChapter) return
    setCreatingChapter(true); setSubmitError(null)
    try {
      const { chapter } = await api.createChapter(workInfo.id, { title: newChapterTitle.trim() })
      setShowNewForm(false); setNewChapterTitle('')
      publish({ scope: 'write', status: 'completed', message: `Created ${chapter.title || 'a new chapter'}.` })
      navigate(writePath(universeId || '', chapter.id))
    } catch (err) {
      const message = (err as Error).message || 'Failed to create chapter'
      setSubmitError(message)
      publish({ scope: 'write', status: 'failed', message })
    } finally { setCreatingChapter(false) }
  }

  const openImport = () => {
    if (universeId) navigate(writeImportPath(universeId, chapterId))
  }

  const closeImport = () => {
    if (universeId && chapterId) navigate(writePath(universeId, chapterId), { replace: true })
  }

  const handleImportCompleted = useCallback(async (workId: string) => {
    const requestRouteKey = routeKey
    const requestGeneration = routeGenerationRef.current
    const isCurrentRequest = () => (
      routeGenerationRef.current === requestGeneration && activeRouteKeyRef.current === requestRouteKey
    )
    if (!universeId || !isCurrentRequest()) return
    const { chapters: importedChapters } = await api.listChapters(workId)
    if (!isCurrentRequest()) return
    const sortedImportedChapters = [...(importedChapters || [])].sort((left, right) => left.order_index - right.order_index)
    const chapter = sortedImportedChapters[sortedImportedChapters.length - 1]
    navigate(chapter ? writePath(universeId, chapter.id) : writePath(universeId))
  }, [navigate, routeKey, universeId])

  const wsStatusClass =
    wsStatus === 'open' ? styles.statusOpen
    : wsStatus === 'reconnecting' ? styles.statusWarn
    : styles.statusClosed

  const sorted = [...chapters].sort((a, b) => a.order_index - b.order_index)
  const effectiveSaveStatus = saveStatus || (isSaving ? 'saving' : lastSavedAt ? 'saved' : 'idle')
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
  const importOpen = searchParams.get('panel') === 'import'

  return (
    <div className={styles.wrap} style={workspaceStyle}>
      {importOpen && universeId && (
        <div className={styles.importOverlay} role="dialog" aria-modal="true" aria-label="Import manuscript">
          <div className={styles.importPanel}>
            <IngestPanel universeId={universeId} onClose={closeImport} onCompleted={handleImportCompleted} />
          </div>
        </div>
      )}
      {/* Chapter rail */}
      <aside id="chapter-panel" className={`${styles.rail} ${railCollapsed ? styles.panelCollapsed : ''}`} aria-label="Chapter navigation">
        {!railCollapsed && <div className={`${styles.railContent} q-scroll`}>
          <div className={styles.railHeader}>
            {workInfo ? (
              <button
                type="button"
                className={styles.railWorkLink}
                onClick={() => universeId && navigate(writePath(universeId))}
              >
                {workInfo.title}
              </button>
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
              <div className={styles.railFormActions}>
                <button type="button" onClick={() => void handleCreateChapter()} disabled={creatingChapter}>{creatingChapter ? 'Creating…' : 'Create'}</button>
                <button type="button" onClick={() => { setShowNewForm(false); setSubmitError(null) }} disabled={creatingChapter}>Cancel</button>
              </div>
              {submitError && <p className={styles.railFormError}>{submitError}</p>}
            </div>
          )}

          <div className={styles.railList}>
            {chaptersError && (
              <div className={styles.railError} role="alert">
                <span>{chaptersError}</span>
                <button type="button" onClick={() => void loadSiblingChapters()}>Retry</button>
              </div>
            )}
            {sorted.map((ch, i) => (
              <button
                key={ch.id}
                className={`${styles.railItem} ${ch.id === chapterId ? styles.railItemActive : ''}`}
                onClick={() => universeId && navigate(writePath(universeId, ch.id))}
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
              {effectiveSaveStatus === 'saving'
                ? <><span className={styles.savingDot} />Saving…</>
                : effectiveSaveStatus === 'failed'
                ? <button type="button" className={styles.saveFailed} onClick={() => chapterId && void saveContent(chapterId)} title={saveError || 'Save failed'}><span className={styles.failedDot} />Save failed · Retry</button>
                : effectiveSaveStatus === 'saved'
                ? <><span className={styles.savedDot} />Saved</>
                : ''}
            </span>
            <span>{wordCount.toLocaleString()} words</span>
            <button type="button" className={styles.exportButton} onClick={() => void handleExport('chapter')} disabled={!chapterId} title="Export chapter as Markdown">↓ Chapter</button>
            <button type="button" className={styles.exportButton} onClick={() => void handleExport('work')} disabled={!workInfo?.id} title="Export work as Markdown">↓ Work</button>
            <button type="button" className={styles.exportButton} onClick={openImport} disabled={!universeId}>Import</button>
            <span className={`glyph ${styles.wsIndicator} ${wsStatusClass}`} title={`WS: ${wsStatus}`}>●</span>
          </div>
        </div>

        {chapterLoadError ? (
          <div className={styles.loadError} role="alert">
            <p>{chapterLoadError}</p>
            <button type="button" onClick={() => void loadChapter()}>Retry loading chapter</button>
          </div>
        ) : !editorReady ? (
          <div className={styles.loading}>Loading editor…</div>
        ) : chapterId && workInfo ? (
          <>
            {recoveryDraft && (
              <div className={styles.recoveryBanner} role="status">
                <span>Unsaved local recovery found from {new Date(recoveryDraft.updatedAt).toLocaleString()}.</span>
                <button type="button" onClick={handleRestoreDraft}>Restore</button>
                <button type="button" onClick={handleDiscardDraft}>Discard</button>
              </div>
            )}
            <TipTapEditor
              key={`${chapterId}:${editorKey}`}
              chapterId={chapterId}
              workId={workInfo.id}
              universeId={workInfo.universe_id || universeId || ''}
              initialContent={content}
              onContentChange={handleContentChange}
              knownEntities={knownEntities}
              onEntityClick={handleEntityClick}
              candidateEntities={visibleCandidateHighlights}
              onCandidateDecision={handleCandidateDecision}
              onCraftReview={handleCraftReview}
              reviewing={craftReviewing}
            />
            {knownEntitiesError && (
              <div className={styles.editorNotice} role="status">
                <span>{knownEntitiesError}</span>
                <button type="button" onClick={() => void loadKnownEntities()}>Retry entity links</button>
              </div>
            )}
            {submitError && <div className={styles.editorNotice} role="alert">{submitError}</div>}
          </>
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
              selectedSkills={requestedCraftSkills}
              onSelectedSkillsChange={setRequestedCraftSkills}
            />
            <EntityCandidateTray
              candidates={visibleCandidates}
              error={candidateError}
              universeId={universeId || ''}
              onChanged={loadCandidates}
              onDecision={removeLiveCandidate}
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
