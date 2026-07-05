import type { Editor } from '@tiptap/react'
import styles from './Toolbar.module.css'

interface ToolbarProps {
  editor: Editor | null
}

export default function Toolbar({ editor }: ToolbarProps) {
  if (!editor) return null

  const setLink = () => {
    const previousUrl = editor.getAttributes('link').href
    const url = window.prompt('URL', previousUrl)
    // cancelled
    if (url === null) return
    // empty string removes link
    if (url === '') {
      editor.chain().focus().extendMarkRange('link').unsetLink().run()
      return
    }
    editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
  }

  return (
    <div className={styles.toolbar}>
      <button
        className={`${styles.button} ${editor.isActive('bold') ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleBold().run()}
        title="Bold (Ctrl+B)"
      >
        <strong>B</strong>
      </button>
      <button
        className={`${styles.button} ${editor.isActive('italic') ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleItalic().run()}
        title="Italic (Ctrl+I)"
      >
        <em>I</em>
      </button>
      <div className={styles.separator} />
      <button
        className={`${styles.button} ${editor.isActive('underline') ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleUnderline().run()}
        title="Underline (Ctrl+U)"
      >
        <span style={{ textDecoration: 'underline' }}>U</span>
      </button>
      <button
        className={`${styles.button} ${editor.isActive('highlight') ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleHighlight().run()}
        title="Highlight"
      >
        <mark>H</mark>
      </button>
      <div className={styles.separator} />
      <button
        className={`${styles.button} ${editor.isActive('link') ? styles.buttonActive : ''}`}
        onClick={setLink}
        title="Add link"
      >
        🔗
      </button>
      <div className={styles.separator} />
      <button
        className={`${styles.button} ${editor.isActive('heading', { level: 1 }) ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
        title="Heading 1"
      >
        H1
      </button>
      <button
        className={`${styles.button} ${editor.isActive('heading', { level: 2 }) ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
        title="Heading 2"
      >
        H2
      </button>
      <button
        className={`${styles.button} ${editor.isActive('heading', { level: 3 }) ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
        title="Heading 3"
      >
        H3
      </button>
      <div className={styles.separator} />
      <button
        className={`${styles.button} ${editor.isActive('bulletList') ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleBulletList().run()}
        title="Bullet list"
      >
        •—
      </button>
      <button
        className={`${styles.button} ${editor.isActive('blockquote') ? styles.buttonActive : ''}`}
        onClick={() => editor.chain().focus().toggleBlockquote().run()}
        title="Blockquote"
      >
        ❝
      </button>
      <button
        className={styles.button}
        onClick={() => editor.chain().focus().setHorizontalRule().run()}
        title="Horizontal rule"
      >
        ―
      </button>
    </div>
  )
}
