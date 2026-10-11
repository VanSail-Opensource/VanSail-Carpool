import { describe, expect, it } from 'vitest'
import type { IPLookupReport } from '@/api/ipLookup'
import { setLocale } from '@/i18n'
import { ipLocationMap } from './map'
import { reportRows } from './reportPresentation'
import { sourceText } from './presentation'

describe('IP location and compact evidence', () => {
  it('keeps zero coordinates and an ordered valid map extent at world edges', () => {
    for (const [latitude, longitude] of [[0, 0], [90, 180], [-90, -180]]) {
      const map = ipLocationMap(latitude, longitude)!
      const embed = new URL(map.embed)
      const [west, south, east, north] = embed.searchParams.get('bbox')!.split(',').map(Number)
      expect(embed.origin).toBe('https://www.openstreetmap.org')
      expect(west).toBeLessThan(east!)
      expect(south).toBeLessThan(north!)
      expect(embed.searchParams.get('marker')).toBe(`${latitude},${longitude}`)
    }
  })
  it('hides maps with missing, nonfinite or out-of-range coordinates', () => {
    for (const [latitude, longitude] of [[null, 0], [0, null], [NaN, 0], [0, Infinity], [91, 0], [0, -181]]) expect(ipLocationMap(latitude, longitude)).toBeNull()
  })
  it('hides absent data and retains zero scores and counts with their sources', () => {
    setLocale('en')
    const report: IPLookupReport = {
      id: 'report', ip: '8.8.8.8', status: 'succeeded', verdict: 'suitable', reasons: [],
      databases: [{ id: 'maxmind', status: 'success' }], refusals: [], checkedAt: '2026-10-10T00:00:00Z',
      policyVersion: 'residential-v3', parserVersion: 'residential-v3', refundRequired: false,
      facts: { country: '', city: '', latitude: 0, longitude: 0, asn: '', asnName: '', networkType: '' },
      maxmind: { ip_risk_snapshot: 0, static_ip_score: null, user_count: 0, user_type: null },
      sources: { 'maxmind.user_count': 'maxmind' },
    }
    const rows = reportRows(report)
    expect(rows.map(row => row.key)).toEqual(['maxmind.ip_risk_snapshot', 'maxmind.user_count'])
    expect(ipLocationMap(report.facts.latitude, report.facts.longitude)).not.toBeNull()
    expect(rows.find(row => row.key === 'maxmind.user_count')).toMatchObject({ value: 0, source: 'MaxMind' })
    expect(sourceText('ipapi,maxmind')).toBe('IPAPI, MaxMind')
    expect(sourceText('scamalytics:maxmind_geolite2')).toBe('Scamalytics · MaxMind GeoLite2')
  })
})
