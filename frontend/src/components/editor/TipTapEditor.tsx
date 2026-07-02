import { useEffect, useRef } from 'react'
import { useEditor, EditorContent, type Editor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import Highlight from '@tiptap/extension-highlight'
import Underline from '@tiptap/extension-underline'
import Link from '@tiptap/extension-link'
import { useWSStore } from '../../stores/wsStore'
import Toolbar from './Toolbar'
import styles from './TipTapEditor.module.css'

interface TipTapEditorProps {
  chapterId: string
  workId: string
  universeId: string
  initialContent?: string
  onContentChange?: (html: string, text: string) => void
}

export default function TipTapEditor({
  chapterId,
  workId,
  universeId,
  initialContent,
  onContentChange,
}: TipTapEditorProps) {
  const send = useWSStore((s) => s.send)
  const submitTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const lastParagraphTextRef = useRef<string>('')

  const editor = useEditor({
    extensions: [
      StarterKit,
      Placeholder.configure({ placeholder: 'Start writing…' }),
      Highlight,
      Underline,
      Link.configure({ openOnClick: false }),
    ],
    content: initialContent || '',
    onUpdate: ({ editor }) => {
      const html = editor.getHTML()
      const text = editor.getText()
      onContentChange?.(html, text)

      // Debounced paragraph submit: detect paragraph boundary on idle
      if (submitTimerRef.current) clearTimeout(submitTimerRef.current)
      submitTimerRef.current = setTimeout(() => {
        const paragraph = getParagraphAtCursor(editor)
        if (paragraph && paragraph.text.trim() && paragraph.text !== lastParagraphTextRef.current) {
          lastParagraphTextRef.current = paragraph.text
          send({
            type: 'paragraph_submit',
            payload: {
              work_id: workId,
              chapter_id: chapterId,
              universe_id: universeId,
              text: paragraph.text,
            },
          })
        }
      }, 5000)
    },
  })

  // Sync initial content when chapter changes
  useEffect(() => {
    if (editor && initialContent !== undefined && editor.getHTML() !== initialContent) {
      // Only reset content if editor is empty or content really changed
      if (!editor.getText() || initialContent !== editor.getHTML()) {
        editor.commands.setContent(initialContent || '')
      }
    }
  }, [chapterId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (submitTimerRef.current) clearTimeout(submitTimerRef.current)
    }
  }, [])

  return (
    <div className={styles.wrapper}>
      <Toolbar editor={editor} />
      <div className={styles.editorContent}>
        <EditorContent editor={editor} />
      </div>
    </div>
  )
}

// ponytail: simple cursor-relative paragraph extraction; could cache paragraph boundaries if perf matters
function getParagraphAtCursor(editor: Editor): { text: string } | null {
  const { from } = editor.state.selection
  const doc = editor.state.doc

  // Find the parent paragraph node containing the cursor
  const resolved = doc.resolve(from)
  let node = resolved.parent

  // Walk up to find the nearest block node (paragraph)
  while (node && !node.isBlock && resolved.depth > 0) {
    const parentResolved = doc.resolve(resolved.before(resolved.depth))
    node = parentResolved.parent
  }

  if (!node || !node.isBlock) return null

  // Get the text content of this block node
  const text = node.textContent
  return { text }
}
