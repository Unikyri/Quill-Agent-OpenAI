import { describe, expect, it } from 'vitest'
import { shortTitle } from '../ReviewPage'

describe('shortTitle', () => {
  it('returns a short description unchanged', () => {
    expect(shortTitle('A minor date conflict')).toBe('A minor date conflict')
  })

  it('cuts at the first clause boundary when the clause is a reasonable title length', () => {
    expect(shortTitle('The oath was broken. This contradicts the vow made in chapter one.'))
      .toBe('The oath was broken')
  })

  it('cuts at a semicolon boundary', () => {
    expect(shortTitle('Elara is in the capital; the letter places her in the forest that same week.'))
      .toBe('Elara is in the capital')
  })

  it('cuts at an em-dash boundary', () => {
    expect(shortTitle('The map was burned — yet chapter nine shows Kessa reading it by firelight.'))
      .toBe('The map was burned')
  })

  it('falls back to a word-boundary truncation with ellipsis for a long single clause', () => {
    const description = 'The timeline established in the first act directly conflicts with the account given by the second narrator regarding the fall of the northern keep'
    const title = shortTitle(description)
    expect(title.endsWith('…')).toBe(true)
    expect(title.length).toBeLessThan(description.length)
    expect(title).not.toContain('  ')
  })

  it('never returns an empty title for a non-empty description', () => {
    expect(shortTitle('Gate.')).not.toBe('')
    expect(shortTitle('')).toBe('')
  })
})
