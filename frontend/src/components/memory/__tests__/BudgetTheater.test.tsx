import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import BudgetTheater from '../BudgetTheater'
import type { ExplainedItem, RecallBudget } from '../../../lib/types'

const budget: RecallBudget = {
  max_context_tokens: 1000,
  available: 400,
  entities_tokens: 200,
  vector_tokens: 300,
  tools_tokens: 100,
  used_percent: 60,
}

const items: ExplainedItem[] = [
  { id: 'i1', entity_id: 'e1', fact: 'fitted item', rrf_score: 0.9, contributions: [], fit_in_budget: true },
  { id: 'i2', entity_id: 'e2', fact: 'dropped item', rrf_score: 0.2, contributions: [], fit_in_budget: false },
]

describe('BudgetTheater', () => {
  it('renders one bar per budget category with fitted vs dropped counts and % used', () => {
    render(<BudgetTheater budget={budget} items={items} />)

    expect(screen.getByTestId('budget-bar-entities_tokens')).toHaveTextContent('200')
    expect(screen.getByTestId('budget-bar-vector_tokens')).toHaveTextContent('300')
    expect(screen.getByTestId('budget-bar-tools_tokens')).toHaveTextContent('100')
    expect(screen.getByText(/60/)).toBeInTheDocument()

    expect(screen.getByTestId('budget-fitted-count')).toHaveTextContent('1')
    expect(screen.getByTestId('budget-dropped-count')).toHaveTextContent('1')
  })

  it('renders an empty state without crashing when budget is null', () => {
    render(<BudgetTheater budget={null} items={[]} />)
    expect(screen.getByText(/no budget data yet/i)).toBeInTheDocument()
  })
})
