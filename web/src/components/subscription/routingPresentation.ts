import type { RoutingView } from '@/api/xnodeMember'

type Endpoint = NonNullable<RoutingView['endpoints']>[number]
type Match = RoutingView['templates'][number]['match']

export function endpointAddress(endpoint?: Endpoint): string {
  if (!endpoint) return ''
  const host = endpoint.address.includes(':') && !endpoint.address.startsWith('[') ? `[${endpoint.address}]` : endpoint.address
  return `${host}:${endpoint.port}`
}

export function selectorLabel(matches: Match[], fallback: string): string {
  const groups: string[] = []
  for (const field of ['domain', 'ip', 'network', 'protocol'] as const) {
    const category: string[][] = []
    for (const match of matches) {
      const values: string[] = match[field] ?? []
      if (!values.length || category.some(previous => previous.every(value => values.includes(value)))) continue
      // Simplify only proven literal inclusion; retain other intersections.
      const retained = category.filter(previous => !values.every(value => previous.includes(value)))
      category.splice(0, category.length, ...retained, [...values])
    }
    for (const values of category) groups.push(values.length > 1 ? `(${values.join(' ∨ ')})` : values[0]!)
  }
  for (const port of new Set(matches.map(match => match.port).filter(Boolean))) groups.push(`port:${port}`)
  return groups.join(' ∩ ') || fallback
}
