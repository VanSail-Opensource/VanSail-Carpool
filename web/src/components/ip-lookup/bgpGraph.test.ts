import { describe, expect, it } from 'vitest'
import type { IPLookupBGPTopology } from '@/api/ipLookup'
import { graphLayout, indexTopology, routesToASN } from './bgpGraph'

function fixture(): IPLookupBGPTopology {
  return { status: 'success', source: 'ripe', observedAt: '', errorCode: '', prefix: '1.1.1.0/24', origins: [10, 11],
    nodes: [10, 11, 20, 21, 22, 23, 174, 701, 999].map(asn => ({ asn, name: '', tier1: asn === 174 || asn === 701 })),
    edges: [{ from: 20, to: 10, relationship: 'upstream' }, { from: 10, to: 21, relationship: 'customer' }, { from: 22, to: 10, relationship: 'peer' }, { from: 23, to: 11, relationship: 'observed' }, { from: 174, to: 11, relationship: 'peer' }],
    tier1Links: [{ origin: 10, neighbor: 20, tier1: 174, hops: 3 }, { origin: 10, neighbor: 20, tier1: 701, hops: 4 }],
    paths: [], truncated: false, relationshipMeta: { status: 'success', source: 'caida', observedAt: '', errorCode: '' } }
}
describe('fixed compact BGP graph', () => {
  it('focuses only observed compact routes to the selected ASN', () => {
    const index = indexTopology(fixture())
    const routes = routesToASN(index, 701)
    expect(routes).toHaveLength(1)
    expect(routes[0]?.asns).toEqual([10, 20, 701])
    expect(routes[0]?.edgeIds).toEqual(['10-20:upstream', 'path:20-701'])
    expect(routes[0]?.hops).toBe(4)
    expect(routesToASN(index, 20).every(route => route.asns.length === 2)).toBe(true)
    expect(routesToASN(index, null)).toEqual([])
  })
  it('keeps every neighbor and observed T1, drops unrelated nodes and normalizes reversed relationships', () => {
    const layout = graphLayout(indexTopology(fixture()))
    expect(layout.nodes.map(node => node.asn)).toEqual([10, 11, 20, 21, 22, 23, 174, 701])
    expect(layout.nodes.find(node => node.asn === 174)?.layer).toBe(1)
    expect(layout.nodes.find(node => node.asn === 701)?.layer).toBe(2)
    expect(layout.edges.find(edge => edge.to === 21)?.relationship).toBe('upstream')
    expect(layout.edges.filter(edge => edge.relationship === 'path').map(edge => edge.hops)).toEqual([3, 4])
    expect(new Set(layout.nodes.map(node => node.asn)).size).toBe(layout.nodes.length)
  })
  it('keeps direct neighbors when classification is unknown and distinguishes no upstream from unknown', () => {
    const topology = fixture()
    topology.nodes.forEach(node => { node.tier1 = null }); topology.tier1Links = []
    let layout = graphLayout(indexTopology(topology))
    expect(layout.fallback).toBe(true)
    expect(layout.nodes).toHaveLength(7)
    topology.edges.forEach(edge => { edge.relationship = 'peer' })
    layout = graphLayout(indexTopology(topology)); expect(layout.upstreamsEmpty).toBe('none')
    topology.edges[0]!.relationship = 'observed'
    expect(graphLayout(indexTopology(topology)).upstreamsEmpty).toBe('unknown')
  })
})
