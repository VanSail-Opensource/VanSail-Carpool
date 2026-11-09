import { describe, expect, it } from 'vitest'
import { readCertificateStatus } from './certificateStatus'

describe('safe certificate status projection', () => {
  it('shows a failed attempt separately from the still-serving certificate', () => {
    const status = readCertificateStatus({ phase: 'failed', active: true, domain: 'proxy.example', fingerprint: 'ab'.repeat(32), acquired_fingerprint: 'cd'.repeat(32), error_code: 'CERTIFICATE_ACTIVATION_FAILED', expires: '2027-01-01T00:00:00Z', key_content: 'private-secret', dns_env: { token: 'provider-secret' }, error: 'raw provider credential' })
    expect(status).toMatchObject({ phase: 'failed', active: true, fingerprint: 'ab'.repeat(32), acquiredFingerprint: 'cd'.repeat(32), errorCode: 'CERTIFICATE_ACTIVATION_FAILED' })
    expect(JSON.stringify(status)).not.toContain('secret')
    expect(JSON.stringify(status)).not.toContain('raw provider')
  })

  it('keeps legacy serving state unknown and bounds malformed telemetry', () => {
    expect(readCertificateStatus(null)).toBeNull()
    expect(readCertificateStatus({ domain: 'proxy.example' })?.active).toBeNull()
    const status = readCertificateStatus({ phase: 'untrusted', error_code: 'raw secret', fingerprint: 'invalid', not_before: 'invalid', expires: '0001-01-01T00:00:00Z', sans: Array.from({ length: 300 }, () => 'x'.repeat(1000)) })!
    expect(status).toMatchObject({ phase: '', errorCode: '', fingerprint: '', notBefore: '', expires: '' })
    expect(status.sans).toHaveLength(128)
    expect(status.sans[0]).toHaveLength(253)
  })
})
