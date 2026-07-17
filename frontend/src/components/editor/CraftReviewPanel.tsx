import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { CraftReviewNote, CraftReviewResult } from '../../lib/types'
import styles from './CraftReviewPanel.module.css'

interface CraftReviewPanelProps {
  review: CraftReviewResult | null
  loading?: boolean
  universeId: string
  chapterId: string
  workId: string
}

type FeedbackSignal = 'accept' | 'reject'

function noteId(note: CraftReviewNote): string {
  return note.id
}

export default function CraftReviewPanel({ review, loading = false, universeId, chapterId, workId }: CraftReviewPanelProps) {
  const [decisions, setDecisions] = useState<Record<string, FeedbackSignal>>({})
  const [pending, setPending] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setDecisions({})
    setError(null)
  }, [review])

  const sendFeedback = async (note: CraftReviewNote, signal: FeedbackSignal) => {
    const id = noteId(note)
    if (pending || decisions[id]) return
    setPending(id)
    setError(null)
    try {
      await api.submitWriterFeedback({
        signal,
        universe_id: universeId,
        work_id: workId,
        chapter_id: chapterId,
        note_id: id,
        payload: {
          category: note.category || note.skill,
          skill: note.skill,
          quote: note.quote,
        },
      })
      setDecisions((current) => ({ ...current, [id]: signal }))
    } catch (feedbackError) {
      setError((feedbackError as Error).message || 'Could not save feedback')
    } finally {
      setPending(null)
    }
  }

  return (
    <section className={styles.panel} aria-label="Craft review">
      <div className={styles.header}>
        <div>
          <span className={styles.kicker}>Selected craft</span>
          <h3 className={styles.title}>Margin review</h3>
        </div>
        {loading && <span className={styles.loading} role="status">Reviewing…</span>}
      </div>

      {!review && !loading && (
        <p className={styles.empty}>Select a passage in the editor, then use <strong>Review selection</strong>. Quill will keep the review anchored to that exact range.</p>
      )}

      {review && (
        <div className={styles.body}>
          <div className={styles.selectionBlock}>
            <span className={styles.sectionLabel}>Skills used</span>
            <div className={styles.skillList}>
              {review.selections.length === 0 ? (
                <span className={styles.muted}>No active skill selected.</span>
              ) : review.selections.map((selection) => (
                <div key={selection.skill} className={styles.skill}>
                  <span className={styles.skillName}>{selection.skill}</span>
                  {selection.rationale && <span className={styles.rationale}>{selection.rationale}</span>}
                </div>
              ))}
            </div>
          </div>

          <div className={styles.notesBlock}>
            <span className={styles.sectionLabel}>Notes</span>
            {review.notes.length === 0 ? (
              <p className={styles.muted}>No notes survived the active preferences and suppression rules.</p>
            ) : review.notes.map((note) => {
              const id = noteId(note)
              const decision = decisions[id]
              return (
                <article key={id} className={styles.note} data-severity={note.severity}>
                  <div className={styles.noteMeta}>
                    <span className={styles.noteSkill}>{note.skill}</span>
                    <span className={styles.severity}>{note.severity}</span>
                  </div>
                  <blockquote className={styles.quote}>{note.quote}</blockquote>
                  <p className={styles.noteText}>{note.note}</p>
                  {decision ? (
                    <span className={styles.decision} data-decision={decision}>
                      {decision === 'accept' ? 'Saved as helpful' : 'Dismissed'}
                    </span>
                  ) : (
                    <div className={styles.actions}>
                      <button
                        type="button"
                        className={styles.acceptButton}
                        disabled={pending !== null}
                        onClick={() => sendFeedback(note, 'accept')}
                      >
                        Accept
                      </button>
                      <button
                        type="button"
                        className={styles.dismissButton}
                        disabled={pending !== null}
                        onClick={() => sendFeedback(note, 'reject')}
                      >
                        Dismiss
                      </button>
                    </div>
                  )}
                </article>
              )
            })}
          </div>
          {error && <p className={styles.error} role="alert">{error}</p>}
        </div>
      )}
    </section>
  )
}
