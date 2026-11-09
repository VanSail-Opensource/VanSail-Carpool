import { describe, expect, it } from 'vitest'
import { firewallForwardDraft, parseFirewallForwards, type FirewallForwardDraft } from './firewallForwarding'

function draft(overrides: Partial<FirewallForwardDraft> = {}): FirewallForwardDraft {
  return { id: 0, listen_ip: '', ports: '1000-1009', target_ip: '192.0.2.10', target_ports: '2000-2009', network: 'udp', ...overrides }
}

describe('native firewall forwarding ranges', () => {
  it('preserves translated ranges and many-to-one forwarding', () => {
    expect(parseFirewallForwards([draft()])).toMatchObject({ valid: true, values: [{ port: 1000, port_end: 1009, target_port: 2000, target_port_end: 2009 }] })
    expect(parseFirewallForwards([draft({ target_ports: '443' })])).toMatchObject({ valid: true, values: [{ port: 1000, port_end: 1009, target_port: 443 }] })
    expect(parseFirewallForwards([draft({ target_ports: '2000-2008' })])).toMatchObject({ valid: false, errors: [{ target_ports: 'forwardLengthInvalid' }] })
  })

  it('reads old singleton and zero-end documents without inventing intervals', () => {
    expect(firewallForwardDraft({ port: 443, port_end: 0, target_ip: '192.0.2.10', target_port: 8443, target_port_end: 0, network: 'tcp' }, 7))
      .toMatchObject({ id: 7, ports: '443', target_ports: '8443', listen_ip: '' })
    const parsed = parseFirewallForwards([draft({ ports: '443-443', target_ports: '8443-8443' })])
    expect(parsed.valid).toBe(true)
    expect(parsed.values[0]).not.toHaveProperty('port_end')
    expect(parsed.values[0]).not.toHaveProperty('target_port_end')
  })

  it('rejects overlapping wildcard/specific listeners but separates networks and families', () => {
    expect(parseFirewallForwards([draft(), draft({ listen_ip: '192.0.2.20', ports: '1009-1010', target_ports: '443' })]))
      .toMatchObject({ valid: false, errors: [{ overlap: 'forwardOverlap' }, { overlap: 'forwardOverlap' }] })
    expect(parseFirewallForwards([draft({ listen_ip: '192.0.2.20' }), draft({ listen_ip: '192.0.2.21' })]).valid).toBe(true)
    expect(parseFirewallForwards([draft(), draft({ network: 'tcp' }), draft({ target_ip: '2001:db8::10' })]).valid).toBe(true)
    expect(parseFirewallForwards([draft({ listen_ip: '2001:db8::1', target_ip: '2001:db8::10' }), draft({ listen_ip: '2001:0db8:0:0:0:0:0:1', target_ip: '2001:db8::20' })]).valid).toBe(false)
  })

  it('deduplicates identical canonical rules before checking overlap', () => {
    const value = parseFirewallForwards([draft(), draft({ listen_ip: '0.0.0.0' })])
    expect(value.valid).toBe(true)
    expect(value.values).toHaveLength(1)
  })

  it('rejects invalid addresses, family mismatches and malformed port drafts', () => {
    for (const target_ip of ['', 'proxy.example', '::ffff:192.0.2.1', '0:0:0:0:0:ffff:c000:201', 'fe80::1%eth0', '::', '0.0.0.0', '224.0.0.1', 'ff02::1']) {
      expect(parseFirewallForwards([draft({ target_ip })])).toMatchObject({ valid: false, errors: [{ target_ip: 'ipInvalid' }] })
    }
    expect(parseFirewallForwards([draft({ listen_ip: '::' })])).toMatchObject({ valid: false, errors: [{ listen_ip: 'forwardFamilyInvalid' }] })
    expect(parseFirewallForwards([draft({ listen_ip: '224.0.0.1' })]).valid).toBe(false)
    for (const ports of ['0', '65536', '1009-1000', '1000-', '1000,1001']) expect(parseFirewallForwards([draft({ ports })]).valid).toBe(false)
  })

  it('bounds aggregate translated maps after deduplication without capping many-to-one ranges', () => {
    const rows = Array.from({ length: 5 }, (_, index) => draft({ id: index, listen_ip: `192.0.2.${index + 1}`, ports: '1-65535', target_ports: '1-65535' }))
    expect(parseFirewallForwards(rows.slice(0, 4)).valid).toBe(true)
    expect(parseFirewallForwards(rows)).toMatchObject({ valid: false, errors: rows.map(() => ({ ports: 'forwardMapLimit' })) })
    expect(parseFirewallForwards([...rows.slice(0, 4), rows[0]!]).valid).toBe(true)
    expect(parseFirewallForwards(rows.map(row => ({ ...row, target_ports: '443' }))).valid).toBe(true)
  })
})
