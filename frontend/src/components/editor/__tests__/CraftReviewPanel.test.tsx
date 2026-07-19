import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import CraftReviewPanel from '../CraftReviewPanel'
import { api } from '../../../lib/api'

vi.mock('../../../lib/api', () => ({
  api: {
    getSkills: vi.fn(),
    getUniverseSkills: vi.fn(),
    submitWriterFeedback: vi.fn(),
  },
}))

describe('CraftReviewPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getSkills).mockResolvedValue({
      skills: [
        { name: 'pacing', description: 'Maintain scene momentum.', genre_tags: [], stage: 'craft' },
      ],
    })
    vi.mocked(api.getUniverseSkills).mockResolvedValue({
      skills: [{ universe_id: 'universe-1', skill_name: 'pacing', activated_at: '2026-07-17T00:00:00Z' }],
    })
  })

  it('toggles the craft-check disclosure and reports manual and automatic selections', async () => {
    const onSelectedSkillsChange = vi.fn()
    const user = userEvent.setup()
    const view = render(
      <CraftReviewPanel
        review={null}
        universeId="universe-1"
        workId="work-1"
        chapterId="chapter-1"
        selectedSkills={null}
        onSelectedSkillsChange={onSelectedSkillsChange}
      />,
    )

    await waitFor(() => expect(screen.getByText(/Quill will run the review using your configured craft checks/)).toBeInTheDocument())
    expect(screen.queryByRole('checkbox', { name: /Pacing/ })).not.toBeInTheDocument()

    const checksSummary = screen.getByText('Craft checks for the next review')
    const checksDisclosure = checksSummary.closest('details')
    const resultsDisclosure = screen.getByText('Review results').closest('details')
    expect(checksDisclosure).toHaveProperty('open', true)
    expect(resultsDisclosure).toHaveProperty('open', true)

    await user.click(checksSummary)
    expect(checksDisclosure).toHaveProperty('open', false)
    await user.click(checksSummary)
    expect(checksDisclosure).toHaveProperty('open', true)

    await user.click(screen.getByRole('button', { name: 'Choose specific checks instead' }))
    await user.click(screen.getByRole('checkbox', { name: /Pacing/ }))
    expect(onSelectedSkillsChange).toHaveBeenLastCalledWith(['pacing'])

    view.rerender(
      <CraftReviewPanel
        review={null}
        universeId="universe-1"
        workId="work-1"
        chapterId="chapter-1"
        selectedSkills={['pacing']}
        onSelectedSkillsChange={onSelectedSkillsChange}
      />,
    )
    await user.click(screen.getByRole('button', { name: 'Let Quill choose' }))
    expect(onSelectedSkillsChange).toHaveBeenLastCalledWith(null)
  })

  it('shows the picker immediately when it mounts with a pre-existing manual selection', async () => {
    render(
      <CraftReviewPanel
        review={null}
        universeId="universe-1"
        workId="work-1"
        chapterId="chapter-1"
        selectedSkills={['pacing']}
        onSelectedSkillsChange={vi.fn()}
      />,
    )

    await waitFor(() => expect(screen.getByRole('checkbox', { name: /Pacing/ })).toBeInTheDocument())
    expect(screen.getByRole('checkbox', { name: /Pacing/ })).toBeChecked()
    expect(screen.queryByText(/Quill will run the review using your configured craft checks/)).not.toBeInTheDocument()
  })
})
