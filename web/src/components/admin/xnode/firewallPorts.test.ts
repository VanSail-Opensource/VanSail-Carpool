import { describe, expect, it } from 'vitest'
import { firewallPortsText, mergeFirewallPortRanges, parseFirewallPortRange, parseFirewallPorts } from './firewallPorts'

describe('firewall port intervals', () => {
  it('merges duplicates, overlaps and adjacent ports without expanding ranges', () => {
    expect(parseFirewallPorts('443 8443, 20000 - 21000\n20900-22000,22001,443')).toEqual({ ports: [443, 8443], ranges: [{ from: 20000, to: 22001 }] })
    expect(parseFirewallPorts('1-65535')).toEqual({ ports: [], ranges: [{ from: 1, to: 65535 }] })
    expect(parseFirewallPorts('')).toEqual({ ports: [], ranges: [] })
  })

  it('reads legacy singletons and additive intervals as one compact list', () => {
    const policy = { enabled: true, protect: false, tcp_ports: [22, 443, 444], tcp_port_ranges: [{ from: 443, to: 445 }] }
    expect(firewallPortsText(policy, 'tcp_ports')).toBe('22, 443-445')
    expect(policy.tcp_port_ranges).toEqual([{ from: 443, to: 445 }])
    const original = [{ from: 81, to: 82 }, { from: 80, to: 80 }]
    expect(mergeFirewallPortRanges(original)).toEqual([{ from: 80, to: 82 }])
    expect(original[0]).toEqual({ from: 81, to: 82 })
  })

  it('rejects out-of-range, descending, incomplete and non-integer input', () => {
    for (const value of ['0', '65536', '500-400', '20-', '-20', '1.5', '1e3', '1:2', '1-2-3']) expect(parseFirewallPorts(value)).toBeNull()
    expect(parseFirewallPortRange('1,2')).toBeNull()
    expect(parseFirewallPortRange('1 2')).toBeNull()
    expect(parseFirewallPortRange('65535')).toEqual({ from: 65535, to: 65535 })
  })
})
