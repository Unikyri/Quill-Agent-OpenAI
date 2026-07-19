import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import BudgetTheater from '../components/memory/BudgetTheater'
import DecayTimeline from '../components/memory/DecayTimeline'
import FusionExplorer from '../components/memory/FusionExplorer'
import type { RecallExplanation } from '../lib/types'
import styles from './MemoryInspectorPage.module.css'

// Recall, Forgetting, and Context Budget render as three always-visible
// stacked "acts" (Docs/FRONTEND-GREENFIELD-PLAN.md §4.7) — none may be
// hidden behind a closed disclosure on initial load. Each component already
// carries its own act header (kicker + heading), so no extra wrapper is
// needed here beyond stacking them with consistent rhythm.
export default function MemoryInspectorPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const [recall, setRecall] = useState<RecallExplanation | null>(null)
  const [recallUniverseId, setRecallUniverseId] = useState<string | null>(null)

  useEffect(() => {
    setRecall(null)
    setRecallUniverseId(null)
  }, [universeId])

  const handleRecallResult = useCallback((nextRecall: RecallExplanation | null) => {
    setRecall(nextRecall)
    setRecallUniverseId(nextRecall && universeId ? universeId : null)
  }, [universeId])

  const currentRecall = recallUniverseId === universeId ? recall : null

  if (!universeId) return null

  return (
    <main className={styles.wrap}>
      <FusionExplorer key={`recall-${universeId}`} universeId={universeId} onResult={handleRecallResult} />
      <DecayTimeline key={`forgetting-${universeId}`} universeId={universeId} />
      <BudgetTheater budget={currentRecall?.budget ?? null} items={currentRecall?.items ?? []} />
    </main>
  )
}
