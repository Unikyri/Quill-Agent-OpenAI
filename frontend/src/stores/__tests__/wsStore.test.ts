import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useWSStore } from '../wsStore'

// Mock WebSocket
class MockWebSocket {
  static instances: MockWebSocket[] = []
  url: string
  readyState: number
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  sentMessages: string[] = []

  constructor(url: string) {
    this.url = url
    this.readyState = WebSocket.CONNECTING
    MockWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sentMessages.push(data)
  }

  close() {
    this.readyState = WebSocket.CLOSED
    this.onclose?.()
  }

  // Helpers for test control
  simulateOpen() {
    this.readyState = WebSocket.OPEN
    this.onopen?.()
  }

  simulateMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) })
  }

  simulateError() {
    this.onerror?.()
  }

  simulateClose() {
    this.readyState = WebSocket.CLOSED
    this.onclose?.()
  }

  static get CONNECTING() { return 0 }
  static get OPEN() { return 1 }
  static get CLOSING() { return 2 }
  static get CLOSED() { return 3 }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
globalThis.WebSocket = MockWebSocket as any

function getStore() {
  return useWSStore.getState()
}

beforeEach(() => {
  MockWebSocket.instances = []
  const s = useWSStore.getState()
  s.disconnect()
  s._clearSlices()
  // Reset store to initial state via setState
  useWSStore.setState({
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
  })
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('wsStore', () => {
  describe('initial state', () => {
    it('starts with idle status', () => {
      expect(getStore().status).toBe('idle')
    })

    it('has empty slices', () => {
      const s = getStore()
      expect(s.analysisResults).toEqual([])
      expect(s.contradictions).toEqual([])
      expect(s.discoveredEntities).toEqual([])
      expect(s.recallItems).toEqual([])
      expect(s.graphPings).toEqual([])
      expect(s.ingestionProgress).toEqual({})
    })

    it('has null lastError and 0 reconnectAttempt', () => {
      const s = getStore()
      expect(s.lastError).toBeNull()
      expect(s.reconnectAttempt).toBe(0)
    })
  })

  describe('connect', () => {
    it('creates a WebSocket and sets status to connecting', () => {
      getStore().connect('test-token')
      expect(MockWebSocket.instances.length).toBe(1)
      expect(getStore().status).toBe('connecting')
    })

    it('sends auth_init on open', () => {
      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      ws.simulateOpen()

      expect(getStore().status).toBe('open')
      const sent = JSON.parse(ws.sentMessages[0])
      expect(sent.type).toBe('auth_init')
      expect(sent.payload.token).toBe('test-token')
    })
  })

  describe('disconnect', () => {
    it('sets status to closed and stops reconnect', () => {
      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      ws.simulateOpen()
      getStore().disconnect()

      expect(getStore().status).toBe('closed')
    })
  })

  describe('send', () => {
    it('sends when status is open', () => {
      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      ws.simulateOpen()

      getStore().send({ type: 'paragraph_submit', payload: { text: 'hello' } })
      expect(ws.sentMessages[1]).toContain('paragraph_submit')
    })

    it('does not send when status is not open', () => {
      getStore().send({ type: 'paragraph_submit', payload: { text: 'hello' } })
      expect(MockWebSocket.instances.length).toBe(0)
    })
  })

  describe('message dispatch', () => {
    beforeEach(() => {
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()
    })

    it('dispatches analysis_result to analysisResults slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'analysis_result', payload: { content: 'analysis' } })
      expect(getStore().analysisResults).toHaveLength(1)
      expect(getStore().analysisResults[0].content).toBe('analysis')
    })

    it('dispatches contradiction_alert to contradictions slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'contradiction_alert', payload: { contradiction: { message: 'conflict', severity: 'high' } } })
      expect(getStore().contradictions).toHaveLength(1)
      expect(getStore().contradictions[0].message).toBe('conflict')
    })

    it('dispatches entity_discovered to discoveredEntities slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'entity_discovered', payload: { name: 'Alice', type: 'character' } })
      expect(getStore().discoveredEntities).toHaveLength(1)
      expect(getStore().discoveredEntities[0].name).toBe('Alice')
    })

    it('dispatches contextual_recall to recallItems slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'contextual_recall', payload: { fact: 'something', score: 0.9 } })
      expect(getStore().recallItems).toHaveLength(1)
      expect(getStore().recallItems[0].fact).toBe('something')
    })

    it('dispatches graph_updated to graphPings slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'graph_updated', payload: { updated: true } })
      expect(getStore().graphPings).toHaveLength(1)
    })

    it('dispatches analysis_progress to pipeline slice, tolerating missing counts', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'analysis_progress',
        payload: { stage: 'checking_contradictions', chapter_id: 'ch-1' },
      })
      expect(getStore().pipeline).toEqual({
        stage: 'checking_contradictions',
        entity_count: undefined,
        contradiction_count: undefined,
        plot_hole_count: undefined,
      })
      expect(getStore().budget).toBeNull()
    })

    it('dispatches analysis_progress budget at the context_budget stage', () => {
      const ws = MockWebSocket.instances[0]
      const budget = {
        max_context_tokens: 8000,
        available: 4000,
        entities_tokens: 1400,
        vector_tokens: 1600,
        tools_tokens: 1000,
        used_percent: 50,
      }
      ws.simulateMessage({
        type: 'analysis_progress',
        payload: { stage: 'context_budget', chapter_id: 'ch-1', entity_count: 3, budget },
      })
      expect(getStore().pipeline?.stage).toBe('context_budget')
      expect(getStore().pipeline?.entity_count).toBe(3)
      expect(getStore().budget).toEqual(budget)
    })

    it('dispatches ingestion_progress keyed by job_id', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'ingestion_progress',
        payload: { job_id: 'job-1', status: 'running', chapters_processed: 2, total_chapters: 5 },
      })
      expect(getStore().ingestionProgress['job-1']).toEqual({
        job_id: 'job-1',
        status: 'running',
        chapters_processed: 2,
        total_chapters: 5,
      })
    })

    it('merges later ingestion_progress updates for the same job_id', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'ingestion_progress',
        payload: { job_id: 'job-1', status: 'running', chapters_processed: 1, total_chapters: 5 },
      })
      ws.simulateMessage({
        type: 'ingestion_progress',
        payload: { job_id: 'job-1', status: 'running', chapters_processed: 3, total_chapters: 5 },
      })
      expect(getStore().ingestionProgress['job-1'].chapters_processed).toBe(3)
      expect(Object.keys(getStore().ingestionProgress)).toHaveLength(1)
    })

    it('handles auth_ok by clearing lastError', () => {
      // Trigger an error first
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'error', payload: { message: 'test error' } })
      expect(getStore().lastError).toBe('test error')

      ws.simulateMessage({ type: 'auth_ok', payload: {} })
      expect(getStore().lastError).toBeNull()
    })

    it('handles error message', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'error', payload: { message: 'auth failed' } })
      expect(getStore().lastError).toBe('auth failed')
    })
  })

  describe('reconnect', () => {
    beforeEach(() => {
      // Lock Math.random to 0 so jitter returns base*0.8 (deterministic)
      vi.spyOn(Math, 'random').mockReturnValue(0)
    })

    afterEach(() => {
      vi.restoreAllMocks()
    })

    it('sets reconnecting status and reconnectAttempt after unexpected close', () => {
      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      ws.simulateOpen()
      expect(getStore().status).toBe('open')

      // Unexpected close triggers reconnect
      ws.simulateClose()

      expect(getStore().status).toBe('reconnecting')
      expect(getStore().reconnectAttempt).toBe(0)
    })

    it('increments reconnectAttempt on each retry cycle', () => {
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()
      MockWebSocket.instances[0].simulateClose()

      // First retry: base=1000, jitter(1000)=800ms
      vi.advanceTimersByTime(800)
      expect(getStore().reconnectAttempt).toBe(1)
      expect(getStore().status).toBe('connecting')

      // Simulate the new connection failing
      MockWebSocket.instances[1].simulateClose()
      // Second retry: base=2000, jitter(2000)=1600ms
      vi.advanceTimersByTime(1600)
      expect(getStore().reconnectAttempt).toBe(2)
      expect(getStore().status).toBe('connecting')

      // Third failure
      MockWebSocket.instances[2].simulateClose()
      // base=4000, jitter=3200ms
      vi.advanceTimersByTime(3200)
      expect(getStore().reconnectAttempt).toBe(3)
    })

    it('caps reconnect delay at 30000ms', () => {
      // Force attempt to a high value where base would exceed 30000
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()

      // Cycle through enough retries to reach the cap
      // attempt 0→800, 1→1600, 2→3200, 3→6400, 4→12800, 5→24000 (base=30000)
      for (let i = 0; i < 5; i++) {
        const idx = MockWebSocket.instances.length - 1
        MockWebSocket.instances[idx].simulateClose()
        // advance by jittered delay: base * 0.8
        const base = Math.min(1000 * Math.pow(2, i), 30000)
        vi.advanceTimersByTime(base * 0.8)
      }

      // At attempt 5, base should be capped at 30000
      // Now trigger close and check the next setTimeout delay
      const lastIdx = MockWebSocket.instances.length - 1
      const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout')
      MockWebSocket.instances[lastIdx].simulateClose()

      // The setTimeout should be called with delay = jitter(capped base)
      const calls = setTimeoutSpy.mock.calls
      const delayArg = calls[calls.length - 1]?.[1] as number
      // Capped base = 30000, jitter with random=0 → 24000
      expect(delayArg).toBeLessThanOrEqual(30000)
    })

    it('applies jitter within [base*0.8, base*1.2]', () => {
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()

      // Test lower bound: random=0 → jitter(base)=base*0.8
      vi.spyOn(Math, 'random').mockReturnValue(0)
      const spyLow = vi.spyOn(globalThis, 'setTimeout')
      MockWebSocket.instances[0].simulateClose()
      const lowCalls = spyLow.mock.calls
      const lowDelay = lowCalls[lowCalls.length - 1]?.[1] as number
      // base = 1000, expected = 800
      expect(lowDelay).toBe(800)
      spyLow.mockRestore()

      // Reset store for upper bound test
      useWSStore.setState({ status: 'idle', reconnectAttempt: 0 })
      MockWebSocket.instances = []
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()

      // Test upper bound: random=1 → jitter(base)=base*1.2
      vi.spyOn(Math, 'random').mockReturnValue(1)
      const spyHigh = vi.spyOn(globalThis, 'setTimeout')
      MockWebSocket.instances[0].simulateClose()
      const highCalls = spyHigh.mock.calls
      const highDelay = highCalls[highCalls.length - 1]?.[1] as number
      // base = 1000, expected = 1200
      expect(highDelay).toBe(1200)
      spyHigh.mockRestore()
    })
  })

  describe('_clearSlices', () => {
    it('clears all message slices', () => {
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()
      MockWebSocket.instances[0].simulateMessage({ type: 'analysis_result', payload: { content: 'x' } })
      MockWebSocket.instances[0].simulateMessage({ type: 'contextual_recall', payload: { fact: 'y' } })

      expect(getStore().analysisResults).toHaveLength(1)
      expect(getStore().recallItems).toHaveLength(1)

      getStore()._clearSlices()

      expect(getStore().analysisResults).toEqual([])
      expect(getStore().recallItems).toEqual([])
    })
  })
})
