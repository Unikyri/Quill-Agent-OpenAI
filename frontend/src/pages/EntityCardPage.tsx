import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { parseVertexRaw } from '../lib/graphParse'
import { NODE_TYPE_META } from '../components/knowledge-graph/nodeTypeMeta'
import styles from './EntityCardPage.module.css'

// Matches backend models.Entity (backend/internal/models/models.go) — do not
// add fields the API doesn't actually return.
interface Entity {
  id: string
  universe_id: string
  type: string
  name: string
  aliases?: string[]
  description?: string
  properties?: Record<string, unknown>
  status: string
  relevance_score: number
}

interface RelatedEntity {
  id: string
  name: string
  type: string
}

// Free-text Properties keys the AI extraction pipeline commonly produces
// (qwen_service.go prompt is freeform, these are just the ones worth a labeled
// section instead of a generic tag).
const KNOWN_PROPERTY_SECTIONS = ['location', 'role', 'appearance', 'abilities']

const STATUS_CLASS: Record<string, string> = {
  active: styles.statusActive,
  deceased: styles.statusDanger,
  exiled: styles.statusDanger,
  archived: styles.statusMuted,
}

export default function EntityCardPage() {
  const { entityId } = useParams<{ entityId: string }>()
  const navigate = useNavigate()
  const [entity, setEntity] = useState<Entity | null>(null)
  const [related, setRelated] = useState<RelatedEntity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchEntity = () => {
    if (!entityId) return
    setLoading(true)
    setError(null)
    api.getEntity(entityId)
      .then((res: { entity: Entity }) => {
        setEntity(res.entity)
        setLoading(false)
        // Relationships/Factions come from the AGE graph, not the entity row itself.
        return api.getEntityNeighbors(entityId, res.entity.universe_id).then((g) => {
          const others = g.nodes
            .map((n) => parseVertexRaw(String(n.properties?.raw || '')))
            .filter((v) => v.entityId && v.entityId !== entityId)
          setRelated(others.map((v) => ({ id: v.entityId, name: v.name, type: v.type })))
        })
      })
      .catch((err: Error) => {
        setError(err.message || 'Failed to load entity')
        setLoading(false)
      })
  }

  useEffect(() => {
    fetchEntity()
  }, [entityId]) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) {
    return <p className={styles.loading}>Loading entity…</p>
  }

  if (error) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>Failed to load entity: {error}</p>
        <button className={styles.retryBtn} onClick={fetchEntity}>
          Retry
        </button>
      </div>
    )
  }

  const meta = NODE_TYPE_META[entity?.type || ''] || NODE_TYPE_META.character
  const relevancePct = Math.round((entity?.relevance_score ?? 0) * 100)
  const relevanceSegments = 5
  const filledSegments = Math.round((relevancePct / 100) * relevanceSegments)

  const properties = entity?.properties || {}
  const knownSections = KNOWN_PROPERTY_SECTIONS.filter((k) => typeof properties[k] === 'string' && properties[k])
  const otherProperties = Object.entries(properties).filter(([k]) => !KNOWN_PROPERTY_SECTIONS.includes(k))

  const factions = related.filter((r) => r.type === 'faction')
  const relationships = related.filter((r) => r.type !== 'faction')

  return (
    <div className={styles.wrap}>
      <nav className={styles.navbar}>
        <button className={styles.backBtn} onClick={() => navigate(-1)}>
          ← Back
        </button>
        <span className={styles.breadcrumb}>
          Entity / {entity?.name || 'Unknown'}
        </span>
      </nav>

      <div className={styles.columns}>
        <main className={styles.main}>
          <div className={styles.titleRow}>
            <h1 className={styles.heading}>{entity?.name || 'Entity'}</h1>
            {entity?.status && (
              <span className={`${styles.statusBadge} ${STATUS_CLASS[entity.status] || styles.statusMuted}`}>
                {entity.status}
              </span>
            )}
          </div>
          <p className={styles.subtitle} style={{ color: meta.color }}>
            <span className="glyph">{meta.icon}</span> {meta.label}
          </p>

          <p className={styles.description}>
            {entity?.description || 'No description provided.'}
          </p>

          {entity?.aliases && entity.aliases.length > 0 && (
            <div className={styles.section}>
              <p className={styles.sectionLabel}>Aliases</p>
              <div className={styles.tags}>
                {entity.aliases.map((alias) => (
                  <span key={alias} className={styles.tag}>{alias}</span>
                ))}
              </div>
            </div>
          )}

          {knownSections.map((key) => (
            <div key={key} className={styles.section}>
              <p className={styles.sectionLabel}>{key}</p>
              <p className={styles.sectionValue}>{String(properties[key])}</p>
            </div>
          ))}

          {otherProperties.length > 0 && (
            <div className={styles.section}>
              <p className={styles.sectionLabel}>Properties</p>
              <div className={styles.tags}>
                {otherProperties.map(([key, val]) => (
                  <span key={key} className={styles.tag}>
                    {key}: {String(val)}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* ponytail: Mentions Timeline is cut — no GET /entities/:id/mentions
              endpoint exists yet (entity_mention rows aren't exposed over HTTP).
              Needs a backend read endpoint before this chart can ship. */}
        </main>

        <aside className={styles.sidebar}>
          <div className={styles.sidebarCard}>
            <p className={styles.sidebarLabel}>Relevance</p>
            <p className={styles.sidebarValue}>{relevancePct}%</p>
            <div className={styles.relevanceBar}>
              {Array.from({ length: relevanceSegments }).map((_, i) => (
                <span
                  key={i}
                  className={styles.relevanceSegment}
                  style={i < filledSegments ? { background: 'var(--success)' } : undefined}
                />
              ))}
            </div>
          </div>

          {relationships.length > 0 && (
            <div className={styles.sidebarCard}>
              <p className={styles.sidebarLabel}>Relationships</p>
              <div className={styles.relatedList}>
                {relationships.map((rel) => (
                  <div
                    key={rel.id}
                    role="button"
                    tabIndex={0}
                    className={styles.relatedItem}
                    onClick={() => navigate(`/universe/${entity?.universe_id}/entities/${rel.id}`)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        navigate(`/universe/${entity?.universe_id}/entities/${rel.id}`)
                      }
                    }}
                  >
                    <span>
                      {rel.name}{' '}
                      <span style={{ color: 'var(--muted-2)', fontSize: 11 }}>
                        ({NODE_TYPE_META[rel.type]?.label || rel.type})
                      </span>
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {factions.length > 0 && (
            <div className={styles.sidebarCard}>
              <p className={styles.sidebarLabel}>Factions</p>
              <div className={styles.relatedList}>
                {factions.map((rel) => (
                  <div
                    key={rel.id}
                    role="button"
                    tabIndex={0}
                    className={styles.relatedItem}
                    onClick={() => navigate(`/universe/${entity?.universe_id}/entities/${rel.id}`)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        navigate(`/universe/${entity?.universe_id}/entities/${rel.id}`)
                      }
                    }}
                  >
                    <span>{rel.name}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </aside>
      </div>
    </div>
  )
}
