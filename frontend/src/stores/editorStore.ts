import { create } from 'zustand'
import { api } from '../lib/api'

interface EditorState {
  content: string
  rawText: string
  wordCount: number
  isSaving: boolean
  lastSavedAt: Date | null
  setContent: (content: string, rawText: string) => void
  saveContent: (chapterId: string) => Promise<void>
}

export const useEditorStore = create<EditorState>((set) => ({
  content: '',
  rawText: '',
  wordCount: 0,
  isSaving: false,
  lastSavedAt: null,

  setContent: (content, rawText) => {
    const wordCount = rawText.split(/\s+/).filter(Boolean).length
    set({ content, rawText, wordCount })
  },

  saveContent: async (chapterId) => {
    set({ isSaving: true })
    try {
      const { rawText, content } = useEditorStore.getState()
      await api.updateChapter(chapterId, { content, raw_text: rawText })
      set({ isSaving: false, lastSavedAt: new Date() })
    } catch (err) {
      set({ isSaving: false })
      throw err
    }
  },
}))
