import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import TimelineSlider from '../TimelineSlider'
import { useGraphStore } from '../../../stores/graphStore'

vi.mock('../TimelineSlider.module.css', () => ({ default: new Proxy({}, { get: (_, k) => k }) }))

const mockGetTimeline = vi.fn()
vi.mock('../../../lib/api', () => ({
  api: {
    getTimeline: (...args: unknown[]) => mockGetTimeline(...args),
  },
}))

beforeEach(() => {
  vi.clearAllMocks()
  useGraphStore.setState({ nodes: [], eventHighlightIds: null })
})

describe('TimelineSlider', () => {
  it('renders nothing when the universe has no timeline events', async () => {
    mockGetTimeline.mockResolvedValue({ events: [] })
    const { container } = render(<TimelineSlider universeId="uni-1" />)

    await waitFor(() => expect(mockGetTimeline).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('sorts events by timeline_position and shows the first event by default, highlighting its participants', async () => {
    mockGetTimeline.mockResolvedValue({
      events: [
        { id: 'e2', title: 'The Battle', timeline_position: 2, participants: [] },
        { id: 'e1', title: 'The Prologue', timeline_position: 1, participants: ['entity-1'] },
      ],
    })
    render(<TimelineSlider universeId="uni-1" />)

    await waitFor(() => {
      expect(screen.getByText('The Prologue')).toBeInTheDocument()
    })
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(useGraphStore.getState().eventHighlightIds).toEqual(['entity-1'])
  })

  it('steps to the next tick and resolves a known participant to its display name', async () => {
    useGraphStore.setState({ nodes: [{ id: 'entity-1', type: 'character', data: { label: 'Kael Drystan' } }] })
    mockGetTimeline.mockResolvedValue({
      events: [
        { id: 'e1', title: 'The Prologue', timeline_position: 1, participants: [] },
        { id: 'e2', title: 'The Battle', timeline_position: 2, participants: ['entity-1'] },
      ],
    })
    const focusNodeSpy = vi.spyOn(useGraphStore.getState(), 'focusNode').mockResolvedValue(undefined)
    render(<TimelineSlider universeId="uni-1" />)

    await waitFor(() => expect(screen.getByText('The Prologue')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Next event' }))

    expect(screen.getByText('The Battle')).toBeInTheDocument()
    expect(useGraphStore.getState().eventHighlightIds).toEqual(['entity-1'])
    const chip = screen.getByRole('button', { name: 'Kael Drystan' })
    await user.click(chip)
    expect(focusNodeSpy).toHaveBeenCalledWith('entity-1')
  })

  it('jumps directly to a tick when clicked, without needing to step through every event', async () => {
    mockGetTimeline.mockResolvedValue({
      events: [
        { id: 'e1', title: 'The Prologue', timeline_position: 1, participants: [] },
        { id: 'e2', title: 'The Battle', timeline_position: 2, participants: [] },
        { id: 'e3', title: 'The Epilogue', timeline_position: 3, participants: [] },
      ],
    })
    render(<TimelineSlider universeId="uni-1" />)

    await waitFor(() => expect(screen.getByText('The Prologue')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'The Epilogue' }))

    expect(screen.getByText('The Epilogue')).toBeInTheDocument()
    expect(screen.getByText('3 / 3')).toBeInTheDocument()
  })

  it('falls back to a shortened ID for a participant outside the loaded graph', async () => {
    mockGetTimeline.mockResolvedValue({
      events: [{ id: 'e1', title: 'The Battle', timeline_position: 1, participants: ['unknown-entity-id'] }],
    })
    render(<TimelineSlider universeId="uni-1" />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /unknown-…/ })).toBeInTheDocument()
    })
  })

  it('shows an error message when the timeline fails to load', async () => {
    mockGetTimeline.mockRejectedValue(new Error('Timeline down'))
    render(<TimelineSlider universeId="uni-1" />)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Timeline down')
    })
  })
})
