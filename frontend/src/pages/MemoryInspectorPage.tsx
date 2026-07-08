import { useParams } from 'react-router-dom'
import { useState } from 'react'
import DecayTimeline from '../components/memory/DecayTimeline'
import FusionExplorer from '../components/memory/FusionExplorer'
import BudgetTheater from '../components/memory/BudgetTheater'
import type { RecallExplanation } from '../lib/types'
import styles from './MemoryInspectorPage.module.css'

// Composes the three "acts" of S3 Memory Theater. DecayTimeline owns its own
// "Advance chapter → run decay" control + refetch (design obs #265) — this
// page just renders it, no duplicate decay trigger needed here.
export default function MemoryInspectorPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const [recall, setRecall] = useState<RecallExplanation | null>(null)

  if (!universeId) return null

  return (
    <div className={styles.wrap}>
      <DecayTimeline universeId={universeId} />
      <FusionExplorer universeId={universeId} onResult={setRecall} />
      <BudgetTheater budget={recall?.budget ?? null} items={recall?.items ?? []} />
    </div>
  )
}
