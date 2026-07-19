import { useRef, useState } from 'react'
import { ArrowLeft, LogOut, User } from 'lucide-react'
import { NavLink, Outlet } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import styles from './ProfileLayout.module.css'

// The first account-scoped (not universe-nested) authenticated shell. Kept
// deliberately thin: one nav entry today (Writer profile); Phase 6 adds
// Integrations as a second entry to this same array, no restructuring needed.
const navigation = [
  { to: '/profile/memory', label: 'Writer profile' },
]

export default function ProfileLayout() {
  const { user, logout } = useAuthStore()
  const [accountOpen, setAccountOpen] = useState(false)
  const accountRef = useRef<HTMLDivElement>(null)
  const userInitial = (user?.display_name || user?.email || '?').charAt(0).toUpperCase()

  return (
    <div className={styles.shell}>
      <header className={styles.appBar}>
        <NavLink className={styles.brand} to="/dashboard" aria-label="Quill Home">
          <span className={styles.brandMark}>Q</span>
          <span>Quill</span>
        </NavLink>

        <NavLink className={styles.backLink} to="/dashboard">
          <ArrowLeft aria-hidden="true" size={15} />
          Back to Home
        </NavLink>

        <nav className={styles.primaryNav} aria-label="Account navigation">
          {navigation.map((item) => (
            <NavLink key={item.to} className={({ isActive }) => `${styles.navItem} ${isActive ? styles.navItemActive : ''}`} to={item.to}>
              {item.label}
            </NavLink>
          ))}
        </nav>

        <div className={styles.menuWrap} ref={accountRef}>
          <button
            className={styles.accountButton}
            type="button"
            aria-expanded={accountOpen}
            aria-haspopup="menu"
            aria-label="Open account menu"
            onClick={() => setAccountOpen((open) => !open)}
          >
            <span className={styles.userAvatar}>{userInitial}</span>
            <User aria-hidden="true" size={15} />
          </button>
          {accountOpen && (
            <div className={styles.menu} role="menu" aria-label="Account menu">
              <span className={styles.accountName}>{user?.display_name || user?.email || 'Writer'}</span>
              <button className={styles.menuItem} role="menuitem" type="button" onClick={logout}>
                <LogOut aria-hidden="true" size={15} />
                Sign out
              </button>
            </div>
          )}
        </div>
      </header>

      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  )
}
