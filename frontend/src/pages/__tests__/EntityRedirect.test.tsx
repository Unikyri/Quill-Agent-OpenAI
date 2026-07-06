import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import EntityRedirect from '../EntityRedirect'

const mockGetEntity = vi.fn()
vi.mock('../../lib/api', () => ({
  api: { getEntity: (...args: unknown[]) => mockGetEntity(...args) },
}))

function Target() {
  return <div>Nested entity screen</div>
}

function Dashboard() {
  return <div>Dashboard screen</div>
}

function renderRedirect(entityId = 'ent-1') {
  return render(
    <MemoryRouter initialEntries={[`/entity/${entityId}`]}>
      <Routes>
        <Route path="/entity/:entityId" element={<EntityRedirect />} />
        <Route path="/universe/:universeId/entities/:entityId" element={<Target />} />
        <Route path="/dashboard" element={<Dashboard />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('EntityRedirect', () => {
  beforeEach(() => vi.clearAllMocks())

  it('redirects to the nested universe-scoped entities route once the entity resolves', async () => {
    mockGetEntity.mockResolvedValue({ entity: { id: 'ent-1', universe_id: 'uni-2' } })
    renderRedirect('ent-1')

    await waitFor(() => {
      expect(screen.getByText('Nested entity screen')).toBeInTheDocument()
    })
  })

  it('redirects to /dashboard when the entity fetch fails', async () => {
    mockGetEntity.mockRejectedValue(new Error('not found'))
    renderRedirect('ent-1')

    await waitFor(() => {
      expect(screen.getByText('Dashboard screen')).toBeInTheDocument()
    })
  })
})
