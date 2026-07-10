import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import IngestPage from '../IngestPage'

vi.mock('../IngestPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockIngestDocument = vi.fn()
const mockListIngestionJobs = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    ingestDocument: (...args: unknown[]) => mockIngestDocument(...args),
    listIngestionJobs: (...args: unknown[]) => mockListIngestionJobs(...args),
  },
}))

const mockConnect = vi.fn()
const mockDisconnect = vi.fn()
let mockIngestionProgress: Record<string, unknown> = {}

vi.mock('../../stores/wsStore', () => ({
  useWSStore: (selector: (s: unknown) => unknown) => {
    const state = { ingestionProgress: mockIngestionProgress, status: 'open', connect: mockConnect, disconnect: mockDisconnect }
    return selector ? selector(state) : state
  },
}))

vi.mock('../../hooks/useWS', () => ({
  useWS: () => ({ status: 'open' }),
}))

function renderPage(universeId = 'uni-1') {
  return render(
    <MemoryRouter initialEntries={[`/universe/${universeId}/ingest`]}>
      <Routes>
        <Route path="/universe/:universeId/ingest" element={<IngestPage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('IngestPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockIngestionProgress = {}
    mockListIngestionJobs.mockResolvedValue({ jobs: [] })
  })

  it('renders the dropzone', () => {
    renderPage()
    expect(screen.getByText(/Drag a \.md, \.txt, \.pdf, or \.docx file/)).toBeInTheDocument()
  })

  it('rejects unsupported file types without calling the API', async () => {
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['binary'], 'cover.png', { type: 'image/png' })

    const user = userEvent.setup()
    await user.upload(input, file)

    expect(mockIngestDocument).not.toHaveBeenCalled()
    expect(screen.getByText('Only .md, .txt, .pdf, and .docx files are supported')).toBeInTheDocument()
  })

  it('rejects legacy .doc files without calling the API', async () => {
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['binary'], 'manuscript.doc', { type: 'application/msword' })

    const user = userEvent.setup()
    await user.upload(input, file)

    expect(mockIngestDocument).not.toHaveBeenCalled()
    expect(screen.getByText('Only .md, .txt, .pdf, and .docx files are supported')).toBeInTheDocument()
  })

  it('uploads an accepted file and lists it as a job', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['# Chapter 1'], 'manuscript.md', { type: 'text/markdown' })

    const user = userEvent.setup()
    await user.upload(input, file)

    await waitFor(() => {
      expect(mockIngestDocument).toHaveBeenCalledWith('uni-1', file)
      expect(screen.getByText('manuscript.md')).toBeInTheDocument()
      expect(screen.getByText('Queued')).toBeInTheDocument()
    })
  })

  it('hydrates persisted jobs from the GET endpoint on mount', async () => {
    mockListIngestionJobs.mockResolvedValue({
      jobs: [
        {
          id: 'job-9',
          universe_id: 'uni-1',
          work_id: 'work-1',
          filename: 'persisted.md',
          status: 'completed',
          total_chapters_detected: 3,
          chapters_processed: 3,
          entities_extracted: 12,
          created_at: '2026-01-01T00:00:00Z',
        },
      ],
    })
    renderPage()

    await waitFor(() => {
      expect(mockListIngestionJobs).toHaveBeenCalledWith('uni-1')
      expect(screen.getByText('persisted.md')).toBeInTheDocument()
      expect(screen.getByText('Completed')).toBeInTheDocument()
    })
  })

  it('shows the existing job instead of a new card on a duplicate upload', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-9', status: 'duplicate' })
    renderPage()
    await waitFor(() => expect(mockListIngestionJobs).toHaveBeenCalledTimes(1))

    mockListIngestionJobs.mockResolvedValue({
      jobs: [
        {
          id: 'job-9',
          universe_id: 'uni-1',
          work_id: 'work-1',
          filename: 'manuscript.md',
          status: 'completed',
          total_chapters_detected: 3,
          chapters_processed: 3,
          entities_extracted: 12,
          created_at: '2026-01-01T00:00:00Z',
        },
      ],
    })

    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['# Chapter 1'], 'manuscript.md', { type: 'text/markdown' })
    const user = userEvent.setup()
    await user.upload(input, file)

    await waitFor(() => {
      expect(mockListIngestionJobs).toHaveBeenCalledTimes(2)
      expect(screen.getAllByText('manuscript.md')).toHaveLength(1)
      expect(screen.getByText('Completed')).toBeInTheDocument()
    })
  })

  it('shows live progress and step checklist from ingestion_progress WS state', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    mockIngestionProgress = {
      'job-1': { job_id: 'job-1', status: 'running', chapters_processed: 2, total_chapters: 4 },
    }
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['# Chapter 1'], 'manuscript.md', { type: 'text/markdown' })

    const user = userEvent.setup()
    await user.upload(input, file)

    await waitFor(() => {
      expect(screen.getByText('Processing…')).toBeInTheDocument()
    })
  })

  it('marks the job Completed when the terminal WS status arrives', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    mockIngestionProgress = {
      'job-1': { job_id: 'job-1', status: 'completed', chapters_processed: 4, total_chapters: 4 },
    }
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['# Chapter 1'], 'manuscript.md', { type: 'text/markdown' })

    const user = userEvent.setup()
    await user.upload(input, file)

    await waitFor(() => {
      expect(screen.getByText('Completed')).toBeInTheDocument()
    })
  })

  it('marks an empty-document job (total_chapters: 0) Completed on the terminal WS status', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    mockIngestionProgress = {
      'job-1': { job_id: 'job-1', status: 'completed', chapters_processed: 0, total_chapters: 0 },
    }
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File([''], 'empty.md', { type: 'text/markdown' })

    const user = userEvent.setup()
    await user.upload(input, file)

    await waitFor(() => {
      expect(screen.getByText('Completed')).toBeInTheDocument()
    })
  })

  it('does not drop a just-uploaded job when the initial fetch resolves late', async () => {
    let resolveFetch!: (v: { jobs: unknown[] }) => void
    mockListIngestionJobs.mockReturnValue(new Promise((r) => { resolveFetch = r }))
    mockIngestDocument.mockResolvedValue({ job_id: 'job-new', status: 'accepted' })
    renderPage()

    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['# Chapter 1'], 'fresh.md', { type: 'text/markdown' })
    const user = userEvent.setup()
    await user.upload(input, file)
    await waitFor(() => expect(screen.getByText('fresh.md')).toBeInTheDocument())

    resolveFetch({
      jobs: [
        {
          id: 'job-old',
          universe_id: 'uni-1',
          work_id: 'work-1',
          filename: 'old.md',
          status: 'completed',
          total_chapters_detected: 1,
          chapters_processed: 1,
          entities_extracted: 2,
          created_at: '2026-01-01T00:00:00Z',
        },
      ],
    })

    await waitFor(() => {
      expect(screen.getByText('old.md')).toBeInTheDocument()
      // The optimistic card from the racing upload must survive the merge.
      expect(screen.getByText('fresh.md')).toBeInTheDocument()
    })
  })

  it('offers a "Check status" fallback for a job stuck Processing, which re-fetches the job list', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    mockIngestionProgress = {
      'job-1': { job_id: 'job-1', status: 'running', chapters_processed: 2, total_chapters: 4 },
    }
    renderPage()
    await waitFor(() => expect(mockListIngestionJobs).toHaveBeenCalledTimes(1))

    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['# Chapter 1'], 'manuscript.md', { type: 'text/markdown' })

    const user = userEvent.setup()
    await user.upload(input, file)

    const checkStatusBtn = await screen.findByText('Check status')
    await user.click(checkStatusBtn)

    expect(mockListIngestionJobs).toHaveBeenCalledTimes(2)
  })
})
