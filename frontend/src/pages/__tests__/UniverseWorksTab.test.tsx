import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import UniverseWorksTab from '../UniverseWorksTab'
import { UniverseContext } from '../../contexts/UniverseContext'

vi.mock('../UniverseWorksTab.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))
vi.mock('../../components/shared/ImageUpload', () => ({ default: () => null }))

const mockPublish = vi.fn(() => 'feedback-id')
vi.mock('../../components/feedback', () => ({
  useFeedback: () => ({ publish: mockPublish }),
}))

const mockDeleteWork = vi.fn()
const mockDeleteChapter = vi.fn()
const mockGetWork = vi.fn()
const mockListChapters = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    deleteWork: (...args: unknown[]) => mockDeleteWork(...args),
    deleteChapter: (...args: unknown[]) => mockDeleteChapter(...args),
    getWork: (...args: unknown[]) => mockGetWork(...args),
    listChapters: (...args: unknown[]) => mockListChapters(...args),
  },
}))

const universe = { id: 'uni-1', name: 'Universe', genre: 'fantasy', format: 'novel' }
const oneWork = [{ id: 'work-1', title: 'First Work', type: 'novel', order_index: 0 }]
const twoWorks = [
  { id: 'work-1', title: 'First Work', type: 'novel', order_index: 0 },
  { id: 'work-2', title: 'Second Work', type: 'novel', order_index: 1 },
]
const mockRefetchWorks = vi.fn().mockResolvedValue(undefined)

function renderTab(works = twoWorks) {
  return render(
    <MemoryRouter initialEntries={['/universe/uni-1/write']}>
      <UniverseContext.Provider value={{ universe, works, refetchWorks: mockRefetchWorks }}>
        <Routes>
          <Route path="/universe/:universeId/write" element={<UniverseWorksTab />} />
        </Routes>
      </UniverseContext.Provider>
    </MemoryRouter>
  )
}

describe('UniverseWorksTab single-screen workspace', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetWork.mockImplementation((id: string) =>
      Promise.resolve({ work: { id, title: id === 'work-1' ? 'First Work' : 'Second Work', type: 'novel', universe_id: 'uni-1' } })
    )
    mockListChapters.mockImplementation((workId: string) =>
      Promise.resolve({
        chapters: workId === 'work-1'
          ? [
              { id: 'ch-1', title: 'Chapter One', order_index: 1, word_count: 100, status: 'draft' },
              { id: 'ch-2', title: 'Chapter Two', order_index: 2, word_count: 200, status: 'analyzed' },
              { id: 'ch-3', title: 'Chapter Three', order_index: 3, word_count: 50, status: 'draft' },
            ]
          : [{ id: 'ch-9', title: 'Other Chapter', order_index: 1, word_count: 10, status: 'draft' }],
      })
    )
  })

  it('renders the hero and chapter list together for a single-work universe with no extra click', async () => {
    renderTab(oneWork)

    expect(await screen.findByText('First Work')).toBeInTheDocument()
    expect(screen.getByText('Chapter One')).toBeInTheDocument()
    expect(screen.getByText('Chapter Two')).toBeInTheDocument()
    expect(screen.getByText('Chapter Three')).toBeInTheDocument()
    expect(mockGetWork).toHaveBeenCalledWith('work-1')
  })

  it('does not render a work switcher when there is only one manuscript', async () => {
    renderTab(oneWork)
    await screen.findByText('First Work')
    expect(screen.queryByRole('tablist', { name: 'Manuscripts' })).not.toBeInTheDocument()
  })

  it('keeps manuscript creation and import reachable even with a single manuscript', async () => {
    renderTab(oneWork)
    await screen.findByText('First Work')
    expect(screen.getByRole('button', { name: 'New manuscript' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Import manuscript' })).toBeInTheDocument()
  })

  it('defaults to the first work and renders a switcher when there are several manuscripts', async () => {
    renderTab()
    await screen.findByText('Chapter One')
    expect(mockGetWork).toHaveBeenCalledWith('work-1')
    expect(screen.getByRole('tablist', { name: 'Manuscripts' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'First Work' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Second Work' })).toHaveAttribute('aria-selected', 'false')
  })

  it('switches the active manuscript without leaving the screen', async () => {
    renderTab()
    await screen.findByText('Chapter One')

    const user = userEvent.setup()
    await user.click(screen.getByRole('tab', { name: 'Second Work' }))

    await screen.findByText('Other Chapter')
    expect(mockGetWork).toHaveBeenCalledWith('work-2')
    expect(screen.queryByText('Chapter One')).not.toBeInTheDocument()
  })

  it('does not delete a work when inline deletion is cancelled', async () => {
    renderTab(oneWork)
    await screen.findByText('First Work')

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Delete First Work'))
    expect(screen.getByRole('alertdialog', { name: 'Confirm deletion' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(mockDeleteWork).not.toHaveBeenCalled()
    expect(mockRefetchWorks).not.toHaveBeenCalled()
  })

  it('deletes the active work and refetches after explicit confirmation', async () => {
    mockDeleteWork.mockResolvedValue(undefined)
    renderTab(oneWork)
    await screen.findByText('First Work')

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Delete First Work'))
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => {
      expect(mockDeleteWork).toHaveBeenCalledWith('work-1')
      expect(mockRefetchWorks).toHaveBeenCalled()
    })
  })

  it('does not delete a chapter when inline deletion is cancelled', async () => {
    renderTab(oneWork)
    await screen.findByText('Chapter One')

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Delete Chapter One'))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(mockDeleteChapter).not.toHaveBeenCalled()
  })

  it('deletes the chapter after explicit confirmation', async () => {
    mockDeleteChapter.mockResolvedValue(undefined)
    renderTab(oneWork)
    await screen.findByText('Chapter One')
    expect(mockListChapters).toHaveBeenCalledTimes(1)

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Delete Chapter One'))
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => {
      expect(mockDeleteChapter).toHaveBeenCalledWith('ch-1')
    })
  })
})
