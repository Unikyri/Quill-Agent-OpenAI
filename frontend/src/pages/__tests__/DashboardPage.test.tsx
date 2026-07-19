import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import DashboardPage from '../DashboardPage'

vi.mock('../DashboardPage.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))

const mockListUniverses = vi.fn()
const mockCreateUniverse = vi.fn()
const mockUpdateUniverse = vi.fn()
const mockDeleteUniverse = vi.fn()
vi.mock('../../lib/api', () => ({
  api: {
    listUniverses: (...args: unknown[]) => mockListUniverses(...args),
    createUniverse: (...args: unknown[]) => mockCreateUniverse(...args),
    updateUniverse: (...args: unknown[]) => mockUpdateUniverse(...args),
    deleteUniverse: (...args: unknown[]) => mockDeleteUniverse(...args),
  },
}))

const mockPublish = vi.fn(() => 'feedback-id')
const mockUpdate = vi.fn()
vi.mock('../../components/feedback', () => ({
  useFeedback: () => ({ publish: mockPublish, update: mockUpdate }),
}))

vi.mock('../../components/genres', () => ({
  GenreTagPicker: ({ value, onChange, label, disabled }: {
    value: string[]
    onChange: (nextValue: string[]) => void
    label?: string
    disabled?: boolean
  }) => (
    <div>
      <span>{label}</span>
      <span>{value.length === 0 ? 'No genres selected' : value.join(', ')}</span>
      <button type="button" disabled={disabled} onClick={() => onChange([...value, 'mystery'])}>Add mystery</button>
    </div>
  ),
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

function renderPage(route = '/dashboard') {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <DashboardPage />
    </MemoryRouter>
  )
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    mockListUniverses.mockResolvedValue({ universes: [] })
    mockCreateUniverse.mockResolvedValue({ universe: { id: 'uni-new', name: 'New World', genre_tags: [] } })
    mockUpdateUniverse.mockResolvedValue({ universe: { id: 'uni-1', name: 'World One', description: 'Updated brief', genre_tags: ['mystery'] } })
    mockDeleteUniverse.mockResolvedValue(undefined)
  })

  it('links to the account-scoped Writer Profile', async () => {
    renderPage()
    expect(await screen.findByRole('link', { name: /writer profile/i })).toHaveAttribute('href', '/profile/memory')
  })

  it('shows real universe details and sends the primary CTA to Write', async () => {
    mockListUniverses.mockResolvedValue({
      universes: [
        { id: 'uni-1', name: 'World One', description: 'A world built around a slow-burning mystery.', genre_tags: ['mystery', 'historical'] },
        { id: 'uni-2', name: 'World Two', genre_tags: [] },
      ],
    })
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByRole('heading', { name: 'World One', level: 3 })).toBeInTheDocument()
    expect(screen.getByText('A world built around a slow-burning mystery.')).toBeInTheDocument()
    expect(screen.getByText('Mystery')).toBeInTheDocument()
    expect(screen.getByText('Historical')).toBeInTheDocument()
    expect(screen.getByText('No genres tagged')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /continue writing/i }))
    expect(mockNavigate).toHaveBeenCalledWith('/universe/uni-1/write')
  })

  it('shows a loading skeleton while the library request is pending', () => {
    mockListUniverses.mockImplementation(() => new Promise(() => undefined))
    renderPage()

    expect(screen.getByLabelText('Loading your universe library')).toBeInTheDocument()
  })

  it('shows a retryable error when the library cannot be loaded', async () => {
    mockListUniverses.mockRejectedValueOnce(new Error('Library unavailable'))
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText('We could not load your library')).toBeInTheDocument()
    expect(screen.getByText('Library unavailable')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(mockListUniverses).toHaveBeenCalledTimes(2))
  })

  it('creates an untagged universe without applying a default genre', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Create your first universe' }))
    expect(screen.getByText('No genres selected')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Name'), 'New World')
    await user.click(screen.getByRole('button', { name: 'Create universe' }))

    await waitFor(() => {
      expect(mockCreateUniverse).toHaveBeenCalledWith({
        name: 'New World',
        description: '',
        genre_tags: [],
      })
    })
    expect(await screen.findByText('New World is ready. Continue writing when you are.')).toBeInTheDocument()
    expect(mockUpdate).toHaveBeenCalledWith('feedback-id', expect.objectContaining({ status: 'completed' }))
  })

  it('opens the creation panel from the explicit new-universe link', async () => {
    renderPage('/dashboard?new=true')

    expect(await screen.findByRole('heading', { name: 'Start with the shape of your story' })).toBeInTheDocument()
  })

  it('edits a universe, updates the library card, and closes the settings dialog', async () => {
    mockListUniverses.mockResolvedValue({
      universes: [{ id: 'uni-1', name: 'World One', description: 'Original brief', genre_tags: ['historical'] }],
    })
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Edit World One' }))
    expect(screen.getByRole('dialog', { name: 'Edit World One' })).toBeInTheDocument()
    await user.clear(screen.getByLabelText('Name'))
    await user.type(screen.getByLabelText('Name'), 'Revised World')
    await user.clear(screen.getByLabelText(/Story brief/i))
    await user.type(screen.getByLabelText(/Story brief/i), 'A revised mystery.')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() => expect(mockUpdateUniverse).toHaveBeenCalledWith('uni-1', {
      name: 'Revised World',
      description: 'A revised mystery.',
      genre_tags: ['historical'],
    }))
    expect(screen.queryByRole('dialog', { name: 'Edit World One' })).not.toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'World One', level: 3 })).toBeInTheDocument()
    expect(mockPublish).toHaveBeenCalledWith(expect.objectContaining({ message: 'Universe details saved.' }))
  })

  it('keeps the edit dialog open after a failed save and lets the user retry', async () => {
    mockListUniverses.mockResolvedValue({ universes: [{ id: 'uni-1', name: 'World One', genre_tags: [] }] })
    mockUpdateUniverse.mockRejectedValueOnce(new Error('Save unavailable'))
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Edit World One' }))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Save unavailable')
    expect(screen.getByRole('dialog', { name: 'Edit World One' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(mockUpdateUniverse).toHaveBeenCalledTimes(2))
    expect(screen.queryByRole('dialog', { name: 'Edit World One' })).not.toBeInTheDocument()
  })

  it('requires deletion confirmation and removes the universe only after a successful API call', async () => {
    mockListUniverses.mockResolvedValue({ universes: [{ id: 'uni-1', name: 'World One', genre_tags: [] }] })
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Delete World One' }))
    expect(screen.getByRole('dialog', { name: 'Delete World One?' })).toBeInTheDocument()
    expect(mockDeleteUniverse).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog', { name: 'Delete World One?' })).not.toBeInTheDocument()
    expect(mockDeleteUniverse).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Delete World One' }))
    await user.click(screen.getByRole('button', { name: 'Delete universe' }))
    await waitFor(() => expect(mockDeleteUniverse).toHaveBeenCalledWith('uni-1'))
    expect(await screen.findByText('No universes yet')).toBeInTheDocument()
    expect(mockPublish).toHaveBeenCalledWith(expect.objectContaining({ message: 'Universe deleted.' }))
  })

  it('retains the deletion confirmation after an API failure so the user can retry', async () => {
    mockListUniverses.mockResolvedValue({ universes: [{ id: 'uni-1', name: 'World One', genre_tags: [] }] })
    mockDeleteUniverse.mockRejectedValueOnce(new Error('Delete unavailable'))
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Delete World One' }))
    await user.click(screen.getByRole('button', { name: 'Delete universe' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Delete unavailable')
    expect(screen.getByRole('dialog', { name: 'Delete World One?' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Delete universe' }))
    await waitFor(() => expect(mockDeleteUniverse).toHaveBeenCalledTimes(2))
    expect(await screen.findByText('No universes yet')).toBeInTheDocument()
  })

  it('keeps universe settings dialogs accessible and returns focus to their triggers', async () => {
    mockListUniverses.mockResolvedValue({ universes: [{ id: 'uni-1', name: 'World One', genre_tags: [] }] })
    const user = userEvent.setup()
    renderPage()

    const editTrigger = await screen.findByRole('button', { name: 'Edit World One' })
    const deleteTrigger = screen.getByRole('button', { name: 'Delete World One' })
    await user.click(editTrigger)

    const editDialog = screen.getByRole('dialog', { name: 'Edit World One' })
    const name = screen.getByLabelText('Name')
    expect(name).toHaveFocus()
    expect(screen.getByRole('button', { name: 'Close edit universe dialog' })).toBeInTheDocument()

    screen.getByRole('button', { name: 'Cancel' }).focus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Close edit universe dialog' })).toHaveFocus()
    await user.tab({ shift: true })
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus()

    await user.keyboard('{Escape}')
    expect(editDialog).not.toBeInTheDocument()
    await waitFor(() => expect(editTrigger).toHaveFocus())

    await user.click(deleteTrigger)
    const deleteDialog = screen.getByRole('dialog', { name: 'Delete World One?' })
    expect(deleteDialog).toHaveAccessibleDescription('This removes its works, chapters, and stored story memory. This cannot be undone.')
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus()

    await user.keyboard('{Escape}')
    expect(deleteDialog).not.toBeInTheDocument()
    await waitFor(() => expect(deleteTrigger).toHaveFocus())
  })

  it('has no demo clone/reset affordance — provisioning now lives only on Landing and Login', async () => {
    mockListUniverses.mockResolvedValue({ universes: [{ id: 'uni-1', name: 'World One', genre_tags: [] }] })
    renderPage()

    expect(await screen.findByRole('heading', { name: 'World One', level: 3 })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Clone demo universe' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Reset demo' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start guided demo' })).not.toBeInTheDocument()
    expect(screen.queryByText('Ask Memory a lore question.')).not.toBeInTheDocument()
  })
})
