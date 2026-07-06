import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ImageUpload from '../ImageUpload'

vi.mock('../ImageUpload.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockUploadImage = vi.fn()
vi.mock('../../../lib/imageUpload', () => ({
  uploadImage: (...args: unknown[]) => mockUploadImage(...args),
  MAX_IMAGE_BYTES: 5 * 1024 * 1024,
}))

function makeFile(name = 'cover.png', type = 'image/png') {
  return new File([new Uint8Array(10)], name, { type })
}

describe('ImageUpload', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows the empty placeholder when there is no value', () => {
    render(<ImageUpload value={null} onChange={vi.fn()} width={104} height={140} placeholder="Cover — drag an image" />)
    expect(screen.getByText('Cover — drag an image')).toBeInTheDocument()
  })

  it('uploads a picked file, shows preview, and calls onChange with the resolved url', async () => {
    mockUploadImage.mockResolvedValue({ url: 'blob:mock-1' })
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<ImageUpload value={null} onChange={onChange} width={104} height={140} />)

    const input = screen.getByTestId('image-upload-input') as HTMLInputElement
    await user.upload(input, makeFile())

    await waitFor(() => expect(onChange).toHaveBeenCalledWith('blob:mock-1'))
    expect(screen.getByRole('img')).toHaveAttribute('src', 'blob:mock-1')
  })

  it('renders an error state and does not call onChange when a dropped file fails validation', async () => {
    // drag-drop bypasses the file input's `accept` filter, so this exercises
    // the real path a non-image file can take (accept only constrains the
    // native OS picker, not drag-and-drop).
    mockUploadImage.mockRejectedValue(new Error('Only image files are supported'))
    const onChange = vi.fn()
    render(<ImageUpload value={null} onChange={onChange} width={104} height={140} />)

    const dropzone = screen.getByTestId('image-upload-input').parentElement as HTMLElement
    const file = makeFile('notes.txt', 'text/plain')
    fireEvent.drop(dropzone, { dataTransfer: { files: [file] } })

    await waitFor(() => {
      expect(screen.getByText('Only image files are supported')).toBeInTheDocument()
    })
    expect(onChange).not.toHaveBeenCalled()
  })
})
