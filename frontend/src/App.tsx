import { Suspense, lazy } from 'react'
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
import WorkPage from './pages/WorkPage'

// ponytail: lazy-loaded — keeps landing page out of main bundle
const LandingPage = lazy(() => import('./pages/LandingPage'))
const EntityCardPage = lazy(() => import('./pages/EntityCardPage'))

// ponytail: bare `editor`/`entities` (no id yet) just send the writer back to
// Works to pick something — the real "no chapter/entity selected" screens are
// part of tasks 11/12/18, not this routing pass.
function ToWorks() {
  const { universeId } = useParams<{ universeId: string }>()
  return <Navigate to={`/universe/${universeId}/works`} replace />
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" />
}

export default function App() {
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
          <Route path="entities" element={<ToWorks />} />
          <Route path="entities/:entityId" element={<EntityCardPage />} />
          <Route path="graph" element={<KnowledgeGraphPage />} />
          <Route path="timeline" element={<TimelinePage />} />
          <Route path="contradictions" element={<ContradictionsPage />} />
          <Route path="plot-holes" element={<PlotHolesPage />} />
          <Route path="ingest" element={<IngestPage />} />
        </Route>
        <Route path="/work/:workId" element={<ProtectedRoute><WorkPage /></ProtectedRoute>} />
        {/* Legacy top-level deep links — redirect into the nested shell (ADR-3, RISK-4) */}
        <Route path="/editor/:chapterId" element={<ProtectedRoute><EditorRedirect /></ProtectedRoute>} />
        <Route path="/entity/:entityId" element={<ProtectedRoute><EntityRedirect /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/dashboard" />} />
      </Routes>
      </Suspense>
    </BrowserRouter>
  )
}
