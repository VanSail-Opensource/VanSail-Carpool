export interface CertificateStatus {
  domain: string
  mode: string
  phase: string
  active: boolean | null
  expiring: boolean
  sans: string[]
  issuer: string
  expires: string
  notBefore: string
  fingerprint: string
  acquiredFingerprint: string
  errorCode: string
}

export function readCertificateStatus(value: unknown): CertificateStatus | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const row = value as Record<string, unknown>
  const text = (key: string, maximum = 1024) => typeof row[key] === 'string' ? (row[key] as string).slice(0, maximum) : ''
  const time = (key: string) => { const value = text(key); return Number.isFinite(Date.parse(value)) && !value.startsWith('0001-') ? value : '' }
  const fingerprint = (key: string) => /^[a-f0-9]{64}$/i.test(text(key)) ? text(key) : ''
  return { domain: text('domain', 253), mode: text('mode', 16), phase: ['idle', 'issuing', 'renewing', 'applying', 'failed'].includes(text('phase')) ? text('phase') : '',
    active: typeof row.active === 'boolean' ? row.active : null, expiring: row.expiring === true, sans: Array.isArray(row.sans) ? row.sans.filter((item): item is string => typeof item === 'string').slice(0, 128).map(value => value.slice(0, 253)) : [],
    issuer: text('issuer'), expires: time('expires'), notBefore: time('not_before'), fingerprint: fingerprint('fingerprint'), acquiredFingerprint: fingerprint('acquired_fingerprint'),
    errorCode: /^[A-Z][A-Z0-9_]{0,127}$/.test(text('error_code')) ? text('error_code') : '' }
}
