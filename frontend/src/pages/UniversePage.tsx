import { useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useUniverseStore } from '../stores/universeStore'

export default function UniversePage() {
  const { universeId } = useParams<{ universeId: string }>()
  const { currentUniverse, works, selectUniverse } = useUniverseStore()
  const navigate = useNavigate()

  useEffect(() => {
    if (universeId) selectUniverse(universeId)
  }, [universeId])

  return (
    <div style={{ padding: 24 }}>
      <button onClick={() => navigate('/dashboard')} style={{ background: 'transparent', color: '#6c5ce7', marginBottom: 16 }}>
        ← Back to Dashboard
      </button>

      <h1>{currentUniverse?.name || 'Loading...'}</h1>
      <p style={{ color: '#888', marginBottom: 24 }}>{currentUniverse?.genre} · {currentUniverse?.format}</p>

      <h2 style={{ marginBottom: 16 }}>Works</h2>
      {works.length === 0 ? (
        <div className="card"><p>No works yet.</p></div>
      ) : (
        works.map((w) => (
          <div key={w.id} className="card" style={{ cursor: 'pointer' }} onClick={() => navigate(`/work/${w.id}`)}>
            <h3>{w.title}</h3>
            <p style={{ color: '#888' }}>{w.type}</p>
          </div>
        ))
      )}
    </div>
  )
}
