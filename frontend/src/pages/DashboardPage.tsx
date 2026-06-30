import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useUniverseStore } from '../stores/universeStore'
import { useAuthStore } from '../stores/authStore'

export default function DashboardPage() {
  const { universes, fetchUniverses } = useUniverseStore()
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()

  useEffect(() => { fetchUniverses() }, [])

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 24 }}>
        <h1>Quill Dashboard</h1>
        <div>
          <span style={{ marginRight: 16, color: '#888' }}>{user?.display_name}</span>
          <button onClick={logout} style={{ background: '#333', color: '#e0e0e0' }}>Logout</button>
        </div>
      </div>

      <h2 style={{ marginBottom: 16 }}>Your Universes</h2>
      {universes.length === 0 ? (
        <div className="card">
          <p>No universes yet. Create your first one!</p>
        </div>
      ) : (
        universes.map((u) => (
          <div key={u.id} className="card" style={{ cursor: 'pointer' }} onClick={() => navigate(`/universe/${u.id}`)}>
            <h3>{u.name}</h3>
            <p style={{ color: '#888' }}>{u.genre} · {u.format}</p>
          </div>
        ))
      )}
    </div>
  )
}
