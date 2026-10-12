import { describe, expect, it } from 'vitest'
import { refusalGroups } from './refusalPresentation'

describe('grouped refusal evidence', () => {
  it('keeps pool, database and abuse attribution in separate groups', () => {
    const groups = refusalGroups([
      { kind: 'proxy_list', source: 'proxy_lists', value: 'Pool source' },
      { kind: 'proxy_database', source: 'proxy_lists', value: 'Database source' },
      { kind: 'abuse_list', source: 'proxy_lists', value: 'Abuse source' },
    ])
    expect(groups.map(group => group.kind)).toEqual(['proxy_list', 'proxy_database', 'abuse_list'])
    expect(groups.map(group => group.items[0]!.value)).toEqual(['Pool source', 'Database source', 'Abuse source'])
  })
  it('deduplicates exact findings while preserving differing sources and values', () => {
    const groups = refusalGroups([{ kind: 'vpn', source: 'ipapi', value: 'Provider A' }, { kind: 'vpn', source: 'ipapi', value: 'Provider A' }, { kind: 'vpn', source: 'scamalytics', value: true }, { kind: 'abuse_reports', source: 'abuseipdb', value: 4 }])
    expect(groups).toHaveLength(2)
    expect(groups[0]!.items).toHaveLength(2)
    expect(groups[1]!.items[0]!.value).toBe(4)
  })

  it('combines services on the same port while retaining other ports', () => {
    const groups = refusalGroups([
      { kind: 'port_observation', source: 'tls:observed,trojan:suspected', value: 'Port 443' },
      { kind: 'port_observation', source: 'tls:observed', value: 'Port 443' },
      { kind: 'active_proxy', source: 'socks5', value: 443 },
      { kind: 'active_proxy', source: 'http', value: 443 },
      { kind: 'active_proxy', source: 'socks5', value: 8443 },
    ])
    expect(groups[0]!.items).toHaveLength(1)
    expect(groups[0]!.items[0]!.source).toContain(', ')
    expect(groups[1]!.items).toHaveLength(2)
    expect(groups[1]!.items[0]!.source).toBe('SOCKS5, HTTP proxy')
  })
})
