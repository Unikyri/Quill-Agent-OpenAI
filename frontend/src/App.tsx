import { lazy, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useParams } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import UniverseLayout from './pages/UniverseLayout'
import EditorRedirect from './pages/EditorRedirect'
import EntityRedirect from './pages/EntityRedirect'
import WorkRedirect from './pages/WorkRedirect'
import { explorePath, reviewPath, writeImportPath, writePath, type ReviewView } from './lib/canonicalRoutes'
import { RouteLoadBoundary } from './components/shared/RouteLoadBoundary'

// ponytail: lazy-loaded — keeps landing page out of main bundle
const LandingPage = lazy(() => import('./pages/LandingPage'))
const UniverseWorksTab = lazy(() => import('./pages/UniverseWorksTab'))
const EditorPage = lazy(() => import('./pages/EditorPage'))
// The relationship map is intentionally isolated from the initial Home/Write bundle.
const KnowledgeGraphPage = lazy(() => import('./pages/KnowledgeGraphPage'))
const MemoryInspectorPage = lazy(() => import('./pages/MemoryInspectorPage'))
const ReviewPage = lazy(() => import('./pages/ReviewPage'))

function MissingUniverseRedirect() {
  return <Navigate to="/dashboard" replace />
}

function ToWrite({ importMode = false }: { importMode?: boolean }) {
  const { universeId, chapterId } = useParams<{ universeId: string; chapterId?: string }>()
  if (!universeId) return <MissingUniverseRedirect />
  return <Navigate to={importMode ? writeImportPath(universeId) : writePath(universeId, chapterId)} replace />
}

// EntitiesPage was removed (fully absorbed into KnowledgeGraphPage's left
// pane + tabbed detail); every legacy/entity-scoped Explore link now folds
// into the consolidated Story Graph map.
function ToExplore() {
  const { universeId } = useParams<{ universeId: string }>()
  if (!universeId) return <MissingUniverseRedirect />
  return <Navigate to={explorePath(universeId, 'map')} replace />
}

function ToReview({ view }: { view: ReviewView }) {
  const { universeId } = useParams<{ universeId: string }>()
  if (!universeId) return <MissingUniverseRedirect />
  return <Navigate to={reviewPath(universeId, view)} replace />
}

function WriteRoute() {
  return (
    <RouteLoadBoundary label="Loading writing workspace…">
      <UniverseWorksTab />
    </RouteLoadBoundary>
  )
}

function MapRoute() {
  return (
    <RouteLoadBoundary label="Loading relationship map…">
      <KnowledgeGraphPage />
    </RouteLoadBoundary>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" />
}

export function AppRoutes() {
  return (
    <Routes>
        <Route path="/" element={<RouteLoadBoundary label="Loading home…"><LandingPage /></RouteLoadBoundary>} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/dashboard" element={<ProtectedRoute><DashboardPage /></ProtectedRoute>} />
        <Route path="/universe/:universeId" element={<ProtectedRoute><UniverseLayout /></ProtectedRoute>}>
          <Route index element={<Navigate to="write" replace />} />

          {/* Canonical Sprint 7 destinations. Existing feature pages are interim bodies. */}
          <Route path="write" element={<WriteRoute />} />
          <Route path="write/:chapterId" element={<RouteLoadBoundary label="Loading editor…"><EditorPage /></RouteLoadBoundary>} />
          {/* EntitiesPage was removed as a duplicate entity-browsing surface (its
              list/filter/detail behavior is fully absorbed into the Story
              Graph's left pane + tabbed detail panel); legacy links redirect
              to the consolidated map. */}
          <Route path="explore/entities" element={<ToExplore />} />
          <Route path="explore/entities/:entityId" element={<ToExplore />} />
          <Route path="explore/map" element={<MapRoute />} />
          {/* The timeline is now a slider embedded in the map (see KnowledgeGraphPage) rather than its own page. */}
          <Route path="explore/timeline" element={<ToExplore />} />
          <Route path="memory" element={<RouteLoadBoundary label="Loading memory…"><MemoryInspectorPage /></RouteLoadBoundary>} />
          <Route path="review/:view" element={<RouteLoadBoundary label="Loading review…"><ReviewPage /></RouteLoadBoundary>} />
          {/* SkillsPage was removed as a duplicate skill-config surface (CraftReviewPanel's
              inline picker is the only one now); legacy links redirect to Conflicts. */}
          <Route path="review/skills" element={<ToReview view="issues" />} />

          {/* Legacy universe-scoped deep links remain explicit redirects while their
              content is consolidated by the Write, Explore, and Review work. */}
          <Route path="panorama" element={<Navigate to="/dashboard" replace />} />
          <Route path="works" element={<ToWrite />} />
          <Route path="editor" element={<ToWrite />} />
          <Route path="editor/:chapterId" element={<ToWrite />} />
          <Route path="entities" element={<ToExplore />} />
          <Route path="entities/:entityId" element={<ToExplore />} />
          <Route path="graph" element={<ToExplore />} />
          <Route path="timeline" element={<ToExplore />} />
          <Route path="contradictions" element={<ToReview view="issues" />} />
          <Route path="plot-holes" element={<ToReview view="issues" />} />
          <Route path="ingest" element={<ToWrite importMode />} />
          <Route path="skills" element={<ToReview view="issues" />} />
          <Route path="*" element={<ToWrite />} />
        </Route>
        {/* Legacy redirects */}
        <Route path="/work/:workId" element={<ProtectedRoute><WorkRedirect /></ProtectedRoute>} />
        <Route path="/editor/:chapterId" element={<ProtectedRoute><EditorRedirect /></ProtectedRoute>} />
        <Route path="/entity/:entityId" element={<ProtectedRoute><EntityRedirect /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/dashboard" />} />
    </Routes>
  )
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
      <AppRoutes />
    </BrowserRouter>
  )
}
