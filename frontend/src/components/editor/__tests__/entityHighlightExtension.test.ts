import { describe, expect, it } from 'vitest'
import { createEntityMatcher, stableEntityColor, type EntityHighlightEntity } from '../entityHighlightExtension'

describe('entity highlight matcher', () => {
  it('prefers the longest known name and resolves aliases to the canonical entity', () => {
    const entities: EntityHighlightEntity[] = [
      { id: 'e-1', name: 'Ann', type: 'character', aliases: ['The Captain'] },
      { id: 'e-2', name: 'Annex', type: 'place' },
    ]
    const matcher = createEntityMatcher(entities)
    const text = 'Annex was mapped. The Captain met Ann.'
    const matches: string[] = []
    let match: RegExpExecArray | null
    while ((match = matcher.regex.exec(text)) !== null) matches.push(match[2])
    expect(matches).toEqual(['Annex', 'The Captain', 'Ann'])
    expect(matcher.byName.get('the captain')?.id).toBe('e-1')
  })

  it('builds a bounded matcher for a 500-entity universe', () => {
    const entities = Array.from({ length: 500 }, (_, index) => ({ id: `e-${index}`, name: `Entity ${index}`, type: 'character' }))
    const started = performance.now()
    const matcher = createEntityMatcher(entities)
    const elapsed = performance.now() - started
    matcher.regex.lastIndex = 0
    expect(matcher.regex.test('Entity 499 appears')).toBe(true)
    expect(elapsed).toBeLessThan(100)
    matcher.regex.lastIndex = 0
    expect(matcher.regex.test('Entity 4999 is not known')).toBe(false)
  })

  it('keeps colour stable for an entity while respecting its type family', () => {
    expect(stableEntityColor({ id: 'e-1', type: 'character' })).toBe(stableEntityColor({ id: 'e-1', type: 'character' }))
    expect(stableEntityColor({ id: 'e-1', type: 'character' })).not.toBe(stableEntityColor({ id: 'e-1', type: 'place' }))
  })
})
