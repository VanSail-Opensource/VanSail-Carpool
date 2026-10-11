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
  it('keeps intermediate transits and sorts each column by observations without inventing focus paths', () => {
    const topology = fixture()
    topology.projectionVersion = 2
    topology.origins = [45671]
    topology.nodes = [45671, 9280, 38195, 58511, 174, 701].map(asn => ({ asn, name: '', observedRoutes: asn === 58511 ? 8 : 3 }))
    topology.edges = [{ from: 9280, to: 45671, relationship: 'upstream' }, { from: 38195, to: 9280, relationship: 'upstream' }, { from: 58511, to: 9280, relationship: 'upstream' }, { from: 174, to: 38195, relationship: 'upstream' }]
    topology.routeSummaries = [{ asns: [45671, 9280, 38195, 174, 701], hops: [1, 1, 1, 3], observedRoutes: 3 }, { asns: [45671, 9280, 58511], hops: [1, 1], observedRoutes: 8 }]
    const index = indexTopology(topology), layout = graphLayout(index)
    expect(layout.nodes.find(node => node.asn === 58511)!.y).toBeLessThan(layout.nodes.find(node => node.asn === 38195)!.y)
    expect(layout.nodes).toHaveLength(6)
    const focused = routesToASN(index, 701)
    expect(focused).toHaveLength(1)
    expect(focused[0]!.edgeIds).toEqual(['45671-9280:upstream', '9280-38195:upstream', '38195-174:upstream', 'path:174-701:3'])
    expect(focused[0]!.edgeIds.some(id => id.includes('58511'))).toBe(false)
  })
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
