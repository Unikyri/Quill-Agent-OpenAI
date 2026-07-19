import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { NavLink, Navigate, useNavigate, useParams } from 'react-router-dom'
import { useFeedback } from '../components/feedback'
import EmptyState from '../components/shared/EmptyState'
import { api } from '../lib/api'
import { reviewPath, writePath } from '../lib/canonicalRoutes'
import type { EntityCandidateDTO } from '../lib/types'
import styles from './ReviewPage.module.css'

type ReviewAction = 'resolve' | 'dismiss'
type IssueKind = 'contradiction' | 'plot-hole'

interface ContradictionIssue {
  id: string
  entity_id?: string
  severity: string
  description: string
  suggestion?: string
  evidence_a?: string
  evidence_a_chapter_id?: string
  evidence_b?: string
  evidence_b_chapter_id?: string
  status?: string
}

interface PlotHoleIssue {
  id: string
  title: string
  description?: string
  related_entity_ids?: string[]
  first_mentioned_chapter_id?: string
  status?: string
}

interface InboxIssue {
  id: string
  kind: IssueKind
  title: string
  description: string
  severity?: string
  status: string
  suggestion?: string
  evidence: Array<{ quote: string; chapterId?: string }>
  firstMentionedChapterId?: string
  relatedEntityCount: number
}

interface PendingConfirmation {
  id: string
  action: ReviewAction
  kind: IssueKind | 'candidate'
  universeId: string
}

interface ActionFailure {
  message: string
  action: ReviewAction
}

interface ActiveAction {
  id: string
  universeId: string
}

const REVIEW_VIEWS = ['issues', 'candidates'] as const
type ReviewTabView = typeof REVIEW_VIEWS[number]
const REVIEW_VIEW_LABELS: Record<ReviewTabView, string> = { issues: 'Conflicts', candidates: 'New entities' }
const SEVERITY_RANK: Record<string, number> = { high: 0, medium: 1, low: 2 }

function messageFor(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

function contradictionEvidence(item: ContradictionIssue): InboxIssue['evidence'] {
  const evidence: InboxIssue['evidence'] = []
  if (item.evidence_a) evidence.push({ quote: item.evidence_a, chapterId: item.evidence_a_chapter_id })
  if (item.evidence_b) evidence.push({ quote: item.evidence_b, chapterId: item.evidence_b_chapter_id })
  return evidence
}

function sortInboxIssues(items: InboxIssue[]): InboxIssue[] {
  return [...items].sort((left, right) => {
    const leftSettled = left.status === 'open' ? 0 : 1
    const rightSettled = right.status === 'open' ? 0 : 1
    if (leftSettled !== rightSettled) return leftSettled - rightSettled
    const leftPriority = left.kind === 'contradiction' ? SEVERITY_RANK[left.severity || 'low'] ?? 3 : 3
    const rightPriority = right.kind === 'contradiction' ? SEVERITY_RANK[right.severity || 'low'] ?? 3 : 3
    return leftPriority - rightPriority
  })
}

const CLAUSE_BOUNDARIES = ['. ', '; ', ': ', ' — ']
const TITLE_CLAUSE_LIMIT = 90
const TITLE_WORD_CUT_LIMIT = 80

// Derives a short card title from a (potentially long) description: cuts at
// the first clause boundary if that's a reasonable title length, otherwise
// falls back to a word-boundary truncation with an ellipsis. Never returns
// an empty title for a non-empty description.
export function shortTitle(description: string): string {
  const text = description.trim()
  if (!text) return text

  let earliestBoundary = -1
  for (const boundary of CLAUSE_BOUNDARIES) {
    const index = text.indexOf(boundary)
    if (index !== -1 && (earliestBoundary === -1 || index < earliestBoundary)) earliestBoundary = index
  }

  if (earliestBoundary !== -1 && earliestBoundary <= TITLE_CLAUSE_LIMIT) {
    return text.slice(0, earliestBoundary).trim()
  }

  if (text.length <= TITLE_WORD_CUT_LIMIT) return text

  const cutoff = text.slice(0, TITLE_WORD_CUT_LIMIT)
  const lastSpace = cutoff.lastIndexOf(' ')
  const trimmed = (lastSpace > 0 ? cutoff.slice(0, lastSpace) : cutoff).replace(/[.,;:—-]+$/, '').trim()
  return `${trimmed}…`
}

function toInboxIssues(contradictions: ContradictionIssue[], plotHoles: PlotHoleIssue[]): InboxIssue[] {
  const contradictionItems = contradictions.map((item) => ({
    id: item.id,
    kind: 'contradiction' as const,
    title: shortTitle(item.description),
    description: item.description,
    severity: item.severity,
    status: item.status || 'open',
    suggestion: item.suggestion,
    evidence: contradictionEvidence(item),
    relatedEntityCount: item.entity_id ? 1 : 0,
  }))
  const plotHoleItems = plotHoles.map((item) => ({
    id: item.id,
    kind: 'plot-hole' as const,
    title: item.title,
    description: item.description || 'Quill found an unresolved narrative thread.',
    status: item.status || 'open',
    evidence: [],
    firstMentionedChapterId: item.first_mentioned_chapter_id,
    relatedEntityCount: item.related_entity_ids?.length || 0,
  }))

  return sortInboxIssues([...contradictionItems, ...plotHoleItems])
}

export default function ReviewPage() {
  const { universeId, view: rawView } = useParams<{ universeId: string; view: string }>()
  const navigate = useNavigate()
  const { publish } = useFeedback()
  const [issues, setIssues] = useState<InboxIssue[]>([])
  const [issuesUniverseId, setIssuesUniverseId] = useState<string | null>(null)
  const [candidates, setCandidates] = useState<EntityCandidateDTO[]>([])
  const [candidatesUniverseId, setCandidatesUniverseId] = useState<string | null>(null)
  const [issuesLoading, setIssuesLoading] = useState(false)
  const [candidatesLoading, setCandidatesLoading] = useState(false)
  const [issuesError, setIssuesError] = useState<string | null>(null)
  const [issuesErrorUniverseId, setIssuesErrorUniverseId] = useState<string | null>(null)
  const [candidatesError, setCandidatesError] = useState<string | null>(null)
  const [candidatesErrorUniverseId, setCandidatesErrorUniverseId] = useState<string | null>(null)
  const [pendingConfirmation, setPendingConfirmation] = useState<PendingConfirmation | null>(null)
  const [activeAction, setActiveAction] = useState<ActiveAction | null>(null)
  const [actionErrors, setActionErrors] = useState<Record<string, ActionFailure>>({})
  const [actionErrorsUniverseId, setActionErrorsUniverseId] = useState<string | null>(null)
  const issuesRequestId = useRef(0)
  const candidatesRequestId = useRef(0)
  const currentUniverseId = useRef(universeId)
  const universeGeneration = useRef(0)
  if (currentUniverseId.current !== universeId) {
    currentUniverseId.current = universeId
    universeGeneration.current += 1
  }

  const view = REVIEW_VIEWS.includes(rawView as ReviewTabView) ? rawView as ReviewTabView : null

  const loadIssues = useCallback(async () => {
    if (!universeId) return false
    const requestId = ++issuesRequestId.current
    const requestUniverseId = universeId
    const requestGeneration = universeGeneration.current
    const isCurrentRequest = () => (
      issuesRequestId.current === requestId
      && universeGeneration.current === requestGeneration
      && currentUniverseId.current === requestUniverseId
    )
    setIssuesLoading(true)
    setIssuesError(null)
    setIssuesErrorUniverseId(null)
    setIssues([])
    setIssuesUniverseId(requestUniverseId)
    const [contradictionsResult, plotHolesResult] = await Promise.allSettled([
      api.getContradictions(universeId),
      api.getPlotHoles(universeId),
    ])
    if (!isCurrentRequest()) return false

    const contradictions = contradictionsResult.status === 'fulfilled' ? contradictionsResult.value.contradictions || [] : []
    const plotHoles = plotHolesResult.status === 'fulfilled' ? plotHolesResult.value.plot_holes || [] : []
    setIssues(toInboxIssues(contradictions, plotHoles))
    setIssuesUniverseId(requestUniverseId)

    const failures = [contradictionsResult, plotHolesResult].filter((result) => result.status === 'rejected')
    if (failures.length > 0) {
      const detail = failures.map((result) => result.status === 'rejected' ? messageFor(result.reason, 'Unknown request failure') : '').filter(Boolean).join(' ')
      const message = failures.length === 2
        ? `Could not load the review inbox. ${detail}`
        : `Some review data could not load. Showing the findings that are available. ${detail}`
      setIssuesError(message)
      setIssuesErrorUniverseId(requestUniverseId)
      publish({ scope: 'review', status: 'failed', message, retry: loadIssues })
    }
    setIssuesLoading(false)
    return failures.length === 0
  }, [publish, universeId])

  const loadCandidates = useCallback(async () => {
    if (!universeId) return false
    const requestId = ++candidatesRequestId.current
    const requestUniverseId = universeId
    const requestGeneration = universeGeneration.current
    const isCurrentRequest = () => (
      candidatesRequestId.current === requestId
      && universeGeneration.current === requestGeneration
      && currentUniverseId.current === requestUniverseId
    )
    setCandidatesLoading(true)
    setCandidatesError(null)
    setCandidatesErrorUniverseId(null)
    setCandidates([])
    setCandidatesUniverseId(requestUniverseId)
    try {
      const response = await api.listEntityCandidates(universeId)
      if (!isCurrentRequest()) return false
      setCandidates(response.candidates || [])
      setCandidatesUniverseId(requestUniverseId)
      return true
    } catch (error) {
      if (!isCurrentRequest()) return false
      const message = messageFor(error, 'Could not load entity candidates.')
      setCandidatesError(message)
      setCandidatesErrorUniverseId(requestUniverseId)
      publish({ scope: 'review', status: 'failed', message, retry: loadCandidates })
      return false
    } finally {
      if (isCurrentRequest()) setCandidatesLoading(false)
    }
  }, [publish, universeId])

  useEffect(() => {
    setIssues([])
    setIssuesUniverseId(null)
    setCandidates([])
    setCandidatesUniverseId(null)
    setIssuesLoading(false)
    setCandidatesLoading(false)
    setIssuesError(null)
    setIssuesErrorUniverseId(null)
    setCandidatesError(null)
    setCandidatesErrorUniverseId(null)
    setPendingConfirmation(null)
    setActiveAction(null)
    setActionErrors({})
    setActionErrorsUniverseId(null)
  }, [universeId])

  useEffect(() => {
    if (view === 'issues') void loadIssues()
  }, [loadIssues, view])

  useEffect(() => {
    if (view === 'candidates') void loadCandidates()
  }, [loadCandidates, view])

  const actOnIssue = useCallback(async (issue: InboxIssue, action: ReviewAction) => {
    if (!universeId) return false
    const requestUniverseId = universeId
    const requestGeneration = universeGeneration.current
    const isCurrentRequest = () => (
      universeGeneration.current === requestGeneration && currentUniverseId.current === requestUniverseId
    )
    setActiveAction({ id: issue.id, universeId: requestUniverseId })
    setActionErrorsUniverseId(requestUniverseId)
    setActionErrors((current) => {
      const next = { ...current }
      delete next[issue.id]
      return next
    })
    try {
      if (issue.kind === 'contradiction') {
        if (action === 'resolve') await api.resolveContradiction(universeId, issue.id)
        else await api.dismissContradiction(universeId, issue.id)
      } else if (action === 'resolve') {
        await api.resolvePlotHole(universeId, issue.id)
      } else {
        await api.dismissPlotHole(universeId, issue.id)
      }
      if (!isCurrentRequest()) return false
      const status = action === 'resolve' ? 'resolved' : 'dismissed'
      setIssues((current) => sortInboxIssues(current.map((item) => item.id === issue.id && item.kind === issue.kind ? { ...item, status } : item)))
      publish({
        scope: 'review',
        status: 'completed',
        message: action === 'resolve' ? 'Review item marked resolved.' : 'Review item marked intentional.',
      })
      return true
    } catch (error) {
      if (!isCurrentRequest()) return false
      const message = messageFor(error, 'Could not save this review decision.')
      setActionErrors((current) => ({ ...current, [issue.id]: { message, action } }))
      publish({ scope: 'review', status: 'failed', message, retry: () => actOnIssue(issue, action) })
      return false
    } finally {
      if (isCurrentRequest()) {
        setActiveAction(null)
        setPendingConfirmation(null)
      }
    }
  }, [publish, universeId])

  const actOnCandidate = useCallback(async (candidate: EntityCandidateDTO, action: ReviewAction) => {
    if (!universeId) return false
    const requestUniverseId = universeId
    const requestGeneration = universeGeneration.current
    const isCurrentRequest = () => (
      universeGeneration.current === requestGeneration && currentUniverseId.current === requestUniverseId
    )
    setActiveAction({ id: candidate.entity_id, universeId: requestUniverseId })
    setActionErrorsUniverseId(requestUniverseId)
    setActionErrors((current) => {
      const next = { ...current }
      delete next[candidate.entity_id]
      return next
    })
    try {
      if (action === 'resolve') await api.acceptEntityCandidate(candidate.entity_id)
      else await api.dismissEntityCandidate(candidate.entity_id)
      if (!isCurrentRequest()) return false
      const status = action === 'resolve' ? 'accepted' : 'dismissed'
      setCandidates((current) => current.map((item) => item.entity_id === candidate.entity_id ? { ...item, status } : item))
      publish({
        scope: 'review',
        status: 'completed',
        message: action === 'resolve' ? 'Entity candidate accepted.' : 'Entity candidate dismissed.',
      })
      return true
    } catch (error) {
      if (!isCurrentRequest()) return false
      const message = messageFor(error, 'Could not save this candidate decision.')
      setActionErrors((current) => ({ ...current, [candidate.entity_id]: { message, action } }))
      publish({ scope: 'review', status: 'failed', message, retry: () => actOnCandidate(candidate, action) })
      return false
    } finally {
      if (isCurrentRequest()) {
        setActiveAction(null)
        setPendingConfirmation(null)
      }
    }
  }, [publish, universeId])

  const currentIssues = issuesUniverseId === universeId ? issues : []
  const currentCandidates = candidatesUniverseId === universeId ? candidates : []
  const currentIssuesLoading = issuesUniverseId === universeId ? issuesLoading : view === 'issues'
  const currentCandidatesLoading = candidatesUniverseId === universeId ? candidatesLoading : view === 'candidates'
  const currentIssuesError = issuesErrorUniverseId === universeId ? issuesError : null
  const currentCandidatesError = candidatesErrorUniverseId === universeId ? candidatesError : null
  const currentPendingConfirmation = pendingConfirmation?.universeId === universeId ? pendingConfirmation : null
  const currentActiveAction = activeAction && activeAction.universeId === universeId ? activeAction.id : null
  const currentActionErrors = actionErrorsUniverseId === universeId ? actionErrors : {}
  const sortedCandidates = useMemo(() => [...currentCandidates].sort((left, right) => right.confidence - left.confidence), [currentCandidates])

  if (!universeId) return null
  if (!view) return <Navigate to={reviewPath(universeId, 'issues')} replace />

  return (
    <section className={styles.wrap} aria-labelledby="review-title">
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Author decisions</p>
          <h1 id="review-title">Review</h1>
          <p className={styles.subtitle}>A prioritized inbox built from the analysis Quill can actually cite.</p>
        </div>
      </header>

      <nav className={styles.tabs} aria-label="Review views">
        {REVIEW_VIEWS.map((item) => (
          <NavLink
            key={item}
            className={({ isActive }) => `${styles.tab} ${isActive ? styles.tabActive : ''}`}
            to={reviewPath(universeId, item)}
          >
            {REVIEW_VIEW_LABELS[item]}
          </NavLink>
        ))}
      </nav>

      {view === 'issues' && (
        <IssuesInbox
          issues={currentIssues}
          loading={currentIssuesLoading}
          error={currentIssuesError}
          onRetry={loadIssues}
          onAct={actOnIssue}
          pendingConfirmation={currentPendingConfirmation}
          onConfirm={(issue, action) => setPendingConfirmation({ id: issue.id, action, kind: issue.kind, universeId })}
          onCancel={() => setPendingConfirmation(null)}
          activeAction={currentActiveAction}
          actionErrors={currentActionErrors}
          onOpenChapter={(chapterId) => navigate(writePath(universeId, chapterId))}
        />
      )}

      {view === 'candidates' && (
        <CandidatesInbox
          candidates={sortedCandidates}
          loading={currentCandidatesLoading}
          error={currentCandidatesError}
          onRetry={loadCandidates}
          onAct={actOnCandidate}
          pendingConfirmation={currentPendingConfirmation}
          onConfirm={(candidate, action) => setPendingConfirmation({ id: candidate.entity_id, action, kind: 'candidate', universeId })}
          onCancel={() => setPendingConfirmation(null)}
          activeAction={currentActiveAction}
          actionErrors={currentActionErrors}
          onOpenChapter={(chapterId) => navigate(writePath(universeId, chapterId))}
        />
      )}
    </section>
  )
}

interface IssuesInboxProps {
  issues: InboxIssue[]
  loading: boolean
  error: string | null
  onRetry: () => Promise<boolean>
  onAct: (issue: InboxIssue, action: ReviewAction) => Promise<boolean>
  pendingConfirmation: PendingConfirmation | null
  onConfirm: (issue: InboxIssue, action: ReviewAction) => void
  onCancel: () => void
  activeAction: string | null
  actionErrors: Record<string, ActionFailure>
  onOpenChapter: (chapterId: string) => void
}

function IssuesInbox({ issues, loading, error, onRetry, onAct, pendingConfirmation, onConfirm, onCancel, activeAction, actionErrors, onOpenChapter }: IssuesInboxProps) {
  if (loading) return <LoadingState label="Loading live review findings…" />
  if (error && issues.length === 0) return <ErrorState message={error} onRetry={onRetry} />
  if (issues.length === 0) {
    return <EmptyState title="No issues need a decision" detail="No contradiction or open plot thread is available from the analysis completed so far. This is not a guarantee that the manuscript has no issues; analyze more passages or import more chapters to extend coverage." />
  }

  return (
    <div className={styles.inbox}>
      {error && <DegradedState message={error} onRetry={onRetry} />}
      <p className={styles.inboxIntro}>Open findings appear first. Contradictions are ordered by the model’s severity; plot holes follow because the current API does not assign them a severity.</p>
      {issues.map((issue) => {
        const settled = issue.status !== 'open'
        const confirmation = pendingConfirmation?.id === issue.id ? pendingConfirmation : null
        return (
          <article key={`${issue.kind}-${issue.id}`} className={`${styles.card} ${settled ? styles.cardSettled : ''}`} data-testid={`review-${issue.kind}-${issue.id}`}>
            <div className={styles.cardMeta}>
              <span className={styles.kind}>{issue.kind === 'contradiction' ? 'Contradiction' : 'Plot hole'}</span>
              {issue.severity && <span className={`${styles.severity} ${styles[`severity${issue.severity.charAt(0).toUpperCase()}${issue.severity.slice(1)}` as keyof typeof styles]}`}>{issue.severity}</span>}
              {settled && <span className={styles.settled}>{issue.status === 'resolved' ? 'Resolved' : 'Marked intentional'}</span>}
            </div>
            <h2>{issue.title}</h2>
            {issue.description !== issue.title && <p className={styles.description}>{issue.description}</p>}

            <div className={styles.detailGrid}>
              <section>
                <h3>Evidence</h3>
                {issue.evidence.length > 0 ? (
                  <ul className={styles.evidenceList}>
                    {issue.evidence.map((evidence, index) => <li key={`${issue.id}-${index}`}>&ldquo;{evidence.quote}&rdquo;{evidence.chapterId ? ` · chapter ${evidence.chapterId.slice(0, 8)}` : ''}</li>)}
                  </ul>
                ) : issue.firstMentionedChapterId ? (
                  <p>First identified in chapter {issue.firstMentionedChapterId.slice(0, 8)}. The API did not supply a source excerpt.</p>
                ) : (
                  <p>No source excerpt was supplied by the current API.</p>
                )}
              </section>
              <section>
                <h3>Impact</h3>
                <p>{issue.kind === 'contradiction'
                  ? `Model severity: ${issue.severity || 'unspecified'}. Review this conflict before relying on the related lore.`
                  : 'Open narrative thread. The plot-hole API currently provides no severity score.'}</p>
                {issue.relatedEntityCount > 0 && <p>{issue.relatedEntityCount} related {issue.relatedEntityCount === 1 ? 'entity' : 'entities'} reported.</p>}
              </section>
            </div>

            {issue.suggestion && <p className={styles.suggestion}><strong>Suggested next step:</strong> {issue.suggestion}</p>}
            {issue.firstMentionedChapterId && <button className={styles.textButton} type="button" onClick={() => onOpenChapter(issue.firstMentionedChapterId!)}>Open source chapter</button>}

            {!settled && !confirmation && (
              <div className={styles.actions}>
                <button type="button" className={styles.primaryButton} onClick={() => onConfirm(issue, 'resolve')}>Resolve</button>
                <button type="button" className={styles.secondaryButton} onClick={() => onConfirm(issue, 'dismiss')}>Mark intentional</button>
              </div>
            )}
            {confirmation && (
              <InlineConfirmation
                label={confirmation.action === 'resolve' ? 'Mark this finding resolved?' : 'Mark this finding as intentional?'}
                confirming={activeAction === issue.id}
                onConfirm={() => void onAct(issue, confirmation.action)}
                onCancel={onCancel}
              />
            )}
            {actionErrors[issue.id] && <ActionError message={actionErrors[issue.id].message} onRetry={() => void onAct(issue, actionErrors[issue.id].action)} />}
          </article>
        )
      })}
    </div>
  )
}

interface CandidatesInboxProps {
  candidates: EntityCandidateDTO[]
  loading: boolean
  error: string | null
  onRetry: () => Promise<boolean>
  onAct: (candidate: EntityCandidateDTO, action: ReviewAction) => Promise<boolean>
  pendingConfirmation: PendingConfirmation | null
  onConfirm: (candidate: EntityCandidateDTO, action: ReviewAction) => void
  onCancel: () => void
  activeAction: string | null
  actionErrors: Record<string, ActionFailure>
  onOpenChapter: (chapterId: string) => void
}

function CandidatesInbox({ candidates, loading, error, onRetry, onAct, pendingConfirmation, onConfirm, onCancel, activeAction, actionErrors, onOpenChapter }: CandidatesInboxProps) {
  if (loading) return <LoadingState label="Loading current entity candidates…" />
  if (error) return <ErrorState message={error} onRetry={onRetry} />
  if (candidates.length === 0) {
    return <EmptyState title="No entity candidates waiting" detail="Candidates are unconfirmed names or places extracted from analyzed passages. They appear here only while the current API reports them, and nothing becomes story lore until you accept it." />
  }

  return (
    <div className={styles.inbox}>
      <p className={styles.inboxIntro}>These are live extraction candidates, not confirmed story facts. A decision is saved only after the API confirms it.</p>
      {candidates.map((candidate) => {
        const confirmation = pendingConfirmation?.id === candidate.entity_id ? pendingConfirmation : null
        const isPending = candidate.status === 'candidate'
        const confidence = Math.round(Math.max(0, Math.min(1, candidate.confidence)) * 100)
        return (
          <article key={candidate.entity_id} className={`${styles.card} ${!isPending ? styles.cardSettled : ''}`} data-testid={`candidate-${candidate.entity_id}`}>
            <div className={styles.cardMeta}>
              <span className={styles.kind}>Candidate</span>
              <span className={styles.confidence}>{confidence}% confidence</span>
              {!isPending && <span className={styles.settled}>{candidate.status}</span>}
            </div>
            <h2>{candidate.name}</h2>
            <p className={styles.description}>{candidate.description || `${candidate.type} candidate`}</p>
            <div className={styles.detailGrid}>
              <section>
                <h3>Evidence</h3>
                <p>{candidate.evidence_quote ? `“${candidate.evidence_quote}”` : 'No source excerpt was supplied by the current API.'}</p>
              </section>
              <section>
                <h3>Impact</h3>
                <p>This extraction is not part of the story until you accept it.</p>
              </section>
            </div>
            {candidate.aliases && candidate.aliases.length > 0 && <p className={styles.aliases}>Aliases: {candidate.aliases.join(', ')}</p>}
            {candidate.chapter_id && <button className={styles.textButton} type="button" onClick={() => onOpenChapter(candidate.chapter_id!)}>Open source chapter</button>}
            {isPending && !confirmation && (
              <div className={styles.actions}>
                <button type="button" className={styles.primaryButton} onClick={() => onConfirm(candidate, 'resolve')}>Accept candidate</button>
                <button type="button" className={styles.secondaryButton} onClick={() => onConfirm(candidate, 'dismiss')}>Dismiss</button>
              </div>
            )}
            {confirmation && (
              <InlineConfirmation
                label={confirmation.action === 'resolve' ? `Accept ${candidate.name} as a story entity?` : `Dismiss ${candidate.name} as a candidate?`}
                confirming={activeAction === candidate.entity_id}
                onConfirm={() => void onAct(candidate, confirmation.action)}
                onCancel={onCancel}
              />
            )}
            {actionErrors[candidate.entity_id] && <ActionError message={actionErrors[candidate.entity_id].message} onRetry={() => void onAct(candidate, actionErrors[candidate.entity_id].action)} />}
          </article>
        )
      })}
    </div>
  )
}

function InlineConfirmation({ label, confirming, onConfirm, onCancel }: { label: string; confirming: boolean; onConfirm: () => void; onCancel: () => void }) {
  return (
    <div className={styles.confirmation} role="status" aria-live="polite">
      <span>{label}</span>
      <button type="button" className={styles.primaryButton} disabled={confirming} onClick={onConfirm}>{confirming ? 'Saving…' : 'Confirm'}</button>
      <button type="button" className={styles.secondaryButton} disabled={confirming} onClick={onCancel}>Cancel</button>
    </div>
  )
}

function ActionError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <p className={styles.actionError} role="alert">Could not save this decision: {message} <button type="button" onClick={onRetry}>Retry</button></p>
}

function LoadingState({ label }: { label: string }) {
  return <div className={styles.state} role="status" aria-live="polite">{label}</div>
}

function ErrorState({ message, onRetry }: { message: string; onRetry: () => Promise<boolean> }) {
  return <div className={styles.state} role="alert"><p>{message}</p><button type="button" className={styles.primaryButton} onClick={() => void onRetry()}>Retry</button></div>
}

function DegradedState({ message, onRetry }: { message: string; onRetry: () => Promise<boolean> }) {
  return <div className={styles.degraded} role="status"><span>{message}</span><button type="button" onClick={() => void onRetry()}>Retry</button></div>
}

