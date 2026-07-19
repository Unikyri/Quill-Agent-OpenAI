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

export function explorePath(universeId: string, view: ExploreView): string {
  return `${universeBase(universeId)}/explore/${view}`
}

export function memoryPath(universeId: string): string {
  return `${universeBase(universeId)}/memory`
}

export function reviewPath(universeId: string, view: ReviewView): string {
  return `${universeBase(universeId)}/review/${view}`
}
