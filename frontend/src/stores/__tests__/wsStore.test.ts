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
    lastErrorRequestId: null,
    reconnectAttempt: 0,
    activeUniverseId: null,
    analysisResults: [],
    contradictions: [],
    discoveredEntities: [],
    recallItems: [],
    graphPings: [],
    ingestionProgress: {},
    pipeline: null,
    budget: null,
    submissions: {},
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

    it('queues a paragraph submission while the socket is connecting and flushes it on open', () => {
      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      getStore().send({
        type: 'paragraph_submit',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', chapter_id: 'chapter', text: 'hello' },
      })
      expect(ws.sentMessages).toHaveLength(0)
      expect(getStore().submissions['submission-1']?.phase).toBe('submitted')

      ws.simulateOpen()
      expect(ws.sentMessages).toHaveLength(2)
      expect(JSON.parse(ws.sentMessages[1]).type).toBe('paragraph_submit')
    })

    it('queues a paragraph submission before connect rather than dropping it', () => {
      getStore().send({
        type: 'paragraph_submit',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', text: 'hello' },
      })
      expect(MockWebSocket.instances.length).toBe(0)
      expect(getStore().submissions['submission-1']?.phase).toBe('submitted')

      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      ws.simulateOpen()
      expect(JSON.parse(ws.sentMessages[1]).payload.submission_id).toBe('submission-1')
    })

    it('applies a same-universe terminal event after a queued retry reconnects', () => {
      getStore().setUniverseScope('uni-a')
      getStore().send({
        type: 'paragraph_submit',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', universe_id: 'uni-a', text: 'hello' },
      })

      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      ws.simulateOpen()
      ws.simulateMessage({
        type: 'analysis_result',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', universe_id: 'uni-a' },
      })

      expect(getStore().submissions['submission-1']?.phase).toBe('done')
      expect(getStore().analysisResults).toHaveLength(1)
    })

    it('bounds the outbound queue and makes an overflow visible as failed', () => {
      getStore().connect('test-token')
      const ws = MockWebSocket.instances[0]
      for (let i = 0; i < 51; i++) {
        getStore().send({
          type: 'paragraph_submit',
          payload: { submission_id: `submission-${i}`, paragraph_ref: `chapter:${i}`, text: `paragraph ${i}` },
        })
      }
      expect(getStore().submissions['submission-0']).toMatchObject({ phase: 'failed' })
      ws.simulateOpen()
      // auth_init plus the bounded queue of 50 paragraph submissions.
      expect(ws.sentMessages).toHaveLength(51)
      expect(JSON.parse(ws.sentMessages[1]).payload.submission_id).toBe('submission-1')
    })
  })

  describe('message dispatch', () => {
    beforeEach(() => {
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()
      getStore().setUniverseScope('uni-a')
    })

    it('dispatches analysis_result to analysisResults slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'analysis_result', payload: { universe_id: 'uni-a', content: 'analysis' } })
      expect(getStore().analysisResults).toHaveLength(1)
      expect(getStore().analysisResults[0].content).toBe('analysis')
    })

    it('surfaces the Arbiter synthesis when analysis_result carries one, and ignores an empty one', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'analysis_result',
        payload: { universe_id: 'uni-a', arbiter_summary: 'The contradiction about Edric matters most.' },
      })
      expect(getStore().arbiterNote).toBe('The contradiction about Edric matters most.')

      ws.simulateMessage({ type: 'analysis_result', payload: { universe_id: 'uni-a', arbiter_summary: '' } })
      // An empty synthesis (nothing to adjudicate) must not blank out a
      // still-relevant prior note from an earlier paragraph in this chapter.
      expect(getStore().arbiterNote).toBe('The contradiction about Edric matters most.')
    })

    it('tracks a submission from progress to terminal result', () => {
      const ws = MockWebSocket.instances[0]
      getStore().send({
        type: 'paragraph_submit',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', chapter_id: 'chapter', universe_id: 'uni-a', text: 'hello' },
      })
      ws.simulateMessage({
        type: 'analysis_progress',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', chapter_id: 'chapter', universe_id: 'uni-a', stage: 'entities_extracted' },
      })
      expect(getStore().submissions['submission-1']?.phase).toBe('analyzing')

      ws.simulateMessage({
        type: 'analysis_result',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', chapter_id: 'chapter', universe_id: 'uni-a' },
      })
      expect(getStore().submissions['submission-1']?.phase).toBe('done')
    })

    it('ignores missing or foreign correlation and applies the matching universe event', () => {
      const ws = MockWebSocket.instances[0]
      getStore().send({
        type: 'paragraph_submit',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', universe_id: 'uni-a', text: 'hello' },
      })

      ws.simulateMessage({ type: 'analysis_result', payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1' } })
      ws.simulateMessage({ type: 'analysis_result', payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', universe_id: 'uni-b' } })
      expect(getStore().submissions['submission-1']?.phase).toBe('submitted')
      expect(getStore().analysisResults).toEqual([])

      ws.simulateMessage({ type: 'analysis_result', payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', universe_id: 'uni-a' } })
      expect(getStore().submissions['submission-1']?.phase).toBe('done')
      expect(getStore().analysisResults).toHaveLength(1)
    })

    it('ignores scoped messages until an active universe is established', () => {
      const ws = MockWebSocket.instances[0]
      getStore().setUniverseScope(null)
      ws.simulateMessage({ type: 'ingestion_progress', payload: { job_id: 'job-1', universe_id: 'uni-a', status: 'running' } })
      expect(getStore().ingestionProgress).toEqual({})
    })

    it('scopes display slices to the active universe without discarding prior submission lifecycle', () => {
      const ws = MockWebSocket.instances[0]
      getStore().setUniverseScope('uni-a')
      getStore().send({
        type: 'paragraph_submit',
        payload: {
          submission_id: 'submission-a',
          paragraph_ref: 'chapter-a:1',
          chapter_id: 'chapter-a',
          universe_id: 'uni-a',
          text: 'hello',
        },
      })
      ws.simulateMessage({
        type: 'analysis_progress',
        payload: { submission_id: 'submission-a', paragraph_ref: 'chapter-a:1', chapter_id: 'chapter-a', universe_id: 'uni-a', stage: 'entities_extracted' },
      })
      expect(getStore().pipeline?.stage).toBe('entities_extracted')

      getStore().setUniverseScope('uni-b')
      expect(getStore().pipeline).toBeNull()
      expect(getStore().submissions['submission-a']?.phase).toBe('analyzing')

      ws.simulateMessage({
        type: 'analysis_result',
        payload: { submission_id: 'submission-a', paragraph_ref: 'chapter-a:1', chapter_id: 'chapter-a', universe_id: 'uni-a' },
      })
      ws.simulateMessage({
        type: 'entity_discovered',
        payload: { universe_id: 'uni-a', entity: { id: 'entity-a', name: 'Alice', status: 'candidate', universe_id: 'uni-a' } },
      })

      expect(getStore().submissions['submission-a']?.phase).toBe('analyzing')
      expect(getStore().analysisResults).toEqual([])
      expect(getStore().discoveredEntities).toEqual([])
      expect(getStore().liveCandidates).toEqual([])
    })

    it('tracks analysis_failed as a visible terminal state', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'analysis_failed',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', chapter_id: 'chapter', universe_id: 'uni-a', reason: 'backend stopped' },
      })
      expect(getStore().submissions['submission-1']).toMatchObject({ phase: 'failed', reason: 'backend stopped' })
    })

    it('marks an in-flight submission failed when the backend connection drops', () => {
      const ws = MockWebSocket.instances[0]
      getStore().send({
        type: 'paragraph_submit',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', chapter_id: 'chapter', universe_id: 'uni-a', text: 'hello' },
      })
      ws.simulateMessage({
        type: 'analysis_progress',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', chapter_id: 'chapter', universe_id: 'uni-a', stage: 'entities_extracted' },
      })
      ws.simulateClose()
      expect(getStore().submissions['submission-1']).toMatchObject({ phase: 'failed' })
    })

    it('dispatches contradiction_alert to contradictions slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'contradiction_alert', payload: { universe_id: 'uni-a', contradiction: { message: 'conflict', severity: 'high' } } })
      expect(getStore().contradictions).toHaveLength(1)
      expect(getStore().contradictions[0].message).toBe('conflict')
    })

    it('dispatches entity_discovered to discoveredEntities slice, unwrapping the nested entity payload', () => {
      const ws = MockWebSocket.instances[0]
      // Matches models.EntityDiscoveredPayload: the entity is nested, not flat.
      ws.simulateMessage({
        type: 'entity_discovered',
        payload: { universe_id: 'uni-a', is_new: true, entity: { id: 'e1', name: 'Alice', type: 'character' } },
      })
      expect(getStore().discoveredEntities).toHaveLength(1)
      expect(getStore().discoveredEntities[0].name).toBe('Alice')
    })

    it('dispatches contextual_recall items to recallItems slice, one entry per item', () => {
      const ws = MockWebSocket.instances[0]
      // Matches models.ContextualRecallPayload: facts arrive as an items array.
      ws.simulateMessage({
        type: 'contextual_recall',
        payload: {
          universe_id: 'uni-a',
          items: [
            { id: 'r1', fact: 'something', score: 0.9 },
            { id: 'r2', fact: 'something else', score: 0.7 },
          ],
        },
      })
      expect(getStore().recallItems).toHaveLength(2)
      expect(getStore().recallItems[0].fact).toBe('something')
      expect(getStore().recallItems[1].fact).toBe('something else')
    })

    it('dispatches graph_updated to graphPings slice', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'graph_updated', payload: { universe_id: 'uni-a', updated: true } })
      expect(getStore().graphPings).toHaveLength(1)
    })

    it('dispatches analysis_progress to pipeline slice, tolerating missing counts', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'analysis_progress',
        payload: { universe_id: 'uni-a', stage: 'checking_contradictions', chapter_id: 'ch-1' },
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
        payload: { universe_id: 'uni-a', stage: 'context_budget', chapter_id: 'ch-1', entity_count: 3, budget },
      })
      expect(getStore().pipeline?.stage).toBe('context_budget')
      expect(getStore().pipeline?.entity_count).toBe(3)
      expect(getStore().budget).toEqual(budget)
    })

    it('dispatches ingestion_progress keyed by job_id', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'ingestion_progress',
        payload: { job_id: 'job-1', universe_id: 'uni-a', status: 'running', chapters_processed: 2, total_chapters: 5, action: 'Extracting entities', eta_seconds: 12 },
      })
      expect(getStore().ingestionProgress['job-1']).toEqual({
        job_id: 'job-1',
        universe_id: 'uni-a',
        status: 'running',
        chapters_processed: 2,
        total_chapters: 5,
        action: 'Extracting entities',
        eta_seconds: 12,
      })
    })

    it('merges later ingestion_progress updates for the same job_id', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'ingestion_progress',
        payload: { job_id: 'job-1', universe_id: 'uni-a', status: 'running', chapters_processed: 1, total_chapters: 5 },
      })
      ws.simulateMessage({
        type: 'ingestion_progress',
        payload: { job_id: 'job-1', universe_id: 'uni-a', status: 'running', chapters_processed: 3, total_chapters: 5 },
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
      expect(getStore().lastErrorRequestId).toBeNull()
    })

    it('retains the request ID attached to a craft review error and clears both fields', () => {
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({ type: 'error', payload: { message: 'craft review failed', request_id: 'review-1' } })

      expect(getStore().lastError).toBe('craft review failed')
      expect(getStore().lastErrorRequestId).toBe('review-1')

      getStore().clearError()
      expect(getStore().lastError).toBeNull()
      expect(getStore().lastErrorRequestId).toBeNull()
    })

    it('drops an uncorrelated craft review result', () => {
      getStore().setUniverseScope('uni-a')
      const ws = MockWebSocket.instances[0]
      ws.simulateMessage({
        type: 'craft_review_result',
        payload: { universe_id: 'uni-a', work_id: 'work-a', chapter_id: 'chapter-a', selections: [], notes: [] },
      })

      expect(getStore().craftReviews).toEqual([])
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
      getStore().setUniverseScope('uni-a')
      MockWebSocket.instances[0].simulateMessage({ type: 'analysis_result', payload: { universe_id: 'uni-a', content: 'x' } })
      MockWebSocket.instances[0].simulateMessage({ type: 'contextual_recall', payload: { universe_id: 'uni-a', items: [{ id: 'r1', fact: 'y' }] } })

      expect(getStore().analysisResults).toHaveLength(1)
      expect(getStore().recallItems).toHaveLength(1)

      getStore()._clearSlices()

      expect(getStore().analysisResults).toEqual([])
      expect(getStore().recallItems).toEqual([])
    })
  })

  describe('resetLiveAnalysis', () => {
    it('clears per-paragraph display slices without touching universe scope or submissions', () => {
      getStore().connect('test-token')
      MockWebSocket.instances[0].simulateOpen()
      getStore().setUniverseScope('uni-a')
      MockWebSocket.instances[0].simulateMessage({ type: 'analysis_result', payload: { universe_id: 'uni-a', content: 'x' } })
      MockWebSocket.instances[0].simulateMessage({ type: 'contextual_recall', payload: { universe_id: 'uni-a', items: [{ id: 'r1', fact: 'y' }] } })
      MockWebSocket.instances[0].simulateMessage({ type: 'graph_updated', payload: { universe_id: 'uni-a' } })
      getStore().send({
        type: 'paragraph_submit',
        payload: { submission_id: 'submission-1', paragraph_ref: 'chapter:1', universe_id: 'uni-a', text: 'hello' },
      })

      expect(getStore().analysisResults).toHaveLength(1)
      expect(getStore().recallItems).toHaveLength(1)
      expect(getStore().graphPings).toHaveLength(1)

      getStore().resetLiveAnalysis()

      expect(getStore().analysisResults).toEqual([])
      expect(getStore().recallItems).toEqual([])
      expect(getStore().graphPings).toEqual([])
      expect(getStore().contradictions).toEqual([])
      expect(getStore().discoveredEntities).toEqual([])
      expect(getStore().pipeline).toBeNull()
      expect(getStore().budget).toBeNull()
      // Chapter navigation must not disturb the WS connection scope or
      // in-flight submission tracking used for save-retry feedback.
      expect(getStore().activeUniverseId).toBe('uni-a')
      expect(getStore().submissions['submission-1']).toBeDefined()
    })
  })
})
