import { describe, expect, it } from 'vitest'
import { refusalGroups } from './refusalPresentation'

describe('grouped refusal evidence', () => {
  it('deduplicates exact findings while preserving differing sources and values', () => {
    const groups = refusalGroups([{ kind: 'vpn', source: 'ipapi', value: 'Provider A' }, { kind: 'vpn', source: 'ipapi', value: 'Provider A' }, { kind: 'vpn', source: 'scamalytics', value: true }, { kind: 'abuse_reports', source: 'abuseipdb', value: 4 }])
    expect(groups).toHaveLength(2)
    expect(groups[0]!.items).toHaveLength(2)
    expect(groups[1]!.items[0]!.value).toBe(4)
  })
})
