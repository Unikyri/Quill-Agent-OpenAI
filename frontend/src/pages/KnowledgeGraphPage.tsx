import { useContext, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowLeft, FileQuestion, Network, RotateCcw, Search } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import GraphCanvas from '../components/knowledge-graph/GraphCanvas'
import GraphControls from '../components/knowledge-graph/GraphControls'
import TimelineSlider from '../components/knowledge-graph/TimelineSlider'
import PageStatus from '../components/shared/PageStatus'
import { UniverseContext } from '../contexts/UniverseContext'
import { relationText } from '../lib/graphElements'
import { writeImportPath } from '../lib/canonicalRoutes'
import { api } from '../lib/api'
import { useGraphStore } from '../stores/graphStore'
import { useWSStore } from '../stores/wsStore'
import styles from './KnowledgeGraphPage.module.css'

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
  const selectedEdgeId = useGraphStore((state) => state.selectedEdgeId)
  const focalNodeId = useGraphStore((state) => state.focalNodeId)
  const breadcrumb = useGraphStore((state) => state.breadcrumb)
  const nodeFilter = useGraphStore((state) => state.nodeFilter)
  const showArchived = useGraphStore((state) => state.showArchived)
  const truncated = useGraphStore((state) => state.truncated)
  const limits = useGraphStore((state) => state.limits)
  const focusNode = useGraphStore((state) => state.focusNode)
  const goBack = useGraphStore((state) => state.goBack)
  const selectEdge = useGraphStore((state) => state.selectEdge)
  const graphPings = useWSStore((state) => state.graphPings)
  const previousPingCount = useRef(graphPings.length)
  const searchRequestVersion = useRef(0)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<Array<{ id: string; name: string; type: string }>>([])
  const [searchError, setSearchError] = useState<string | null>(null)
  const [searching, setSearching] = useState(false)
  const [searchRetry, setSearchRetry] = useState(0)

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
    const requestVersion = ++searchRequestVersion.current
    const query = searchQuery.trim()
    if (!universeId || !query) {
      setSearchResults([])
      setSearchError(null)
      setSearching(false)
      return
    }

    setSearchResults([])
    setSearchError(null)
    setSearching(true)

    const timer = window.setTimeout(() => {
      api.listEntities(universeId, { search: query, limit: '8' })
        .then((response) => {
          if (searchRequestVersion.current === requestVersion) {
            if (!Array.isArray(response.entities)) {
              throw new Error('Search returned an invalid response.')
            }
            setSearchResults(response.entities)
            setSearching(false)
          }
        })
        .catch((searchFailure: unknown) => {
          if (searchRequestVersion.current === requestVersion) {
            const message = searchFailure instanceof Error ? searchFailure.message : 'Search is unavailable.'
            setSearchResults([])
            setSearchError(message)
            setSearching(false)
          }
        })
    }, 180)

    return () => window.clearTimeout(timer)
  }, [searchQuery, searchRetry, universeId])

  const visibleNodes = useMemo(() => nodes.filter((node) => (
    nodeFilter[node.type] !== false && (showArchived || node.data.status !== 'archived')
  )), [nodeFilter, nodes, showArchived])
  const visibleNodeIds = useMemo(() => new Set(visibleNodes.map((node) => node.id)), [visibleNodes])
  const visibleEdges = useMemo(() => edges.filter((edge) => (
    visibleNodeIds.has(edge.source) && visibleNodeIds.has(edge.target)
  )), [edges, visibleNodeIds])
  const selectedNode = selectedNodeId ? nodes.find((node) => node.id === selectedNodeId) : undefined
  const selectedEdge = selectedEdgeId ? edges.find((edge) => edge.id === selectedEdgeId) : undefined

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
        <div className={styles.searchSection}>
          <label className={styles.kicker} htmlFor="entity-search">Jump to entity</label>
          <div className={styles.searchField}>
            <Search size={15} aria-hidden="true" />
            <input
              id="entity-search"
              placeholder="Search a name or alias"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
            />
          </div>
          {searchError ? (
            <div className={styles.searchFailure} role="alert">
              <span>Search unavailable. {searchError}</span>
              <button type="button" onClick={() => setSearchRetry((attempt) => attempt + 1)}>Retry search</button>
            </div>
          ) : searchResults.length > 0 ? (
            <ul className={styles.searchResults} aria-label="Matching entities">
              {searchResults.map((entity) => (
                <li key={entity.id}>
                  <button
                    type="button"
                    onClick={() => {
                      void focusNode(entity.id)
                      setSearchQuery('')
                      setSearchResults([])
                      setSearchError(null)
                    }}
                  >
                    <span>{entity.name}</span>
                    <small>{entity.type.replace(/_/g, ' ')}</small>
                  </button>
                </li>
              ))}
            </ul>
          ) : searchQuery.trim() && !searching ? (
            <p className={styles.searchEmpty}>No matching entities.</p>
          ) : null}
        </div>

        {breadcrumb.length > 0 && (
          <button className={styles.backButton} type="button" onClick={() => void goBack()}>
            <ArrowLeft size={14} aria-hidden="true" />
            Previous focus
          </button>
        )}

        {selectedEdge ? (
          <section className={styles.focusedInspector} aria-labelledby="relationship-inspector-heading">
            <p className={styles.kicker}>Selected relationship</p>
            <h2 id="relationship-inspector-heading">{relationText(selectedEdge, nodes)}</h2>
            <dl className={styles.detailList}>
              <div>
                <dt>Relationship type</dt>
                <dd>{selectedEdge.relationshipType || 'Unavailable'}</dd>
              </div>
              <div>
                <dt>Evidence</dt>
                <dd>Unavailable — the neighborhood API does not provide relationship evidence.</dd>
              </div>
              <div>
                <dt>Source chapter</dt>
                <dd>Unavailable — the neighborhood API does not provide a source chapter.</dd>
              </div>
              <div>
                <dt>Related conflicts</dt>
                <dd>Unavailable — the neighborhood API does not include conflict context.</dd>
              </div>
            </dl>
          </section>
        ) : selectedNode ? (
          <section className={styles.focusedInspector} aria-labelledby="entity-inspector-heading">
            <p className={styles.kicker}>Focused entity</p>
            <h2 id="entity-inspector-heading">{selectedNode.data.label}</h2>
            <dl className={styles.detailList}>
              <div>
                <dt>Type</dt>
                <dd>{selectedNode.type.replace(/_/g, ' ')}</dd>
              </div>
              <div>
                <dt>Connections shown</dt>
                <dd>{edges.filter((edge) => edge.source === selectedNode.id || edge.target === selectedNode.id).length}</dd>
              </div>
              <div>
                <dt>Relevance</dt>
                <dd>{typeof selectedNode.data.relevanceScore === 'number' ? `${Math.round(selectedNode.data.relevanceScore * 100)}%` : 'Unavailable'}</dd>
              </div>
              <div>
                <dt>Related conflicts</dt>
                <dd>Unavailable — the neighborhood API does not include conflict context.</dd>
              </div>
            </dl>
          </section>
        ) : (
          <section className={styles.focusedInspector}>
            <p className={styles.kicker}>Map summary</p>
            <h2>{visibleNodes.length} entities · {visibleEdges.length} relationships</h2>
            <p>Search or tap an entity to center the map on it.</p>
          </section>
        )}

        <section className={styles.accessibleSummary} aria-labelledby="map-summary-heading">
          <div className={styles.summaryHeading}>
            <FileQuestion size={15} aria-hidden="true" />
            <div>
              <p className={styles.kicker}>Keyboard map</p>
              <h2 id="map-summary-heading">Entities and relationships</h2>
            </div>
          </div>
          <p className={styles.summaryCopy}>This list mirrors the visible map and can be used without the canvas.</p>

          <h3>Relationships</h3>
          {visibleEdges.length > 0 ? (
            <ul className={styles.relationshipList}>
              {visibleEdges.map((edge) => (
                <li key={edge.id}>
                  <button
                    type="button"
                    aria-pressed={selectedEdgeId === edge.id}
                    onClick={() => selectEdge(edge.id)}
                  >
                    {relationText(edge, nodes)}
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className={styles.unavailable}>No relationships are available for the visible entities.</p>
          )}

          <h3>Entities</h3>
          <ul className={styles.entityList}>
            {visibleNodes.map((node) => (
              <li key={node.id}>
                <button
                  type="button"
                  aria-pressed={selectedNodeId === node.id}
                  onClick={() => void focusNode(node.id)}
                >
                  <span>{node.data.label}</span>
                  <small>{node.type.replace(/_/g, ' ')}</small>
                </button>
              </li>
            ))}
          </ul>
        </section>

        {focalNodeId && <p className={styles.focalNote}>Showing the focal entity and its {limits?.hops || 2}-hop neighborhood in {universe?.name || 'this universe'}.</p>}
      </aside>
    </div>
  )
}
