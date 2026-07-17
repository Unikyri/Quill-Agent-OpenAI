import { Extension } from '@tiptap/core'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import type { Node } from '@tiptap/pm/model'
import { createEntityMatcher, stableEntityColor, type EntityHighlightEntity } from './entityHighlightExtension'

export interface CandidateHighlightEntity extends EntityHighlightEntity {
  confidence?: number
  evidence_quote?: string
}

function decorationsFor(doc: Node, entities: CandidateHighlightEntity[]): DecorationSet {
  const matcher = createEntityMatcher(entities)
  const decorations: Decoration[] = []
  doc.descendants((node, position) => {
    if (!node.isText || !node.text) return
    matcher.regex.lastIndex = 0
    let match: RegExpExecArray | null
    while ((match = matcher.regex.exec(node.text)) !== null) {
      const name = match[2]
      const candidate = matcher.byName.get(name.toLocaleLowerCase())
      if (!candidate) continue
      const from = position + match.index + match[1].length
      const to = from + name.length
      decorations.push(Decoration.inline(from, to, {
        class: 'candidate-highlight',
        'data-candidate-id': candidate.id,
        'data-candidate-name': candidate.name,
        style: `--candidate-color: ${stableEntityColor(candidate)}`,
      }, { inclusive: false }))
    }
  })
  return DecorationSet.create(doc, decorations)
}

export const candidateHighlightKey = new PluginKey('quill-candidate-highlights')

interface CandidatePluginState {
  candidates: CandidateHighlightEntity[]
  decorations: DecorationSet
}

const CandidateHighlight = Extension.create<{ candidates: CandidateHighlightEntity[] }>({
  name: 'candidateHighlight',

  addOptions() {
    return { candidates: [] }
  },

  addProseMirrorPlugins() {
    const candidates = this.options.candidates
    const plugin: Plugin = new (class extends Plugin {
      constructor() {
        super({
			key: candidateHighlightKey,
			state: {
				init: (_config, state): CandidatePluginState => ({ candidates, decorations: decorationsFor(state.doc, candidates) }),
				apply: (transaction, old) => {
					const previous = old as CandidatePluginState
					const meta = transaction.getMeta(candidateHighlightKey)
					const nextCandidates = Array.isArray(meta) ? meta as CandidateHighlightEntity[] : previous.candidates
					const decorations = transaction.docChanged || Array.isArray(meta)
						? decorationsFor(transaction.doc, nextCandidates)
						: previous.decorations.map(transaction.mapping, transaction.doc)
					return { candidates: nextCandidates, decorations }
				},
          },
          props: {
            decorations: (state) => (this.getState(state) as CandidatePluginState).decorations,
          },
        })
      }
    })()
    return [plugin]
  },
})

export default CandidateHighlight
