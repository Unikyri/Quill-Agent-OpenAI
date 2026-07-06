// ponytail: inline map replaces a ConfigLoader/theme system for 6 fixed types.
// Keep in sync with backend entity types (qwen_service.go prompt, entity_service.go
// CreateNode label) and the --node-* CSS tokens in index.css.
// Icons are monochrome Unicode glyphs (not emoji) per ADR-2 — render inside a
// `.glyph` span so the symbol-font fallback stack in index.css applies.
export const NODE_TYPE_META: Record<string, { color: string; icon: string; label: string }> = {
  character: { color: 'var(--node-character)', icon: '●', label: 'Character' },
  place: { color: 'var(--node-place)', icon: '◆', label: 'Place' },
  event: { color: 'var(--node-event)', icon: '▲', label: 'Event' },
  faction: { color: 'var(--node-faction)', icon: '■', label: 'Faction' },
  world_rule: { color: 'var(--node-worldrule)', icon: '◈', label: 'World Rule' },
  plot_arc: { color: 'var(--node-plotarc)', icon: '◉', label: 'Plot Arc' },
}
