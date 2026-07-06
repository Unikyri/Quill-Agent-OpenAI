import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import IngestPage from '../IngestPage'

vi.mock('../IngestPage.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockIngestDocument = vi.fn()
vi.mock('../../lib/api', () => ({
  api: { ingestDocument: (...args: unknown[]) => mockIngestDocument(...args) },
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

vi.mock('../../stores/authStore', () => ({
  useAuthStore: (selector: (s: unknown) => unknown) => {
    const state = { token: 'test-token' }
    return selector ? selector(state) : state
  },
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
  })

  it('renders the dropzone', () => {
    renderPage()
    expect(screen.getByText(/Drag a \.md or \.txt file/)).toBeInTheDocument()
  })

  it('rejects unsupported file types without calling the API', async () => {
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['binary'], 'cover.png', { type: 'image/png' })

    const user = userEvent.setup()
    await user.upload(input, file)

    expect(mockIngestDocument).not.toHaveBeenCalled()
    expect(screen.getByText('Only .md and .txt files are supported')).toBeInTheDocument()
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

  it('marks the job Completed once processed chapters reach the total', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    mockIngestionProgress = {
      'job-1': { job_id: 'job-1', status: 'running', chapters_processed: 4, total_chapters: 4 },
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

  it('marks an empty-file job (total_chapters: 0) Completed immediately, not stuck as Processing', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    mockIngestionProgress = {
      'job-1': { job_id: 'job-1', status: 'running', chapters_processed: 0, total_chapters: 0 },
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

  it('offers a "Check status" fallback for a job stuck Processing, which forces a WS reconnect', async () => {
    mockIngestDocument.mockResolvedValue({ job_id: 'job-1', status: 'accepted' })
    mockIngestionProgress = {
      'job-1': { job_id: 'job-1', status: 'running', chapters_processed: 2, total_chapters: 4 },
    }
    renderPage()
    const input = screen.getByTestId('ingest-file-input') as HTMLInputElement
    const file = new File(['# Chapter 1'], 'manuscript.md', { type: 'text/markdown' })

    const user = userEvent.setup()
    await user.upload(input, file)

    const checkStatusBtn = await screen.findByText('Check status')
    await user.click(checkStatusBtn)

    expect(mockConnect).toHaveBeenCalledWith('test-token')
  })
})
