import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import WorkPage from '../WorkPage'

// CSS module mock
vi.mock('../WorkPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

// Mock api
const mockGetWork = vi.fn()
const mockListChapters = vi.fn()
const mockUpdateWork = vi.fn()
const mockUpdateChapter = vi.fn()
const mockCreateChapter = vi.fn()

vi.mock('../../lib/api', () => ({
  api: {
    getWork: (...args: unknown[]) => mockGetWork(...args),
    listChapters: (...args: unknown[]) => mockListChapters(...args),
    updateWork: (...args: unknown[]) => mockUpdateWork(...args),
    updateChapter: (...args: unknown[]) => mockUpdateChapter(...args),
    createChapter: (...args: unknown[]) => mockCreateChapter(...args),
  },
}))

function renderPage(workId = 'work-123') {
  return render(
    <MemoryRouter initialEntries={[`/work/${workId}`]}>
      <Routes>
        <Route path="/work/:workId" element={<WorkPage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('WorkPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading state initially', () => {
    mockGetWork.mockReturnValue(new Promise(() => {})) // never resolves
    mockListChapters.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders work title and chapters on load', async () => {
    mockGetWork.mockResolvedValue({
      work: { id: 'work-123', title: 'My Novel', type: 'Novel', synopsis: '' },
    })
    mockListChapters.mockResolvedValue({
      chapters: [
        { id: 'ch-1', title: 'Chapter 1', order_index: 1, word_count: 500, status: 'draft' },
        { id: 'ch-2', title: 'Chapter 2', order_index: 2, word_count: 300, status: 'draft' },
      ],
    })

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('My Novel')).toBeInTheDocument()
    })

    expect(screen.getByText('Novel')).toBeInTheDocument()
    expect(screen.getByText('Chapter 1')).toBeInTheDocument()
    expect(screen.getByText('Chapter 2')).toBeInTheDocument()
    expect(screen.getByText('500 words')).toBeInTheDocument()
    expect(screen.getByText('300 words')).toBeInTheDocument()
  })

  it('shows error message on fetch failure', async () => {
    mockGetWork.mockRejectedValue(new Error('Network error'))
    mockListChapters.mockRejectedValue(new Error('Network error'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/Network error/)).toBeInTheDocument()
    })
  })

  it('shows empty state when no chapters', async () => {
    mockGetWork.mockResolvedValue({
      work: { id: 'work-123', title: 'Empty Work', type: 'Novel', synopsis: '' },
    })
    mockListChapters.mockResolvedValue({ chapters: [] })

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('No chapters yet.')).toBeInTheDocument()
    })
  })

  it('edits the title inline and saves via updateWork', async () => {
    mockGetWork.mockResolvedValue({
      work: { id: 'work-123', title: 'Old Title', type: 'Novel', synopsis: '' },
    })
    mockListChapters.mockResolvedValue({ chapters: [] })
    mockUpdateWork.mockResolvedValue({ work: { id: 'work-123', title: 'New Title' } })

    renderPage()
    await waitFor(() => expect(screen.getByText('Old Title')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Edit title'))
    const input = screen.getByDisplayValue('Old Title')
    await user.clear(input)
    await user.type(input, 'New Title{Enter}')

    await waitFor(() => {
      expect(mockUpdateWork).toHaveBeenCalledWith('work-123', { title: 'New Title' })
      expect(screen.getByText('New Title')).toBeInTheDocument()
    })
  })

  it('renames a chapter inline and saves via updateChapter', async () => {
    mockGetWork.mockResolvedValue({
      work: { id: 'work-123', title: 'My Novel', type: 'Novel', synopsis: '' },
    })
    mockListChapters.mockResolvedValue({
      chapters: [{ id: 'ch-1', title: 'Chapter One', order_index: 1, word_count: 100, status: 'draft' }],
    })
    mockUpdateChapter.mockResolvedValue({ chapter: { id: 'ch-1', title: 'Renamed Chapter' } })

    renderPage()
    await waitFor(() => expect(screen.getByText('Chapter One')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Rename chapter'))
    const input = screen.getByDisplayValue('Chapter One')
    await user.clear(input)
    await user.type(input, 'Renamed Chapter{Enter}')

    await waitFor(() => {
      expect(mockUpdateChapter).toHaveBeenCalledWith('ch-1', { title: 'Renamed Chapter' })
      expect(screen.getByText('Renamed Chapter')).toBeInTheDocument()
    })
  })

  it('disables the Create button while a chapter creation is in flight and does not double-submit', async () => {
    mockGetWork.mockResolvedValue({
      work: { id: 'work-123', title: 'My Novel', type: 'Novel', synopsis: '' },
    })
    mockListChapters.mockResolvedValue({ chapters: [] })
    let resolveCreate: (value: { chapter: { id: string } }) => void = () => {}
    mockCreateChapter.mockReturnValue(new Promise((resolve) => { resolveCreate = resolve }))

    renderPage()
    await waitFor(() => expect(screen.getByText('My Novel')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByText('+ New Chapter'))
    await user.type(screen.getByPlaceholderText('Chapter title'), 'Chapter One')

    const createBtn = screen.getByText('Create')
    await user.click(createBtn)
    await user.click(createBtn)

    expect(createBtn).toBeDisabled()
    expect(mockCreateChapter).toHaveBeenCalledTimes(1)

    resolveCreate({ chapter: { id: 'ch-new' } })
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/editor/ch-new'))
  })
})
