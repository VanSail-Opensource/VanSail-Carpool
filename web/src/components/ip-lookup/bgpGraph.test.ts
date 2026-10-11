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
  it('distinguishes an observed Tier-1 peer-only origin from unavailable classifications', () => {
    const topology: IPLookupBGPTopology = {
      status: 'success', source: 'ripe', observedAt: '', errorCode: '', prefix: '172.127.0.0/16', origins: [7018],
      nodes: [{ asn: 7018, name: 'AT&T', tier1: true }, { asn: 174, name: 'Cogent', tier1: true }, { asn: 6939, name: 'Hurricane Electric', tier1: false }],
      edges: [{ from: 174, to: 7018, relationship: 'peer' }, { from: 6939, to: 7018, relationship: 'peer' }],
      paths: [{ asPath: [174, 7018], collector: 'RRC', exchange: '', observedAt: '' }], truncated: false,
      relationshipMeta: { status: 'success', source: 'caida', observedAt: '2026-10-01', errorCode: '' },
    }
    expect(graphLayout(indexTopology(topology), 'upstreams').upstreamsEmpty).toBe('none')
    expect(graphLayout(indexTopology(topology), 'upstreams').nodes.map(node => node.asn)).toEqual([7018])
    topology.edges[0]!.relationship = 'observed'
    expect(graphLayout(indexTopology(topology), 'upstreams').upstreamsEmpty).toBe('unknown')
    topology.edges[0]!.relationship = 'upstream'
    const upstream = graphLayout(indexTopology(topology), 'upstreams')
    expect(upstream.upstreamsEmpty).toBeNull()
    expect(upstream.nodes.map(node => node.asn)).toEqual([7018, 174])
    expect(graphLayout(indexTopology(topology), 'tier1').nodes.some(node => node.asn === 6939)).toBe(false)
  })
})
