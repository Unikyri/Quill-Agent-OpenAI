import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import SkillsPage from '../SkillsPage'

vi.mock('../SkillsPage.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))

const mockGetSkills = vi.fn()
const mockGetUniverseSkills = vi.fn()
const mockUpdateUniverseSkills = vi.fn()

vi.mock('../../lib/api', () => ({
  api: {
    getSkills: (...args: unknown[]) => mockGetSkills(...args),
    getUniverseSkills: (...args: unknown[]) => mockGetUniverseSkills(...args),
    updateUniverseSkills: (...args: unknown[]) => mockUpdateUniverseSkills(...args),
  },
}))

const catalogue = [{
  name: 'line-edit',
  description: 'Tightens sentences without changing the writer’s voice.',
  genre_tags: ['literary'],
  stage: 'craft',
}]

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/universe/uni-1/skills']}>
      <Routes>
        <Route path="/universe/:universeId/skills" element={<SkillsPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('SkillsPage', () => {
  it('shows a retryable load error without presenting stale skill choices', async () => {
    mockGetSkills
      .mockRejectedValueOnce(new Error('Catalogue unavailable'))
      .mockResolvedValueOnce({ skills: catalogue })
    mockGetUniverseSkills.mockResolvedValue({ skills: [] })

    renderPage()

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(/catalogue unavailable/i))
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Retry loading skills' }))

    await waitFor(() => expect(mockGetSkills).toHaveBeenCalledTimes(2))
    expect(screen.getByRole('checkbox', { name: /line edit/i })).toBeInTheDocument()
  })

  it('keeps selections unsaved and explains recovery after a save failure', async () => {
    mockGetSkills.mockResolvedValue({ skills: catalogue })
    mockGetUniverseSkills.mockResolvedValue({ skills: [] })
    mockUpdateUniverseSkills.mockRejectedValue(new Error('Save unavailable'))
    renderPage()

    const user = userEvent.setup()
    const checkbox = await screen.findByRole('checkbox', { name: /line edit/i })
    await user.click(checkbox)
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(/save unavailable/i))
    expect(checkbox).toBeChecked()
    expect(mockUpdateUniverseSkills).toHaveBeenCalledWith('uni-1', ['line-edit'])
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  })
})
