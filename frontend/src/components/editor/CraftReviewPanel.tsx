import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { CraftReviewNote, CraftReviewResult, SkillCatalogueItem } from '../../lib/types'
import { displaySkillName, shortDescription } from '../../lib/skillDisplay'
import styles from './CraftReviewPanel.module.css'

interface CraftReviewPanelProps {
  review: CraftReviewResult | null
  loading?: boolean
  universeId: string
  chapterId: string
  workId: string
  selectedSkills?: string[] | null
  onSelectedSkillsChange?: (skills: string[] | null) => void
}

type FeedbackSignal = 'accept' | 'reject'

function noteId(note: CraftReviewNote): string {
  return note.id
}

export default function CraftReviewPanel({ review, loading = false, universeId, chapterId, workId, selectedSkills = null, onSelectedSkillsChange }: CraftReviewPanelProps) {
  const [decisions, setDecisions] = useState<Record<string, FeedbackSignal>>({})
  const [pending, setPending] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [catalogue, setCatalogue] = useState<SkillCatalogueItem[]>([])
  const [activeNames, setActiveNames] = useState<string[]>([])
  const [skillLoadError, setSkillLoadError] = useState<string | null>(null)
  // Once the writer has configured a fixed set of craft checks (Review →
  // Craft), the editor should not ask them to re-pick those same checks for
  // every passage. manualOverride reveals the picker only when the writer
  // explicitly asks for it, or when they already have a manual pick in
  // flight — it never auto-collapses mid-interaction (see the effect below).
  const [manualOverride, setManualOverride] = useState(selectedSkills !== null)

  useEffect(() => {
    setDecisions({})
    setError(null)
  }, [review])

  useEffect(() => {
    if (selectedSkills !== null) setManualOverride(true)
  }, [selectedSkills])

  useEffect(() => {
    let live = true
    if (!universeId) return () => { live = false }
    void Promise.all([api.getSkills(), api.getUniverseSkills(universeId)])
      .then(([catalogueResponse, activeResponse]) => {
        if (!live) return
        setCatalogue(catalogueResponse.skills || [])
        setActiveNames((activeResponse.skills || []).map((skill) => skill.skill_name))
        setSkillLoadError(null)
      })
      .catch((loadError) => {
        if (!live) return
        setSkillLoadError((loadError as Error).message || 'Craft checks could not be loaded.')
      })
    return () => { live = false }
  }, [universeId])

  const toggleSkill = (name: string) => {
    const current = selectedSkills || []
    const next = current.includes(name) ? current.filter((item) => item !== name) : [...current, name]
    if (next.length > 3) {
      setSkillLoadError('Choose up to three craft checks for one passage.')
      return
    }
    setSkillLoadError(null)
    onSelectedSkillsChange?.(next.length > 0 ? next : null)
  }

  const activeSkills = activeNames.map((name) => catalogue.find((skill) => skill.name === name) || { name, description: '', genre_tags: [], stage: 'craft' })

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

      <details className={styles.disclosure} open>
        <summary className={styles.disclosureSummary}>
          <span className={styles.sectionLabel}>Craft checks for the next review</span>
          <span className={styles.disclosureChevron} aria-hidden="true">⌄</span>
        </summary>
        <div className={styles.skillPicker}>
          {activeSkills.length === 0 && !skillLoadError ? (
            <p className={styles.muted}>No craft checks are active. Configure them in Review → Craft.</p>
          ) : !manualOverride ? (
            <div className={styles.autoNotice}>
              <p className={styles.muted}>
                Quill will run the review using your configured craft checks
                ({activeSkills.map((skill) => displaySkillName(skill.name)).join(', ')}) and pick the best fit for this passage.
              </p>
              <button type="button" className={styles.linkButton} onClick={() => setManualOverride(true)}>
                Choose specific checks instead
              </button>
            </div>
          ) : (
            <>
              <div className={styles.skillPickerHeading}>
                <span className={styles.muted}>Choose up to three checks, or let Quill choose.</span>
                <button
                  type="button"
                  className={styles.autoButton}
                  onClick={() => { onSelectedSkillsChange?.(null); setManualOverride(false) }}
                >
                  Let Quill choose
                </button>
              </div>
              <div className={styles.skillChoices} aria-label="Choose craft checks">
                {activeSkills.map((skill) => (
                  <label key={skill.name} className={styles.skillChoice}>
                    <input type="checkbox" checked={(selectedSkills || []).includes(skill.name)} onChange={() => toggleSkill(skill.name)} disabled={loading} />
                    <span><strong>{displaySkillName(skill.name)}</strong>{skill.description && <small>{shortDescription(skill.description, 80)}</small>}</span>
                  </label>
                ))}
              </div>
            </>
          )}
          {skillLoadError && <p className={styles.error} role="alert">{skillLoadError}</p>}
        </div>
      </details>

      <details className={styles.disclosure} open>
        <summary className={styles.disclosureSummary}>
          <span className={styles.sectionLabel}>Review results</span>
          <span className={styles.disclosureChevron} aria-hidden="true">⌄</span>
        </summary>
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
                  <span className={styles.skillName}>{displaySkillName(selection.skill)}</span>
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
                    <span className={styles.noteSkill}>{displaySkillName(note.skill)}</span>
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
      </details>
    </section>
  )
}
