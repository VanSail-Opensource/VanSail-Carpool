import { describe, expect, it } from 'vitest'
import type { IPLookupBGPTopology } from '@/api/ipLookup'
import { graphLayout, pathEdges, indexTopology } from './bgpGraph'

describe('BGP graph visibility', () => {
  it('keeps multiple origins, prioritizes identified upstreams and expands indirect paths', () => {
    const direct = Array.from({ length: 16 }, (_, index) => index + 100)
    const topology: IPLookupBGPTopology = {
      status: 'success', source: 'ripe', observedAt: '', errorCode: '', prefix: '1.1.1.0/24', origins: [13335, 48266],
      nodes: [13335, 48266, ...direct, 200].map(asn => ({ asn, name: '', tier1: asn === 200 })),
      edges: [...direct.map(asn => ({ from: asn, to: 13335, relationship: asn === 115 ? 'upstream' as const : 'observed' as const })), { from: 200, to: 115, relationship: 'observed' }],
      paths: [{ asPath: [200, 115, 13335, 13335], collector: 'RRC01', exchange: '', observedAt: '' }], truncated: false,
      relationshipMeta: { status: 'success', source: 'caida', observedAt: '', errorCode: '' },
    }
    const initial = graphLayout(indexTopology(topology), 'upstreams')
    expect(initial.nodes.map(node => node.asn)).toContain(115)
    expect(initial.nodes.map(node => node.asn)).toContain(48266)
    expect(initial.nodes.map(node => node.asn)).not.toContain(200)
    expect(graphLayout(indexTopology(topology), 'tier1').nodes.map(node => node.asn)).toContain(200)
    expect(graphLayout(indexTopology(topology), 'tier1').nodes.map(node => node.asn)).toContain(115)
    expect(graphLayout(indexTopology(topology), 'neighbors').nodes).toHaveLength(18)
    expect(topology.paths[0]!.asPath).toEqual([200, 115, 13335, 13335])
    expect(pathEdges([174, 13335])).toContain('174-13335')
    const chain = Array.from({ length: 14 }, (_, index) => 300 + index)
    topology.nodes.push(...chain.map(asn => ({ asn, name: '' })))
    topology.edges.push(...chain.map((asn, index) => ({ from: asn, to: index ? chain[index - 1]! : 115, relationship: 'observed' as const })))
    expect(graphLayout(indexTopology(topology), 'all').nodes.map(node => node.asn)).toContain(313)
    topology.nodes.forEach(node => { node.tier1 = null })
    const fallback = graphLayout(indexTopology(topology), 'tier1')
    expect(fallback.fallback).toBe(true)
    expect(fallback.nodes).toHaveLength(18)
    expect(pathEdges([174, 174, 13335])).toEqual(new Set(['174-13335']))
  })
})
