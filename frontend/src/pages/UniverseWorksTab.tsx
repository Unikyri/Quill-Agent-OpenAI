import { useCallback, useContext, useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useFeedback } from '../components/feedback'
import ImageUpload from '../components/shared/ImageUpload'
import { UniverseContext } from '../contexts/UniverseContext'
import { WORK_FORMAT_OPTIONS } from '../lib/genres'
import { api } from '../lib/api'
import { IngestPanel } from '../components/editor/IngestPanel'
import { writeImportPath, writePath } from './writeRoutes'
import styles from './UniverseWorksTab.module.css'

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

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message.trim() ? error.message : fallback
}

function relativeTime(iso?: string) {
  if (!iso) return '—'
  const difference = Date.now() - new Date(iso).getTime()
  const hours = Math.floor(difference / 3_600_000)
  if (hours < 1) return `${Math.max(1, Math.floor(difference / 60_000))} min ago`
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function StatusChip({ status }: { status: string }) {
  const normalized = status.toLowerCase()
  if (normalized === 'analyzed') return <span className={styles.statusChip} data-s="analyzed">Analyzed</span>
  if (normalized === 'analyzing' || normalized === 'pending') return <span className={styles.statusChip} data-s="analyzing">Analyzing</span>
  if (normalized === 'contradiction') return <span className={styles.statusChip} data-s="contradiction">Needs review</span>
  if (normalized === 'error') return <span className={styles.statusChip} data-s="error">Analysis failed</span>
  return <span className={styles.statusChip} data-s="">Draft</span>
}

function InlineConfirmation({
  message,
  onCancel,
  onConfirm,
  confirming,
}: {
  message: string
  onCancel: () => void
  onConfirm: () => void
  confirming?: boolean
}) {
  return (
    <div className={styles.confirmation} role="alertdialog" aria-label="Confirm deletion">
      <p>{message}</p>
      <div className={styles.confirmationActions}>
        <button type="button" className={styles.formCancel} onClick={onCancel} disabled={confirming}>Cancel</button>
        <button type="button" className={styles.dangerButton} onClick={onConfirm} disabled={confirming}>
          {confirming ? 'Deleting…' : 'Delete'}
        </button>
      </div>
    </div>
  )
}

function WorkDetail({
  workId,
  universeId,
  onBack,
  onOpenImport,
}: {
  workId: string
  universeId: string
  onBack: () => void
  onOpenImport: () => void
}) {
  const navigate = useNavigate()
  const { publish } = useFeedback()
  const [work, setWork] = useState<Work | null>(null)
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [cover, setCover] = useState<string | null>(null)
  const [showNewForm, setShowNewForm] = useState(false)
  const [chapterTitle, setChapterTitle] = useState('')
  const [creatingChapter, setCreatingChapter] = useState(false)
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState('')
  const [editingSynopsis, setEditingSynopsis] = useState(false)
  const [synopsisDraft, setSynopsisDraft] = useState('')
  const [renamingChapterId, setRenamingChapterId] = useState<string | null>(null)
  const [chapterTitleDraft, setChapterTitleDraft] = useState('')
  const [pendingChapterDelete, setPendingChapterDelete] = useState<Chapter | null>(null)
  const [deletingChapter, setDeletingChapter] = useState(false)

  const fetchData = useCallback(async () => {
    if (!workId) return
    setLoading(true)
    setError(null)
    try {
      const [{ work: nextWork }, { chapters: nextChapters }] = await Promise.all([
        api.getWork(workId),
        api.listChapters(workId),
      ])
      setWork(nextWork)
      setChapters(nextChapters || [])
    } catch (requestError) {
      const message = errorMessage(requestError, 'We could not load this manuscript.')
      setError(message)
      publish({ scope: 'write', status: 'failed', message })
    } finally {
      setLoading(false)
    }
  }, [publish, workId])

  useEffect(() => {
    void fetchData()
    setCover(null)
  }, [fetchData, workId])

  const handleCreateChapter = async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault()
    if (!workId || creatingChapter) return
    const title = chapterTitle.trim()
    if (!title) {
      setActionError('Give the chapter a title before creating it.')
      return
    }

    setActionError(null)
    setCreatingChapter(true)
    try {
      const { chapter } = await api.createChapter(workId, { title })
      publish({ scope: 'write', status: 'completed', message: `Created ${chapter.title || title}.` })
      navigate(writePath(universeId, chapter.id))
    } catch (requestError) {
      const message = errorMessage(requestError, 'We could not create that chapter.')
      setActionError(message)
      publish({ scope: 'write', status: 'failed', message })
    } finally {
      setCreatingChapter(false)
    }
  }

  const saveWork = async (patch: Record<string, string>, nextWork: Work, fallback: string) => {
    if (!work) return
    const previous = work
    setActionError(null)
    setWork(nextWork)
    try {
      await api.updateWork(work.id, patch)
      publish({ scope: 'write', status: 'completed', message: 'Manuscript details saved.' })
    } catch (requestError) {
      const message = errorMessage(requestError, fallback)
      setWork(previous)
      setActionError(message)
      publish({ scope: 'write', status: 'failed', message })
    }
  }

  const saveTitle = async () => {
    if (!work) return
    const title = titleDraft.trim()
    setEditingTitle(false)
    if (!title || title === work.title) return
    await saveWork({ title }, { ...work, title }, 'We could not rename this manuscript.')
  }

  const saveSynopsis = async () => {
    if (!work) return
    const synopsis = synopsisDraft.trim()
    setEditingSynopsis(false)
    if (synopsis === (work.synopsis || '')) return
    await saveWork({ synopsis }, { ...work, synopsis }, 'We could not update the synopsis.')
  }

  const saveType = async (type: string) => {
    if (!work || type === work.type) return
    await saveWork({ type }, { ...work, type }, 'We could not update the manuscript format.')
  }

  const saveChapterTitle = async (chapterId: string) => {
    const title = chapterTitleDraft.trim()
    const current = chapters.find((chapter) => chapter.id === chapterId)
    setRenamingChapterId(null)
    if (!current || !title || title === current.title) return

    setActionError(null)
    setChapters((previous) => previous.map((chapter) => chapter.id === chapterId ? { ...chapter, title } : chapter))
    try {
      await api.updateChapter(chapterId, { title })
      publish({ scope: 'write', status: 'completed', message: 'Chapter renamed.' })
    } catch (requestError) {
      const message = errorMessage(requestError, 'We could not rename that chapter.')
      setActionError(message)
      setChapters((previous) => previous.map((chapter) => chapter.id === chapterId ? current : chapter))
      publish({ scope: 'write', status: 'failed', message })
    }
  }

  const confirmDeleteChapter = async () => {
    if (!pendingChapterDelete || deletingChapter) return
    const chapter = pendingChapterDelete
    setDeletingChapter(true)
    setActionError(null)
    try {
      await api.deleteChapter(chapter.id)
      setChapters((previous) => previous.filter((item) => item.id !== chapter.id))
      setPendingChapterDelete(null)
      publish({ scope: 'write', status: 'completed', message: `${chapter.title} was deleted.` })
    } catch (requestError) {
      const message = errorMessage(requestError, 'We could not delete that chapter.')
      setActionError(message)
      publish({ scope: 'write', status: 'failed', message })
    } finally {
      setDeletingChapter(false)
    }
  }

  if (loading) return <div className={styles.loading} role="status">Loading manuscript…</div>
  if (error) {
    return (
      <div className={styles.error} role="alert">
        <p>{error}</p>
        <button type="button" className={styles.newBtn} onClick={() => void fetchData()}>Retry</button>
      </div>
    )
  }

  const sorted = [...chapters].sort((left, right) => left.order_index - right.order_index)
  const totalWords = chapters.reduce((total, chapter) => total + chapter.word_count, 0)

  return (
    <div className={styles.wrap}>
      <div className={styles.writeToolbar}>
        <button type="button" className={styles.backBtn} onClick={onBack}>All manuscripts</button>
        <button type="button" className={styles.newBtn} onClick={onOpenImport}>Import manuscript</button>
      </div>

      {actionError && <div className={styles.inlineError} role="alert">{actionError}</div>}

      <div className={styles.workHeaderCard}>
        <div className={styles.coverCol}>
          <ImageUpload value={cover} onChange={setCover} shape="rounded" radius={8} width={104} height={140} placeholder="Upload cover" />
        </div>
        <div className={styles.headerInfo}>
          <div className={styles.metaRow}>
            <select className={styles.typePill} value={work?.type || 'novel'} onChange={(event) => void saveType(event.target.value)} aria-label="Work format">
              {WORK_FORMAT_OPTIONS.map((format) => <option key={format.value} value={format.value}>{format.label}</option>)}
            </select>
            <span className={styles.metaText}>{chapters.length} chapter{chapters.length === 1 ? '' : 's'} · {totalWords.toLocaleString()} words</span>
          </div>

          {editingTitle ? (
            <input
              className={styles.titleInput}
              value={titleDraft}
              autoFocus
              onChange={(event) => setTitleDraft(event.target.value)}
              onBlur={() => void saveTitle()}
              onKeyDown={(event) => { if (event.key === 'Enter') void saveTitle() }}
            />
          ) : (
            <div className={styles.titleRow}>
              <h1 className={styles.heading}>{work?.title || 'Untitled manuscript'}</h1>
              <button type="button" className={styles.editIcon} aria-label="Edit title" onClick={() => { setTitleDraft(work?.title || ''); setEditingTitle(true) }}>Edit</button>
            </div>
          )}

          {editingSynopsis ? (
            <textarea
              className={styles.synopsisInput}
              value={synopsisDraft}
              autoFocus
              onChange={(event) => setSynopsisDraft(event.target.value)}
              onBlur={() => void saveSynopsis()}
            />
          ) : (
            <div className={styles.synopsisRow}>
              <p className={styles.synopsis}>{work?.synopsis || 'Add a synopsis to keep the manuscript oriented.'}</p>
              <button type="button" className={styles.editIcon} aria-label="Edit synopsis" onClick={() => { setSynopsisDraft(work?.synopsis || ''); setEditingSynopsis(true) }}>Edit</button>
            </div>
          )}

          {sorted.length > 0 && (
            <button type="button" className={styles.openEditorBtn} onClick={() => navigate(writePath(universeId, sorted[0].id))}>Continue writing</button>
          )}
        </div>
      </div>

      <section className={styles.chaptersSection} aria-labelledby="chapters-heading">
        <div className={styles.chaptersSectionHeader}>
          <h2 id="chapters-heading" className={styles.sectionHeading}>Chapters</h2>
          {!showNewForm && <button type="button" className={styles.newBtn} onClick={() => setShowNewForm(true)}>New chapter</button>}
        </div>

        {showNewForm && (
          <form className={styles.inlineFormRow} onSubmit={handleCreateChapter}>
            <input
              className={styles.formInput}
              placeholder="Chapter title"
              value={chapterTitle}
              disabled={creatingChapter}
              autoFocus
              onChange={(event) => setChapterTitle(event.target.value)}
            />
            <button type="submit" className={styles.formSubmit} disabled={creatingChapter}>{creatingChapter ? 'Creating…' : 'Create'}</button>
            <button type="button" className={styles.formCancel} onClick={() => { setShowNewForm(false); setActionError(null) }} disabled={creatingChapter}>Cancel</button>
          </form>
        )}

        {pendingChapterDelete && (
          <InlineConfirmation
            message={`Delete “${pendingChapterDelete.title}”? This cannot be undone.`}
            onCancel={() => setPendingChapterDelete(null)}
            onConfirm={() => void confirmDeleteChapter()}
            confirming={deletingChapter}
          />
        )}

        {sorted.length === 0 ? (
          <div className={styles.emptyState}>
            <p>No chapters yet. Start writing from a blank chapter, or import a manuscript.</p>
            <div className={styles.emptyActions}>
              <button type="button" className={styles.formSubmit} onClick={() => setShowNewForm(true)}>Create chapter</button>
              <button type="button" className={styles.formCancel} onClick={onOpenImport}>Import manuscript</button>
            </div>
          </div>
        ) : (
          <>
            <div className={styles.tableHeaderRow} aria-hidden="true">
              <span className={styles.tableHeaderCell}>#</span>
              <span className={styles.tableHeaderCell}>Chapter</span>
              <span className={styles.tableHeaderCell}>Words</span>
              <span className={styles.tableHeaderCell}>Analysis</span>
              <span className={styles.tableHeaderCell}>Edited</span>
              <span />
            </div>
            {sorted.map((chapter, index) => (
              <div key={chapter.id} className={styles.tableRow}>
                <span className={styles.colIndex}>{String(index + 1).padStart(2, '0')}</span>
                {renamingChapterId === chapter.id ? (
                  <input
                    className={styles.colTitleInput}
                    value={chapterTitleDraft}
                    autoFocus
                    onChange={(event) => setChapterTitleDraft(event.target.value)}
                    onBlur={() => void saveChapterTitle(chapter.id)}
                    onKeyDown={(event) => { if (event.key === 'Enter') void saveChapterTitle(chapter.id) }}
                  />
                ) : (
                  <button type="button" className={styles.colTitle} onClick={() => navigate(writePath(universeId, chapter.id))}>{chapter.title}</button>
                )}
                <span className={styles.colWords}>{chapter.word_count > 0 ? chapter.word_count.toLocaleString() : '—'}</span>
                <span className={styles.colStatus}><StatusChip status={chapter.status} /></span>
                <span className={styles.colEdited}>{relativeTime(chapter.updated_at)}</span>
                <span className={styles.rowActions}>
                  <button type="button" className={styles.colRenameBtn} aria-label={`Rename ${chapter.title}`} onClick={() => { setRenamingChapterId(chapter.id); setChapterTitleDraft(chapter.title) }}>Rename</button>
                  <button type="button" className={styles.deleteButton} aria-label={`Delete ${chapter.title}`} onClick={() => setPendingChapterDelete(chapter)}>Delete</button>
                </span>
              </div>
            ))}
          </>
        )}
      </section>
    </div>
  )
}

export default function UniverseWorksTab() {
  const { universeId } = useParams<{ universeId: string }>()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { publish } = useFeedback()
  const { works, universe, refetchWorks } = useContext(UniverseContext)
  const [selectedWorkId, setSelectedWorkId] = useState<string | null>(null)
  const [showNewForm, setShowNewForm] = useState(false)
  const [title, setTitle] = useState('')
  const [type, setType] = useState('novel')
  const [synopsis, setSynopsis] = useState('')
  const [createError, setCreateError] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [pendingWorkDelete, setPendingWorkDelete] = useState<{ id: string; title: string } | null>(null)
  const [deletingWork, setDeletingWork] = useState(false)
  const [workActionError, setWorkActionError] = useState<string | null>(null)

  const importOpen = searchParams.get('panel') === 'import'
  const openImport = () => {
    if (universeId) navigate(writeImportPath(universeId))
  }
  const closeImport = () => {
    if (universeId) navigate(writePath(universeId), { replace: true })
  }

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!universe) return
    const nextTitle = title.trim()
    if (!nextTitle) {
      setCreateError('Give the manuscript a title before creating it.')
      return
    }

    setCreateError(null)
    setIsCreating(true)
    try {
      const { work } = await api.createWork(universe.id, { title: nextTitle, type, synopsis: synopsis.trim() })
      await refetchWorks()
      publish({ scope: 'write', status: 'completed', message: `${work.title || nextTitle} is ready for chapters.` })
      setShowNewForm(false)
      setTitle('')
      setType('novel')
      setSynopsis('')
      setSelectedWorkId(work.id)
    } catch (requestError) {
      const message = errorMessage(requestError, 'We could not create that manuscript.')
      setCreateError(message)
      publish({ scope: 'write', status: 'failed', message })
    } finally {
      setIsCreating(false)
    }
  }

  const confirmDeleteWork = async () => {
    if (!pendingWorkDelete || deletingWork) return
    const work = pendingWorkDelete
    setDeletingWork(true)
    setWorkActionError(null)
    try {
      await api.deleteWork(work.id)
      await refetchWorks()
      if (selectedWorkId === work.id) setSelectedWorkId(null)
      setPendingWorkDelete(null)
      publish({ scope: 'write', status: 'completed', message: `${work.title} was deleted.` })
    } catch (requestError) {
      const message = errorMessage(requestError, 'We could not delete that manuscript.')
      setWorkActionError(message)
      publish({ scope: 'write', status: 'failed', message })
    } finally {
      setDeletingWork(false)
    }
  }

  const handleIngestionComplete = useCallback(async (workId: string) => {
    if (!universeId) return
    await refetchWorks()
    const { chapters } = await api.listChapters(workId)
    const sorted = [...(chapters || [])].sort((left, right) => left.order_index - right.order_index)
    const destination = sorted[sorted.length - 1]
    if (destination) {
      navigate(writePath(universeId, destination.id))
      return
    }
    setSelectedWorkId(workId)
    closeImport()
  }, [navigate, refetchWorks, universeId])

  if (!universeId) return null

  if (selectedWorkId) {
    return (
      <>
        {importOpen && <div className={styles.importSurface}><IngestPanel universeId={universeId} onClose={closeImport} onCompleted={handleIngestionComplete} /></div>}
        <WorkDetail workId={selectedWorkId} universeId={universeId} onBack={() => setSelectedWorkId(null)} onOpenImport={openImport} />
      </>
    )
  }

  return (
    <div className={styles.wrap}>
      {importOpen && <div className={styles.importSurface}><IngestPanel universeId={universeId} onClose={closeImport} onCompleted={handleIngestionComplete} /></div>}
      <div className={styles.sectionHeaderRow}>
        <div>
          <h1 className={styles.sectionTitle}>Write</h1>
          <p className={styles.sectionIntro}>Choose a manuscript, create a chapter, or import an existing draft.</p>
        </div>
        <div className={styles.headerActions}>
          <button type="button" className={styles.newBtn} onClick={openImport}>Import manuscript</button>
          {!showNewForm && <button type="button" className={styles.formSubmit} onClick={() => setShowNewForm(true)}>New manuscript</button>}
        </div>
      </div>

      {workActionError && <div className={styles.inlineError} role="alert">{workActionError}</div>}

      {showNewForm && (
        <form className={styles.newWorkForm} onSubmit={handleCreate}>
          <div className={styles.newWorkFormRow}>
            <input className={styles.formInput} placeholder="Manuscript title" value={title} autoFocus disabled={isCreating} onChange={(event) => setTitle(event.target.value)} />
            <select className={styles.formSelect} value={type} disabled={isCreating} onChange={(event) => setType(event.target.value)}>
              {WORK_FORMAT_OPTIONS.map((format) => <option key={format.value} value={format.value}>{format.label}</option>)}
            </select>
          </div>
          <input className={styles.formInput} placeholder="Synopsis (optional)" value={synopsis} disabled={isCreating} onChange={(event) => setSynopsis(event.target.value)} />
          <div className={styles.newWorkFormRow}>
            <button type="submit" className={styles.formSubmit} disabled={isCreating}>{isCreating ? 'Creating…' : 'Create manuscript'}</button>
            <button type="button" className={styles.formCancel} disabled={isCreating} onClick={() => { setShowNewForm(false); setCreateError(null) }}>Cancel</button>
          </div>
          {createError && <p className={styles.formError} role="alert">{createError}</p>}
        </form>
      )}

      {pendingWorkDelete && (
        <InlineConfirmation
          message={`Delete “${pendingWorkDelete.title}” and all of its chapters? This cannot be undone.`}
          onCancel={() => setPendingWorkDelete(null)}
          onConfirm={() => void confirmDeleteWork()}
          confirming={deletingWork}
        />
      )}

      {works.length === 0 ? (
        <section className={styles.emptyState} aria-label="No manuscripts yet">
          <h2>Start your story</h2>
          <p>Create a manuscript for a blank chapter, or bring in material you already have.</p>
          <div className={styles.emptyActions}>
            <button type="button" className={styles.formSubmit} onClick={() => setShowNewForm(true)}>New manuscript</button>
            <button type="button" className={styles.formCancel} onClick={openImport}>Import manuscript</button>
          </div>
        </section>
      ) : (
        <div className={styles.worksGrid} aria-label="Manuscripts">
          {works.map((work) => (
            <article key={work.id} className={styles.workCard}>
              <button type="button" className={styles.workOpenButton} aria-label={`Open ${work.title}`} onClick={() => setSelectedWorkId(work.id)}>
                <span className={styles.workCardType}>{work.type}</span>
                <span className={styles.workCardTitle}>{work.title}</span>
                <span className={styles.workCardAction}>Open chapters</span>
              </button>
              <button type="button" className={styles.deleteButton} aria-label={`Delete ${work.title}`} onClick={() => setPendingWorkDelete(work)}>Delete</button>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}
