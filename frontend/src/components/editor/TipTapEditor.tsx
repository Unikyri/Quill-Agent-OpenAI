import { useEffect, useRef, useState, type MouseEvent, type ReactNode } from 'react'
import { useEditor, EditorContent, type Editor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import Highlight from '@tiptap/extension-highlight'
import Underline from '@tiptap/extension-underline'
import Link from '@tiptap/extension-link'
import { useWSStore, type SubmissionLifecycle } from '../../stores/wsStore'
import EntityHighlight, { type EntityHighlightEntity } from './entityHighlightExtension'
import CandidateHighlight, { candidateHighlightKey, type CandidateHighlightEntity } from './candidateHighlightExtension'
import styles from './TipTapEditor.module.css'

interface TipTapEditorProps {
  chapterId: string
  workId: string
  universeId: string
  initialContent?: string
  onContentChange?: (html: string, text: string) => void
  onCraftReview?: (selection: { passage: string; from: number; to: number }) => void
  knownEntities?: EntityHighlightEntity[]
  onEntityClick?: (entityId: string) => void
  candidateEntities?: CandidateHighlightEntity[]
  onCandidateDecision?: (candidateId: string, decision: 'accept' | 'dismiss') => void
  reviewing?: boolean
}

function ToolbarButton({
  active, title, children, onClick, disabled, onMouseDown, ariaLabel,
}: {
  active?: boolean; title: string; children: ReactNode; onClick: () => void; disabled?: boolean; onMouseDown?: (event: MouseEvent<HTMLButtonElement>) => void; ariaLabel?: string
}) {
  return (
    <button
      type="button"
      className={`${styles.toolbarBtn} ${active ? styles.toolbarBtnActive : ''}`}
      title={title}
      aria-label={ariaLabel}
      onClick={onClick}
      onMouseDown={onMouseDown}
      disabled={disabled}
      tabIndex={-1}
    >
      {children}
    </button>
  )
}

function Toolbar({
  editor,
  fontSize,
  setFontSize,
  onCraftReview,
  reviewing,
}: {
  editor: Editor | null
  fontSize: number
  setFontSize: (s: number) => void
  onCraftReview?: (selection: { passage: string; from: number; to: number }) => void
  reviewing?: boolean
}) {
  if (!editor) return null
  return (
    <div className={styles.toolbar}>
      <ToolbarButton title="Decrease font size" onClick={() => setFontSize(Math.max(12, fontSize - 1))}>
        <span style={{ fontSize: 13 }}>A-</span>
      </ToolbarButton>
      <ToolbarButton title="Increase font size" onClick={() => setFontSize(Math.min(32, fontSize + 1))}>
        <span style={{ fontSize: 15 }}>A+</span>
      </ToolbarButton>
      <div className={styles.toolbarDivider} />
      <ToolbarButton
        active={editor.isActive('bold')}
        title="Bold (⌘B)"
        onClick={() => editor.chain().focus().toggleBold().run()}
      >
        <b>B</b>
      </ToolbarButton>
      <ToolbarButton
        active={editor.isActive('italic')}
        title="Italic (⌘I)"
        onClick={() => editor.chain().focus().toggleItalic().run()}
      >
        <i>I</i>
      </ToolbarButton>
      <ToolbarButton
        active={editor.isActive('underline')}
        title="Underline (⌘U)"
        onClick={() => editor.chain().focus().toggleUnderline().run()}
      >
        <u>U</u>
      </ToolbarButton>
      <div className={styles.toolbarDivider} />
      <ToolbarButton
        active={editor.isActive('heading', { level: 1 })}
        title="Heading 1"
        onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
      >
        H1
      </ToolbarButton>
      <ToolbarButton
        active={editor.isActive('heading', { level: 2 })}
        title="Heading 2"
        onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
      >
        H2
      </ToolbarButton>
      <div className={styles.toolbarDivider} />
      <ToolbarButton
        active={editor.isActive('bulletList')}
        title="Bullet list"
        onClick={() => editor.chain().focus().toggleBulletList().run()}
      >
        •
      </ToolbarButton>
      <ToolbarButton
        active={editor.isActive('orderedList')}
        title="Numbered list"
        onClick={() => editor.chain().focus().toggleOrderedList().run()}
      >
        1.
      </ToolbarButton>
      <div className={styles.toolbarDivider} />
      <ToolbarButton
        active={editor.isActive('highlight')}
        title="Highlight"
        onClick={() => editor.chain().focus().toggleHighlight().run()}
      >
        <span style={{ fontSize: 11 }}>▐</span>
      </ToolbarButton>
      <ToolbarButton
        active={editor.isActive('blockquote')}
        title="Quote block"
        onClick={() => editor.chain().focus().toggleBlockquote().run()}
      >
        "
      </ToolbarButton>
      {onCraftReview && (
        <>
          <div className={styles.toolbarDivider} />
          <ToolbarButton
            title={reviewing ? 'Reviewing selection…' : 'Review selected passage'}
            ariaLabel="Review selected passage"
            disabled={reviewing || editor.state.selection.empty}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              const { from, to } = editor.state.selection
              const passage = editor.state.doc.textBetween(from, to, '\n').trim()
              if (passage) onCraftReview({ passage, from, to })
            }}
          >
            {reviewing ? '◌' : '✦'}
          </ToolbarButton>
        </>
      )}
    </div>
  )
}

export default function TipTapEditor({
  chapterId,
  workId,
  universeId,
  initialContent,
  onContentChange,
  onCraftReview,
  knownEntities = [],
  onEntityClick,
  candidateEntities = [],
  onCandidateDecision,
  reviewing,
}: TipTapEditorProps) {
  const send = useWSStore((s) => s.send)
  const submissions = useWSStore((s) => s.submissions)
  const submitTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const lastParagraphTextByRef = useRef<Record<string, string>>({})
  const submissionSequenceRef = useRef(0)
  const [fontSize, setFontSize] = useState(17)
  const [selectedCandidateId, setSelectedCandidateId] = useState<string | null>(null)
  const chapterSubmissions = Object.values(submissions)
    .filter((submission) => submission.chapterId === chapterId)
    .sort((left, right) => right.updatedAt - left.updatedAt)

  const editor = useEditor({
    extensions: [
      StarterKit,
      Placeholder.configure({ placeholder: 'Start writing…' }),
      Highlight,
      Underline,
      Link.configure({ openOnClick: false }),
      EntityHighlight.configure({ entities: knownEntities }),
      CandidateHighlight.configure({ candidates: candidateEntities }),
    ],
    content: initialContent || '',
    onUpdate: ({ editor }) => {
      const html = editor.getHTML()
      const text = editor.getText()
      onContentChange?.(html, text)

      // Capture the changed block while the edit transaction is current. The
      // cursor may move before the debounce expires, so resolving it later
      // would submit a different paragraph.
      const selectedParagraph = getParagraphAtSelection(editor)
      const paragraph = selectedParagraph ? { ...selectedParagraph, ref: `${chapterId}:${selectedParagraph.ref}` } : null
      if (!paragraph || !paragraph.text.trim()) return

      // Debounced paragraph submit for live AI analysis.
      if (submitTimerRef.current) clearTimeout(submitTimerRef.current)
      submitTimerRef.current = setTimeout(() => {
        if (paragraph.text !== lastParagraphTextByRef.current[paragraph.ref]) {
          lastParagraphTextByRef.current[paragraph.ref] = paragraph.text
          submissionSequenceRef.current += 1
          const submissionId = createSubmissionId(submissionSequenceRef.current)
          send({
            type: 'paragraph_submit',
            payload: {
              submission_id: submissionId,
              paragraph_ref: paragraph.ref,
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

  // Keep the ProseMirror plugin instance stable while live candidates arrive.
  // Recreating TipTap here would reset the document selection/cursor on every
  // WebSocket event; plugin metadata lets it rebuild only its decorations.
  useEffect(() => {
    if (!editor?.view?.dispatch) return
    editor.view.dispatch(editor.state.tr.setMeta(candidateHighlightKey, candidateEntities))
  }, [editor, candidateEntities])

  // Sync initial content when chapter changes
  useEffect(() => {
    if (editor && initialContent !== undefined && editor.getHTML() !== initialContent) {
      if (!editor.getText() || initialContent !== editor.getHTML()) {
        editor.commands.setContent(initialContent || '')
      }
    }
  }, [chapterId]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    return () => {
      if (submitTimerRef.current) clearTimeout(submitTimerRef.current)
    }
  }, [])

  return (
    <div className={styles.wrapper}>
      <Toolbar
        editor={editor}
        fontSize={fontSize}
        setFontSize={setFontSize}
        onCraftReview={onCraftReview}
        reviewing={reviewing}
      />
      {chapterSubmissions.length > 0 && <AnalysisStatusList submissions={chapterSubmissions} />}
      {selectedCandidateId && onCandidateDecision && (() => {
        const candidate = candidateEntities.find((item) => item.id === selectedCandidateId)
        if (!candidate) return null
        return (
          <div className={styles.candidateActions} role="status">
            <strong>{candidate.name}</strong>
            <span>{Math.round((candidate.confidence ?? 0) * 100)}% confidence</span>
            <button type="button" onClick={() => { onCandidateDecision(candidate.id, 'accept'); setSelectedCandidateId(null) }}>Accept</button>
            <button type="button" onClick={() => { onCandidateDecision(candidate.id, 'dismiss'); setSelectedCandidateId(null) }}>Dismiss</button>
          </div>
        )
      })()}
      <div
        className={`${styles.editorContent} q-scroll`}
        style={{ fontSize: `${fontSize}px` }}
        onClick={(event) => {
          if (!onEntityClick) return
          const target = event.target as HTMLElement
          const entity = target.closest<HTMLElement>('[data-entity-id]')
          if (entity?.dataset.entityId) onEntityClick(entity.dataset.entityId)
          const candidate = target.closest<HTMLElement>('[data-candidate-id]')
          if (candidate?.dataset.candidateId) setSelectedCandidateId(candidate.dataset.candidateId)
        }}
      >
        <EditorContent editor={editor} />
      </div>
    </div>
  )
}

function getParagraphAtSelection(editor: Editor): { text: string; ref: string } | null {
  const { from } = editor.state.selection
  const doc = editor.state.doc
  const resolved = doc.resolve(from)
  const node = resolved.parent
  if (!node || !node.isBlock) return null
  const start = typeof resolved.start === 'function' ? resolved.start(resolved.depth) : from
  return { text: node.textContent, ref: `${start}` }
}

function createSubmissionId(sequence: number): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `submission-${Date.now()}-${sequence}`
}

function AnalysisStatusList({ submissions }: { submissions: SubmissionLifecycle[] }) {
  return (
    <div className={styles.analysisStatusList} aria-label="Paragraph analysis statuses">
      {submissions.map((submission) => <AnalysisStatus key={submission.submissionId} submission={submission} />)}
    </div>
  )
}

function AnalysisStatus({ submission }: { submission: SubmissionLifecycle }) {
  const label = submission.phase === 'submitted'
    ? 'Queued for analysis'
    : submission.phase === 'analyzing'
      ? 'Analyzing paragraph'
      : submission.phase === 'done'
        ? 'Analysis complete'
        : 'Analysis failed'
  const glyph = submission.phase === 'done' ? '✓' : submission.phase === 'failed' ? '!' : '◌'
  return (
    <div className={styles.analysisStatus} data-phase={submission.phase} data-testid="analysis-submission-status" data-paragraph-ref={submission.paragraphRef} role="status">
      <span className={styles.analysisStatusGlyph}>{glyph}</span>
      <span>{label}</span>
      {submission.phase === 'failed' && submission.reason && <span className={styles.analysisStatusReason}>{submission.reason}</span>}
    </div>
  )
}
