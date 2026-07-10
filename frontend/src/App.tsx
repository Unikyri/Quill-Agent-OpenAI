import { Suspense, lazy, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useParams } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import UniverseLayout from './pages/UniverseLayout'
import UniverseWorksTab from './pages/UniverseWorksTab'
import PanoramaPage from './pages/PanoramaPage'
import IngestPage from './pages/IngestPage'
import KnowledgeGraphPage from './pages/KnowledgeGraphPage'
import TimelinePage from './pages/TimelinePage'
import ContradictionsPage from './pages/ContradictionsPage'
import PlotHolesPage from './pages/PlotHolesPage'
import EditorPage from './pages/EditorPage'
import EditorRedirect from './pages/EditorRedirect'
import EntityRedirect from './pages/EntityRedirect'
import EntitiesPage from './pages/EntitiesPage'
import MemoryInspectorPage from './pages/MemoryInspectorPage'

// ponytail: lazy-loaded — keeps landing page out of main bundle
const LandingPage = lazy(() => import('./pages/LandingPage'))

// ponytail: redirect bare `editor` (no chapter) to Works to pick one
function ToWorks() {
  const { universeId } = useParams<{ universeId: string }>()
  return <Navigate to={`/universe/${universeId}/works`} replace />
}

// ponytail: /work/:workId still in the wild (bookmarks, emails etc) — redirects to dashboard
// because we can't look up universeId from workId on the client alone.
function WorkRedirect() {
  return <Navigate to="/dashboard" replace />
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" />
}

export default function App() {
  // Hydrate the user from the stored token on mount, so a reloaded session
  // shows the real display_name instead of a "?" fallback until some other
  // action happens to trigger it.
  useEffect(() => {
    useAuthStore.getState().init()
  }, [])

  return (
    <BrowserRouter>
      <Suspense fallback={<div style={{ minHeight: '100vh', background: 'var(--bg-app)' }} />}>
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/dashboard" element={<ProtectedRoute><DashboardPage /></ProtectedRoute>} />
        <Route path="/universe/:universeId" element={<ProtectedRoute><UniverseLayout /></ProtectedRoute>}>
          <Route index element={<Navigate to="panorama" replace />} />
          <Route path="panorama" element={<PanoramaPage />} />
          <Route path="works" element={<UniverseWorksTab />} />
          <Route path="editor" element={<ToWorks />} />
          <Route path="editor/:chapterId" element={<EditorPage />} />
          {/* Entities — split-pane: list + optional entity detail in same component */}
          <Route path="entities" element={<EntitiesPage />} />
          <Route path="entities/:entityId" element={<EntitiesPage />} />
          <Route path="graph" element={<KnowledgeGraphPage />} />
          <Route path="timeline" element={<TimelinePage />} />
          <Route path="contradictions" element={<ContradictionsPage />} />
          <Route path="plot-holes" element={<PlotHolesPage />} />
          <Route path="ingest" element={<IngestPage />} />
          <Route path="memory" element={<MemoryInspectorPage />} />
        </Route>
        {/* Legacy redirects */}
        <Route path="/work/:workId" element={<ProtectedRoute><WorkRedirect /></ProtectedRoute>} />
        <Route path="/editor/:chapterId" element={<ProtectedRoute><EditorRedirect /></ProtectedRoute>} />
        <Route path="/entity/:entityId" element={<ProtectedRoute><EntityRedirect /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/dashboard" />} />
      </Routes>
      </Suspense>
    </BrowserRouter>
  )
}
