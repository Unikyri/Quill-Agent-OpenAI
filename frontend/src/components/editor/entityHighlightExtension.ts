import { Extension } from '@tiptap/core'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import type { Node } from '@tiptap/pm/model'

export interface EntityHighlightEntity {
  id: string
  name: string
  type?: string
  aliases?: string[]
}

interface EntityMatcher {
  regex: RegExp
  byName: Map<string, EntityHighlightEntity>
}

const TYPE_PALETTES: Record<string, string[]> = {
  character: ['var(--node-character)', 'var(--teal)'],
  place: ['var(--node-place)', 'var(--node-worldrule)'],
  object: ['var(--gold)', 'var(--gold-ink)'],
  faction: ['var(--node-faction)', 'var(--muted-2)'],
  event: ['var(--node-event)', 'var(--gold-ink)'],
  world_rule: ['var(--node-worldrule)', 'var(--teal)'],
  plot_arc: ['var(--node-plotarc)', 'var(--node-event)'],
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function normalize(value: string): string {
  return value.trim().toLocaleLowerCase()
}

/** Build one case-insensitive matcher for an entity set. */
export function createEntityMatcher(entities: EntityHighlightEntity[]): EntityMatcher {
  const byName = new Map<string, EntityHighlightEntity>()
  for (const entity of entities) {
    if (!entity.id || !entity.name.trim()) continue
    const names = [entity.name, ...(entity.aliases || [])]
    for (const name of names) {
      const key = normalize(name)
      if (key && !byName.has(key)) byName.set(key, entity)
    }
  }
  const names = [...byName.keys()].sort((left, right) => right.length - left.length)
  // Keep the boundary in the matcher so "Ann" never decorates "Annex" while
  // still allowing names that contain punctuation or spaces.
  const source = names.length === 0
    ? '(?!)'
    : `(^|[^\\p{L}\\p{N}_])(${names.map(escapeRegExp).join('|')})(?![\\p{L}\\p{N}_])`
  return { regex: new RegExp(source, 'giu'), byName }
}

export function stableEntityColor(entity: Pick<EntityHighlightEntity, 'id' | 'type'>): string {
  const palette = TYPE_PALETTES[entity.type || ''] || TYPE_PALETTES.character
  let hash = 0
  for (let index = 0; index < entity.id.length; index += 1) {
    hash = ((hash << 5) - hash + entity.id.charCodeAt(index)) | 0
  }
  return palette[Math.abs(hash) % palette.length]
}

function decorationsFor(doc: Node, matcher: EntityMatcher): DecorationSet {
  const decorations: Decoration[] = []
  doc.descendants((node, position) => {
    if (!node.isText || !node.text) return
    matcher.regex.lastIndex = 0
    let match: RegExpExecArray | null
    while ((match = matcher.regex.exec(node.text)) !== null) {
      const matchedName = match[2]
      const entity = matcher.byName.get(normalize(matchedName))
      if (!entity) continue
      const from = position + match.index + match[1].length
      const to = from + matchedName.length
      decorations.push(Decoration.inline(from, to, {
        class: 'entity-highlight',
        'data-entity-id': entity.id,
        'data-entity-name': entity.name,
        style: `--entity-color: ${stableEntityColor(entity)}`,
      }, { inclusive: false }))
      // Avoid a zero-width loop if a future matcher is changed to allow it.
      if (match[0].length === 0) matcher.regex.lastIndex += 1
    }
  })
  return DecorationSet.create(doc, decorations)
}

const entityHighlightKey = new PluginKey('quill-entity-highlights')

export const EntityHighlight = Extension.create<{ entities: EntityHighlightEntity[] }>({
  name: 'entityHighlight',

  addOptions() {
    return { entities: [] }
  },

  addProseMirrorPlugins() {
    const matcher = createEntityMatcher(this.options.entities)
    const plugin: Plugin = new (class extends Plugin {
      constructor() {
        super({
          key: entityHighlightKey,
          state: {
            init: (_config, state) => decorationsFor(state.doc, matcher),
            apply: (transaction, old) => transaction.docChanged ? decorationsFor(transaction.doc, matcher) : old.map(transaction.mapping, transaction.doc),
          },
          props: {
            decorations: (state) => this.getState(state) as DecorationSet,
          },
        })
      }
    })()
    return [plugin]
  },
})

export default EntityHighlight
