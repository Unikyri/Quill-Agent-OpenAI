import { useEffect, useState } from 'react'
import { GenreTagPicker } from '../genres'
import { api } from '../../lib/api'
import type { WriterObservationDTO, WriterPreferenceDTO, WriterPreferenceEvidenceDTO } from '../../lib/types'
import styles from './WriterMemoryPanel.module.css'

const OBSERVATION_META: Record<string, { label: string; definition: string; format: (value: number) => string; sample: (size: number) => string }> = {
  mean_sentence_length: {
    label: 'Average sentence length',
    definition: 'Average words in each detected sentence. This is a count, not a quality score.',
    format: (value) => `${value.toFixed(1)} words`,
    sample: (size) => `${size.toLocaleString()} sentences measured`,
  },
  adverb_density: {
    label: 'Adverb density',
    definition: 'Share of measured words that match Quill’s deterministic adverb list.',
    format: (value) => `${value.toFixed(2)}%`,
    sample: (size) => `${size.toLocaleString()} words measured`,
  },
  dialogue_ratio: {
    label: 'Dialogue ratio',
    definition: 'Share of measured words found inside quoted dialogue.',
    format: (value) => `${value.toFixed(1)}%`,
    sample: (size) => `${size.toLocaleString()} words measured`,
  },
  lexical_richness: {
    label: 'Vocabulary variety',
    definition: 'Unique words divided by all measured words. 1.00 means every measured word was unique.',
    format: (value) => value.toFixed(2),
    sample: (size) => `${size.toLocaleString()} words measured`,
  },
}

function observationMeta(metric: string) {
  return OBSERVATION_META[metric] || {
    label: metric.replace(/_/g, ' '),
    definition: 'A measured writing statistic from the current corpus.',
    format: (value: number) => value.toFixed(2),
    sample: (size: number) => `${size.toLocaleString()} items measured`,
  }
}

export default function WriterMemoryPanel() {
  const [preferences, setPreferences] = useState<WriterPreferenceDTO[]>([])
  const [observations, setObservations] = useState<WriterObservationDTO[]>([])
  const [universeNames, setUniverseNames] = useState<Record<string, string>>({})
  const [evidence, setEvidence] = useState<Record<string, WriterPreferenceEvidenceDTO>>({})
  const [expanded, setExpanded] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  // No universeId here: this panel is account-scoped and shows every universe
  // the writer has worked in, so a genre-bound correction can no longer infer
  // its genres from "the current universe" (there isn't one). Instead the
  // writer picks genres from the same global list the backend validates
  // against (see correctScope below).
  const [genreDraft, setGenreDraft] = useState<{ id: string; tags: string[] } | null>(null)

  const load = async () => {
    setLoading(true)
    setError(null)
    try {
      const [result, universesResult] = await Promise.all([
        api.getWriterPreferences(),
        api.listUniverses(),
      ])
      setPreferences(result.preferences || [])
      setObservations(result.observations || [])
      const names: Record<string, string> = {}
      for (const universe of universesResult.universes || []) names[universe.id] = universe.name
      setUniverseNames(names)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const toggleEvidence = async (id: string) => {
    if (expanded === id) { setExpanded(null); return }
    try {
      const result = await api.getWriterPreferenceEvidence(id)
      setEvidence((current) => ({ ...current, [id]: result }))
      setExpanded(id)
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const applyCorrectedPreference = (preference: WriterPreferenceDTO) => {
    setPreferences((current) => current.map((item) => item.id === preference.id ? preference : item))
  }

  const makeUniversal = async (preference: WriterPreferenceDTO) => {
    try {
      const result = await api.correctWriterPreference(preference.id, { scope: 'universal', genre_tags: [] })
      applyCorrectedPreference(result.preference)
    } catch (err) { setError((err as Error).message) }
  }

  const beginGenreBind = (preference: WriterPreferenceDTO) => {
    setError(null)
    setGenreDraft({ id: preference.id, tags: [] })
  }

  const cancelGenreBind = () => setGenreDraft(null)

  const confirmGenreBind = async (preference: WriterPreferenceDTO) => {
    if (!genreDraft || genreDraft.tags.length === 0) {
      setError('Choose at least one genre before making this preference genre-bound.')
      return
    }
    try {
      const result = await api.correctWriterPreference(preference.id, { scope: 'genre_bound', genre_tags: genreDraft.tags })
      applyCorrectedPreference(result.preference)
      setGenreDraft(null)
    } catch (err) { setError((err as Error).message) }
  }

  const deactivate = async (id: string) => {
    try {
      await api.deactivateWriterPreference(id)
      setPreferences((current) => current.filter((item) => item.id !== id))
      setExpanded((current) => current === id ? null : current)
    } catch (err) { setError((err as Error).message) }
  }

  return (
    <section className={styles.panel} aria-labelledby="writer-memory-title">
      <div className={styles.heading}>
        <div>
          <p className={styles.eyebrow}>Act IV · Writer Memory</p>
          <h2 id="writer-memory-title" className={styles.title}>Measured writing baseline</h2>
        </div>
        <span className={styles.eyebrow}>{preferences.length} active</span>
      </div>
      <p className={styles.intro}>
        These are measurements from the saved or imported text across every universe you have worked in. They describe the current corpus; they do not judge quality or infer an intention.
      </p>
      {loading && <p className={styles.state} role="status" aria-live="polite">Reading your evidence trail…</p>}
      {error && (
        <div className={`${styles.state} ${styles.error}`} role="alert">
          <p>{error}</p>
          <button className={styles.retryButton} type="button" onClick={() => void load()}>Retry</button>
        </div>
      )}
      {!loading && !error && (
        <section className={styles.observations} aria-labelledby="writer-observations-title">
          <div className={styles.subheading}>
            <h3 id="writer-observations-title">Measured observations</h3>
            <span className={styles.eyebrow}>{observations.length} facts</span>
          </div>
          {observations.length === 0 ? (
            <p className={styles.state}>No observations yet. Save a chapter or import a manuscript and Quill will measure sentence length, dialogue, adverbs, and vocabulary without inferring intent.</p>
          ) : (
            <div className={styles.observationList}>
              {observations.map((observation) => {
                const meta = observationMeta(observation.metric)
                const scopeLabel = observation.universe_id ? (universeNames[observation.universe_id] ?? 'a universe') : 'all universes'
                return (
                  <div className={styles.observation} key={observation.id} title={meta.definition}>
                    <span>{meta.label}</span>
                    <strong>{meta.format(observation.value)}</strong>
                    <span className={styles.scopeTag}>applies: {scopeLabel}</span>
                    <small>{meta.sample(observation.sample_size)}</small>
                    <p>{meta.definition}</p>
                  </div>
                )
              })}
            </div>
          )}
        </section>
      )}
      {!loading && !error && preferences.length === 0 && (
        <p className={styles.state}>No preferences yet. Keep writing and respond to craft notes when you want Quill to learn an intention; measurable observations can exist without inventing a belief.</p>
      )}
      <div className={styles.list}>
        {preferences.map((preference) => {
          const confidence = Math.round(Math.max(0, Math.min(1, preference.confidence)) * 100)
          const itemEvidence = evidence[preference.id]
          const isDrafting = genreDraft?.id === preference.id
          return (
            <article className={styles.card} key={preference.id}>
              <p className={styles.statement}>{preference.statement}</p>
              <div className={styles.meta}>
                <span>confidence {confidence}%</span>
                <span className={styles.badge}>{preference.scope === 'universal' ? 'universal' : 'genre-bound'}</span>
                {preference.genre_tags.map((tag) => <span className={styles.genre} key={tag}>{tag}</span>)}
              </div>
              <div className={styles.bar} aria-label={`Confidence ${confidence}%`}><div className={styles.fill} style={{ width: `${confidence}%` }} /></div>
              <div className={styles.actions}>
                <button className={styles.button} onClick={() => void toggleEvidence(preference.id)}>{expanded === preference.id ? 'Hide evidence' : 'Why? Show evidence'}</button>
                {preference.scope === 'universal' ? (
                  <button className={styles.button} onClick={() => beginGenreBind(preference)}>Make genre-bound</button>
                ) : (
                  <button className={styles.button} onClick={() => void makeUniversal(preference)}>Make universal</button>
                )}
                <button className={`${styles.button} ${styles.danger}`} onClick={() => void deactivate(preference.id)}>Deactivate</button>
              </div>
              {isDrafting && (
                <div className={styles.genreDraft}>
                  <GenreTagPicker
                    id={`genre-bind-${preference.id}`}
                    label="Choose genres to bind this preference to"
                    value={genreDraft.tags}
                    onChange={(tags) => setGenreDraft({ id: preference.id, tags })}
                  />
                  <div className={styles.actions}>
                    <button className={styles.button} onClick={() => void confirmGenreBind(preference)}>Save scope</button>
                    <button className={styles.button} onClick={cancelGenreBind}>Cancel</button>
                  </div>
                </div>
              )}
              {expanded === preference.id && itemEvidence && (
                <div className={styles.evidence}>
                  <h4>Observations</h4>
                  <ul>{itemEvidence.observations.map((item) => <li key={item.id}>{item.metric}: {item.value.toFixed(2)} · sample {item.sample_size}</li>)}</ul>
                  <h4>Writer signals</h4>
                  <ul>{itemEvidence.feedback_events.map((item) => <li key={item.id}>{item.signal} · {new Date(item.created_at).toLocaleDateString()}</li>)}</ul>
                  <h4>Decay history</h4>
                  <ul>{itemEvidence.history.map((item) => <li key={item.id}>{Math.round(item.relevance_score * 100)}% relevance · {item.lifecycle}</li>)}</ul>
                </div>
              )}
            </article>
          )
        })}
      </div>
    </section>
  )
}
