export interface GuidedDemoProgress {
  openedWriting: boolean
  observedAnalysis: boolean
  openedMap: boolean
}

const DEMO_UNIVERSE_KEY = 'quill-guided-demo-universe-id'
const DEMO_SESSION_KEY = 'quill-guided-demo-session-id'
const DEMO_PROGRESS_PREFIX = 'quill-guided-demo-progress:'

const EMPTY_PROGRESS: GuidedDemoProgress = {
  openedWriting: false,
  observedAnalysis: false,
  openedMap: false,
}

function safeStorage(kind: 'localStorage' | 'sessionStorage'): Storage | null {
  if (typeof window === 'undefined') return null

  try {
    return window[kind]
  } catch {
    return null
  }
}

function progressKey(universeId: string) {
  return `${DEMO_PROGRESS_PREFIX}${universeId}`
}

export function createOpaqueDemoId() {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }

  // crypto.randomUUID() is gated to secure contexts (HTTPS or localhost);
  // crypto.getRandomValues() is not, so it still works over plain HTTP on a
  // public IP — which is how this app gets deployed for hackathon judging.
  if (typeof globalThis.crypto?.getRandomValues === 'function') {
    const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16))
    bytes[6] = (bytes[6] & 0x0f) | 0x40
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  }

  throw new Error('Secure browser randomness is required to start the guided demo.')
}

export function guidedDemoSessionId(): string {
  const storage = safeStorage('localStorage')
  if (!storage) {
    throw new Error('Browser storage is required to keep the guided demo session private.')
  }
  const existing = storage.getItem(DEMO_SESSION_KEY)
  if (existing) return existing

  const sessionId = createOpaqueDemoId()
  storage.setItem(DEMO_SESSION_KEY, sessionId)
  return sessionId
}

export function rememberGuidedDemoUniverse(universeId: string) {
  const storage = safeStorage('localStorage')
  const previousUniverseId = storage?.getItem(DEMO_UNIVERSE_KEY)
  const progressStorage = safeStorage('sessionStorage')

  if (previousUniverseId && previousUniverseId !== universeId) {
    progressStorage?.removeItem(progressKey(previousUniverseId))
  }

  storage?.setItem(DEMO_UNIVERSE_KEY, universeId)
  progressStorage?.removeItem(progressKey(universeId))
}

export function isGuidedDemoUniverse(universeId: string | undefined): boolean {
  return Boolean(universeId && safeStorage('localStorage')?.getItem(DEMO_UNIVERSE_KEY) === universeId)
}

export function readGuidedDemoProgress(universeId: string): GuidedDemoProgress {
  const raw = safeStorage('sessionStorage')?.getItem(progressKey(universeId))
  if (!raw) return EMPTY_PROGRESS

  try {
    const parsed = JSON.parse(raw) as Partial<GuidedDemoProgress>
    return {
      openedWriting: parsed.openedWriting === true,
      observedAnalysis: parsed.observedAnalysis === true,
      openedMap: parsed.openedMap === true,
    }
  } catch {
    return EMPTY_PROGRESS
  }
}

export function recordGuidedDemoProgress(
  universeId: string,
  updates: Partial<GuidedDemoProgress>,
): GuidedDemoProgress {
  const next = { ...readGuidedDemoProgress(universeId), ...updates }
  safeStorage('sessionStorage')?.setItem(progressKey(universeId), JSON.stringify(next))
  return next
}
