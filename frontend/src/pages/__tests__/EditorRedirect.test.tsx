import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import EditorRedirect from '../EditorRedirect'

const mockGetChapter = vi.fn()
vi.mock('../../lib/api', () => ({
  api: { getChapter: (...args: unknown[]) => mockGetChapter(...args) },
}))

function Target() {
  return <div>Nested editor screen</div>
}

function Dashboard() {
  return <div>Dashboard screen</div>
}

function renderRedirect(chapterId = 'ch-1') {
  return render(
    <MemoryRouter initialEntries={[`/editor/${chapterId}`]}>
      <Routes>
        <Route path="/editor/:chapterId" element={<EditorRedirect />} />
        <Route path="/universe/:universeId/editor/:chapterId" element={<Target />} />
        <Route path="/dashboard" element={<Dashboard />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('EditorRedirect', () => {
  beforeEach(() => vi.clearAllMocks())

  it('redirects to the nested universe-scoped editor route once the chapter resolves', async () => {
    mockGetChapter.mockResolvedValue({ chapter: { id: 'ch-1', universe_id: 'uni-1' } })
    renderRedirect('ch-1')

    await waitFor(() => {
      expect(screen.getByText('Nested editor screen')).toBeInTheDocument()
    })
  })

  it('redirects to /dashboard when the chapter fetch fails', async () => {
    mockGetChapter.mockRejectedValue(new Error('not found'))
    renderRedirect('ch-1')

    await waitFor(() => {
      expect(screen.getByText('Dashboard screen')).toBeInTheDocument()
    })
  })
})
