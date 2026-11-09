import { describe, expect, it } from 'vitest'
import { certificateBindingsValid, certificateBoundInbounds, certificateErrors, maxCertificatePEMBytes } from './certificateForm'

const providers = { managed: { type: 'cloudflare', credentials: {} } }
const certificate = '-----BEGIN CERTIFICATE-----\nQUJD\n-----END CERTIFICATE-----'
const privateKey = '-----BEGIN PRIVATE KEY-----\nQUJD\n-----END PRIVATE KEY-----'

describe('certificate mode validation', () => {
  it('validates native ACME identities without making optional email mandatory', () => {
    expect(certificateErrors({ config: { cert_mode: 'http', domain: 'proxy.example' } }, providers)).toEqual({})
    expect(certificateErrors({ config: { cert_mode: 'http', domain: '*.proxy.example' } }, providers)).toHaveProperty('domain')
    expect(certificateErrors({ config: { cert_mode: 'http', domain: 'proxy.example', email: 'invalid' } }, providers)).toHaveProperty('email')
    expect(certificateErrors({ provider: 'managed', config: { cert_mode: 'dns', domain: '*.proxy.example', email: 'operator@proxy.example' } }, providers)).toEqual({})
    expect(certificateErrors({ provider: 'missing', config: { cert_mode: 'dns', domain: 'proxy.example' } }, providers)).toHaveProperty('provider')
  })

  it('retains existing inline DNS credentials without exposing or rewriting them', () => {
    const profile = { config: { cert_mode: 'dns', domain: 'proxy.example', dns_provider: 'cloudflare', dns_env: { CLOUDFLARE_DNS_API_TOKEN: 'retained-secret' } } }
    expect(certificateErrors(profile, {})).toEqual({})
    expect(profile.config.dns_env.CLOUDFLARE_DNS_API_TOKEN).toBe('retained-secret')
  })

  it('requires node paths only for file profiles and bounded PEM envelopes for supplied material', () => {
    expect(certificateErrors({ config: { cert_mode: 'file', cert_file: '/etc/node/cert.pem', key_file: '/etc/node/key.pem' } }, {})).toEqual({})
    expect(certificateErrors({ config: { cert_mode: 'file', cert_file: 'relative.pem', key_file: '' } }, {})).toHaveProperty('cert_file')
    expect(certificateErrors({ config: { cert_mode: 'content', domain: 'proxy.example', cert_content: certificate, key_content: privateKey } }, {})).toEqual({})
    expect(certificateErrors({ config: { cert_mode: 'content', domain: 'proxy.example', cert_content: certificate, key_content: 'invalid' } }, {})).toHaveProperty('key_content')
    expect(certificateErrors({ config: { cert_mode: 'content', domain: 'proxy.example', cert_content: 'a'.repeat(maxCertificatePEMBytes + 1), key_content: privateKey } }, {})).toHaveProperty('cert_content')
    expect(certificateErrors({ config: { cert_mode: 'self', domain: 'localhost' } }, {})).toEqual({})
  })

  it('preserves explicit inbound references and identifies profiles that cannot be removed', () => {
    const settings = { certificates: { primary: { config: { cert_mode: 'self', domain: 'proxy.example' } } }, inboundCertificates: { paid: 'primary', alternate: 'primary' } }
    expect(certificateBoundInbounds(settings, 'primary')).toEqual(['paid', 'alternate'])
    expect(certificateBindingsValid(settings, ['paid', 'alternate'])).toBe(true)
    expect(certificateBindingsValid(settings, ['paid'])).toBe(false)
    expect(certificateBindingsValid({ ...settings, certificates: {} }, ['paid', 'alternate'])).toBe(false)
  })
})
