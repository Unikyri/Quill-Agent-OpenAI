import { describe, expect, it } from 'vitest'
import { displaySkillName, shortDescription } from '../skillDisplay'

describe('displaySkillName', () => {
  it('title-cases a kebab-case skill slug', () => {
    expect(displaySkillName('dialogue-and-voice')).toBe('Dialogue And Voice')
    expect(displaySkillName('beta-reader')).toBe('Beta Reader')
  })

  it('applies small-word fixups for known acronyms and contractions', () => {
    expect(displaySkillName('pov-and-tense')).toBe('POV And Tense')
    expect(displaySkillName('show-dont-tell')).toBe("Show Don't Tell")
  })
})

describe('shortDescription', () => {
  it('returns the first sentence when it fits within the max length', () => {
    expect(shortDescription('Tightens sentences without changing voice. More detail follows here.'))
      .toBe('Tightens sentences without changing voice.')
  })

  it('hard-truncates a long run-on first sentence with an ellipsis', () => {
    const long = 'a'.repeat(200)
    const result = shortDescription(long, 20)
    expect(result).toBe(`${'a'.repeat(20)}…`)
  })

  it('collapses internal whitespace before measuring', () => {
    expect(shortDescription('Line one.\n  Line two continues.')).toBe('Line one.')
  })
})
