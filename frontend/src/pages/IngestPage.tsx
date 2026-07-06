import { useRef, useState, type DragEvent } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { useWSStore } from '../stores/wsStore'
import { useWS } from '../hooks/useWS'
import { useAuthStore } from '../stores/authStore'
import styles from './IngestPage.module.css'

const ACCEPTED_EXTENSIONS = ['.md', '.txt']

interface IngestJob {
  jobId: string
  filename: string
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
  const wsConnect = useWSStore((s) => s.connect)
  const token = useAuthStore((s) => s.token)

  // Opens the WS connection so `ingestion_progress` events reach this page —
  // otherwise the socket is only opened from the Editor screen.
  useWS()

  // ponytail: the backend has no GET job-status endpoint (only the POST
  // /ingest that kicks a job off), so there is no way to poll for a missed
  // terminal state. This forces a fresh WS session as a best-effort nudge —
  // real reconciliation for a job that finished while disconnected needs a
  // backend job-status endpoint to poll/refetch from.
  const handleCheckStatus = () => {
    if (token) wsConnect(token)
  }

  const handleFile = async (file: File | undefined) => {
    if (!file || !universeId) return
    if (!isAcceptedFile(file)) {
      setError('Only .md and .txt files are supported')
      return
    }
    setError(null)
    try {
      const { job_id } = await api.ingestDocument(universeId, file)
      setJobs((prev) => [{ jobId: job_id, filename: file.name }, ...prev])
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
        <p className={styles.dropText}>Drag a .md or .txt file here, or click to browse</p>
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
            const totalReported = progress?.total_chapters
            const total = totalReported ?? 0
            const processed = progress?.chapters_processed ?? 0
            // ponytail: backend never emits a terminal "completed" WS event
            // (only persists it to the DB) — infer completion once every
            // chapter has been processed instead of watching for a status
            // string that never arrives over the wire. `total_chapters: 0`
            // (empty file / no headers) must count as done immediately —
            // distinct from `undefined`, which means "not yet reported".
            const done = totalReported === 0 || (total > 0 && processed >= total)
            const pct = total > 0 ? Math.round((processed / total) * 100) : done ? 100 : 0
            const activeSteps = done ? STEPS.length : Math.min(STEPS.length - 1, Math.floor((pct / 100) * STEPS.length))

            return (
              <div key={job.jobId} className={styles.jobCard}>
                <div className={styles.jobHeader}>
                  <span className={styles.jobFilename}>{job.filename}</span>
                  <span className={styles.jobStatus} data-done={done || undefined}>
                    {done ? 'Completed' : progress ? 'Processing…' : 'Queued'}
                  </span>
                  {!done && (
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
