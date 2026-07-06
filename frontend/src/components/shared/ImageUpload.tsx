import { useEffect, useRef, useState, type DragEvent } from 'react'
import { uploadImage } from '../../lib/imageUpload'
import styles from './ImageUpload.module.css'

type ImageUploadShape = 'rounded' | 'circle' | 'pill'

interface ImageUploadProps {
  value?: string | null
  onChange: (url: string | null) => void
  shape?: ImageUploadShape
  radius?: number
  width: number
  height: number
  placeholder?: string
  disabled?: boolean
}

type Status = 'empty' | 'loading' | 'preview' | 'error'

export default function ImageUpload({
  value,
  onChange,
  shape = 'rounded',
  radius = 8,
  width,
  height,
  placeholder = 'Drag an image, or click to pick',
  disabled = false,
}: ImageUploadProps) {
  const [status, setStatus] = useState<Status>(value ? 'preview' : 'empty')
  const [error, setError] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)
  // ponytail: optimistic local preview — avoids waiting on the parent to
  // round-trip the resolved url back through the controlled `value` prop.
  const [localUrl, setLocalUrl] = useState<string | null>(value ?? null)
  const inputRef = useRef<HTMLInputElement>(null)
  const previewUrl = localUrl ?? value

  // Sync local preview when the parent's controlled `value` changes
  // externally (e.g. navigating to a different work) — otherwise the
  // previous preview lingers since `localUrl` only initialized on mount.
  useEffect(() => {
    setLocalUrl(value ?? null)
    setStatus(value ? 'preview' : 'empty')
  }, [value])

  const shapeStyle =
    shape === 'circle' ? { borderRadius: '50%' } : shape === 'pill' ? { borderRadius: '999px' } : { borderRadius: radius }

  const handleFile = async (file: File | undefined) => {
    if (!file || disabled) return
    setStatus('loading')
    setError(null)
    try {
      const { url } = await uploadImage(file)
      setStatus('preview')
      setLocalUrl(url)
      onChange(url)
    } catch (err) {
      setStatus('error')
      setError((err as Error).message || 'Upload failed')
    }
  }

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragOver(false)
    handleFile(e.dataTransfer.files?.[0])
  }

  const handleRemove = (e: React.MouseEvent) => {
    e.stopPropagation()
    onChange(null)
    setLocalUrl(null)
    setStatus('empty')
  }

  return (
    <div
      className={styles.slot}
      style={{ width, height, ...shapeStyle }}
      onClick={() => !disabled && inputRef.current?.click()}
      onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handleDrop}
      data-status={status}
      data-drag-over={dragOver || undefined}
    >
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        data-testid="image-upload-input"
        className={styles.input}
        disabled={disabled}
        onChange={(e) => handleFile(e.target.files?.[0])}
      />

      {status === 'preview' && previewUrl && (
        <>
          <img src={previewUrl} alt="Uploaded preview" className={styles.previewImg} />
          <button type="button" className={`glyph ${styles.removeBtn}`} onClick={handleRemove} aria-label="Remove image">
            ✗
          </button>
        </>
      )}

      {status === 'loading' && <div className={`${styles.spinner} glyph`}>⟳</div>}

      {status === 'empty' && (
        <p className={styles.placeholder}>{placeholder}</p>
      )}

      {status === 'error' && (
        <p className={styles.errorText}>{error}</p>
      )}
    </div>
  )
}
