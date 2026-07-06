// Pure path builder for legacy top-level deep links (ADR-3, RISK-4).
// `EditorRedirect`/`EntityRedirect` fetch the record to learn its
// universe_id, then Navigate here — this is the only piece of that
// migration with actual branching logic, so it's the one worth unit-testing.
export function buildNestedPath(
  universeId: string | undefined | null,
  segment: string,
  id: string
): string | null {
  if (!universeId) return null
  return `/universe/${universeId}/${segment}/${id}`
}
