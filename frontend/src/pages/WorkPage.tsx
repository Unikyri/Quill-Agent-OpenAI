import { useEffect, useState, useCallback, type KeyboardEvent } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import ImageUpload from '../components/shared/ImageUpload'
import styles from './WorkPage.module.css'

// ponytail: shared Enter/Space activation handler for clickable non-button
// elements — keeps them keyboard-operable without a full <button> restyle.
function onActivateKey(fn: () => void) {
  return (e: KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      fn()
    }
  }
}

interface Work {
  id: string
  title: string
  type: string
  synopsis?: string
  universe_id: string
}

interface Chapter {
  id: string
  title: string
  order_index: number
  word_count: number
  status: string
  updated_at?: string
}

export default function WorkPage() {
  const { workId } = useParams<{ workId: string }>()
  const navigate = useNavigate()
  const [work, setWork] = useState<Work | null>(null)
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [cover, setCover] = useState<string | null>(null)

  const [showNewForm, setShowNewForm] = useState(false)
  const [chapterTitle, setChapterTitle] = useState('')
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [creatingChapter, setCreatingChapter] = useState(false)

  // Inline title/synopsis editing (ponytail: cover image is client-side only —
  // no `cover_url` column on the work model yet, so it resets on reload)
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState('')
  const [editingSynopsis, setEditingSynopsis] = useState(false)
  const [synopsisDraft, setSynopsisDraft] = useState('')
  const [renamingChapterId, setRenamingChapterId] = useState<string | null>(null)
  const [chapterTitleDraft, setChapterTitleDraft] = useState('')

  const fetchData = useCallback(() => {
    if (!workId) return
    setLoading(true)
    setError(null)
    Promise.all([api.getWork(workId), api.listChapters(workId)])
      .then(([{ work }, { chapters }]) => {
        setWork(work)
        setChapters(chapters)
      })
      .catch((err) => setError(err.message || 'Failed to load work'))
      .finally(() => setLoading(false))
  }, [workId])

  useEffect(() => { fetchData() }, [fetchData])

  // Reset cover when navigating between works — otherwise the previous
  // work's cover stays displayed since `/work/:workId` reuses this component.
  useEffect(() => { setCover(null) }, [workId])

  const handleCreateChapter = async () => {
    if (!workId || creatingChapter) return; if (!chapterTitle.trim()) { setSubmitError('Title is required'); return }
    setSubmitError(null)
    setCreatingChapter(true)
    try {
      const { chapter } = await api.createChapter(workId, { title: chapterTitle.trim() })
      setShowNewForm(false)
      setChapterTitle('')
      navigate(`/editor/${chapter.id}`)
    } catch (err) {
      setSubmitError((err as Error).message || 'Failed to create chapter')
    } finally {
      setCreatingChapter(false)
    }
  }

  const startEditTitle = () => {
    setTitleDraft(work?.title || '')
    setEditingTitle(true)
  }

  const saveTitle = async () => {
    if (!work || !titleDraft.trim()) { setEditingTitle(false); return }
    const title = titleDraft.trim()
    setEditingTitle(false)
    setWork({ ...work, title })
    try {
      await api.updateWork(work.id, { title })
    } catch {
      // ponytail: no rollback UI for this rare failure path — refetch is the recovery
      fetchData()
    }
  }

  const startEditSynopsis = () => {
    setSynopsisDraft(work?.synopsis || '')
    setEditingSynopsis(true)
  }

  const saveSynopsis = async () => {
    if (!work) { setEditingSynopsis(false); return }
    const synopsis = synopsisDraft.trim()
    setEditingSynopsis(false)
    setWork({ ...work, synopsis })
    try {
      await api.updateWork(work.id, { synopsis })
    } catch {
      fetchData()
    }
  }

  const startRenameChapter = (ch: Chapter) => {
    setRenamingChapterId(ch.id)
    setChapterTitleDraft(ch.title)
  }

  const saveChapterTitle = async (chId: string) => {
    const title = chapterTitleDraft.trim()
    setRenamingChapterId(null)
    if (!title) return
    setChapters((prev) => prev.map((c) => (c.id === chId ? { ...c, title } : c)))
    try {
      await api.updateChapter(chId, { title })
    } catch {
      fetchData()
    }
  }

  if (loading) {
    return <p className={styles.loading}>Loading…</p>
  }

  if (error) {
    return <p className={styles.error}>Error: {error}</p>
  }

  const totalWords = chapters.reduce((sum, ch) => sum + ch.word_count, 0)

  return (
    <div className={styles.layout}>
      <div className={styles.wrap}>
        <button
          className={styles.backBtn}
          onClick={() => work?.universe_id ? navigate(`/universe/${work.universe_id}`) : navigate(-1)}
        >
          ← Back
        </button>

        <div className={styles.headerCard}>
          <div className={styles.coverCol}>
            <ImageUpload
              value={cover}
              onChange={setCover}
              shape="rounded"
              radius={8}
              width={104}
              height={140}
              placeholder="Cover — drag an image"
            />
          </div>
          <div className={styles.headerInfo}>
            <div className={styles.metaRow}>
              <span className={styles.typePill}>{work?.type || 'Untitled'}</span>
              <span className={styles.metaText}>
                {chapters.length} chapter{chapters.length === 1 ? '' : 's'} · {totalWords.toLocaleString()} words
              </span>
            </div>
            {editingTitle ? (
              <input
                className={styles.titleInput}
                value={titleDraft}
                autoFocus
                onChange={(e) => setTitleDraft(e.target.value)}
                onBlur={saveTitle}
                onKeyDown={(e) => e.key === 'Enter' && saveTitle()}
              />
            ) : (
              <div className={styles.titleRow}>
                <h1 className={styles.heading}>{work?.title || 'Untitled Work'}</h1>
                <span
                  role="button"
                  tabIndex={0}
                  aria-label="Edit title"
                  title="Edit title"
                  className={`glyph ${styles.editIcon}`}
                  onClick={startEditTitle}
                  onKeyDown={onActivateKey(startEditTitle)}
                >
                  ✎
                </span>
              </div>
            )}
            {editingSynopsis ? (
              <textarea
                className={styles.synopsisInput}
                value={synopsisDraft}
                autoFocus
                onChange={(e) => setSynopsisDraft(e.target.value)}
                onBlur={saveSynopsis}
              />
            ) : (
              <div className={styles.synopsisRow}>
                <p className={styles.synopsis}>{work?.synopsis || 'No synopsis yet.'}</p>
                <span
                  role="button"
                  tabIndex={0}
                  aria-label="Edit synopsis"
                  title="Edit synopsis"
                  className={`glyph ${styles.editIcon}`}
                  onClick={startEditSynopsis}
                  onKeyDown={onActivateKey(startEditSynopsis)}
                >
                  ✎
                </span>
              </div>
            )}
            {chapters.length > 0 && (
              <button className={styles.openEditorBtn} onClick={() => navigate(`/editor/${chapters[0].id}`)}>
                Open in editor
              </button>
            )}
          </div>
        </div>

        <div className={styles.headerRow}>
          <h2 className={styles.sectionHeading}>Chapters</h2>
          {!showNewForm ? (
            <button className={styles.newBtn} onClick={() => setShowNewForm(true)}>
              + New Chapter
            </button>
          ) : (
            <div className={styles.inlineForm}>
              <input
                className={styles.formInput}
                placeholder="Chapter title"
                value={chapterTitle}
                disabled={creatingChapter}
                onChange={(e) => setChapterTitle(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleCreateChapter()}
              />
              <button className={styles.formSubmit} onClick={handleCreateChapter} disabled={creatingChapter}>Create</button>
              <button className={styles.formCancel} onClick={() => { setShowNewForm(false); setSubmitError(null) }}>Cancel</button>
            </div>
          )}
          {submitError && <p className={styles.formError}>{submitError}</p>}
        </div>
        {chapters.length === 0 ? (
          <p className={styles.empty}>No chapters yet.</p>
        ) : (
          <div className={styles.chapterTable}>
            <div className={styles.tableHeaderRow}>
              <span className={styles.colIndex}>#</span>
              <span className={styles.colTitle}>Chapter</span>
              <span className={styles.colWords}>Words</span>
              <span className={styles.colStatus}>Analysis</span>
              <span className={styles.colRename} />
            </div>
            {chapters
              .sort((a, b) => a.order_index - b.order_index)
              .map((ch, i) => (
                <div key={ch.id} className={styles.tableRow}>
                  <span className={styles.colIndex}>{String(i + 1).padStart(2, '0')}</span>
                  {renamingChapterId === ch.id ? (
                    <input
                      className={styles.colTitle}
                      value={chapterTitleDraft}
                      autoFocus
                      onChange={(e) => setChapterTitleDraft(e.target.value)}
                      onBlur={() => saveChapterTitle(ch.id)}
                      onKeyDown={(e) => e.key === 'Enter' && saveChapterTitle(ch.id)}
                    />
                  ) : (
                    <span
                      role="button"
                      tabIndex={0}
                      className={styles.colTitle}
                      onClick={() => navigate(`/editor/${ch.id}`)}
                      onKeyDown={onActivateKey(() => navigate(`/editor/${ch.id}`))}
                    >
                      {ch.title}
                    </span>
                  )}
                  <span className={styles.colWords}>{ch.word_count > 0 ? `${ch.word_count} words` : 'Empty'}</span>
                  <span className={styles.colStatus} data-status={ch.status}>{ch.status}</span>
                  <span
                    role="button"
                    tabIndex={0}
                    aria-label="Rename chapter"
                    title="Rename chapter"
                    className={`glyph ${styles.colRename}`}
                    onClick={() => startRenameChapter(ch)}
                    onKeyDown={onActivateKey(() => startRenameChapter(ch))}
                  >
                    ✎
                  </span>
                </div>
              ))}
          </div>
        )}
      </div>
    </div>
  )
}
