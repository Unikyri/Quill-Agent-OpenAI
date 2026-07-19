import { type FormEvent, type KeyboardEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { GenreTagPicker } from '../components/genres'
import { useFeedback } from '../components/feedback'
import { api } from '../lib/api'
import { profileMemoryPath } from '../lib/canonicalRoutes'
import { GENRE_OPTIONS } from '../lib/genres'
import styles from './DashboardPage.module.css'

type UniverseSummary = {
  id: string
  name: string
  description?: string
  genre_tags?: string[]
  genre?: string
}

type StatusTone = 'info' | 'success' | 'error'
type HomeStatus = { tone: StatusTone; message: string }

const genreLabels = new Map<string, string>(
  GENRE_OPTIONS.map(({ value, label }) => [value, label] as [string, string]),
)

function writePath(universeId: string) {
  return `/universe/${universeId}/write`
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback
}

function tagsFor(universe: UniverseSummary) {
  if (universe.genre_tags) return universe.genre_tags.filter(Boolean)
  return universe.genre ? [universe.genre] : []
}

function genreName(tag: string) {
  return genreLabels.get(tag) ?? tag
}

function pluralize(count: number, singular: string, plural = `${singular}s`) {
  return count === 1 ? singular : plural
}

export default function DashboardPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { publish, update } = useFeedback()
  const isForcingNew = new URLSearchParams(location.search).get('new') === 'true'

  const [universes, setUniverses] = useState<UniverseSummary[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(isForcingNew)
  const [newUniverseName, setNewUniverseName] = useState('')
  const [newUniverseDescription, setNewUniverseDescription] = useState('')
  const [newUniverseGenres, setNewUniverseGenres] = useState<string[]>([])
  const [createError, setCreateError] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [status, setStatus] = useState<HomeStatus | null>(null)
  const [editingUniverse, setEditingUniverse] = useState<UniverseSummary | null>(null)
  const [editName, setEditName] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [editGenres, setEditGenres] = useState<string[]>([])
  const [editError, setEditError] = useState<string | null>(null)
  const [savingEdit, setSavingEdit] = useState(false)
  const [deletingUniverse, setDeletingUniverse] = useState<UniverseSummary | null>(null)
  const [deleting, setDeleting] = useState(false)
  const dialogRef = useRef<HTMLElement | null>(null)
  const editNameRef = useRef<HTMLInputElement | null>(null)
  const deleteCancelRef = useRef<HTMLButtonElement | null>(null)
  const dialogTriggerRef = useRef<HTMLElement | null>(null)

  const loadUniverses = useCallback(async (showLoader = true) => {
    if (showLoader) setIsLoading(true)
    setLoadError(null)

    try {
      const { universes: result } = await api.listUniverses()
      setUniverses(Array.isArray(result) ? result : [])
      return true
    } catch (error) {
      setLoadError(errorMessage(error, 'We could not load your universe library.'))
      return false
    } finally {
      if (showLoader) setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadUniverses()
  }, [loadUniverses])

  useEffect(() => {
    if (isForcingNew) setShowCreate(true)
  }, [isForcingNew])

  const summary = useMemo(() => {
    const withBrief = universes.filter((universe) => Boolean(universe.description?.trim())).length
    const tagged = universes.filter((universe) => tagsFor(universe).length > 0).length

    return {
      count: universes.length,
      withBrief,
      tagged,
    }
  }, [universes])

  const primaryUniverse = universes[0]

  const closeCreate = () => {
    setShowCreate(false)
    setCreateError(null)
    if (isForcingNew) navigate('/dashboard', { replace: true })
  }

  const restoreDialogTrigger = () => {
    const trigger = dialogTriggerRef.current
    dialogTriggerRef.current = null
    requestAnimationFrame(() => trigger?.focus())
  }

  const closeEdit = () => {
    setEditingUniverse(null)
    setEditError(null)
    restoreDialogTrigger()
  }

  const closeDelete = () => {
    setDeletingUniverse(null)
    setEditError(null)
    restoreDialogTrigger()
  }

  const openEdit = (universe: UniverseSummary, trigger: HTMLElement) => {
    dialogTriggerRef.current = trigger
    setEditingUniverse(universe)
    setEditName(universe.name)
    setEditDescription(universe.description || '')
    setEditGenres(tagsFor(universe))
    setEditError(null)
  }

  const openDelete = (universe: UniverseSummary, trigger: HTMLElement) => {
    dialogTriggerRef.current = trigger
    setDeletingUniverse(universe)
    setEditError(null)
  }

  const handleDialogKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Escape') {
      if (savingEdit || deleting) return
      if (editingUniverse) closeEdit()
      if (deletingUniverse) closeDelete()
      return
    }

    if (event.key !== 'Tab') return

    const focusable = dialogRef.current?.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    )
    if (!focusable?.length) return

    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  useEffect(() => {
    if (editingUniverse) editNameRef.current?.focus()
  }, [editingUniverse])

  useEffect(() => {
    if (deletingUniverse) deleteCancelRef.current?.focus()
  }, [deletingUniverse])

  const saveEdit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!editingUniverse || !editName.trim()) return
    setSavingEdit(true); setEditError(null)
    try {
      const { universe } = await api.updateUniverse(editingUniverse.id, { name: editName.trim(), description: editDescription.trim(), genre_tags: editGenres })
      setUniverses((current) => current.map((item) => item.id === editingUniverse.id ? universe as UniverseSummary : item))
      closeEdit()
      publish({ scope: 'home', status: 'completed', message: 'Universe details saved.' })
    } catch (error) {
      setEditError(errorMessage(error, 'We could not save this universe. Try again.'))
    } finally { setSavingEdit(false) }
  }

  const confirmDelete = async () => {
    if (!deletingUniverse || deleting) return
    setDeleting(true)
    try {
      await api.deleteUniverse(deletingUniverse.id)
      setUniverses((current) => current.filter((item) => item.id !== deletingUniverse.id))
      setDeletingUniverse(null)
      dialogTriggerRef.current = null
      publish({ scope: 'home', status: 'completed', message: 'Universe deleted.' })
    } catch (error) {
      setEditError(errorMessage(error, 'We could not delete this universe. Try again.'))
    } finally { setDeleting(false) }
  }

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const name = newUniverseName.trim()
    if (!name) {
      setCreateError('Give your universe a name before creating it.')
      return
    }

    const feedbackId = publish({
      scope: 'home',
      status: 'running',
      message: 'Creating your universe…',
    })
    setCreateError(null)
    setStatus({ tone: 'info', message: 'Creating your universe…' })
    setIsCreating(true)

    try {
      const { universe } = await api.createUniverse({
        name,
        description: newUniverseDescription.trim(),
        genre_tags: newUniverseGenres,
      })
      const createdUniverse = universe as UniverseSummary
      setUniverses((current) => [
        createdUniverse,
        ...current.filter((existing) => existing.id !== createdUniverse.id),
      ])
      setNewUniverseName('')
      setNewUniverseDescription('')
      setNewUniverseGenres([])
      setShowCreate(false)
      const message = `${createdUniverse.name} is ready. Continue writing when you are.`
      setStatus({ tone: 'success', message })
      update(feedbackId, { status: 'completed', message })
    } catch (error) {
      const message = errorMessage(error, 'We could not create that universe. Please try again.')
      setCreateError(message)
      setStatus({ tone: 'error', message })
      update(feedbackId, { status: 'failed', message })
    } finally {
      setIsCreating(false)
    }
  }

  return (
    <main className={styles.layout}>
      <nav className={styles.accountNav} aria-label="Account">
        <Link className={styles.accountLink} to={profileMemoryPath()}>Writer profile</Link>
      </nav>

      <section className={styles.hero} aria-labelledby="home-title">
        <div>
          <p className={styles.eyebrow}>Home</p>
          <h1 id="home-title" className={styles.title}>Your writing worlds</h1>
          <p className={styles.intro}>
            Pick up a story, make a new home for an idea, or enter the guided demo.
          </p>
        </div>
        <div className={styles.heroActions}>
          {primaryUniverse && (
            <button
              type="button"
              className={styles.primaryButton}
              onClick={() => navigate(writePath(primaryUniverse.id))}
            >
              Continue writing
              <span className={styles.buttonDetail}>{primaryUniverse.name}</span>
            </button>
          )}
          {!showCreate && (
            <button type="button" className={styles.secondaryButton} onClick={() => setShowCreate(true)}>
              Create universe
            </button>
          )}
        </div>
      </section>

      {status && (
        <p
          className={`${styles.status} ${styles[`status${status.tone[0].toUpperCase()}${status.tone.slice(1)}`]}`}
          role={status.tone === 'error' ? 'alert' : 'status'}
        >
          {status.message}
        </p>
      )}

      {showCreate && (
        <section className={styles.createPanel} aria-labelledby="create-universe-title">
          <div className={styles.panelHeading}>
            <div>
              <p className={styles.eyebrow}>New universe</p>
              <h2 id="create-universe-title">Start with the shape of your story</h2>
              <p>A name is enough to begin. Add a brief or genres when they help.</p>
            </div>
            <button type="button" className={styles.textButton} onClick={closeCreate} disabled={isCreating}>
              Back to library
            </button>
          </div>

          <form className={styles.createForm} onSubmit={handleCreate}>
            <label className={styles.field} htmlFor="universe-name">
              <span>Name</span>
              <input
                id="universe-name"
                name="name"
                placeholder="e.g. The Farthest Shore"
                value={newUniverseName}
                onChange={(event) => setNewUniverseName(event.target.value)}
                autoFocus
                disabled={isCreating}
                required
              />
            </label>

            <label className={styles.field} htmlFor="universe-description">
              <span>Story brief <em>Optional</em></span>
              <textarea
                id="universe-description"
                name="description"
                placeholder="What makes this world worth returning to?"
                value={newUniverseDescription}
                onChange={(event) => setNewUniverseDescription(event.target.value)}
                disabled={isCreating}
                rows={3}
              />
            </label>

            <GenreTagPicker
              id="universe-genres"
              label="Genres"
              value={newUniverseGenres}
              onChange={setNewUniverseGenres}
              disabled={isCreating}
            />

            {createError && <p className={styles.formError} role="alert">{createError}</p>}

            <div className={styles.formActions}>
              <button type="submit" className={styles.primaryButton} disabled={isCreating || !newUniverseName.trim()}>
                {isCreating ? 'Creating universe…' : createError ? 'Try again' : 'Create universe'}
              </button>
              <button type="button" className={styles.secondaryButton} onClick={closeCreate} disabled={isCreating}>
                Cancel
              </button>
            </div>
          </form>
        </section>
      )}

      <section className={styles.overview} aria-labelledby="library-title">
        <div className={styles.libraryHeading}>
          <div>
            <p className={styles.eyebrow}>Library</p>
            <h2 id="library-title">Your universes</h2>
          </div>
          {!isLoading && !loadError && summary.count > 0 && (
            <p className={styles.summary}>
              {summary.count} {pluralize(summary.count, 'universe')} · {summary.withBrief} {pluralize(summary.withBrief, 'story brief', 'story briefs')} · {summary.tagged} tagged
            </p>
          )}
        </div>

        {loadError && (
          <div className={styles.errorPanel} role="alert">
            <div>
              <h3>We could not load your library</h3>
              <p>{loadError}</p>
            </div>
            <button type="button" className={styles.secondaryButton} onClick={() => void loadUniverses()}>
              Retry
            </button>
          </div>
        )}

        {isLoading && universes.length === 0 && (
          <div className={styles.skeletonGrid} role="status" aria-label="Loading your universe library" aria-busy="true">
            <div className={styles.skeletonCard} />
            <div className={styles.skeletonCard} />
            <div className={styles.skeletonCard} />
          </div>
        )}

        {!isLoading && universes.length === 0 && !loadError && (
          <div className={styles.emptyState}>
            <p className={styles.eyebrow}>A blank shelf</p>
            <h3>No universes yet</h3>
            <p>Create one when you are ready, or use the guided demo to see Quill with a working story.</p>
            <button type="button" className={styles.primaryButton} onClick={() => setShowCreate(true)}>
              Create your first universe
            </button>
          </div>
        )}

        {universes.length > 0 && (
          <div className={styles.universeGrid}>
            {universes.map((universe) => {
              const tags = tagsFor(universe)
              return (
                <article className={styles.universeCard} key={universe.id}>
                  <div className={styles.cardHeading}>
                    <p className={styles.cardLabel}>Universe</p>
                    <h3>{universe.name}</h3>
                  </div>
                  <p className={styles.description}>
                    {universe.description?.trim() || 'No story brief yet — begin where the idea is clearest.'}
                  </p>
                  <div className={styles.genreList} aria-label={`Genres for ${universe.name}`}>
                    {tags.length > 0 ? tags.map((tag) => <span className={styles.genreTag} key={tag}>{genreName(tag)}</span>) : (
                      <span className={styles.noGenre}>No genres tagged</span>
                    )}
                  </div>
                  <div className={styles.cardFooter}>
                    <span className={styles.cardSignal}>
                      {universe.description?.trim() ? 'Story brief added' : 'Story brief open'}
                    </span>
                    <button type="button" className={styles.cardButton} onClick={() => navigate(writePath(universe.id))}>
                      Open writing
                    </button>
                    <button type="button" className={styles.cardButton} aria-label={`Edit ${universe.name}`} onClick={(event) => openEdit(universe, event.currentTarget)}>Edit</button>
                    <button type="button" className={styles.deleteButton} aria-label={`Delete ${universe.name}`} onClick={(event) => openDelete(universe, event.currentTarget)}>Delete</button>
                  </div>
                </article>
              )
            })}
          </div>
        )}
      </section>

      {editingUniverse && (
        <div className={styles.dialogBackdrop} role="presentation">
          <section ref={dialogRef} className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="edit-universe-title" onKeyDown={handleDialogKeyDown}>
            <div className={styles.panelHeading}><div><p className={styles.eyebrow}>Universe settings</p><h2 id="edit-universe-title">Edit {editingUniverse.name}</h2></div><button className={styles.textButton} type="button" aria-label="Close edit universe dialog" disabled={savingEdit} onClick={closeEdit}>Close</button></div>
            <form className={styles.createForm} onSubmit={saveEdit}>
              <label className={styles.field} htmlFor="edit-universe-name"> <span>Name</span><input ref={editNameRef} id="edit-universe-name" value={editName} onChange={(event) => setEditName(event.target.value)} required disabled={savingEdit} /></label>
              <label className={styles.field}> <span>Story brief <em>Optional</em></span><textarea rows={3} value={editDescription} onChange={(event) => setEditDescription(event.target.value)} disabled={savingEdit} /></label>
              <GenreTagPicker id="edit-universe-genres" label="Genres" value={editGenres} onChange={setEditGenres} disabled={savingEdit} />
              {editError && <p className={styles.formError} role="alert">{editError}</p>}
              <div className={styles.formActions}><button type="submit" className={styles.primaryButton} disabled={savingEdit || !editName.trim()}>{savingEdit ? 'Saving…' : 'Save changes'}</button><button type="button" className={styles.secondaryButton} disabled={savingEdit} onClick={closeEdit}>Cancel</button></div>
            </form>
          </section>
        </div>
      )}

      {deletingUniverse && (
        <div className={styles.dialogBackdrop} role="presentation">
          <section ref={dialogRef} className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="delete-universe-title" aria-describedby="delete-universe-description" onKeyDown={handleDialogKeyDown}>
            <p className={styles.eyebrow}>Permanent action</p><h2 id="delete-universe-title">Delete {deletingUniverse.name}?</h2>
            <p id="delete-universe-description">This removes its works, chapters, and stored story memory. This cannot be undone.</p>
            {editError && <p className={styles.formError} role="alert">{editError}</p>}
            <div className={styles.formActions}><button type="button" className={styles.deleteButton} disabled={deleting} onClick={() => void confirmDelete()}>{deleting ? 'Deleting…' : 'Delete universe'}</button><button ref={deleteCancelRef} type="button" className={styles.secondaryButton} disabled={deleting} onClick={closeDelete}>Cancel</button></div>
          </section>
        </div>
      )}
    </main>
  )
}
