export type ExploreView = 'map' | 'timeline'
export type ReviewView = 'issues' | 'candidates'

function universeBase(universeId: string): string {
  return `/universe/${encodeURIComponent(universeId)}`
}

export function writePath(universeId: string, chapterId?: string): string {
  const base = `${universeBase(universeId)}/write`
  return chapterId ? `${base}/${encodeURIComponent(chapterId)}` : base
}

export function writeImportPath(universeId: string): string {
  return `${writePath(universeId)}?panel=import`
}

export function explorePath(universeId: string, view: ExploreView, entityId?: string): string {
  const base = `${universeBase(universeId)}/explore/${view}`
  return entityId ? `${base}?entity=${encodeURIComponent(entityId)}` : base
}

export function memoryPath(universeId: string): string {
  return `${universeBase(universeId)}/memory`
}

export function reviewPath(universeId: string, view: ReviewView): string {
  return `${universeBase(universeId)}/review/${view}`
}

// Account-scoped (not universe-nested) — see ProfileLayout.
export function profileMemoryPath(): string {
  return '/profile/memory'
}
