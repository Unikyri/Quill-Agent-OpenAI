import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useFeedback } from '../feedback'
import { api } from '../../lib/api'
import { useWSStore } from '../../stores/wsStore'
import { useWS } from '../../hooks/useWS'
import { writePath } from '../../pages/writeRoutes'
import styles from './IngestPanel.module.css'

const ACCEPTED_EXTENSIONS = ['.md', '.txt', '.pdf', '.docx']

interface IngestJob {
  jobId: string
  workId?: string
  filename: string
  status?: string
  processed?: number
  total?: number
  errorMessage?: string
}

interface IngestPanelProps {
  universeId: string
  workId?: string
  onClose?: () => void
  onCompleted?: (workId: string) => void | Promise<void>
  standalone?: boolean
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message.trim() ? error.message : fallback
}

function isAcceptedFile(file: File): boolean {
  const lower = file.name.toLowerCase()
  return ACCEPTED_EXTENSIONS.some((extension) => lower.endsWith(extension))
}

function isTerminal(status?: string) {
  return status === 'completed' || status === 'failed'
}

export function IngestPanel({ universeId, workId, onClose, onCompleted, standalone = false }: IngestPanelProps) {
  const navigate = useNavigate()
  const { publish, update } = useFeedback()
  const [jobs, setJobs] = useState<IngestJob[]>([])
  const [jobsUniverseId, setJobsUniverseId] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [loadingUniverseId, setLoadingUniverseId] = useState<string | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [uploadUniverseId, setUploadUniverseId] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [loadErrorUniverseId, setLoadErrorUniverseId] = useState<string | null>(null)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [uploadErrorUniverseId, setUploadErrorUniverseId] = useState<string | null>(null)
  const [failedFile, setFailedFile] = useState<File | null>(null)
  const [failedFileUniverseId, setFailedFileUniverseId] = useState<string | null>(null)
  const [activeJobId, setActiveJobId] = useState<string | null>(null)
  const [activeJobUniverseId, setActiveJobUniverseId] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const completionNotified = useRef(new Set<string>())
  const completionHydrated = useRef(new Set<string>())
  const activeFeedbackId = useRef<string | null>(null)
  const jobsRequestId = useRef(0)
  const uploadRequestId = useRef(0)
  const currentUniverseId = useRef(universeId)
  const universeGeneration = useRef(0)
  if (currentUniverseId.current !== universeId) {
    currentUniverseId.current = universeId
    universeGeneration.current += 1
  }
  const ingestionProgress = useWSStore((state) => state.ingestionProgress)

  // Import status continues to arrive through the same connection used by the editor.
  useWS()

  const loadJobs = useCallback(async (showLoading = true) => {
    if (!universeId) return false
    const requestId = ++jobsRequestId.current
    const requestUniverseId = universeId
    const requestGeneration = universeGeneration.current
    const isCurrentRequest = () => (
      jobsRequestId.current === requestId
      && universeGeneration.current === requestGeneration
      && currentUniverseId.current === requestUniverseId
    )
    if (showLoading) {
      setIsLoading(true)
      setLoadingUniverseId(requestUniverseId)
    }
    setLoadError(null)
    setLoadErrorUniverseId(null)
    setJobsUniverseId(requestUniverseId)

    try {
      const { jobs: result } = await api.listIngestionJobs(universeId)
      if (!isCurrentRequest()) return false
      setJobs((previous) => {
        const fetched = result.map((job) => ({
          jobId: job.id,
          workId: job.work_id,
          filename: job.filename || 'document',
          status: job.status,
          processed: job.chapters_processed,
          total: job.total_chapters_detected,
          errorMessage: job.error_message,
        }))
        const fetchedIds = new Set(fetched.map((job) => job.jobId))
        // A just-uploaded job can race the first list request. Keep it until
        // the server returns the authoritative record.
        return [...previous.filter((job) => !fetchedIds.has(job.jobId)), ...fetched]
      })
      setJobsUniverseId(requestUniverseId)
      return true
    } catch (error) {
      if (!isCurrentRequest()) return false
      const message = errorMessage(error, 'We could not load import status.')
      setLoadError(message)
      setLoadErrorUniverseId(requestUniverseId)
      publish({
        scope: 'write',
        status: 'failed',
        message,
        retry: () => loadJobs(true),
      })
      return false
    } finally {
      if (showLoading && isCurrentRequest()) setIsLoading(false)
    }
  }, [publish, universeId])

  useEffect(() => {
    completionNotified.current.clear()
    completionHydrated.current.clear()
    activeFeedbackId.current = null
    setJobs([])
    setJobsUniverseId(null)
    setIsLoading(true)
    setLoadingUniverseId(null)
    setIsUploading(false)
    setUploadUniverseId(null)
    setLoadError(null)
    setLoadErrorUniverseId(null)
    setUploadError(null)
    setUploadErrorUniverseId(null)
    setFailedFile(null)
    setFailedFileUniverseId(null)
    setActiveJobId(null)
    setActiveJobUniverseId(null)
    void loadJobs()
  }, [loadJobs])

  const hasCurrentJobs = jobsUniverseId === universeId
  const currentJobs = hasCurrentJobs ? jobs : []
  const resolvedJobs = useMemo(() => currentJobs.map((job) => {
    const progress = ingestionProgress[job.jobId]
    return {
      ...job,
      status: (progress?.status as string | undefined) ?? job.status,
      processed: progress?.chapters_processed ?? job.processed ?? 0,
      total: progress?.total_chapters ?? job.total ?? 0,
      action: progress?.action as string | undefined,
      etaSeconds: progress?.eta_seconds as number | undefined,
    }
  }), [currentJobs, ingestionProgress])

  const currentJob = useMemo(() => {
    const currentActiveJobId = activeJobUniverseId === universeId ? activeJobId : null
    if (currentActiveJobId) {
      const active = resolvedJobs.find((job) => job.jobId === currentActiveJobId)
      if (active) return active
    }
    return resolvedJobs.find((job) => !isTerminal(job.status)) ?? resolvedJobs[0] ?? null
  }, [activeJobId, activeJobUniverseId, resolvedJobs, universeId])

  const isLoadingCurrentUniverse = isLoading && loadingUniverseId === universeId
  const isUploadingCurrentUniverse = isUploading && uploadUniverseId === universeId
  const currentLoadError = loadErrorUniverseId === universeId ? loadError : null
  const currentUploadError = uploadErrorUniverseId === universeId ? uploadError : null
  const currentFailedFile = failedFileUniverseId === universeId ? failedFile : null
  const awaitingJobStatus = !hasCurrentJobs || isLoadingCurrentUniverse

  const openCompletedWork = useCallback(async (workId: string): Promise<boolean> => {
    const requestUniverseId = universeId
    const requestGeneration = universeGeneration.current
    const isCurrentRequest = () => (
      universeGeneration.current === requestGeneration && currentUniverseId.current === requestUniverseId
    )
    if (onCompleted) {
      if (!isCurrentRequest()) return false
      await onCompleted(workId)
      return isCurrentRequest()
    }
    if (!isCurrentRequest()) return false
    navigate(writePath(requestUniverseId))
    return true
  }, [navigate, onCompleted, universeId])

  useEffect(() => {
    if (!activeJobId || !currentJob || currentJob.jobId !== activeJobId) return
    if (currentJob.status !== 'completed' || completionNotified.current.has(currentJob.jobId)) return
    if (!currentJob.workId) {
      if (!completionHydrated.current.has(currentJob.jobId)) {
        completionHydrated.current.add(currentJob.jobId)
        void loadJobs(false)
      }
      return
    }

    completionNotified.current.add(currentJob.jobId)
    if (activeFeedbackId.current) {
      update(activeFeedbackId.current, { status: 'completed', message: `${currentJob.filename} is ready to write in.` })
      activeFeedbackId.current = null
    } else {
      publish({ scope: 'write', status: 'completed', message: `${currentJob.filename} is ready to write in.` })
    }
    if (!onCompleted) return
    void openCompletedWork(currentJob.workId).catch((error) => {
      publish({
        scope: 'write',
        status: 'failed',
        message: errorMessage(error, 'The import finished, but we could not open the manuscript.'),
        retry: () => openCompletedWork(currentJob.workId!),
      })
    })
  }, [activeJobId, currentJob, loadJobs, onCompleted, openCompletedWork, publish, update])

  useEffect(() => {
    if (!activeJobId || !currentJob || currentJob.jobId !== activeJobId || currentJob.status !== 'failed') return
    if (activeFeedbackId.current) {
      update(activeFeedbackId.current, {
        status: 'failed',
        message: currentJob.errorMessage || 'The import could not be completed. Upload the file again to retry.',
      })
      activeFeedbackId.current = null
    }
  }, [activeJobId, currentJob, update])

  const handleFile = useCallback(async (file: File | undefined): Promise<boolean> => {
    if (!file || !universeId || isUploadingCurrentUniverse) return false
    if (!isAcceptedFile(file)) {
      setUploadError('Only .md, .txt, .pdf, and .docx files are supported.')
      setUploadErrorUniverseId(universeId)
      return false
    }

    const requestId = ++uploadRequestId.current
    const requestUniverseId = universeId
    const requestGeneration = universeGeneration.current
    const isCurrentRequest = () => (
      uploadRequestId.current === requestId
      && universeGeneration.current === requestGeneration
      && currentUniverseId.current === requestUniverseId
    )
    setUploadError(null)
    setUploadErrorUniverseId(null)
    setFailedFile(null)
    setFailedFileUniverseId(null)
    setIsUploading(true)
    setUploadUniverseId(requestUniverseId)
    const feedbackId = publish({ scope: 'write', status: 'running', message: `Uploading ${file.name}…` })
    activeFeedbackId.current = feedbackId

    try {
      const { job_id, status } = await (workId
        ? api.ingestDocument(universeId, file, workId)
        : api.ingestDocument(universeId, file))
      if (!isCurrentRequest()) return false
      setActiveJobId(job_id)
      setActiveJobUniverseId(requestUniverseId)
      if (status === 'duplicate') {
        update(feedbackId, { status: 'completed', message: `${file.name} was already imported. Showing its current status.` })
        activeFeedbackId.current = null
        return loadJobs(false)
      }

      setJobs((previous) => [{ jobId: job_id, filename: file.name, status: 'pending' }, ...previous])
      // Keep the feedback event running while the server processes the import;
      // the job card owns the honest queued/running/ETA detail.
      update(feedbackId, { status: 'running', message: `${file.name} is queued for import.` })
      return true
    } catch (error) {
      if (!isCurrentRequest()) return false
      const message = errorMessage(error, 'Upload failed. Please try again.')
      setUploadError(message)
      setUploadErrorUniverseId(requestUniverseId)
      setFailedFile(file)
      setFailedFileUniverseId(requestUniverseId)
      update(feedbackId, { status: 'failed', message, retry: () => handleFile(file) })
      activeFeedbackId.current = null
      return false
    } finally {
      if (isCurrentRequest()) {
        setIsUploading(false)
        if (inputRef.current) inputRef.current.value = ''
      }
    }
  }, [isUploadingCurrentUniverse, loadJobs, publish, universeId, update, workId])

  const handleDrop = (event: DragEvent<HTMLButtonElement>) => {
    event.preventDefault()
    setDragOver(false)
    void handleFile(event.dataTransfer.files?.[0])
  }

  const status = currentJob?.status
  const done = status === 'completed'
  const failed = status === 'failed'
  const total = currentJob?.total ?? 0
  const processed = currentJob?.processed ?? 0
  const percentage = total > 0 ? Math.round((processed / total) * 100) : done ? 100 : 0

  return (
    <section className={`${styles.wrap} ${standalone ? styles.standalone : ''}`} aria-labelledby="import-heading">
      <div className={styles.headerRow}>
        <div>
          <h1 id="import-heading" className={styles.heading}>Import manuscript</h1>
          <p className={styles.subhead}>Bring in a .md, .txt, .pdf, or .docx draft. Quill will report its real progress here.</p>
        </div>
        {onClose && <button type="button" className={styles.closeButton} onClick={onClose}>Close import</button>}
      </div>

      {awaitingJobStatus && <div className={styles.loading} role="status">Loading import status…</div>}
      {currentLoadError && (
        <div className={styles.errorState} role="alert">
          <p>{currentLoadError}</p>
          <button type="button" className={styles.checkStatusBtn} onClick={() => void loadJobs()}>Retry</button>
        </div>
      )}
      <>
        <button
            type="button"
            className={styles.dropzone}
            data-drag-over={dragOver || undefined}
            onClick={() => inputRef.current?.click()}
            onDragOver={(event) => { event.preventDefault(); setDragOver(true) }}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            disabled={isUploadingCurrentUniverse}
            aria-busy={isUploadingCurrentUniverse}
          >
            <span className={`${styles.dropGlyph} glyph`} aria-hidden="true">⇩</span>
            <span className={styles.dropText}>{isUploadingCurrentUniverse ? 'Uploading manuscript…' : 'Choose a manuscript or drop it here'}</span>
        </button>
        <input
            ref={inputRef}
            type="file"
            data-testid="ingest-file-input"
            className={styles.input}
          onChange={(event) => void handleFile(event.target.files?.[0])}
        />

        {currentUploadError && (
          <div className={styles.errorState} role="alert">
            <p>{currentUploadError}</p>
            {currentFailedFile && <button type="button" className={styles.checkStatusBtn} onClick={() => void handleFile(currentFailedFile)}>Retry upload</button>}
          </div>
        )}

        {(!awaitingJobStatus || currentJob) && !currentLoadError && (currentJob ? (
            <div className={styles.jobList} aria-live="polite">
              <div key={currentJob.jobId} className={styles.jobCard}>
                <div className={styles.jobHeader}>
                  <span className={styles.jobFilename}>{currentJob.filename}</span>
                  <span className={styles.jobStatus} data-done={done || undefined} data-failed={failed || undefined}>
                    {done ? 'Completed' : failed ? 'Failed' : status === 'running' || ingestionProgress[currentJob.jobId] ? 'Processing…' : 'Queued'}
                  </span>
                  {!done && !failed && (
                    <button type="button" className={styles.checkStatusBtn} onClick={() => void loadJobs(false)}>
                      Check status
                    </button>
                  )}
                </div>
                <div className={styles.progressTrack} aria-label={`${percentage}% imported`}>
                  <div className={styles.progressFill} style={{ width: `${percentage}%` }} />
                </div>
                {(currentJob.action || currentJob.etaSeconds !== undefined) && (
                  <p className={styles.progressMeta}>
                    {currentJob.action}{currentJob.etaSeconds !== undefined && ` · ~${currentJob.etaSeconds}s remaining`}
                  </p>
                )}
                {failed && <p className={styles.errorText}>{currentJob.errorMessage || 'The import could not complete. Upload the file again to retry.'}</p>}
                {done && currentJob.workId && (
                  <button type="button" className={styles.continueButton} onClick={() => void openCompletedWork(currentJob.workId!)}>
                    Open imported manuscript
                  </button>
                )}
              </div>
            </div>
        ) : (
          <p className={styles.empty}>No imports yet. Start with a manuscript, or create a chapter from Write.</p>
        ))}
      </>
    </section>
  )
}
