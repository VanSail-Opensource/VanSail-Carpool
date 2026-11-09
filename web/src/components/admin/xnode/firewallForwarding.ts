import { z } from 'zod'
import { formatFirewallPortRange, parseFirewallPortRange, type FirewallPolicy } from './firewallPorts'

export type FirewallForward = NonNullable<FirewallPolicy['forward']>[number]
export interface FirewallForwardDraft {
  id: number
  listen_ip: string
  ports: string
  target_ip: string
  target_ports: string
  network: string
}
type ForwardField = 'listen_ip' | 'ports' | 'target_ip' | 'target_ports' | 'network'
export type FirewallForwardErrors = Partial<Record<ForwardField | 'overlap', string>>
const ip = z.union([z.ipv4(), z.ipv6()])

export function firewallForwardDraft(row: FirewallForward, id: number): FirewallForwardDraft {
  return { id, listen_ip: row.listen_ip ?? '', ports: formatFirewallPortRange({ from: row.port, to: row.port_end || row.port }),
    target_ip: row.target_ip, target_ports: formatFirewallPortRange({ from: row.target_port, to: row.target_port_end || row.target_port }), network: row.network }
}

function canonicalIP(value: string): string {
  return value.includes(':') ? new globalThis.URL(`http://[${value}]/`).hostname.slice(1, -1) : value
}

function validIP(value: string, allowUnspecified = false): boolean {
  if (!ip.safeParse(value).success || value.includes('%')) return false
  try {
    const canonical = canonicalIP(value)
    const first = Number(canonical.split('.')[0])
    return !canonical.startsWith('::ffff:') && !canonical.startsWith('ff') && !(first >= 224 && first <= 239)
      && (allowUnspecified || !['::', '0.0.0.0'].includes(canonical))
  } catch { return false }
}

function sameListener(a: FirewallForward, b: FirewallForward): boolean {
  if (a.network !== b.network || a.target_ip.includes(':') !== b.target_ip.includes(':')) return false
  const left = a.listen_ip || (a.target_ip.includes(':') ? '::' : '0.0.0.0')
  const right = b.listen_ip || (b.target_ip.includes(':') ? '::' : '0.0.0.0')
  return ['::', '0.0.0.0'].includes(left) || ['::', '0.0.0.0'].includes(right) || canonicalIP(left) === canonicalIP(right)
}

export function parseFirewallForwards(drafts: readonly FirewallForwardDraft[]) {
  const errors: FirewallForwardErrors[] = drafts.map(() => ({}))
  const rows: { value: FirewallForward; index: number }[] = []
  const seen = new Set<string>()
  for (const [index, draft] of drafts.entries()) {
    const error = errors[index]!
    const source = parseFirewallPortRange(draft.ports)
    const target = parseFirewallPortRange(draft.target_ports)
    const listen = draft.listen_ip.trim(), address = draft.target_ip.trim()
    if (!source) error.ports = 'portRangeInvalid'
    if (!target) error.target_ports = 'portRangeInvalid'
    if (source && target && target.to !== target.from && target.to - target.from !== source.to - source.from) error.target_ports = 'forwardLengthInvalid'
    if (!validIP(address)) error.target_ip = 'ipInvalid'
    if (listen && !validIP(listen, true)) error.listen_ip = 'ipInvalid'
    else if (listen && address && listen.includes(':') !== address.includes(':')) error.listen_ip = 'forwardFamilyInvalid'
    if (!['tcp', 'udp'].includes(draft.network)) error.network = 'forwardNetworkInvalid'
    if (Object.keys(error).length || !source || !target) continue
    const value: FirewallForward = { listen_ip: !listen || ['::', '0.0.0.0'].includes(canonicalIP(listen)) ? '' : canonicalIP(listen),
      port: source.from, ...(source.to !== source.from ? { port_end: source.to } : {}), target_ip: canonicalIP(address),
      target_port: target.from, ...(target.to !== target.from ? { target_port_end: target.to } : {}), network: draft.network }
    const signature = JSON.stringify(value)
    if (!seen.has(signature)) rows.push({ index, value })
    seen.add(signature)
  }
  for (const [position, row] of rows.entries()) {
    for (const other of rows.slice(position + 1)) {
      if (sameListener(row.value, other.value) && row.value.port <= (other.value.port_end ?? other.value.port) && other.value.port <= (row.value.port_end ?? row.value.port)) {
        errors[row.index]!.overlap = 'forwardOverlap'
        errors[other.index]!.overlap = 'forwardOverlap'
      }
    }
  }
  const mapped = rows.filter(row => row.value.target_port_end)
  if (mapped.reduce((total, row) => total + (row.value.port_end ?? row.value.port) - row.value.port + 1, 0) > 4 * 65535) {
    for (const row of mapped) errors[row.index]!.ports ??= 'forwardMapLimit'
  }
  return { values: rows.map(row => row.value), errors, valid: errors.every(error => !Object.keys(error).length) }
}
