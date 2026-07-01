import { Suspense, lazy } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import UniverseLayout from './pages/UniverseLayout'
import UniverseWorksTab from './pages/UniverseWorksTab'
import KnowledgeGraphPage from './pages/KnowledgeGraphPage'
import TimelinePage from './pages/TimelinePage'
import ContradictionsPage from './pages/ContradictionsPage'
import PlotHolesPage from './pages/PlotHolesPage'
import EditorPage from './pages/EditorPage'
import WorkPage from './pages/WorkPage'

// ponytail: lazy-loaded — GSAP stays in this chunk, not the main bundle
const LandingPage = lazy(() => import('./pages/LandingPage'))

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" />
}

export default function App() {
  return (
    <BrowserRouter>
      <Suspense fallback={<div style={{ minHeight: '100vh', background: 'var(--paper)' }} />}>
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/dashboard" element={<ProtectedRoute><DashboardPage /></ProtectedRoute>} />
        <Route path="/universe/:universeId" element={<ProtectedRoute><UniverseLayout /></ProtectedRoute>}>
          <Route index element={<Navigate to="works" replace />} />
          <Route path="works" element={<UniverseWorksTab />} />
          <Route path="graph" element={<KnowledgeGraphPage />} />
          <Route path="timeline" element={<TimelinePage />} />
          <Route path="contradictions" element={<ContradictionsPage />} />
          <Route path="plot-holes" element={<PlotHolesPage />} />
        </Route>
        <Route path="/work/:workId" element={<ProtectedRoute><WorkPage /></ProtectedRoute>} />
        <Route path="/editor/:chapterId" element={<ProtectedRoute><EditorPage /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/dashboard" />} />
      </Routes>
      </Suspense>
    </BrowserRouter>
  )
}
