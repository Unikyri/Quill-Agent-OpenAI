import { useContext, useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { UniverseContext } from '../contexts/UniverseContext'
import { api } from '../lib/api'
import { NODE_TYPE_META } from '../components/knowledge-graph/nodeTypeMeta'
import styles from './PanoramaPage.module.css'

interface EntitySummary { id: string; name: string; type: string }
interface TimelineEvent { id: string; label: string; timestamp: string; description: string }
interface Contradiction { id: string; description: string; severity: string; status: string }
interface LatestChapter { id: string; title: string; word_count: number }

export default function PanoramaPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const navigate = useNavigate()
  const { works } = useContext(UniverseContext)

  const [entityCount, setEntityCount] = useState(0)
  const [recentEntities, setRecentEntities] = useState<EntitySummary[]>([])
  const [events, setEvents] = useState<TimelineEvent[]>([])
  const [contradictionCount, setContradictionCount] = useState(0)
  const [topContradiction, setTopContradiction] = useState<Contradiction | null>(null)
  const [plotHoleCount, setPlotHoleCount] = useState(0)
  const [latestChapter, setLatestChapter] = useState<LatestChapter | null>(null)

  useEffect(() => {
    if (!universeId) return

    api.listEntities(universeId, { limit: '4' })
      .then((res) => {
        setEntityCount(res.pagination?.total ?? res.entities.length)
        setRecentEntities(res.entities)
      })
      .catch(() => {})

    api.getTimeline(universeId).then((res) => setEvents(res.events || [])).catch(() => {})

    api.getContradictions(universeId)
      .then((res) => {
        setContradictionCount(res.contradictions.length)
        setTopContradiction(res.contradictions[0] || null)
      })
      .catch(() => {})

    api.getPlotHoles(universeId).then((res) => setPlotHoleCount(res.plot_holes.length)).catch(() => {})
  }, [universeId])

  useEffect(() => {
    // ponytail: "continue writing" picks the first work's most recent chapter.
    // A real "most recently edited across all works" query would need a
    // dedicated backend endpoint; this is good enough for a single-work demo.
    const firstWork = works[0]
    if (!firstWork) {
      setLatestChapter(null)
      return
    }
    api.listChapters(firstWork.id)
      .then((res) => {
        const chapters = res.chapters || []
        const last = chapters[chapters.length - 1]
        setLatestChapter(last ? { id: last.id, title: last.title, word_count: last.word_count } : null)
      })
      .catch(() => setLatestChapter(null))
  }, [works])

  return (
    <div className={styles.wrap}>
      <div className={styles.statGrid}>
        <div className={styles.statCard}>
          <div className={styles.statValue}>{entityCount}</div>
          <div className={styles.statLabel}>Entities</div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statValue}>{events.length}</div>
          <div className={styles.statLabel}>Events</div>
        </div>
        <div className={`${styles.statCard} ${styles.statCardDanger}`}>
          <div className={`${styles.statValue} ${styles.statValueDanger}`}>{contradictionCount}</div>
          <div className={styles.statLabelDanger}>Contradictions</div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statValue}>{plotHoleCount}</div>
          <div className={styles.statLabel}>Plot Holes</div>
        </div>
      </div>

      {latestChapter ? (
        <div className={styles.continueBanner}>
          <div>
            <div className={styles.continueKicker}>Continue where you left off</div>
            <div className={styles.continueTitle}>{latestChapter.title}</div>
            <div className={styles.continueMeta}>{latestChapter.word_count} words</div>
          </div>
          <button
            className={styles.continueBtn}
            onClick={() => navigate(`/universe/${universeId}/editor/${latestChapter.id}`)}
          >
            Continue writing →
          </button>
        </div>
      ) : (
        <div className={styles.ingestCard}>
          <p className={styles.ingestText}>No active ingestion jobs. Import a manuscript to get started.</p>
        </div>
      )}

      <div className={styles.columns}>
        <div className={styles.mainCol}>
          <div className={styles.card}>
            <div className={styles.cardHeader}>
              <h3 className={styles.cardTitle}>Recent Entities</h3>
              <span
                role="button"
                tabIndex={0}
                className={styles.cardLink}
                onClick={() => navigate(`/universe/${universeId}/entities`)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    navigate(`/universe/${universeId}/entities`)
                  }
                }}
              >
                View the encyclopedia →
              </span>
            </div>
            <div className={styles.entityGrid}>
              {recentEntities.map((entity) => {
                const meta = NODE_TYPE_META[entity.type] || NODE_TYPE_META.character
                return (
                  <div key={entity.id} className={styles.entityRow}>
                    <span className={styles.entitySwatch} style={{ background: meta.color }} />
                    <div>
                      <div className={styles.entityName}>{entity.name}</div>
                      <div className={styles.entityType} style={{ color: meta.color }}>{meta.label.toUpperCase()}</div>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>

          <div className={styles.card}>
            <div className={styles.cardHeader}>
              <h3 className={styles.cardTitle}>Timeline</h3>
              <span
                role="button"
                tabIndex={0}
                className={styles.cardLink}
                onClick={() => navigate(`/universe/${universeId}/timeline`)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    navigate(`/universe/${universeId}/timeline`)
                  }
                }}
              >
                View full timeline →
              </span>
            </div>
            <div className={styles.timelineList}>
              {events.slice(0, 3).map((event) => (
                <div key={event.id} className={styles.timelineItem}>
                  <div className={styles.timelineEra}>{event.timestamp}</div>
                  <div className={styles.timelineLabel}>{event.label}</div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className={styles.sideCol}>
          {topContradiction && (
            <div className={styles.aiCard}>
              <h3 className={styles.aiTitle}>
                <span className="glyph">△</span> Detected by AI
              </h3>
              <p className={styles.aiMessage}>{topContradiction.description}</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
