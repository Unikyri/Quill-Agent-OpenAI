import { describe, expect, it } from 'vitest'
import { adaptEntityNeighborhood, toCytoscapeElements } from '../graphElements'

describe('graph element adapter', () => {
  const neighborhood = adaptEntityNeighborhood({
    nodes: [
      {
        id: 'vertex-1',
        properties: {
          raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice","status":"active","relevance_score":0.7}}',
        },
      },
      {
        id: 'vertex-2',
        properties: {
          raw: '{"id":2,"label":"place","properties":{"entity_id":"n2","name":"Archive","status":"archived"}}',
        },
      },
    ],
    edges: [{ id: 'e1', source: 'n1', target: 'n2', type: 'lives_in' }],
    truncated: false,
    limits: { hops: 2, max_hops: 2, node_limit: 96, edge_limit: 160, result_limit: 256 },
  })

  it('maps the neighborhood API into renderer-neutral story elements', () => {
    expect(neighborhood.nodes).toEqual([
      {
        id: 'n1',
        type: 'character',
        data: { label: 'Alice', relevanceScore: 0.7, status: 'active' },
      },
      {
        id: 'n2',
        type: 'place',
        data: { label: 'Archive', relevanceScore: undefined, status: 'archived' },
      },
    ])
    expect(neighborhood.edges).toEqual([
      { id: 'e1', source: 'n1', target: 'n2', relationshipType: 'lives_in' },
    ])
    expect(neighborhood.truncated).toBe(false)
    expect(neighborhood.limits?.node_limit).toBe(96)
  })

  it('keeps server truncation metadata instead of presenting a partial map as complete', () => {
    const partial = adaptEntityNeighborhood({
      nodes: [{ id: 'vertex-1', properties: { raw: '{"id":1,"label":"character","properties":{"entity_id":"n1","name":"Alice"}}' } }],
      edges: [],
      truncated: true,
      limits: { hops: 2, max_hops: 2, node_limit: 96, edge_limit: 160, result_limit: 256 },
    })

    expect(partial.truncated).toBe(true)
  })

  it('keeps relationship meaning for selection without putting labels on the map edge', () => {
    const elements = toCytoscapeElements(neighborhood, 'n1')
    const focalNode = elements.find((element) => element.group === 'nodes' && element.data.id === 'n1')
    const edge = elements.find((element) => element.group === 'edges')

    expect(focalNode?.data.focal).toBe(true)
    expect(edge?.data).toMatchObject({
      id: 'e1',
      source: 'n1',
      target: 'n2',
      relationshipType: 'lives_in',
    })
    expect(edge?.data).not.toHaveProperty('label')
  })

  it('stamps each node with its ego-graph hop and each edge with its peripheral tier', () => {
    const ego = adaptEntityNeighborhood({
      nodes: [
        { id: 'v0', properties: { hop: 0, raw: '{"id":1,"label":"character","properties":{"entity_id":"focal","name":"Focal"}}' } },
        { id: 'v1', properties: { hop: 1, raw: '{"id":2,"label":"character","properties":{"entity_id":"d1","name":"Direct"}}' } },
        { id: 'v1b', properties: { hop: 1, raw: '{"id":4,"label":"character","properties":{"entity_id":"d1b","name":"Direct2"}}' } },
        { id: 'v2', properties: { hop: 2, raw: '{"id":3,"label":"character","properties":{"entity_id":"d2","name":"Second"}}' } },
      ],
      edges: [
        { id: 'e-focal-d1', source: 'focal', target: 'd1', type: 'ALLY_OF' },
        { id: 'e-d1-d1b', source: 'd1', target: 'd1b', type: 'KNOWS' },
        { id: 'e-d1-d2', source: 'd1', target: 'd2', type: 'KNOWS' },
      ],
      truncated: false,
      limits: { hops: 2, max_hops: 2, node_limit: 96, edge_limit: 160, result_limit: 256 },
    })

    const elements = toCytoscapeElements(ego, 'focal')
    const nodeHop = (id: string) => elements.find((el) => el.group === 'nodes' && el.data.id === id)?.data.hop
    expect(nodeHop('focal')).toBe(0)
    expect(nodeHop('d1')).toBe(1)
    expect(nodeHop('d2')).toBe(2)

    const edgeTier = (id: string) => elements.find((el) => el.group === 'edges' && el.data.id === id)?.data.edgeTier
    expect(edgeTier('e-focal-d1')).toBe(0) // touches the focal entity
    expect(edgeTier('e-d1-d1b')).toBe(1) // between two direct neighbors
    expect(edgeTier('e-d1-d2')).toBe(2) // reaches a second-degree node
  })
})
