import type { IPLookupBGPTopology } from '@/api/ipLookup'
import { placeNodes, routeEdges } from './bgpLayout'

export type GraphPreset = 'tier1' | 'upstreams' | 'neighbors' | 'all'
export const graphPresets: GraphPreset[] = ['tier1', 'upstreams', 'neighbors', 'all']

export function indexTopology(topology: IPLookupBGPTopology) {
  const origins = new Set(topology.origins)
  const neighbors = new Map<number, Set<number>>()
  const frequency = new Map<number, number>()
  const edges = new Map(topology.edges.filter(edge => edge.from !== edge.to).map(edge => [`${edge.from}-${edge.to}`, edge]))
  for (const edge of edges.values()) {
    for (const [a, b] of [[edge.from, edge.to], [edge.to, edge.from]] as const) {
      const group = neighbors.get(a) ?? new Set<number>()
      group.add(b); neighbors.set(a, group)
    }
  }
  for (const path of topology.paths) {
    for (const asn of new Set(path.asPath)) frequency.set(asn, (frequency.get(asn) ?? 0) + 1)
  }
  const distance = new Map(topology.origins.map(asn => [asn, 0]))
  const queue = [...topology.origins]
  for (let i = 0; i < queue.length; i++) for (const asn of neighbors.get(queue[i]!) ?? []) {
    if (!distance.has(asn)) { distance.set(asn, distance.get(queue[i]!)! + 1); queue.push(asn) }
  }
  return { topology, origins, neighbors, frequency, edges, distance }
}

export function graphLayout(index: ReturnType<typeof indexTopology>, preset: GraphPreset, focusPath?: number[]) {
  const { topology, origins, distance } = index
  const tier1 = new Set(topology.nodes.filter(node => node.tier1 === true).map(node => node.asn))
  const visible = new Set(origins)
  let allowedEdges: Set<string> | undefined
  if (focusPath) {
    visible.clear(); focusPath.forEach(asn => visible.add(asn)); allowedEdges = pathEdges(focusPath)
  } else if (preset === 'tier1') {
    allowedEdges = new Set<string>()
    for (const path of topology.paths) {
      const first = path.asPath.findIndex(asn => tier1.has(asn))
      if (first < 0) continue
      const route = path.asPath.slice(first)
      route.forEach(asn => visible.add(asn))
      for (const id of pathEdges(route)) allowedEdges.add(id)
    }
  } else if (preset === 'all') topology.nodes.forEach(node => visible.add(node.asn))
  else if (preset === 'neighbors') topology.nodes.filter(node => distance.get(node.asn) === 1).forEach(node => visible.add(node.asn))
  else for (const edge of index.edges.values()) {
    if (origins.has(edge.to) && edge.relationship === 'upstream') visible.add(edge.from)
    if (origins.has(edge.from) && edge.relationship === 'customer') visible.add(edge.to)
  }
  const fallback = !focusPath && preset === 'tier1' && visible.size === origins.size
  if (fallback) {
    allowedEdges = undefined
    topology.nodes.filter(node => distance.get(node.asn) === 1).forEach(node => visible.add(node.asn))
  }
  const nodes = topology.nodes.filter(node => visible.has(node.asn))
  const edges = [...index.edges.entries()].filter(([id, edge]) => visible.has(edge.from) && visible.has(edge.to) && (!allowedEdges || allowedEdges.has(id))).map(([id, edge]) => ({ ...edge, id }))
  const placed = placeNodes(nodes, edges, distance, index.frequency)
  return { ...placed, edges: routeEdges(placed.nodes, edges), hidden: topology.nodes.length - nodes.length, fallback }
}

export function pathEdges(path: number[]) {
  return new Set(path.slice(1).flatMap((asn, i) => asn === path[i] ? [] : [`${path[i]}-${asn}`]))
}
