import { describe, it, expect } from 'vitest'
import { buildNestedPath } from '../routeRedirect'

describe('buildNestedPath', () => {
  it('builds a nested universe-scoped path when universeId is known', () => {
    expect(buildNestedPath('uni-1', 'editor', 'ch-1')).toBe('/universe/uni-1/editor/ch-1')
  })

  it('builds a nested path for a different segment/id pair', () => {
    expect(buildNestedPath('uni-2', 'entities', 'ent-9')).toBe('/universe/uni-2/entities/ent-9')
  })

  it('returns null when universeId is not yet known', () => {
    expect(buildNestedPath(undefined, 'editor', 'ch-1')).toBeNull()
  })
})
