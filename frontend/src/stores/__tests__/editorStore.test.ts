import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../lib/api'
import { useEditorStore } from '../editorStore'

vi.mock('../../lib/api', () => ({
  api: { updateChapter: vi.fn() },
}))

describe('editorStore durability', () => {
  beforeEach(() => {
    window.localStorage.clear()
    vi.clearAllMocks()
    useEditorStore.setState({
      content: '', rawText: '', wordCount: 0, isSaving: false,
      saveStatus: 'idle', saveError: null, lastSavedAt: null,
    })
  })

  it('persists a chapter draft and removes it only after a successful save', async () => {
    vi.mocked(api.updateChapter).mockResolvedValue({ chapter: {} })
    useEditorStore.getState().setContent('<p>Draft</p>', 'Draft', 'chapter-1')

    const draft = useEditorStore.getState().getLocalDraft('chapter-1')
    expect(draft?.content).toBe('<p>Draft</p>')
    expect(draft?.updatedAt).toBeGreaterThan(0)

    await expect(useEditorStore.getState().saveContent('chapter-1')).resolves.toBe(true)
    expect(api.updateChapter).toHaveBeenCalledWith('chapter-1', { content: '<p>Draft</p>', raw_text: 'Draft' })
    expect(useEditorStore.getState().saveStatus).toBe('saved')
    expect(useEditorStore.getState().getLocalDraft('chapter-1')).toBeNull()
  })

  it('keeps the local draft and reports failed after retry exhaustion', async () => {
    vi.useFakeTimers()
    vi.mocked(api.updateChapter).mockRejectedValue(new Error('offline'))
    useEditorStore.getState().setContent('<p>Offline</p>', 'Offline', 'chapter-2')
    const promise = useEditorStore.getState().saveContent('chapter-2')
    await vi.runAllTimersAsync()
    await expect(promise).resolves.toBe(false)
    expect(useEditorStore.getState().saveStatus).toBe('failed')
    expect(useEditorStore.getState().getLocalDraft('chapter-2')?.rawText).toBe('Offline')
    expect(api.updateChapter).toHaveBeenCalledTimes(3)
    vi.useRealTimers()
  })
})
