export interface UploadResult {
  url: string
}

export const MAX_IMAGE_BYTES = 5 * 1024 * 1024 // 5MB

// ponytail: stub — returns a local object-URL, no network call. This is the
// single seam for image upload (ADR-4): when a real backend endpoint exists,
// swap this function body for a multipart POST; the signature and every
// caller (ImageUpload component) stay identical.
export async function uploadImage(file: File): Promise<UploadResult> {
  if (!file.type.startsWith('image/')) {
    throw new Error('Only image files are supported')
  }
  if (file.size > MAX_IMAGE_BYTES) {
    throw new Error(`Image exceeds the ${MAX_IMAGE_BYTES / (1024 * 1024)}MB size limit`)
  }
  return { url: URL.createObjectURL(file) }
}
