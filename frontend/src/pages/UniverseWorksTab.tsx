import { useContext, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { UniverseContext } from '../contexts/UniverseContext'
import { api } from '../lib/api'
import styles from './UniverseWorksTab.module.css'

export default function UniverseWorksTab() {
  const { works, universe, refetchWorks } = useContext(UniverseContext)
  const navigate = useNavigate()

  const [showNewForm, setShowNewForm] = useState(false)
  const [title, setTitle] = useState('')
  const [type, setType] = useState('novel')
  const [synopsis, setSynopsis] = useState('')
  const [submitError, setSubmitError] = useState<string | null>(null)

  const handleCreate = async () => {
    if (!universe) return; if (!title.trim()) { setSubmitError('Title is required'); return }
    setSubmitError(null)
    try {
      await api.createWork(universe.id, { title: title.trim(), type, synopsis: synopsis.trim() })
      await refetchWorks()
      setShowNewForm(false)
      setTitle('')
      setType('novel')
      setSynopsis('')
    } catch (err) {
      setSubmitError((err as Error).message || 'Failed to create work')
    }
  }

  return (
    <div className={styles.wrap}>
      <h2 className={styles.heading}>Works</h2>

      <div className={styles.headerRow}>
        {!showNewForm ? (
          <button className={styles.newBtn} onClick={() => setShowNewForm(true)}>
            + New Work
          </button>
        ) : (
          <div className={styles.inlineForm}>
            <input
              className={styles.formInput}
              placeholder="Work title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <select className={styles.formSelect} value={type} onChange={(e) => setType(e.target.value)}>
              <option value="novel">Novel</option>
              <option value="short-story">Short Story</option>
              <option value="screenplay">Screenplay</option>
              <option value="comic">Comic</option>
              <option value="interactive">Interactive</option>
            </select>
            <input
              className={styles.formInput}
              placeholder="Synopsis (optional)"
              value={synopsis}
              onChange={(e) => setSynopsis(e.target.value)}
            />
            <button className={styles.formSubmit} onClick={handleCreate}>Create</button>
            <button className={styles.formCancel} onClick={() => { setShowNewForm(false); setSubmitError(null) }}>Cancel</button>
          </div>
        )}
        {submitError && <p className={styles.formError}>{submitError}</p>}
      </div>
      {works.length === 0 ? (
        <p className={styles.empty}>No works yet.</p>
      ) : (
        works.map((w) => (
          <div
            key={w.id}
            role="button"
            tabIndex={0}
            className={styles.card}
            onClick={() => navigate(`/work/${w.id}`)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                navigate(`/work/${w.id}`)
              }
            }}
          >
            <span className={styles.cardTypePill}>{w.type}</span>
            <h3 className={styles.cardTitle}>{w.title}</h3>
          </div>
        ))
      )}
    </div>
  )
}
