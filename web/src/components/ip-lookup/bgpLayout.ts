import type { IPLookupBGPTopology } from '@/api/ipLookup'

type Node = IPLookupBGPTopology['nodes'][number]
export type GraphEdge = Omit<IPLookupBGPTopology['edges'][number], 'relationship'> & { id: string; relationship: IPLookupBGPTopology['edges'][number]['relationship'] | 'path'; hops?: number }
type Edge = GraphEdge
export type PlacedNode = Node & { layer: number; x: number; y: number }
export const nodeWidth = 168
export const nodeHeight = 44

export function placeNodes(nodes: Node[], edges: Edge[], distance: Map<number, number>, frequency: Map<number, number>) {
  const furthest = Math.max(0, ...distance.values())
  const placed: PlacedNode[] = nodes.map(node => ({ ...node, layer: distance.get(node.asn) ?? furthest + 1, x: 0, y: 0 }))
  const columns = new Map<number, PlacedNode[]>()
  const adjacency = new Map<number, number[]>()
  for (const edge of edges) {
    for (const [a, b] of [[edge.from, edge.to], [edge.to, edge.from]] as const) {
      const group = adjacency.get(a) ?? []; group.push(b); adjacency.set(a, group)
    }
  }
  for (const node of placed) { const column = columns.get(node.layer) ?? []; column.push(node); columns.set(node.layer, column) }
  const layers = [...columns.keys()].sort((a, b) => a - b)
  for (const column of columns.values()) column.sort((a, b) => (frequency.get(b.asn) ?? 0) - (frequency.get(a.asn) ?? 0) || a.asn - b.asn)
  const ranks = new Map<number, number>()
  const rank = () => { for (const column of columns.values()) column.forEach((node, i) => ranks.set(node.asn, i)) }
  rank()
  for (let pass = 0; pass < 6; pass++) {
    for (const layer of pass % 2 ? [...layers].reverse() : layers) {
      const score = (node: PlacedNode) => {
        const adjacent = (adjacency.get(node.asn) ?? []).filter(asn => ranks.has(asn))
        return adjacent.length ? adjacent.reduce((sum, asn) => sum + ranks.get(asn)!, 0) / adjacent.length : ranks.get(node.asn)!
      }
      columns.get(layer)!.sort((a, b) => score(a) - score(b) || a.asn - b.asn)
      rank()
    }
  }
  const height = Math.max(240, Math.max(0, ...[...columns.values()].map(column => column.length)) * 64 + 64)
  for (const [layer, column] of columns) column.forEach((node, i) => {
    node.x = 32 + layer * 220
    node.y = (height - (column.length * 64 - 20)) / 2 + i * 64
  })
  return { nodes: placed, width: Math.max(264, (layers.at(-1) ?? 0) * 220 + 264), height }
}

export function routeEdges(nodes: PlacedNode[], edges: Edge[]) {
  const byASN = new Map(nodes.map(node => [node.asn, node]))
  const ports = new Map<string, Edge[]>()
  for (const edge of edges) for (const asn of [edge.from, edge.to]) {
    const group = ports.get(`${asn}`) ?? []; group.push(edge); ports.set(`${asn}`, group)
  }
  for (const [asn, group] of ports) group.sort((a, b) => {
    const other = (edge: Edge) => byASN.get(edge.from === Number(asn) ? edge.to : edge.from)!.y
    return other(a) - other(b) || a.id.localeCompare(b.id)
  })
  const portPositions = new Map<string, number>()
  for (const [asn, group] of ports) group.forEach((edge, i) => portPositions.set(`${asn}:${edge.id}`, byASN.get(Number(asn))!.y + 6 + (i + 1) * 32 / (group.length + 1)))
  const portY = (node: PlacedNode, edge: Edge) => portPositions.get(`${node.asn}:${edge.id}`)!
  return edges.map((edge, i) => {
    const from = byASN.get(edge.from)!, to = byASN.get(edge.to)!
    const sy = portY(from, edge), ty = portY(to, edge)
    const direction = from.x > to.x ? -1 : 1
    const sx = from.x + (direction < 0 ? 0 : nodeWidth), tx = to.x + (direction < 0 ? nodeWidth : 0)
    let d: string, extraX = 0, extraY = 0
    if (from.layer === to.layer) {
      extraX = from.x + nodeWidth + 16 + (i % 5) * 8
      d = `M ${from.x + nodeWidth} ${sy} C ${extraX} ${sy}, ${extraX} ${ty}, ${to.x + nodeWidth} ${ty}`
    } else if (Math.abs(from.layer - to.layer) > 1) {
      extraY = 12 + (i % 4) * 4
      d = `M ${sx} ${sy} C ${sx + direction * 22} ${extraY}, ${tx - direction * 22} ${extraY}, ${tx} ${ty}`
    } else {
      const trunk = (sx + tx) / 2
      d = `M ${sx} ${sy} L ${trunk - direction * 12} ${sy} C ${trunk} ${sy}, ${trunk} ${ty}, ${trunk + direction * 12} ${ty} L ${tx} ${ty}`
    }
    const segments: Segment[] = from.layer === to.layer
      ? [[from.x + nodeWidth, sy, extraX, sy], [extraX, sy, extraX, ty], [extraX, ty, to.x + nodeWidth, ty]]
      : [[sx, sy, tx, ty]]
    return { ...edge, d, segments, bounds: { left: Math.min(sx, tx), right: Math.max(sx, tx, extraX), top: Math.min(sy, ty, extraY || Infinity), bottom: Math.max(sy, ty) } }
  })
}

export type Segment = [number, number, number, number]

// Merge coincident line intervals without joining gaps or inventing geometry.
export function mergeSegments(segments: Segment[]) {
  const carriers = new Map<string, { vertical: boolean; fixed: number; intervals: [number, number][] }>()
  const diagonal = new Set<string>()
  for (const [sx, sy, tx, ty] of segments) {
    if (sx !== tx && sy !== ty) { diagonal.add(`M ${sx} ${sy} L ${tx} ${ty}`); continue }
    const vertical = sx === tx, fixed = vertical ? sx : sy
    const key = `${vertical}:${fixed}`
    const carrier = carriers.get(key) ?? { vertical, fixed, intervals: [] }
    const a = vertical ? sy : sx, b = vertical ? ty : tx
    carrier.intervals.push([Math.min(a, b), Math.max(a, b)])
    carriers.set(key, carrier)
  }
  const paths = [...diagonal]
  for (const { vertical, fixed, intervals } of carriers.values()) {
    intervals.sort((a, b) => a[0] - b[0])
    let start = intervals[0]![0], end = intervals[0]![1]
    const append = () => { paths.push(vertical ? `M ${fixed} ${start} L ${fixed} ${end}` : `M ${start} ${fixed} L ${end} ${fixed}`) }
    for (const [a, b] of intervals.slice(1)) {
      if (a <= end) end = Math.max(end, b)
      else { append(); start = a; end = b }
    }
    append()
  }
  return paths.join(' ')
}
