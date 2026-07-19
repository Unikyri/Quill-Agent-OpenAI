import { useContext, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowLeft, Network, RotateCcw } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import GraphCanvas from '../components/knowledge-graph/GraphCanvas'
import GraphControls from '../components/knowledge-graph/GraphControls'
import TimelineSlider from '../components/knowledge-graph/TimelineSlider'
import EntityOverviewTab from '../components/knowledge-graph/EntityOverviewTab'
import RelationshipsTab from '../components/knowledge-graph/RelationshipsTab'
import MentionsTab from '../components/knowledge-graph/MentionsTab'
import RelevanceHistoryTab from '../components/knowledge-graph/RelevanceHistoryTab'
import PageStatus from '../components/shared/PageStatus'
import { UniverseContext } from '../contexts/UniverseContext'
import { writeImportPath } from '../lib/canonicalRoutes'
import { api } from '../lib/api'
import { ENTITY_TYPES, ENTITY_TYPE_META, type EntityType } from '../lib/entityTypes'
import { useGraphStore } from '../stores/graphStore'
import { useWSStore } from '../stores/wsStore'
import styles from './KnowledgeGraphPage.module.css'

interface FilterEntitySummary {
  id: string; name: string; type: string; aliases?: string[]
}

const ENTITY_PAGE_SIZE = 100
const TYPE_FILTERS = ['All', ...ENTITY_TYPES] as const

function emptyTypeCounts(): Record<EntityType, number> {
  return Object.fromEntries(ENTITY_TYPES.map((type) => [type, 0])) as Record<EntityType, number>
}

function getInitial(name: string) {
  return (name || '?').charAt(0).toUpperCase()
}

type DetailTab = 'overview' | 'relationships' | 'mentions' | 'relevance'

const DETAIL_TABS: Array<{ id: DetailTab; label: string }> = [
  { id: 'overview', label: 'Overview' },
  { id: 'relationships', label: 'Relationships' },
  { id: 'mentions', label: 'Mentions' },
  { id: 'relevance', label: 'Relevance history' },
]

export default function KnowledgeGraphPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const { universe } = useContext(UniverseContext)
  const navigate = useNavigate()
  const fetchGraph = useGraphStore((state) => state.fetchGraph)
  const refresh = useGraphStore((state) => state.refresh)
  const resetFocus = useGraphStore((state) => state.resetFocus)
  const loading = useGraphStore((state) => state.loading)
  const error = useGraphStore((state) => state.error)
  const nodes = useGraphStore((state) => state.nodes)
  const edges = useGraphStore((state) => state.edges)
  const selectedNodeId = useGraphStore((state) => state.selectedNodeId)
  const focalNodeId = useGraphStore((state) => state.focalNodeId)
  const breadcrumb = useGraphStore((state) => state.breadcrumb)
  const nodeFilter = useGraphStore((state) => state.nodeFilter)
  const showArchived = useGraphStore((state) => state.showArchived)
  const truncated = useGraphStore((state) => state.truncated)
  const limits = useGraphStore((state) => state.limits)
  const focusNode = useGraphStore((state) => state.focusNode)
  const goBack = useGraphStore((state) => state.goBack)
  const graphPings = useWSStore((state) => state.graphPings)
  const previousPingCount = useRef(graphPings.length)

  // ── Left pane: entity search/filter/list, carried over from EntitiesPage's
  // list rail. Selection re-centers the map in-page (no URL navigation).
  const [filterEntities, setFilterEntities] = useState<FilterEntitySummary[]>([])
  const [filterLoading, setFilterLoading] = useState(true)
  const [filterLoadingMore, setFilterLoadingMore] = useState(false)
  const [filterListError, setFilterListError] = useState<string | null>(null)
  const [filterLoadMoreError, setFilterLoadMoreError] = useState<string | null>(null)
  const [filterListRetry, setFilterListRetry] = useState(0)
  const [filterTotal, setFilterTotal] = useState(0)
  const [filterCountsByType, setFilterCountsByType] = useState<Record<EntityType, number>>(emptyTypeCounts)
  const [filterSearch, setFilterSearch] = useState('')
  const [filterType, setFilterType] = useState<(typeof TYPE_FILTERS)[number]>('All')
  const filterQueryKey = `${universeId ?? ''}\u0000${filterType}\u0000${filterSearch.trim()}`
  const activeFilterQueryKey = useRef(filterQueryKey)
  activeFilterQueryKey.current = filterQueryKey

  // ── Right pane: tabbed detail panel for the focused/selected entity.
  const [activeTab, setActiveTab] = useState<DetailTab>('overview')
  const detailEntityId = selectedNodeId ?? focalNodeId ?? null

  useEffect(() => {
    if (universeId) void fetchGraph(universeId)
  }, [fetchGraph, universeId])

  useEffect(() => {
    if (graphPings.length > previousPingCount.current) {
      previousPingCount.current = graphPings.length
      void refresh()
    }
  }, [graphPings, refresh])

  useEffect(() => {
    setActiveTab('overview')
  }, [detailEntityId])

  useEffect(() => {
    if (!universeId) return
    let cancelled = false
    setFilterEntities([])
    setFilterTotal(0)
    setFilterCountsByType(emptyTypeCounts())
    setFilterLoading(true)
    setFilterLoadingMore(false)
    setFilterListError(null)
    setFilterLoadMoreError(null)

    const params: Record<string, string> = { limit: String(ENTITY_PAGE_SIZE), page: '1' }
    if (filterType !== 'All') params.type = filterType
    if (filterSearch.trim()) params.search = filterSearch.trim()

    void api.listEntities(universeId, params)
      .then((res) => {
        if (cancelled) return
        const nextEntities = res.entities || []
        setFilterEntities(nextEntities)
        setFilterTotal(res.pagination?.total ?? nextEntities.length)
        setFilterCountsByType({ ...emptyTypeCounts(), ...(res.counts_by_type || {}) })
      })
      .catch(() => {
        if (!cancelled) setFilterListError('Could not load entities for this universe. Retry to try again.')
      })
      .finally(() => {
        if (!cancelled) setFilterLoading(false)
      })

    return () => { cancelled = true }
  }, [universeId, filterType, filterSearch, filterListRetry])

  const loadMoreFiltered = async () => {
    if (!universeId || filterLoadingMore) return
    const requestQueryKey = filterQueryKey
    setFilterLoadingMore(true)
    setFilterLoadMoreError(null)

    const params: Record<string, string> = {
      limit: String(ENTITY_PAGE_SIZE),
      page: String(Math.floor(filterEntities.length / ENTITY_PAGE_SIZE) + 1),
    }
    if (filterType !== 'All') params.type = filterType
    if (filterSearch.trim()) params.search = filterSearch.trim()

    try {
      const res = await api.listEntities(universeId, params)
      if (activeFilterQueryKey.current !== requestQueryKey) return
      setFilterEntities((current) => [...current, ...(res.entities || [])])
      setFilterTotal(res.pagination?.total ?? filterTotal)
    } catch {
      if (activeFilterQueryKey.current === requestQueryKey) {
        setFilterLoadMoreError('Could not load more entities. Showing the results already loaded.')
      }
    } finally {
      if (activeFilterQueryKey.current === requestQueryKey) setFilterLoadingMore(false)
    }
  }

  const allFilterEntityCount = useMemo(
    () => Object.values(filterCountsByType).reduce((sum, count) => sum + count, 0),
    [filterCountsByType],
  )

  const visibleNodes = useMemo(() => nodes.filter((node) => (
    nodeFilter[node.type] !== false && (showArchived || node.data.status !== 'archived')
  )), [nodeFilter, nodes, showArchived])
  const visibleNodeIds = useMemo(() => new Set(visibleNodes.map((node) => node.id)), [visibleNodes])
  const visibleEdges = useMemo(() => edges.filter((edge) => (
    visibleNodeIds.has(edge.source) && visibleNodeIds.has(edge.target)
  )), [edges, visibleNodeIds])

  if ((loading || error) && nodes.length === 0) {
    return (
      <PageStatus
        loading={loading}
        error={error}
        onRetry={() => { if (universeId) void fetchGraph(universeId) }}
      />
    )
  }

  return (
    <div className={styles.wrap}>
      <nav className={`${styles.filterRail} q-scroll`} aria-label="Browse entities">
        <div className={styles.filterSearchBar}>
          <input
            className={styles.filterSearchInput}
            placeholder="Search entity or alias…"
            value={filterSearch}
            onChange={(event) => setFilterSearch(event.target.value)}
          />
        </div>

        <div className={styles.filterChips}>
          {TYPE_FILTERS.map((typeOption) => (
            <button
              key={typeOption}
              type="button"
              className={`${styles.filterChip} ${filterType === typeOption ? styles.filterChipActive : ''}`}
              onClick={() => setFilterType(typeOption)}
            >
              {typeOption === 'All'
                ? `All (${allFilterEntityCount})`
                : `${ENTITY_TYPE_META[typeOption].label}s (${filterCountsByType[typeOption]})`}
            </button>
          ))}
        </div>

        <div className={styles.filterEntityListWrap}>
          {filterListError ? (
            <PageStatus error={filterListError} onRetry={() => setFilterListRetry((attempt) => attempt + 1)} />
          ) : filterLoading ? (
            <p className={styles.filterEntityListStatus}>Loading…</p>
          ) : filterEntities.length === 0 ? (
            <p className={styles.filterEntityListStatus}>No entities found.</p>
          ) : (
            <>
              <ul className={styles.filterEntityList}>
                {filterEntities.map((entity) => {
                  const meta = ENTITY_TYPE_META[entity.type as keyof typeof ENTITY_TYPE_META] || ENTITY_TYPE_META.character
                  return (
                    <li key={entity.id}>
                      <button
                        type="button"
                        aria-pressed={selectedNodeId === entity.id}
                        className={`${styles.filterEntityItem} ${selectedNodeId === entity.id ? styles.filterEntityItemActive : ''}`}
                        onClick={() => void focusNode(entity.id)}
                      >
                        <span className={styles.filterEntityAvatar} style={{ background: meta.color }}>
                          {getInitial(entity.name)}
                        </span>
                        <span className={styles.filterEntityInfo}>
                          <span className={styles.filterEntityName}>{entity.name}</span>
                          <span className={styles.filterEntityType} style={{ color: meta.color }}>
                            {meta.label.toUpperCase()}
                          </span>
                        </span>
                      </button>
                    </li>
                  )
                })}
              </ul>
              {filterEntities.length < filterTotal && (
                <button
                  className={styles.filterLoadMore}
                  type="button"
                  onClick={() => void loadMoreFiltered()}
                  disabled={filterLoadingMore}
                >
                  {filterLoadingMore ? 'Loading…' : `Load more (${filterEntities.length} of ${filterTotal})`}
                </button>
              )}
              {filterLoadMoreError && (
                <p className={styles.filterLoadMoreError} role="status">
                  {filterLoadMoreError}{' '}
                  <button type="button" onClick={() => void loadMoreFiltered()}>Retry</button>
                </p>
              )}
            </>
          )}
        </div>
      </nav>

      <section className={styles.canvasArea} aria-labelledby="relationship-map-heading">
        <header className={styles.mapHeader}>
          <div>
            <p className={styles.kicker}>Explore</p>
            <h1 id="relationship-map-heading">Relationship map</h1>
            <p>Centered on your most relevant entity — search or tap any node to re-center the map on it.</p>
          </div>
          <div className={styles.mapActions}>
            <button className={styles.resetButton} type="button" onClick={() => void refresh()} disabled={loading}>
              <RotateCcw size={15} aria-hidden="true" />
              {loading ? 'Updating…' : 'Refresh map'}
            </button>
            <button
              className={styles.resetButton}
              type="button"
              onClick={() => void resetFocus()}
              disabled={!focalNodeId || loading}
            >
              Reset focus
            </button>
          </div>
        </header>

        {(loading || error) && (
          <div className={error ? styles.mapFailure : styles.mapStatus} role={error ? 'alert' : 'status'} aria-live="polite">
            <span>{error || 'Updating the relationship map. Your entity-type filters remain applied.'}</span>
            {error && <button type="button" onClick={() => void refresh()}>Try again</button>}
          </div>
        )}

        {truncated && (
          <div className={styles.truncationNotice} role="status">
            <span>Large neighborhood: showing a bounded partial map.</span>
            <button type="button" onClick={() => void refresh()}>Retry map</button>
          </div>
        )}

        {nodes.length === 0 ? (
          <div className={styles.emptyState}>
            <Network size={32} aria-hidden="true" />
            <h2>No relationship map yet</h2>
            <p>Import a manuscript or keep writing to let Quill extract story entities and their relationships.</p>
            <button type="button" className={styles.importButton} onClick={() => universeId && navigate(writeImportPath(universeId))}>
              Go to import
            </button>
          </div>
        ) : (
          <>
            <GraphControls />
            <GraphCanvas />
          </>
        )}

        {universeId && (
          <TimelineSlider universeId={universeId} />
        )}
      </section>

      <aside className={`${styles.inspector} q-scroll`} aria-label="Relationship map details">
        {breadcrumb.length > 0 && (
          <button className={styles.backButton} type="button" onClick={() => void goBack()}>
            <ArrowLeft size={14} aria-hidden="true" />
            Previous focus
          </button>
        )}

        {detailEntityId ? (
          <section className={styles.detailPanelTabs} aria-labelledby="entity-detail-heading">
            <p className={styles.kicker} id="entity-detail-heading">Focused entity</p>
            <div className={styles.tabStrip} role="tablist" aria-label="Entity detail tabs">
              {DETAIL_TABS.map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  aria-selected={activeTab === tab.id}
                  className={`${styles.tabButton} ${activeTab === tab.id ? styles.tabButtonActive : ''}`}
                  onClick={() => setActiveTab(tab.id)}
                >
                  {tab.label}
                </button>
              ))}
            </div>
            <div className={styles.tabPanel} role="tabpanel">
              {activeTab === 'overview' && <EntityOverviewTab entityId={detailEntityId} />}
              {activeTab === 'relationships' && universeId && (
                <RelationshipsTab entityId={detailEntityId} universeId={universeId} />
              )}
              {activeTab === 'mentions' && universeId && (
                <MentionsTab entityId={detailEntityId} universeId={universeId} />
              )}
              {activeTab === 'relevance' && universeId && (
                <RelevanceHistoryTab entityId={detailEntityId} universeId={universeId} />
              )}
            </div>
          </section>
        ) : (
          <section className={styles.focusedInspector}>
            <p className={styles.kicker}>Map summary</p>
            <h2>{visibleNodes.length} entities · {visibleEdges.length} relationships</h2>
            <p>Search or tap an entity to center the map on it.</p>
          </section>
        )}

        {focalNodeId && <p className={styles.focalNote}>Showing the focal entity and its {limits?.hops || 2}-hop neighborhood in {universe?.name || 'this universe'}.</p>}
      </aside>
    </div>
  )
}
