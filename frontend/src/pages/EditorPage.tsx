import { useEffect, useRef, useCallback, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useEditorStore } from '../stores/editorStore'
import { useWSStore } from '../stores/wsStore'
import { useWS } from '../hooks/useWS'
import { api } from '../lib/api'
import TipTapEditor from '../components/editor/TipTapEditor'
import ContextPanel from '../components/context-panel/ContextPanel'
import styles from './EditorPage.module.css'

interface Chapter {
  id: string; title: string; order_index: number; status: string; updated_at?: string
}

interface WorkInfo {
  id: string; title: string; universe_id: string
}

export default function EditorPage() {
  const { chapterId, universeId } = useParams<{ chapterId: string; universeId: string }>()
  const navigate = useNavigate()
  const { content, wordCount, isSaving, lastSavedAt, setContent, saveContent } = useEditorStore()
  const wsStatus = useWSStore((s) => s.status)
  const saveTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const [workInfo, setWorkInfo] = useState<WorkInfo | null>(null)
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [chapterTitle, setChapterTitle] = useState('')
  const [showNewForm, setShowNewForm] = useState(false)
  const [newChapterTitle, setNewChapterTitle] = useState('')
  const [creatingChapter, setCreatingChapter] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [editorReady, setEditorReady] = useState(false)

  useWS()

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

  const handleContentChange = useCallback((_html: string, text: string) => {
    setContent(_html, text)
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    saveTimerRef.current = setTimeout(() => {
      if (chapterId) saveContent(chapterId)
    }, 5000)
  }, [chapterId, setContent, saveContent])

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

  return (
    <div className={styles.wrap}>
      {/* Chapter rail */}
      <aside className={styles.rail + ' q-scroll'}>
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
          />
        ) : (
          <div className={styles.noChapterState}>
            <span className={`glyph ${styles.noChapterGlyph}`}>✎</span>
            <p className={styles.noChapterText}>Select a chapter from the list to start writing.</p>
          </div>
        )}
      </div>

      {/* Context panel */}
      <ContextPanel status={wsStatus} universeId={workInfo?.universe_id || universeId} />
    </div>
  )
}
