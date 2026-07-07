import { create } from 'zustand'

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

const WS_URL = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1/ws`

const MAX_RECONNECT_DELAY_MS = 30_000

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
  connect: (token: string) => void
  disconnect: () => void
  send: (msg: WSMessage) => void
  _clearSlices: () => void
}

export const useWSStore = create<WSState>((set, get) => {
  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let intentionalClose = false

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
        set({ lastError: (payload.message as string) || 'Unknown WS error' })
        break
      case 'analysis_result':
        set({ analysisResults: [...get().analysisResults, payload as AnalysisResult].slice(-200) })
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
        break
      }
      case 'ingestion_progress': {
        const progress = payload as IngestionProgress
        if (progress.job_id) {
          set({ ingestionProgress: { ...get().ingestionProgress, [progress.job_id]: progress } })
        }
        break
      }
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

    try {
      ws = new WebSocket(WS_URL)
    } catch {
      set({ status: 'closed', lastError: 'WebSocket constructor failed' })
      scheduleReconnect(token, attempt)
      return
    }

    set({ status: 'connecting', reconnectAttempt: attempt })

    ws.onopen = () => {
      set({ status: 'open', lastError: null })
      // Send auth_init
      ws?.send(JSON.stringify({ type: 'auth_init', payload: { token } }))
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data as string)
        dispatch(msg)
      } catch {
        set({ lastError: 'Failed to parse WS message' })
      }
    }

    ws.onerror = () => {
      // The close event will fire after error
    }

    ws.onclose = () => {
      if (intentionalClose) {
        set({ status: 'closed' })
        return
      }
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

    connect: (token: string) => {
      intentionalClose = false
      doConnect(token, 0)
    },

    disconnect: () => {
      intentionalClose = true
      clearReconnectTimer()
      ws?.close()
      ws = null
      set({ status: 'closed' })
    },

    send: (msg: WSMessage) => {
      const state = get()
      if (state.status !== 'open' || !ws) return
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
      })
    },
  }
})
