import { useCallback } from 'react'
import { useWSStore, type WSStatus } from '../../stores/wsStore'
import ContextCard from './ContextCard'
import styles from './ContextPanel.module.css'

interface ContextPanelProps {
  status: WSStatus
}

export default function ContextPanel({ status }: ContextPanelProps) {
  const contradictions = useWSStore((s) => s.contradictions)
  const discoveredEntities = useWSStore((s) => s.discoveredEntities)
  const recallItems = useWSStore((s) => s.recallItems)
  const graphPings = useWSStore((s) => s.graphPings)

  const dismissRecall = useCallback((id: string) => {
    useWSStore.setState((s) => ({
      recallItems: s.recallItems.filter((r) => r.id !== id),
    }))
  }, [])

  const dismissContradiction = useCallback((id: string) => {
    useWSStore.setState((s) => ({
      contradictions: s.contradictions.filter((c) => c.id !== id),
    }))
  }, [])

  const dismissEntity = useCallback((id: string) => {
    useWSStore.setState((s) => ({
      discoveredEntities: s.discoveredEntities.filter((e) => e.id !== id),
    }))
  }, [])

  const statusClass =
    status === 'open' ? styles.statusOpen : status === 'reconnecting' ? styles.statusReconnecting : styles.statusClosed
  const totalCards = recallItems.length + contradictions.length + discoveredEntities.length + graphPings.length

  return (
    <div className={styles.panel}>
      <div className={styles.panelHeader}>
        <h3 className={styles.panelTitle}>
          Context Panel
          <span className={`glyph ${styles.statusIndicator} ${statusClass}`} title={`WS: ${status}`}>
            ●
          </span>
        </h3>
        {totalCards > 0 && (
          <span className={styles.cardCount}>{totalCards} active</span>
        )}
      </div>

      <div className={styles.cardList}>
        {totalCards === 0 && (
          <p className={styles.emptyState}>
            AI insights will appear here as you write
          </p>
        )}

        {/* Contradictions first (highest priority) */}
        {contradictions.map((c) => (
          <ContextCard
            key={c.id || String(Math.random())}
            id={c.id || String(Math.random())}
            type="contradiction"
            title="Contradiction"
            detail={c.message || String(c)}
            severity={(c.severity as 'low' | 'medium' | 'high') || 'medium'}
            onDismiss={dismissContradiction}
          />
        ))}

        {/* New entities */}
        {discoveredEntities.map((e) => (
          <ContextCard
            key={e.id || String(Math.random())}
            id={e.id || String(Math.random())}
            type="entity"
            title={e.name || 'New Entity'}
            detail={`Type: ${e.type || 'unknown'}`}
            isNew
            onDismiss={dismissEntity}
          />
        ))}

        {/* Recall items */}
        {recallItems.map((r) => (
          <ContextCard
            key={r.id || String(Math.random())}
            id={r.id || String(Math.random())}
            type="recall"
            title={r.fact || 'Recall'}
            detail={r.score ? `Confidence: ${(r.score * 100).toFixed(0)}%` : ''}
            onDismiss={dismissRecall}
          />
        ))}

        {/* Graph pings */}
        {graphPings.map((_g, i) => (
          <ContextCard
            key={`graph-${i}`}
            id={`graph-${i}`}
            type="recall"
            title="Graph Updated"
            detail="Knowledge graph has new connections"
            onDismiss={() => {
              useWSStore.setState((s) => ({
                graphPings: s.graphPings.filter((_, idx) => idx !== i),
              }))
            }}
          />
        ))}
      </div>
    </div>
  )
}
