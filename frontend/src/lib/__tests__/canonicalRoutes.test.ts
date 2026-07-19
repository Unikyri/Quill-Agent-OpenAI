import { describe, expect, it } from 'vitest'
import { explorePath, memoryPath, profileMemoryPath, reviewPath, writeImportPath, writePath } from '../canonicalRoutes'

describe('canonical Sprint 7 routes', () => {
  it('builds the Write chapter-picker and selected-chapter destinations', () => {
    expect(writePath('uni-1')).toBe('/universe/uni-1/write')
    expect(writePath('uni-1', 'chapter-1')).toBe('/universe/uni-1/write/chapter-1')
    expect(writeImportPath('uni-1')).toBe('/universe/uni-1/write?panel=import')
  })

  it('builds the Explore views (entities was folded into the Story Graph map)', () => {
    expect(explorePath('uni-1', 'map')).toBe('/universe/uni-1/explore/map')
    expect(explorePath('uni-1', 'timeline')).toBe('/universe/uni-1/explore/timeline')
  })

  it('carries a target entity id through to the map as a query param', () => {
    expect(explorePath('uni-1', 'map', 'ent-1')).toBe('/universe/uni-1/explore/map?entity=ent-1')
    // No entityId provided → no query param (plain auto-focus entry point).
    expect(explorePath('uni-1', 'map', undefined)).toBe('/universe/uni-1/explore/map')
  })

  it('builds Memory and Review destinations', () => {
    expect(memoryPath('uni-1')).toBe('/universe/uni-1/memory')
    expect(reviewPath('uni-1', 'issues')).toBe('/universe/uni-1/review/issues')
    expect(reviewPath('uni-1', 'candidates')).toBe('/universe/uni-1/review/candidates')
  })

  it('builds the account-scoped Writer Profile destination', () => {
    expect(profileMemoryPath()).toBe('/profile/memory')
  })
})
