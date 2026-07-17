import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import type { SkillCatalogueItem } from '../lib/types'
import styles from './SkillsPage.module.css'

const GROUP_LABELS: Record<string, string> = {
  editorial: 'Editorial team',
  craft: 'Craft',
  genre: 'Genre',
}

export default function SkillsPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const [catalogue, setCatalogue] = useState<SkillCatalogueItem[]>([])
  const [activeNames, setActiveNames] = useState<string[]>([])
  const [savedNames, setSavedNames] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (!universeId) return
    setLoading(true)
    setError(null)
    Promise.all([api.getSkills(), api.getUniverseSkills(universeId)])
      .then(([catalogueResponse, activeResponse]) => {
        const names = activeResponse.skills.map((skill) => skill.skill_name)
        setCatalogue(catalogueResponse.skills)
        setActiveNames(names)
        setSavedNames(names)
      })
      .catch((loadError) => setError((loadError as Error).message || 'Could not load editorial skills'))
      .finally(() => setLoading(false))
  }, [universeId])

  const groupedSkills = useMemo(() => {
    const groups = new Map<string, SkillCatalogueItem[]>()
    for (const skill of catalogue) {
      const groupKey = skill.name === 'genre-conventions' ? 'genre' : skill.stage === 'craft' ? 'craft' : 'editorial'
      const group = groups.get(groupKey) || []
      group.push(skill)
      groups.set(groupKey, group)
    }
    const order = ['editorial', 'craft', 'genre']
    return [...groups.entries()].sort(([left], [right]) => order.indexOf(left) - order.indexOf(right))
  }, [catalogue])

  const toggle = (skillName: string) => {
    setSaved(false)
    setActiveNames((current) => current.includes(skillName)
      ? current.filter((name) => name !== skillName)
      : [...current, skillName])
  }

  const save = async () => {
    if (!universeId || saving) return
    setSaving(true)
    setError(null)
    setSaved(false)
    try {
      const response = await api.updateUniverseSkills(universeId, activeNames)
      const names = response.skills.map((skill) => skill.skill_name)
      setActiveNames(names)
      setSavedNames(names)
      setSaved(true)
    } catch (saveError) {
      setError((saveError as Error).message || 'Could not save skill settings')
    } finally {
      setSaving(false)
    }
  }

  const hasChanges = activeNames.length !== savedNames.length
    || activeNames.some((name) => !savedNames.includes(name))

  if (!universeId) return null

  return (
    <main className={styles.wrap}>
      <div className={styles.pageHeader}>
        <div>
          <span className={styles.kicker}>Editorial system</span>
          <h1 className={styles.title}>Skills</h1>
          <p className={styles.subtitle}>Choose the editorial voices available when you explicitly review a passage.</p>
        </div>
        <div className={styles.headerActions}>
          <span className={styles.count}>{activeNames.length} active</span>
          <button type="button" className={styles.saveButton} disabled={saving || loading || !hasChanges} onClick={save}>
            {saving ? 'Saving…' : saved ? 'Saved' : 'Save changes'}
          </button>
        </div>
      </div>

      {error && <p className={styles.error} role="alert">{error}</p>}
      {loading ? (
        <div className={styles.state}>Loading skill catalogue…</div>
      ) : catalogue.length === 0 ? (
        <div className={styles.state}>No editorial skills are available.</div>
      ) : (
        <div className={styles.groups}>
          {groupedSkills.map(([stage, skills]) => (
            <section key={stage} className={styles.group} aria-labelledby={`skill-stage-${stage}`}>
              <div className={styles.groupHeader}>
                <h2 id={`skill-stage-${stage}`} className={styles.groupTitle}>{GROUP_LABELS[stage] || stage}</h2>
                <span className={styles.groupCount}>{skills.length}</span>
              </div>
              <div className={styles.skillGrid}>
                {skills.map((skill) => {
                  const active = activeNames.includes(skill.name)
                  return (
                    <label key={skill.name} className={`${styles.card} ${active ? styles.cardActive : ''}`}>
                      <input type="checkbox" checked={active} onChange={() => toggle(skill.name)} />
                      <span className={styles.cardBody}>
                        <span className={styles.cardTopline}>
                          <span className={styles.skillName}>{skill.name}</span>
                          <span className={styles.status}>{active ? 'Active' : 'Off'}</span>
                        </span>
                        <span className={styles.description}>{skill.description}</span>
                        {skill.genre_tags.length > 0 && <span className={styles.tags}>{skill.genre_tags.join(' · ')}</span>}
                      </span>
                    </label>
                  )
                })}
              </div>
            </section>
          ))}
        </div>
      )}
    </main>
  )
}
