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
  id: string
  title: string
  order_index: number
  status: string
}

export default function EditorPage() {
  const { chapterId } = useParams<{ chapterId: string }>()
  const navigate = useNavigate()
  const { content, wordCount, isSaving, lastSavedAt, setContent, saveContent } = useEditorStore()
  const wsStatus = useWSStore((s) => s.status)
  const saveTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const [workId, setWorkId] = useState<string>('')
  const [universeId, setUniverseId] = useState<string>('')
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [showNewForm, setShowNewForm] = useState(false)
  const [newChapterTitle, setNewChapterTitle] = useState('')
  const [creatingChapter, setCreatingChapter] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  useWS()

  useEffect(() => {
    if (chapterId) {
      api.getChapter(chapterId).then(({ chapter }) => {
        setContent(chapter.content || '', chapter.raw_text || '')
        if (chapter.work_id) setWorkId(chapter.work_id)
        if (chapter.universe_id) setUniverseId(chapter.universe_id)
      })
    }
  }, [chapterId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Chapter rail — sibling chapters of the same work, refetched whenever the
  // active chapter changes so the analysis-status dot stays current.
  useEffect(() => {
    if (!workId) return
    api.listChapters(workId).then(({ chapters }) => setChapters(chapters || [])).catch(() => {})
  }, [workId, chapterId])

  const handleContentChange = useCallback((_html: string, text: string) => {
    setContent(_html, text)

    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    saveTimerRef.current = setTimeout(() => {
      if (chapterId) saveContent(chapterId)
    }, 5000)
  }, [chapterId, setContent, saveContent])

  const handleCreateChapter = async () => {
    if (!workId || !newChapterTitle.trim() || creatingChapter) return
    setCreatingChapter(true)
    setSubmitError(null)
    try {
      const { chapter } = await api.createChapter(workId, { title: newChapterTitle.trim() })
      setShowNewForm(false)
      setNewChapterTitle('')
      navigate(`/universe/${universeId}/editor/${chapter.id}`)
    } catch (err) {
      setSubmitError((err as Error).message || 'Failed to create chapter')
    } finally {
      setCreatingChapter(false)
    }
  }

  const wsStatusClass =
    wsStatus === 'open' ? styles.statusOpen : wsStatus === 'reconnecting' ? styles.statusWarn : styles.statusClosed

  return (
    <div className={styles.wrap}>
      <aside className={styles.rail}>
        <div className={styles.railHeader}>
          <span className={styles.railTitle}>Chapters</span>
          <button
            className={styles.railAddBtn}
            onClick={() => setShowNewForm((v) => !v)}
            aria-label="New chapter"
            title="New chapter"
          >
            +
          </button>
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
          {chapters
            .slice()
            .sort((a, b) => a.order_index - b.order_index)
            .map((ch) => (
              <button
                key={ch.id}
                className={`${styles.railItem} ${ch.id === chapterId ? styles.railItemActive : ''}`}
                onClick={() => navigate(`/universe/${universeId}/editor/${ch.id}`)}
              >
                <span className={styles.railDot} data-status={ch.status} />
                <span className={styles.railItemTitle}>{ch.title}</span>
              </button>
            ))}
        </div>
      </aside>

      <div className={styles.editorPanel}>
        <div className={styles.headerBar}>
          <span className={styles.headerLeft}>
            Chapter Editor
            <span className={`${styles.wsIndicator} ${wsStatusClass}`} title={`WS: ${wsStatus}`}>
              ●
            </span>
          </span>
          <div className={styles.headerRight}>
            <span>{wordCount} words</span>
            <span>{isSaving ? 'Saving…' : lastSavedAt ? `Saved ${lastSavedAt.toLocaleTimeString()}` : ''}</span>
          </div>
        </div>

        {chapterId && workId && universeId ? (
          <TipTapEditor
            chapterId={chapterId}
            workId={workId}
            universeId={universeId}
            initialContent={content}
            onContentChange={handleContentChange}
          />
        ) : (
          <div className={styles.loading}>Loading editor…</div>
        )}
      </div>

      <ContextPanel status={wsStatus} />
    </div>
  )
}
