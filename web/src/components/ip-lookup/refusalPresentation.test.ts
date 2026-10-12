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
})
