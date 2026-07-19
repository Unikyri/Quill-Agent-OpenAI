import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import EditorPage from '../EditorPage'
import type { CraftReviewResult } from '../../lib/types'

vi.mock('../EditorPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))
vi.mock('../../components/editor/IngestPanel', () => ({ IngestPanel: () => null }))

const mockPublish = vi.fn(() => 'feedback-id')
const mockFeedbackUpdate = vi.fn()
vi.mock('../../components/feedback', () => ({
  useFeedback: () => ({ publish: mockPublish, update: mockFeedbackUpdate }),
}))

const mockNavigate = vi.fn()
const mockRouteParams = vi.hoisted(() => ({ universeId: 'uni-1', chapterId: 'ch-1' }))
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate, useParams: () => mockRouteParams }
})

const mockGetChapter = vi.fn()
const mockGetWork = vi.fn()
const mockListChapters = vi.fn()
const mockCreateChapter = vi.fn()
const mockStoreState = vi.hoisted(() => ({
  ws: {
    status: 'open',
    lastError: null as string | null,
    lastErrorRequestId: null as string | null,
    send: vi.fn(),
    clearError: vi.fn(() => {
      mockStoreState.ws.lastError = null
      mockStoreState.ws.lastErrorRequestId = null
    }),
    craftReviews: [] as CraftReviewResult[],
    liveCandidates: [],
    removeLiveCandidate: vi.fn(),
    resetLiveAnalysis: vi.fn(),
  },
  editor: {
    content: '',
    wordCount: 42,
    isSaving: false,
    saveStatus: 'idle',
    saveError: null,
    lastSavedAt: null,
    setContent: vi.fn(),
    saveContent: vi.fn(),
    getLocalDraft: vi.fn(() => null),
    clearLocalDraft: vi.fn(),
  },
}))
vi.mock('../../lib/api', () => ({
  api: {
    getChapter: (...args: unknown[]) => mockGetChapter(...args),
    getWork: (...args: unknown[]) => mockGetWork(...args),
    listChapters: (...args: unknown[]) => mockListChapters(...args),
    createChapter: (...args: unknown[]) => mockCreateChapter(...args),
  },
}))

vi.mock('../../hooks/useWS', () => ({ useWS: () => ({ status: 'open' }) }))

vi.mock('../../stores/wsStore', () => ({
  useWSStore: (selector: (s: unknown) => unknown) => {
    return selector ? selector(mockStoreState.ws) : mockStoreState.ws
  },
}))

vi.mock('../../stores/editorStore', () => ({
  useEditorStore: () => mockStoreState.editor,
}))

// These child components own independent editor, review, candidate, and WebSocket
// behavior. Stub them so this suite verifies EditorPage's data loading and workspace
// wiring without inheriting their asynchronous effects.
vi.mock('../../components/editor/TipTapEditor', () => ({
  default: ({ chapterId, workId, universeId, onContentChange, onCraftReview }: { chapterId: string; workId: string; universeId: string; onContentChange: (html: string, text: string) => void; onCraftReview?: (selection: { passage: string; from: number; to: number }) => void }) => (
    <div data-testid="tiptap-editor">
      {`${chapterId}:${workId}:${universeId}`}
      <button type="button" onClick={() => onContentChange('<p>Updated chapter</p>', 'Updated chapter')}>Update chapter</button>
      <button type="button" onClick={() => onCraftReview?.({ passage: 'Review this passage.', from: 1, to: 21 })}>Review selection</button>
    </div>
  ),
}))

vi.mock('../../components/context-panel/ContextPanel', () => ({
  default: ({ status }: { status: string }) => <div data-testid="context-panel">{status}</div>,
}))

vi.mock('../../components/editor/CraftReviewPanel', () => ({
  default: ({ loading, review }: { loading?: boolean; review?: { request_id: string } | null }) => (
    <span>{loading ? 'Craft reviewing' : review ? `Craft result ${review.request_id}` : 'Craft idle'}</span>
  ),
}))

vi.mock('../../components/editor/EntityCandidateTray', () => ({
  default: () => null,
}))

function pageTree() {
  return (
    <MemoryRouter initialEntries={[`/universe/${mockRouteParams.universeId}/write/${mockRouteParams.chapterId}`]}>
      <Routes>
        <Route path="/universe/:universeId/write/:chapterId" element={<EditorPage />} />
      </Routes>
    </MemoryRouter>
  )
}

function renderPage(chapterId = 'ch-1') {
  mockRouteParams.chapterId = chapterId
  return render(pageTree())
}

describe('EditorPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    mockRouteParams.universeId = 'uni-1'
    mockRouteParams.chapterId = 'ch-1'
	mockStoreState.ws.craftReviews = []
	mockStoreState.ws.lastError = null
	mockStoreState.ws.lastErrorRequestId = null
    mockListChapters.mockResolvedValue({ chapters: [] })
    mockGetWork.mockResolvedValue({ work: { id: 'work-1', title: 'Test Work', universe_id: 'uni-1' } })
  })

  it('shows a loading state until the chapter resolves', () => {
    mockGetChapter.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByText('Loading editor…')).toBeInTheDocument()
  })

  it('renders the TipTap editor and context panel once the chapter loads', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('tiptap-editor')).toHaveTextContent('ch-1:work-1:uni-1')
    })
    expect(screen.getByTestId('context-panel')).toHaveTextContent('open')
    expect(screen.getByText('42 words')).toBeInTheDocument()
  })

  it('keeps a craft review pending through stale or unrelated WS errors, then handles only its tagged error', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    mockStoreState.ws.lastError = 'A previous analysis failed'
    mockStoreState.ws.lastErrorRequestId = 'previous-request'
    const user = userEvent.setup()
    const view = renderPage()

    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: 'Review selection' }))
    await waitFor(() => expect(mockStoreState.ws.send).toHaveBeenCalledTimes(1))
    expect(mockStoreState.ws.clearError).toHaveBeenCalledTimes(1)
    expect(screen.getByText('Craft reviewing')).toBeInTheDocument()

    const requestId = mockStoreState.ws.send.mock.calls[0][0].payload.request_id as string
    mockStoreState.ws.lastError = 'A different request failed'
    mockStoreState.ws.lastErrorRequestId = 'different-request'
    view.rerender(pageTree())
    expect(screen.getByText('Craft reviewing')).toBeInTheDocument()

    mockStoreState.ws.lastError = 'Craft review failed'
    mockStoreState.ws.lastErrorRequestId = requestId
    view.rerender(pageTree())
    expect(await screen.findByText('Craft review could not finish: Craft review failed')).toBeInTheDocument()
    expect(screen.getByText('Craft idle')).toBeInTheDocument()
  })

  it('ignores a different craft result until the active request resolves', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    const user = userEvent.setup()
    const view = renderPage()

    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: 'Review selection' }))
    await waitFor(() => expect(mockStoreState.ws.send).toHaveBeenCalledTimes(1))
    const requestId = mockStoreState.ws.send.mock.calls[0][0].payload.request_id as string

    mockStoreState.ws.craftReviews = [{ universe_id: 'uni-1', work_id: 'work-1', chapter_id: 'ch-1', request_id: 'different-request', selections: [], notes: [] }]
    view.rerender(pageTree())
    expect(screen.getByText('Craft reviewing')).toBeInTheDocument()

    mockStoreState.ws.craftReviews = [
      { universe_id: 'uni-1', work_id: 'work-1', chapter_id: 'ch-1', request_id: requestId, selections: [], notes: [] },
      { universe_id: 'uni-1', work_id: 'work-1', chapter_id: 'ch-1', request_id: 'unrelated-after-active', selections: [], notes: [] },
    ]
    view.rerender(pageTree())
    await waitFor(() => expect(screen.getByText(`Craft result ${requestId}`)).toBeInTheDocument())
  })

  it('rejects a timed-out result and only renders the later request it belongs to', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    const user = userEvent.setup()
    const view = renderPage()

    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())
    vi.useFakeTimers()
    fireEvent.click(screen.getByRole('button', { name: 'Review selection' }))
    const firstRequest = mockStoreState.ws.send.mock.calls[0][0].payload.request_id as string
    act(() => { vi.advanceTimersByTime(45_000) })
    expect(screen.getByText('Craft idle')).toBeInTheDocument()
    expect(screen.getByText(/Craft review took too long/)).toBeInTheDocument()
    vi.useRealTimers()

    await user.click(screen.getByRole('button', { name: 'Review selection' }))
    const secondRequest = mockStoreState.ws.send.mock.calls[1][0].payload.request_id as string
    mockStoreState.ws.craftReviews = [{ universe_id: 'uni-1', work_id: 'work-1', chapter_id: 'ch-1', request_id: firstRequest, selections: [], notes: [] }]
    view.rerender(pageTree())
    expect(screen.getByText('Craft reviewing')).toBeInTheDocument()
    expect(screen.queryByText(`Craft result ${firstRequest}`)).not.toBeInTheDocument()

    mockStoreState.ws.craftReviews = [
      { universe_id: 'uni-1', work_id: 'work-1', chapter_id: 'ch-1', request_id: firstRequest, selections: [], notes: [] },
      { universe_id: 'uni-1', work_id: 'work-1', chapter_id: 'ch-1', request_id: secondRequest, selections: [], notes: [] },
    ]
    view.rerender(pageTree())
    await waitFor(() => expect(screen.getByText(`Craft result ${secondRequest}`)).toBeInTheDocument())
  })

  it('collapses the chapter panel and saves the workspace preference', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    renderPage()

    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Collapse chapter panel' }))

    expect(screen.getByRole('button', { name: 'Expand chapter panel' })).toHaveAttribute('aria-expanded', 'false')
    expect(window.localStorage.getItem('quill:editor-workspace-panels')).toContain('"railCollapsed":true')
  })

  it('renders sibling chapters in the rail and navigates on click', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    mockListChapters.mockResolvedValue({
      chapters: [
        { id: 'ch-1', title: 'Chapter One', order_index: 1, status: 'draft' },
        { id: 'ch-2', title: 'Chapter Two', order_index: 2, status: 'analyzed' },
      ],
    })
    renderPage()

    const chapterTwoBtn = await screen.findByRole('button', { name: /Chapter Two/ })

    const user = userEvent.setup()
    await user.click(chapterTwoBtn)
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/write/ch-2')
  })

  it('creates a new chapter and navigates into it', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    mockCreateChapter.mockResolvedValue({ chapter: { id: 'ch-new', title: 'New Chapter' } })
    renderPage()

    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('New chapter'))
    await user.type(screen.getByPlaceholderText('Chapter title'), 'New Chapter{Enter}')

    await waitFor(() => {
      expect(mockCreateChapter).toHaveBeenCalledWith('work-1', { title: 'New Chapter' })
      expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/write/ch-new')
    })
  })

  it('shows an error and keeps the form open when chapter creation fails', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    mockCreateChapter.mockRejectedValue(new Error('Failed to create chapter'))
    renderPage()

    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('New chapter'))
    await user.type(screen.getByPlaceholderText('Chapter title'), 'New Chapter{Enter}')

    await waitFor(() => {
      expect(screen.getAllByText('Failed to create chapter').length).toBeGreaterThan(0)
    })
    expect(mockNavigate).not.toHaveBeenCalledWith(expect.stringContaining('write/undefined'))
    expect(screen.getByPlaceholderText('Chapter title')).toBeInTheDocument()
  })

  it('does not fire duplicate creates on rapid double-submit', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'ch-1', content: '', raw_text: '', work_id: 'work-1', universe_id: 'uni-1' },
    })
    let resolveCreate: (v: { chapter: { id: string; title: string } }) => void = () => {}
    mockCreateChapter.mockReturnValue(new Promise((resolve) => { resolveCreate = resolve }))
    renderPage()

    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('New chapter'))
    const input = screen.getByPlaceholderText('Chapter title')
    await user.type(input, 'New Chapter')
    await user.keyboard('{Enter}{Enter}{Enter}')

    resolveCreate({ chapter: { id: 'ch-new', title: 'New Chapter' } })
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/write/ch-new'))

    expect(mockCreateChapter).toHaveBeenCalledTimes(1)
  })

  it('keeps the newer chapter when A resolves after a route change to B', async () => {
    let resolveA!: (value: { chapter: { id: string; title: string; content: string; raw_text: string; work_id: string } }) => void
    let resolveB!: (value: { chapter: { id: string; title: string; content: string; raw_text: string; work_id: string } }) => void
    mockGetChapter.mockImplementation((id: string) => new Promise((resolve) => {
      if (id === 'chapter-a') resolveA = resolve
      else resolveB = resolve
    }))
    mockGetWork.mockImplementation((id: string) => Promise.resolve({ work: { id, title: `Work ${id}`, universe_id: 'uni-1' } }))

    const view = renderPage('chapter-a')
    await waitFor(() => expect(mockGetChapter).toHaveBeenCalledWith('chapter-a'))

    mockRouteParams.chapterId = 'chapter-b'
    view.rerender(pageTree())
    await waitFor(() => expect(mockGetChapter).toHaveBeenCalledWith('chapter-b'))

    resolveB({ chapter: { id: 'chapter-b', title: 'Chapter B', content: '<p>B</p>', raw_text: 'B', work_id: 'work-b' } })
    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toHaveTextContent('chapter-b:work-b:uni-1'))
    expect(screen.getByText('Chapter B')).toBeInTheDocument()

    resolveA({ chapter: { id: 'chapter-a', title: 'Chapter A', content: '<p>A</p>', raw_text: 'A', work_id: 'work-a' } })
    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toHaveTextContent('chapter-b:work-b:uni-1'))
    expect(screen.queryByText('Chapter A')).not.toBeInTheDocument()
    expect(mockStoreState.editor.setContent).toHaveBeenLastCalledWith('<p>B</p>', 'B')
  })

  it('does not run an A autosave timer after the route has moved to B', async () => {
    mockGetChapter.mockResolvedValue({
      chapter: { id: 'chapter-a', content: '', raw_text: '', work_id: 'work-a', universe_id: 'uni-1' },
    })
    mockGetWork.mockResolvedValue({ work: { id: 'work-a', title: 'Work A', universe_id: 'uni-1' } })
    const view = renderPage('chapter-a')
    await waitFor(() => expect(screen.getByTestId('tiptap-editor')).toBeInTheDocument())

    vi.useFakeTimers()
    fireEvent.click(screen.getByRole('button', { name: 'Update chapter' }))
    mockRouteParams.chapterId = 'chapter-b'
    view.rerender(pageTree())
    await vi.advanceTimersByTimeAsync(5000)

    expect(mockStoreState.editor.saveContent).not.toHaveBeenCalledWith('chapter-a')
    vi.useRealTimers()
  })
})
