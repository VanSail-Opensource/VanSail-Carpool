import { describe, expect, it } from 'vitest'
import { portWarnings, visibleProbePorts } from './probePresentation'

describe('compact port observations', () => {
  it('hides closed and unsupported ports while retaining plain TCP observations', () => {
    expect(visibleProbePorts([
      { port: 443, status: 'closed', tasks: [{ key: 'tcp', status: 'success', startedAt: '', nextMilestone: 0, errorCode: '' }], findings: [] },
      { port: 4430, status: 'error', tasks: [], findings: [] },
      { port: 8443, status: 'success', tasks: [{ key: 'tcp', status: 'success', startedAt: '', nextMilestone: 0, errorCode: '' }], findings: [] },
    ])).toEqual([{ port: 8443, findings: [{ protocol: 'tcp', confidence: 'observed', evidence: '' }] }])
  })

  it('retains confirmed findings and deduplicates profile observations', () => {
    const rows = visibleProbePorts([{ port: 443, status: 'success', tasks: [], findings: [
      { protocol: 'trojan', confidence: 'suspected', evidence: 'one' },
      { protocol: 'trojan', confidence: 'suspected', evidence: 'two' },
      { protocol: 'socks5', confidence: 'confirmed', evidence: 'response' },
      { protocol: 'reality', confidence: 'unavailable', evidence: 'dns_unavailable' },
    ] }])
    expect(rows[0]?.findings.map(finding => finding.protocol)).toEqual(['trojan', 'socks5'])
  })

  it('shows observed and suspected items as warnings while keeping confirmed findings in failures', () => {
    const warnings = portWarnings([{ port: 443, status: 'success', tasks: [], findings: [
      { protocol: 'tls', confidence: 'observed', evidence: 'handshake' },
      { protocol: 'trojan', confidence: 'suspected', evidence: 'two_profiles' },
      { protocol: 'socks5', confidence: 'confirmed', evidence: 'response' },
    ] }])
    expect(warnings.map(item => item.source)).toEqual(['tls', 'trojan'])
    expect(warnings.every(item => item.kind === 'port_observation')).toBe(true)
  })
})
