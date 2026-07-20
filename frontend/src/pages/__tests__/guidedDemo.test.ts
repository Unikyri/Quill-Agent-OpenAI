import { describe, it, expect, vi, afterEach } from 'vitest'
import { createOpaqueDemoId } from '../guidedDemo'

describe('createOpaqueDemoId', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('uses crypto.randomUUID when available (secure context)', () => {
    const id = createOpaqueDemoId()
    expect(id).toMatch(/^[0-9a-f-]{36}$/)
  })

  it('falls back to crypto.getRandomValues when randomUUID is undefined (insecure context, e.g. plain HTTP on a public IP)', () => {
    const original = globalThis.crypto
    vi.stubGlobal('crypto', {
      getRandomValues: original.getRandomValues.bind(original),
    })

    const id = createOpaqueDemoId()
    expect(id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
  })

  it('throws when neither randomUUID nor getRandomValues is available', () => {
    vi.stubGlobal('crypto', {})
    expect(() => createOpaqueDemoId()).toThrow('Secure browser randomness is required')
  })
})
