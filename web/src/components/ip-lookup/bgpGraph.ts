import type { IPLookupBGPTopology } from '@/api/ipLookup'
import { placeNodes, routeEdges, type GraphEdge } from './bgpLayout'

export function indexTopology(topology: IPLookupBGPTopology) {
  const origins = new Set(topology.origins)
  const distance = new Map(topology.origins.map(asn => [asn, 0]))
  const edges = new Map<string, GraphEdge>()
  const frequency = new Map<number, number>()
  for (const edge of topology.edges) {
    if (edge.from === edge.to || (!origins.has(edge.from) && !origins.has(edge.to))) continue
    const origin = origins.has(edge.to) ? edge.to : edge.from
    const neighbor = origin === edge.to ? edge.from : edge.to
    const relationship = origin === edge.to ? edge.relationship
      : edge.relationship === 'customer' ? 'upstream' : edge.relationship === 'upstream' ? 'customer' : edge.relationship
    const id = `${origin}-${neighbor}:${relationship}`
    edges.set(id, { from: origin, to: neighbor, relationship, id })
    if (!origins.has(neighbor)) distance.set(neighbor, 1)
  }
  for (const node of topology.nodes) if (!distance.has(node.asn) && node.tier1 === true) distance.set(node.asn, 2)
  for (const link of topology.tier1Links ?? []) {
    const id = `path:${link.neighbor}-${link.tier1}`
    if (distance.has(link.neighbor) && distance.has(link.tier1) && link.neighbor !== link.tier1) {
      const previous = edges.get(id)
      edges.set(id, { from: link.neighbor, to: link.tier1, relationship: 'path', id, hops: Math.min(previous?.hops ?? Infinity, link.hops) })
    }
  }
  for (const edge of edges.values()) for (const asn of [edge.from, edge.to]) frequency.set(asn, (frequency.get(asn) ?? 0) + 1)
  return { topology, origins, edges, distance, frequency }
}

export function graphLayout(index: ReturnType<typeof indexTopology>) {
  const nodes = index.topology.nodes.filter(node => index.distance.has(node.asn))
  const edges = [...index.edges.values()]
  const placed = placeNodes(nodes, edges, index.distance, index.frequency)
  const direct = edges.filter(edge => edge.relationship !== 'path')
  const upstreamsEmpty = direct.some(edge => edge.relationship === 'upstream') ? null
    : direct.some(edge => edge.relationship === 'observed') ? 'unknown' : 'none'
  return { ...placed, edges: routeEdges(placed.nodes, edges), fallback: !nodes.some(node => node.tier1 === true), upstreamsEmpty }
}

export type FocusRoute = { id: string; asns: number[]; edgeIds: string[]; hops?: number }

export function routesToASN(index: ReturnType<typeof indexTopology>, asn: number | null): FocusRoute[] {
  if (asn === null) return []
  const routes: FocusRoute[] = []
  for (const edge of index.edges.values()) if (edge.relationship !== 'path' && (edge.to === asn || edge.from === asn)) {
    routes.push({ id: edge.id, asns: [edge.from, edge.to], edgeIds: [edge.id] })
  }
  for (const link of index.topology.tier1Links ?? []) if (link.tier1 === asn || link.origin === asn) {
    const direct = [...index.edges.values()].filter(edge => edge.relationship !== 'path' && edge.from === link.origin && edge.to === link.neighbor)
    if (direct.length) routes.push({ id: `${link.origin}:${link.neighbor}:${link.tier1}`, asns: [link.origin, link.neighbor, link.tier1], edgeIds: [...direct.map(edge => edge.id), `path:${link.neighbor}-${link.tier1}`], hops: link.hops })
  }
  return routes
}
