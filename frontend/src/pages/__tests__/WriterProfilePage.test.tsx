import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import WriterProfilePage from '../WriterProfilePage'

vi.mock('../WriterProfilePage.module.css', () => ({ default: new Proxy({}, { get: (_, key) => key }) }))

vi.mock('../../components/memory/WriterMemoryPanel', () => ({
  default: () => <div>Writer memory panel rendered with no universeId</div>,
}))

describe('WriterProfilePage', () => {
  it('renders the account-scoped Writer Memory panel', () => {
    render(<WriterProfilePage />)
    expect(screen.getByRole('heading', { name: /writer profile/i })).toBeInTheDocument()
    expect(screen.getByText('Writer memory panel rendered with no universeId')).toBeInTheDocument()
  })
})
