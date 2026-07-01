import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useUniverseStore } from '../stores/universeStore'
import { useAuthStore } from '../stores/authStore'
import styles from './DashboardPage.module.css'

export default function DashboardPage() {
  const { universes, fetchUniverses } = useUniverseStore()
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()

  useEffect(() => { fetchUniverses() }, [])

  return (
    <div className={styles.layout}>
      <aside className={styles.sidebar}>
        <h1 className={styles.sidebarHeading}>Quill</h1>
        <p className={styles.sidebarSub}>Writer's Desk</p>

        <div className={styles.sidebarDivider} />

        <div className={styles.userSection}>
          <p className={styles.userName}>{user?.display_name}</p>
          <p className={styles.userEmail}>{user?.email}</p>
        </div>

        <div className={styles.sidebarDivider} />

        <div className={styles.stats}>
          <div className={styles.statItem}>
            <p className={styles.statLabel}>Universes</p>
            <p className={styles.statValue}>{universes.length}</p>
          </div>
        </div>

        <div className={styles.memoryBar}>
          <p className={styles.memoryLabel}>Memory</p>
          <div className={styles.memoryTrack}>
            <div className={styles.memoryFill} style={{ width: '24%' }} />
          </div>
          <p className={styles.memoryPercent}>24 GB</p>
        </div>

        <button className={styles.logoutBtn} onClick={logout}>
          Sign Out
        </button>
      </aside>

      <main className={styles.main}>
        <h2 className={styles.mainHeading}>Your Universes</h2>
        <p className={styles.mainSub}>Worlds waiting for ink</p>

        {universes.length === 0 ? (
          <div className={styles.emptyCard}>
            <p>No universes yet. Your first world awaits.</p>
          </div>
        ) : (
          <div className={styles.universeGrid}>
            {universes.map((u) => (
              <div
                key={u.id}
                className={styles.universeCard}
                onClick={() => navigate(`/universe/${u.id}`)}
              >
                <h3 className={styles.cardTitle}>{u.name}</h3>
                <div className={styles.cardMeta}>
                  <span className={styles.cardMetaItem}>{u.genre}</span>
                  <span className={styles.cardMetaItem}>{u.format}</span>
                </div>
                {/* ponytail: random progress for visual interest until real data exists */}
                <div className={styles.cardProgress}>
                  <p className={styles.cardProgressLabel}>Progress</p>
                  <div className={styles.cardProgressTrack}>
                    <div
                      className={styles.cardProgressFill}
                      style={{ width: `${(u.id.length % 40) + 20}%` }}
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
