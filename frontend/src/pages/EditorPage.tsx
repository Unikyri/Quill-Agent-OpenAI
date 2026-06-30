import { useEffect, useRef, useCallback } from 'react'
import { useParams } from 'react-router-dom'
import { useEditorStore } from '../stores/editorStore'
import { api } from '../lib/api'

export default function EditorPage() {
  const { chapterId } = useParams<{ chapterId: string }>()
  const { content, rawText, wordCount, isSaving, lastSavedAt, setContent, saveContent } = useEditorStore()
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const saveTimerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    if (chapterId) {
      api.getChapter(chapterId).then(({ chapter }) => {
        setContent(chapter.content || '', chapter.raw_text || '')
      })
    }
  }, [chapterId])

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newRaw = e.target.value
    const html = `<p>${newRaw.split('\n').filter(Boolean).map((p) => `<p>${p}</p>`).join('')}</p>`
    setContent(html, newRaw)

    // Auto-save after 5 seconds of inactivity
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    saveTimerRef.current = setTimeout(() => {
      if (chapterId) saveContent(chapterId)
    }, 5000)
  }, [chapterId, setContent, saveContent])

  return (
    <div style={{ display: 'flex', height: '100vh' }}>
      {/* Editor */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        <div style={{ padding: '12px 24px', borderBottom: '1px solid #333', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ color: '#888' }}>Chapter Editor</span>
          <div style={{ display: 'flex', gap: 16, color: '#888', fontSize: 12 }}>
            <span>{wordCount} words</span>
            <span>{isSaving ? 'Saving...' : lastSavedAt ? `Saved ${lastSavedAt.toLocaleTimeString()}` : ''}</span>
          </div>
        </div>
        <textarea
          ref={textareaRef}
          value={rawText}
          onChange={handleChange}
          placeholder="Start writing..."
          style={{
            flex: 1,
            background: 'transparent',
            border: 'none',
            padding: 24,
            fontSize: 16,
            lineHeight: 1.8,
            resize: 'none',
            color: '#e0e0e0',
          }}
        />
      </div>

      {/* Sidebar placeholder */}
      <div style={{ width: 320, borderLeft: '1px solid #333', padding: 16 }}>
        <h3 style={{ marginBottom: 16 }}>Context Panel</h3>
        <p style={{ color: '#888', fontSize: 14 }}>
          Contextual memories and contradictions will appear here when the memory engine is implemented.
        </p>
      </div>
    </div>
  )
}
