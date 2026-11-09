import { z } from 'zod'
import type { XNodeSettings } from '@/api/xnode'

export type FirewallPolicy = NonNullable<XNodeSettings['firewall']>
export type FirewallPortKey = 'tcp_ports' | 'udp_ports' | 'ssh_ports'
export type FirewallPortRange = NonNullable<FirewallPolicy['tcp_port_ranges']>[number]
export const firewallPortKeys: FirewallPortKey[] = ['tcp_ports', 'udp_ports', 'ssh_ports']
export const firewallRangeKeys = { tcp_ports: 'tcp_port_ranges', udp_ports: 'udp_port_ranges', ssh_ports: 'ssh_port_ranges' } as const

const port = z.number().int().min(1).max(65535)
const interval = z.object({ from: port, to: port }).refine(value => value.from <= value.to)

export function parseFirewallPortRange(text: string): FirewallPortRange | null {
  const match = /^(\d+)(?:\s*-\s*(\d+))?$/.exec(text.trim())
  if (!match) return null
  const result = interval.safeParse({ from: Number(match[1]), to: Number(match[2] ?? match[1]) })
  return result.success ? result.data : null
}

export function mergeFirewallPortRanges(values: readonly FirewallPortRange[]): FirewallPortRange[] {
  const result: FirewallPortRange[] = []
  for (const value of [...values].sort((a, b) => a.from - b.from || a.to - b.to)) {
    const previous = result.at(-1)
    if (previous && value.from <= previous.to + 1) previous.to = Math.max(previous.to, value.to)
    else result.push({ from: value.from, to: value.to })
  }
  return result
}

export function parseFirewallPorts(text: string): { ports: number[]; ranges: FirewallPortRange[] } | null {
  const values: FirewallPortRange[] = []
  for (const token of text.replace(/\s*-\s*/g, '-').split(/[\s,]+/).filter(Boolean)) {
    const value = parseFirewallPortRange(token)
    if (!value) return null
    values.push(value)
  }
  const merged = mergeFirewallPortRanges(values)
  return { ports: merged.filter(value => value.from === value.to).map(value => value.from), ranges: merged.filter(value => value.from !== value.to) }
}

export function formatFirewallPortRange(value: FirewallPortRange): string {
  return value.from === value.to ? String(value.from) : `${value.from}-${value.to}`
}

export function firewallPortsText(policy: FirewallPolicy, key: FirewallPortKey): string {
  const values = [...(policy[key] ?? []).map(port => ({ from: port, to: port })), ...(policy[firewallRangeKeys[key]] ?? [])]
  return mergeFirewallPortRanges(values).map(formatFirewallPortRange).join(', ')
}
