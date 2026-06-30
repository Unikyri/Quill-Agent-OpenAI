import { create } from 'zustand'
import { api } from '../lib/api'

interface Universe { id: string; name: string; genre: string; format: string }
interface Work { id: string; title: string; type: string; order_index: number }
interface Chapter { id: string; title: string; order_index: number; word_count: number }

interface UniverseState {
  universes: Universe[]
  currentUniverse: Universe | null
  currentWork: Work | null
  currentChapter: Chapter | null
  works: Work[]
  chapters: Chapter[]
  fetchUniverses: () => Promise<void>
  selectUniverse: (id: string) => Promise<void>
  selectWork: (id: string) => Promise<void>
  selectChapter: (id: string) => Promise<void>
}

export const useUniverseStore = create<UniverseState>((set) => ({
  universes: [],
  currentUniverse: null,
  currentWork: null,
  currentChapter: null,
  works: [],
  chapters: [],

  fetchUniverses: async () => {
    const { universes } = await api.listUniverses()
    set({ universes })
  },

  selectUniverse: async (id) => {
    const { universe } = await api.getUniverse(id)
    const { works } = await api.listWorks(id)
    set({ currentUniverse: universe, works, currentWork: null, currentChapter: null })
  },

  selectWork: async (id) => {
    const work = useUniverseStore.getState().works.find((w) => w.id === id) || null
    if (work && useUniverseStore.getState().currentUniverse) {
      const { chapters } = await api.listChapters(id)
      set({ currentWork: work, chapters, currentChapter: null })
    }
  },

  selectChapter: async (id) => {
    const { chapter } = await api.getChapter(id)
    set({ currentChapter: chapter })
  },
}))
