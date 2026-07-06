import { useNavigate } from 'react-router-dom'
import styles from './PlotHoleList.module.css'

export interface PlotHole {
  id: string
  title: string
  description?: string
  status: string
  related_entity_ids?: string[]
  first_mentioned_chapter_id?: string
}

interface PlotHoleListProps {
  plotHoles: PlotHole[]
  universeId: string
}

export default function PlotHoleList({ plotHoles, universeId }: PlotHoleListProps) {
  const navigate = useNavigate()

  return (
    <div className={styles.listWrap}>
      {plotHoles.map((ph) => (
        <div key={ph.id} className={styles.card}>
          <div className={styles.cardHeader}>
            <span className={styles.kicker}>OPEN THREAD</span>
          </div>
          <h3 className={styles.title}>{ph.title}</h3>
          {ph.description && <p className={styles.cardDesc}>{ph.description}</p>}
          <div className={styles.cardFooter}>
            {/* ponytail: entity chip needs a name lookup by id (no entity data
                plumbed to this list); showing raw ids would be worse than
                omitting — deferred until a universe-wide entity map is passed in. */}
            {ph.first_mentioned_chapter_id && (
              <button
                className={styles.chapterLink}
                onClick={() => navigate(`/universe/${universeId}/editor/${ph.first_mentioned_chapter_id}`)}
              >
                Go to chapter →
              </button>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
