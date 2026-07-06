import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { uploadImage, MAX_IMAGE_BYTES } from '../imageUpload'

function makeFile(name: string, type: string, sizeBytes: number): File {
  const blob = new Blob([new Uint8Array(sizeBytes)], { type })
  return new File([blob], name, { type })
}

describe('uploadImage', () => {
  beforeEach(() => {
    vi.stubGlobal('URL', {
      ...URL,
      createObjectURL: vi.fn(() => 'blob:mock-url-1'),
      revokeObjectURL: vi.fn(),
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('resolves with a local object-URL for a valid image file, no network call', async () => {
    const file = makeFile('cover.png', 'image/png', 1024)
    const result = await uploadImage(file)
    expect(result).toEqual({ url: 'blob:mock-url-1' })
    expect(URL.createObjectURL).toHaveBeenCalledWith(file)
  })

  it('rejects a file over the size limit', async () => {
    const file = makeFile('huge.png', 'image/png', MAX_IMAGE_BYTES + 1)
    await expect(uploadImage(file)).rejects.toThrow(/size/i)
  })

  it('rejects a non-image file type', async () => {
    const file = makeFile('notes.txt', 'text/plain', 100)
    await expect(uploadImage(file)).rejects.toThrow(/image/i)
  })
})
