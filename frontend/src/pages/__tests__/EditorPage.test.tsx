import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import EditorPage from '../EditorPage'

vi.mock('../EditorPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

const mockGetChapter = vi.fn()
const mockGetWork = vi.fn()
const mockListChapters = vi.fn()
const mockCreateChapter = vi.fn()
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
    const state = { status: 'open' }
    return selector ? selector(state) : state
  },
}))

vi.mock('../../stores/editorStore', () => ({
  useEditorStore: () => ({
    content: '',
    wordCount: 42,
    isSaving: false,
    lastSavedAt: null,
    setContent: vi.fn(),
    saveContent: vi.fn(),
  }),
}))

// TipTapEditor/ContextPanel are pulled in via TipTap internals + their own
// wsStore slices — stub them so this test asserts EditorPage's own wiring
// (chapter fetch, rail, header) without re-testing their internals (already
// covered by TipTapEditor.test.tsx / ContextPanel usage elsewhere).
vi.mock('../../components/editor/TipTapEditor', () => ({
  default: ({ chapterId, workId, universeId }: { chapterId: string; workId: string; universeId: string }) => (
    <div data-testid="tiptap-editor">{`${chapterId}:${workId}:${universeId}`}</div>
  ),
}))

vi.mock('../../components/context-panel/ContextPanel', () => ({
  default: ({ status }: { status: string }) => <div data-testid="context-panel">{status}</div>,
}))

function renderPage(chapterId = 'ch-1') {
  return render(
    <MemoryRouter initialEntries={[`/universe/uni-1/editor/${chapterId}`]}>
      <Routes>
        <Route path="/universe/:universeId/editor/:chapterId" element={<EditorPage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('EditorPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/editor/ch-2')
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
      expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/editor/ch-new')
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
      expect(screen.getByText('Failed to create chapter')).toBeInTheDocument()
    })
    expect(mockNavigate).not.toHaveBeenCalledWith(expect.stringContaining('editor/undefined'))
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
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/editor/ch-new'))

    expect(mockCreateChapter).toHaveBeenCalledTimes(1)
  })
})
