import { create } from 'zustand'
import { api } from '../lib/api'

export type SaveStatus = 'idle' | 'saving' | 'saved' | 'failed'

export interface LocalEditorDraft {
  chapterId: string
  content: string
  rawText: string
  updatedAt: number
}

const DRAFT_PREFIX = 'quill:editor-draft:'
const MAX_SAVE_ATTEMPTS = 3
const RETRY_DELAYS_MS = [500, 1_000]
let saveVersion = 0

function draftKey(chapterId: string) {
  return `${DRAFT_PREFIX}${chapterId}`
}

function readDraft(chapterId: string): LocalEditorDraft | null {
  if (typeof window === 'undefined' || !chapterId) return null
  try {
    const value = JSON.parse(window.localStorage.getItem(draftKey(chapterId)) || 'null') as Partial<LocalEditorDraft> | null
    if (!value || typeof value.content !== 'string' || typeof value.rawText !== 'string' || typeof value.updatedAt !== 'number') return null
    return { chapterId, content: value.content, rawText: value.rawText, updatedAt: value.updatedAt }
  } catch {
    return null
  }
}

function writeDraft(chapterId: string, content: string, rawText: string) {
  if (typeof window === 'undefined' || !chapterId) return
  const draft: LocalEditorDraft = { chapterId, content, rawText, updatedAt: Date.now() }
  try {
    window.localStorage.setItem(draftKey(chapterId), JSON.stringify(draft))
  } catch {
    // A full/private storage area must not interrupt typing or saving.
  }
}

function wait(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms))
}

interface EditorState {
  content: string
  rawText: string
  wordCount: number
  isSaving: boolean
  saveStatus: SaveStatus
  saveError: string | null
  lastSavedAt: Date | null
  setContent: (content: string, rawText: string, chapterId?: string) => void
  saveContent: (chapterId: string) => Promise<boolean>
  getLocalDraft: (chapterId: string) => LocalEditorDraft | null
  clearLocalDraft: (chapterId: string) => void
}

export const useEditorStore = create<EditorState>((set) => ({
  content: '',
  rawText: '',
  wordCount: 0,
  isSaving: false,
  saveStatus: 'idle',
  saveError: null,
  lastSavedAt: null,

  setContent: (content, rawText, chapterId) => {
    saveVersion += 1
    const wordCount = rawText.split(/\s+/).filter(Boolean).length
    set({ content, rawText, wordCount, isSaving: false, saveStatus: 'idle', saveError: null })
    if (chapterId) writeDraft(chapterId, content, rawText)
  },

  saveContent: async (chapterId) => {
    const requestVersion = ++saveVersion
    const snapshot = useEditorStore.getState()
    const content = snapshot.content
    const rawText = snapshot.rawText
    set({ isSaving: true, saveStatus: 'saving', saveError: null })

    let lastError: unknown = null
    for (let attempt = 1; attempt <= MAX_SAVE_ATTEMPTS; attempt += 1) {
      try {
        await api.updateChapter(chapterId, { content, raw_text: rawText })
        if (requestVersion !== saveVersion) return false
        const current = useEditorStore.getState()
        const isCurrent = current.content === content && current.rawText === rawText
        if (isCurrent) {
          try {
            window.localStorage.removeItem(draftKey(chapterId))
          } catch {
            // Storage cleanup is best effort; the server save already succeeded.
          }
          set({ isSaving: false, saveStatus: 'saved', saveError: null, lastSavedAt: new Date() })
        } else {
          // A newer edit landed while the request was in flight. Keep its draft
          // and let the next debounce save it instead of reporting a false save.
          set({ isSaving: false, saveStatus: 'idle' })
        }
        return true
      } catch (error) {
        if (requestVersion !== saveVersion) return false
        lastError = error
        if (attempt < MAX_SAVE_ATTEMPTS) {
          await wait(RETRY_DELAYS_MS[attempt - 1])
          if (requestVersion !== saveVersion) return false
        }
      }
    }

    if (requestVersion !== saveVersion) return false
    const message = lastError instanceof Error ? lastError.message : 'Could not save chapter'
    set({ isSaving: false, saveStatus: 'failed', saveError: message })
    return false
  },

  getLocalDraft: (chapterId) => readDraft(chapterId),

  clearLocalDraft: (chapterId) => {
    if (typeof window === 'undefined' || !chapterId) return
    try {
      window.localStorage.removeItem(draftKey(chapterId))
    } catch {
      // Ignore storage failures; the editor remains usable.
    }
  },
}))

export { DRAFT_PREFIX }
