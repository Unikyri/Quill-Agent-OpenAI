import { useCallback, useEffect, useRef, useState, type DragEvent } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { useWSStore } from '../stores/wsStore'
import { useWS } from '../hooks/useWS'
import styles from './IngestPage.module.css'

const ACCEPTED_EXTENSIONS = ['.md', '.txt', '.pdf', '.docx']

interface IngestJob {
  jobId: string
  filename: string
  status?: string
  processed?: number
  total?: number
}

// ponytail: fixed step labels mirroring the backend pipeline order
// (chunk by heading -> extract entities -> embed -> populate graph). The
// backend doesn't report per-step granularity, only chapter counts, so
// steps are inferred from overall percent complete rather than tracked
// individually.
const STEPS = [
  { key: 'split', label: 'Split chapters' },
  { key: 'segment', label: 'Segment paragraphs' },
  { key: 'extract', label: 'Extract entities' },
  { key: 'embed', label: 'Generate embeddings' },
  { key: 'graph', label: 'Populate graph' },
]

function isAcceptedFile(file: File): boolean {
  const lower = file.name.toLowerCase()
  return ACCEPTED_EXTENSIONS.some((ext) => lower.endsWith(ext))
}

export default function IngestPage() {
  const { universeId } = useParams<{ universeId: string }>()
  const [jobs, setJobs] = useState<IngestJob[]>([])
  const [dragOver, setDragOver] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const ingestionProgress = useWSStore((s) => s.ingestionProgress)

  // Opens the WS connection so `ingestion_progress` events reach this page —
  // otherwise the socket is only opened from the Editor screen.
  useWS()

  // Hydrate persisted jobs so the list survives page reloads; live updates
  // still arrive over WS.
  const fetchJobs = useCallback(() => {
    if (!universeId) return
    api
      .listIngestionJobs(universeId)
      .then(({ jobs }) =>
        setJobs((prev) => {
          // Merge by jobId: fetched data wins, but keep jobs the server
          // doesn't know yet (an upload racing an in-flight fetch).
          const fetched = jobs.map((j) => ({
            jobId: j.id,
            filename: j.filename || 'document',
            status: j.status,
            processed: j.chapters_processed,
            total: j.total_chapters_detected,
          }))
          const fetchedIds = new Set(fetched.map((j) => j.jobId))
          return [...prev.filter((j) => !fetchedIds.has(j.jobId)), ...fetched]
        })
      )
      .catch((err) => setError((err as Error).message || 'Failed to load jobs'))
  }, [universeId])

  useEffect(() => {
    fetchJobs()
  }, [fetchJobs])

  const handleCheckStatus = fetchJobs

  const handleFile = async (file: File | undefined) => {
    if (!file || !universeId) return
    if (!isAcceptedFile(file)) {
      setError('Only .md, .txt, .pdf, and .docx files are supported')
      return
    }
    setError(null)
    try {
      const { job_id, status } = await api.ingestDocument(universeId, file)
      if (status === 'duplicate') {
        // Same content already ingested — surface the existing job instead
        // of adding a new card.
        fetchJobs()
        return
      }
      setJobs((prev) => [{ jobId: job_id, filename: file.name, status: 'pending' }, ...prev])
    } catch (err) {
      setError((err as Error).message || 'Upload failed')
    }
  }

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragOver(false)
    handleFile(e.dataTransfer.files?.[0])
  }

  return (
    <div className={styles.wrap}>
      <h1 className={styles.heading}>Ingestion</h1>
      <p className={styles.subhead}>
        Upload a manuscript to extract entities, timeline events, and relationships.
      </p>

      <div
        className={styles.dropzone}
        data-drag-over={dragOver || undefined}
        onClick={() => inputRef.current?.click()}
        onDragOver={(e) => {
          e.preventDefault()
          setDragOver(true)
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
      >
        <span className={`${styles.dropGlyph} glyph`}>⇩</span>
        <p className={styles.dropText}>Drag a .md, .txt, .pdf, or .docx file here, or click to browse</p>
        <input
          ref={inputRef}
          type="file"
          data-testid="ingest-file-input"
          className={styles.input}
          onChange={(e) => handleFile(e.target.files?.[0])}
        />
      </div>

      {error && <p className={styles.errorText}>{error}</p>}

      {jobs.length > 0 && (
        <div className={styles.jobList}>
          {jobs.map((job) => {
            const progress = ingestionProgress[job.jobId]
            // Live WS progress wins; hydrated DB state covers reloads and
            // jobs that finished while disconnected.
            const status = (progress?.status as string | undefined) ?? job.status
            const total = progress?.total_chapters ?? job.total ?? 0
            const processed = progress?.chapters_processed ?? job.processed ?? 0
            const done = status === 'completed'
            const failed = status === 'failed'
            const pct = total > 0 ? Math.round((processed / total) * 100) : done ? 100 : 0
            const activeSteps = done ? STEPS.length : Math.min(STEPS.length - 1, Math.floor((pct / 100) * STEPS.length))

            return (
              <div key={job.jobId} className={styles.jobCard}>
                <div className={styles.jobHeader}>
                  <span className={styles.jobFilename}>{job.filename}</span>
                  <span className={styles.jobStatus} data-done={done || undefined}>
                    {done ? 'Completed' : failed ? 'Failed' : status === 'running' || progress ? 'Processing…' : 'Queued'}
                  </span>
                  {!done && !failed && (
                    <button type="button" className={styles.checkStatusBtn} onClick={handleCheckStatus}>
                      Check status
                    </button>
                  )}
                </div>
                <div className={styles.progressTrack}>
                  <div className={styles.progressFill} style={{ width: `${done ? 100 : pct}%` }} />
                </div>
                <div className={styles.stepList}>
                  {STEPS.map((step, i) => (
                    <span
                      key={step.key}
                      className={styles.stepChip}
                      data-done={i < activeSteps || done || undefined}
                    >
                      {step.label}
                    </span>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
