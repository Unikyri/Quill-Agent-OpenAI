import { create } from 'zustand'
import type { CraftReviewResult, EntityCandidateDTO } from '../lib/types'

export type WSStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'

export interface WSMessage {
  type: string
  payload: Record<string, unknown>
}

interface AnalysisResult {
  id?: string
  content?: string
  [key: string]: unknown
}

interface Contradiction {
  id?: string
  message?: string
  severity?: 'low' | 'medium' | 'high'
  [key: string]: unknown
}

interface DiscoveredEntity {
  id?: string
  name?: string
  type?: string
  [key: string]: unknown
}

interface RecallItem {
  id?: string
  fact?: string
  score?: number
  source?: string
  [key: string]: unknown
}

// eslint-disable-next-line @typescript-eslint/no-empty-object-type
interface GraphPing {
  [key: string]: unknown
}

interface IngestionProgress {
  job_id?: string
  status?: string
  chapters_processed?: number
  total_chapters?: number
  action?: string
  eta_seconds?: number
  [key: string]: unknown
}

export interface BudgetReport {
  max_context_tokens: number
  available: number
  entities_tokens: number
  vector_tokens: number
  tools_tokens: number
  used_percent: number
}

export interface PipelineState {
  stage: string
  entity_count?: number
  contradiction_count?: number
  plot_hole_count?: number
}

export type SubmissionPhase = 'submitted' | 'analyzing' | 'done' | 'failed'

export interface SubmissionLifecycle {
  submissionId: string
  paragraphRef: string
  chapterId?: string
  phase: SubmissionPhase
  reason?: string
  updatedAt: number
}

const WS_URL = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1/ws`

const MAX_RECONNECT_DELAY_MS = 30_000
const MAX_OUTBOUND_QUEUE = 50

function jitter(base: number): number {
  // ±20% jitter
  const range = base * 0.4
  return base - range * 0.5 + Math.random() * range
}

interface WSState {
  status: WSStatus
  lastError: string | null
  reconnectAttempt: number
  analysisResults: AnalysisResult[]
  contradictions: Contradiction[]
  discoveredEntities: DiscoveredEntity[]
  recallItems: RecallItem[]
  graphPings: GraphPing[]
  ingestionProgress: Record<string, IngestionProgress>
  pipeline: PipelineState | null
  budget: BudgetReport | null
  submissions: Record<string, SubmissionLifecycle>
  craftReviews: CraftReviewResult[]
  liveCandidates: EntityCandidateDTO[]
  removeLiveCandidate: (candidateId: string) => void
  connect: (token: string) => void
  disconnect: () => void
  send: (msg: WSMessage) => void
  _clearSlices: () => void
}

export const useWSStore = create<WSState>((set, get) => {
  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let intentionalClose = false
  let outboundQueue: WSMessage[] = []

  function submissionFromPayload(payload: Record<string, unknown>): SubmissionLifecycle | null {
    const submissionId = typeof payload.submission_id === 'string' ? payload.submission_id : ''
    const paragraphRef = typeof payload.paragraph_ref === 'string' ? payload.paragraph_ref : ''
    if (!submissionId || !paragraphRef) return null
    return {
      submissionId,
      paragraphRef,
      chapterId: typeof payload.chapter_id === 'string' ? payload.chapter_id : undefined,
      phase: 'submitted',
      updatedAt: Date.now(),
    }
  }

  function updateSubmission(payload: Record<string, unknown>, phase: SubmissionPhase, reason?: string) {
    const current = submissionFromPayload(payload)
    if (!current) return
    const existing = get().submissions[current.submissionId]
    set({
      submissions: {
        ...get().submissions,
        [current.submissionId]: {
          ...existing,
          ...current,
          phase,
          updatedAt: Date.now(),
          ...(reason ? { reason } : {}),
        },
      },
    })
  }

  function queueOutbound(msg: WSMessage) {
    if (outboundQueue.length >= MAX_OUTBOUND_QUEUE) {
      const dropped = outboundQueue.shift()
      if (dropped?.type === 'paragraph_submit') {
        updateSubmission(dropped.payload, 'failed', 'Submission queue is full. Please try again.')
      }
    }
    outboundQueue.push(msg)
  }

  function failInFlightSubmissions(reason: string) {
    const queuedIDs = new Set(
      outboundQueue
        .filter((message) => message.type === 'paragraph_submit')
        .map((message) => message.payload.submission_id)
        .filter((value): value is string => typeof value === 'string')
    )
    const next = Object.fromEntries(Object.entries(get().submissions).map(([id, submission]) => {
      if ((submission.phase === 'submitted' || submission.phase === 'analyzing') && !queuedIDs.has(id)) {
        return [id, { ...submission, phase: 'failed' as const, reason, updatedAt: Date.now() }]
      }
      return [id, submission]
    }))
    set({ submissions: next })
  }

  function clearReconnectTimer() {
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  function dispatch(msg: WSMessage) {
    const payload = msg.payload || {}
    switch (msg.type) {
      case 'auth_ok':
        set({ lastError: null })
        break
      case 'error':
        set({ lastError: (payload.message as string) || (payload.error as string) || 'Unknown WS error' })
        break
      case 'analysis_result':
        set({ analysisResults: [...get().analysisResults, payload as AnalysisResult].slice(-200) })
        updateSubmission(payload, 'done')
        break
      case 'analysis_failed':
        updateSubmission(payload, 'failed', (payload.reason as string) || 'Analysis failed')
        break
      case 'contradiction_alert':
        set({
          contradictions: [
            ...get().contradictions,
            (payload.contradiction as Contradiction) ?? (payload as Contradiction),
          ].slice(-200),
        })
        break
      case 'entity_discovered':
        set({ discoveredEntities: [...get().discoveredEntities, payload as DiscoveredEntity].slice(-200) })
        {
          const entity = (payload.entity as Record<string, unknown> | undefined) || payload
          const status = typeof entity.status === 'string' ? entity.status : ''
          const confidence = typeof entity.confidence === 'number' ? entity.confidence : undefined
          const threshold = typeof payload.candidate_threshold === 'number' ? payload.candidate_threshold : 0.7
          // The backend status is authoritative. In particular, an active
          // entity with a low historical confidence must not be shown as a
          // review candidate merely because the client guessed a threshold.
          if (status === 'candidate' || (!status && confidence !== undefined && confidence < threshold)) {
            const candidate = {
              entity_id: String(entity.id || entity.entity_id || ''),
              universe_id: String(entity.universe_id || payload.universe_id || ''),
              chapter_id: typeof entity.chapter_id === 'string' ? entity.chapter_id : undefined,
              name: String(entity.name || ''),
              type: String(entity.type || 'character'),
              aliases: Array.isArray(entity.aliases) ? entity.aliases.filter((alias): alias is string => typeof alias === 'string') : [],
              description: typeof entity.description === 'string' ? entity.description : undefined,
              confidence: confidence ?? 0,
              status: status || 'candidate',
              evidence_quote: typeof entity.evidence_quote === 'string' ? entity.evidence_quote : undefined,
            } satisfies EntityCandidateDTO
            if (candidate.entity_id && candidate.name) {
              const current = get().liveCandidates.filter((item) => item.entity_id !== candidate.entity_id)
              set({ liveCandidates: [...current, candidate].slice(-100) })
            }
          }
        }
        break
      case 'contextual_recall':
        set({ recallItems: [...get().recallItems, payload as RecallItem].slice(-200) })
        break
      case 'graph_updated':
        set({ graphPings: [...get().graphPings, payload as GraphPing].slice(-200) })
        break
      case 'analysis_progress': {
        const p = payload as {
          stage?: string
          entity_count?: number
          contradiction_count?: number
          plot_hole_count?: number
          budget?: BudgetReport
        }
        set({
          pipeline: {
            stage: p.stage ?? '',
            entity_count: p.entity_count,
            contradiction_count: p.contradiction_count,
            plot_hole_count: p.plot_hole_count,
          },
          ...(p.budget ? { budget: p.budget } : {}),
        })
        updateSubmission(payload, 'analyzing')
        break
      }
      case 'ingestion_progress': {
        const progress = payload as IngestionProgress
        if (progress.job_id) {
          set({ ingestionProgress: { ...get().ingestionProgress, [progress.job_id]: progress } })
        }
        break
      }
      case 'craft_review_result':
        set({ craftReviews: [...get().craftReviews, payload as unknown as CraftReviewResult].slice(-20) })
        break
      default:
        break
    }
  }

  function scheduleReconnect(token: string, attempt: number) {
    clearReconnectTimer()
    const base = Math.min(1000 * Math.pow(2, attempt), MAX_RECONNECT_DELAY_MS)
    const delay = jitter(base)
    set({ status: 'reconnecting', reconnectAttempt: attempt })
    reconnectTimer = setTimeout(() => {
      if (intentionalClose) return
      doConnect(token, attempt + 1)
    }, delay)
  }

  function doConnect(token: string, attempt: number) {
    clearReconnectTimer()

    // Close any existing socket
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      intentionalClose = true
      ws.close()
      intentionalClose = false
    }
    ws = null

    let socket: WebSocket
    try {
      socket = new WebSocket(WS_URL)
    ws = socket
    } catch {
      set({ status: 'closed', lastError: 'WebSocket constructor failed' })
      scheduleReconnect(token, attempt)
      return
    }

    set({ status: 'connecting', reconnectAttempt: attempt })

    socket.onopen = () => {
      if (ws !== socket) return
      set({ status: 'open', lastError: null })
      // Send auth_init
      socket.send(JSON.stringify({ type: 'auth_init', payload: { token } }))
      const queued = outboundQueue
      outboundQueue = []
      for (const message of queued) {
        socket.send(JSON.stringify(message))
      }
    }

    socket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data as string)
        dispatch(msg)
      } catch {
        set({ lastError: 'Failed to parse WS message' })
      }
    }

    socket.onerror = () => {
      // The close event will fire after error
    }

    socket.onclose = () => {
      if (ws !== socket) return
      if (intentionalClose) {
        set({ status: 'closed' })
        return
      }
      failInFlightSubmissions('Connection lost before analysis completed.')
      set({ lastError: 'Connection lost' })
      scheduleReconnect(token, attempt)
    }
  }

  return {
    status: 'idle',
    lastError: null,
    reconnectAttempt: 0,
    analysisResults: [],
    contradictions: [],
    discoveredEntities: [],
    recallItems: [],
    graphPings: [],
    ingestionProgress: {},
    pipeline: null,
    budget: null,
    submissions: {},
    craftReviews: [],
    liveCandidates: [],

    removeLiveCandidate: (candidateId: string) => {
      set({ liveCandidates: get().liveCandidates.filter((candidate) => candidate.entity_id !== candidateId) })
    },

    connect: (token: string) => {
      intentionalClose = false
      doConnect(token, 0)
    },

    disconnect: () => {
      intentionalClose = true
      clearReconnectTimer()
      ws?.close()
      ws = null
      for (const message of outboundQueue) {
        if (message.type === 'paragraph_submit') {
          updateSubmission(message.payload, 'failed', 'Connection closed before the submission was sent.')
        }
      }
      outboundQueue = []
      set({ status: 'closed' })
    },

    send: (msg: WSMessage) => {
      if (msg.type === 'paragraph_submit') {
        updateSubmission(msg.payload, 'submitted')
      }
      const state = get()
      if (state.status !== 'open' || !ws || ws.readyState !== WebSocket.OPEN) {
        queueOutbound(msg)
        return
      }
      ws.send(JSON.stringify(msg))
    },

    _clearSlices: () => {
      set({
        analysisResults: [],
        contradictions: [],
        discoveredEntities: [],
        recallItems: [],
        graphPings: [],
        ingestionProgress: {},
        pipeline: null,
        budget: null,
        submissions: {},
        craftReviews: [],
        liveCandidates: [],
      })
    },
  }
})
