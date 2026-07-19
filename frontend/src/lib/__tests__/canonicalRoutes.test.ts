import { describe, expect, it } from 'vitest'
import { explorePath, memoryPath, reviewPath, writeImportPath, writePath } from '../canonicalRoutes'

describe('canonical Sprint 7 routes', () => {
  it('builds the Write chapter-picker and selected-chapter destinations', () => {
    expect(writePath('uni-1')).toBe('/universe/uni-1/write')
    expect(writePath('uni-1', 'chapter-1')).toBe('/universe/uni-1/write/chapter-1')
    expect(writeImportPath('uni-1')).toBe('/universe/uni-1/write?panel=import')
  })

  it('builds the three Explore views and preserves entity deep links', () => {
    expect(explorePath('uni-1', 'entities')).toBe('/universe/uni-1/explore/entities')
    expect(explorePath('uni-1', 'entities', 'entity-1')).toBe('/universe/uni-1/explore/entities/entity-1')
    expect(explorePath('uni-1', 'map')).toBe('/universe/uni-1/explore/map')
    expect(explorePath('uni-1', 'timeline')).toBe('/universe/uni-1/explore/timeline')
  })

  it('builds Memory and Review destinations', () => {
    expect(memoryPath('uni-1')).toBe('/universe/uni-1/memory')
    expect(reviewPath('uni-1', 'issues')).toBe('/universe/uni-1/review/issues')
    expect(reviewPath('uni-1', 'candidates')).toBe('/universe/uni-1/review/candidates')
  })
})
