// Skill catalogue entries store an LLM-facing slug (e.g. "dialogue-and-voice")
// and a long "when to use this" description meant as prompt context, not UI
// copy. These helpers derive a human-readable title and a short summary for
// display, without touching the underlying skill content the LLM reads.
const SMALL_WORD_FIXUPS: Record<string, string> = {
  pov: 'POV',
  dont: "Don't",
}

export function displaySkillName(name: string): string {
  return name
    .split(/[-_]+/)
    .filter(Boolean)
    .map((word) => SMALL_WORD_FIXUPS[word.toLowerCase()] || word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

const SHORT_DESCRIPTION_MAX_LENGTH = 110

export function shortDescription(description: string, maxLength = SHORT_DESCRIPTION_MAX_LENGTH): string {
  const normalized = description.replace(/\s+/g, ' ').trim()
  const firstSentence = normalized.match(/^.*?[.!?](?=\s|$)/)?.[0] || normalized

  if (firstSentence.length <= maxLength) return firstSentence
  return `${normalized.slice(0, maxLength).trimEnd()}…`
}
