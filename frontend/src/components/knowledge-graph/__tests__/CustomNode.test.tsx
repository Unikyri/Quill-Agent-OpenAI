import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { ReactFlowProvider, Position } from 'reactflow'
import CustomNode from '../CustomNode'

// ReactFlow nodes need a provider in tests
function renderNode(type: string, label: string) {
  return render(
    <ReactFlowProvider>
      <CustomNode
        id="test-1"
        type="custom"
        data={{ type, label }}
        xPos={0}
        yPos={0}
        zIndex={0}
        selected={false}
        dragging={false}
        isConnectable={true}
        sourcePosition={Position.Bottom}
        targetPosition={Position.Top}
      />
    </ReactFlowProvider>
  )
}

describe('CustomNode', () => {
  it('renders character node with the character token color and monochrome glyph', () => {
    const { container } = renderNode('character', 'Alice')
    const node = container.firstElementChild as HTMLElement
    expect(node.style.borderColor).toBe('var(--node-character)')
    expect(container.textContent).toContain('●')
    expect(container.textContent).toContain('Alice')
  })

  it('renders place node with the place token color and monochrome glyph', () => {
    const { container } = renderNode('place', 'Castle')
    const node = container.firstElementChild as HTMLElement
    expect(node.style.borderColor).toBe('var(--node-place)')
    expect(container.textContent).toContain('◆')
    expect(container.textContent).toContain('Castle')
  })

  it('renders faction node with the faction token color and monochrome glyph', () => {
    const { container } = renderNode('faction', 'The Order')
    const node = container.firstElementChild as HTMLElement
    expect(node.style.borderColor).toBe('var(--node-faction)')
    expect(container.textContent).toContain('■')
  })

  it('renders event node with the event token color and monochrome glyph', () => {
    const { container } = renderNode('event', 'Battle')
    const node = container.firstElementChild as HTMLElement
    expect(node.style.borderColor).toBe('var(--node-event)')
    expect(container.textContent).toContain('▲')
  })

  it('renders world_rule node with the world-rule token color and monochrome glyph', () => {
    const { container } = renderNode('world_rule', 'Magic System')
    const node = container.firstElementChild as HTMLElement
    expect(node.style.borderColor).toBe('var(--node-worldrule)')
    expect(container.textContent).toContain('◈')
  })

  it('renders plot_arc node with the plot-arc token color and monochrome glyph', () => {
    const { container } = renderNode('plot_arc', 'The Rebellion')
    const node = container.firstElementChild as HTMLElement
    expect(node.style.borderColor).toBe('var(--node-plotarc)')
    expect(container.textContent).toContain('◉')
  })

  it('falls back to character style for unknown type', () => {
    const { container } = renderNode('unknown_type', 'Mystery')
    const node = container.firstElementChild as HTMLElement
    expect(node.style.borderColor).toBe('var(--node-character)')
    expect(container.textContent).toContain('●')
  })

  it('shows "Untitled" when label is empty', () => {
    const { container } = renderNode('character', '')
    expect(container.textContent).toContain('Untitled')
  })
})
