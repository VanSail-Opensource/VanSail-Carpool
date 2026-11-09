import { describe, expect, it } from 'vitest'
import { certificateBindingsValid, certificateBoundInbounds, certificateErrors, certificateMode, maxCertificatePathLength, maxCertificatePEMBytes } from './certificateForm'

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
  it('derives native modes with full-pair precedence while preserving the retained config', () => {
    const file = { cert_mode: '', cert_file: './cert.pem', key_file: '../key.pem' }
    expect(certificateMode(file)).toBe('file')
    expect(certificateErrors({ config: file }, {})).toEqual({})
    expect(file.cert_mode).toBe('')
    expect(certificateMode({ ...file, cert_content: certificate })).toBe('file')
    expect(certificateErrors({ config: { ...file, cert_content: certificate } }, {})).toEqual({})
    expect(certificateMode({ ...file, cert_content: certificate, key_content: privateKey })).toBe('content')
    expect(certificateMode({ ...file, cert_content: certificate, key_content: privateKey, auto_tls: true })).toBe('http')
    const explicit = { ...file, cert_mode: ' DNS ', auto_tls: true }
    expect(certificateMode(explicit)).toBe('dns')
    expect(explicit.cert_mode).toBe(' DNS ')
    expect(certificateMode({ ...file, cert_mode: '   ' })).toBe('file')
    expect(certificateMode({ cert_mode: '', cert_file: './only-one.pem' })).toBe('none')
    expect(certificateErrors({ config: { cert_mode: '', cert_file: './only-one.pem' } }, {})).toEqual({ cert_mode: 'modeInvalid' })
  })
  it('allows absent/empty content names and nonblank self names without rewriting material', () => {
    const config = { cert_mode: '', cert_content: certificate, key_content: privateKey, email: 'inactive-old-value' }
    expect(certificateErrors({ config }, {})).toEqual({})
    expect(certificateErrors({ config: { ...config, domain: '' } }, {})).toEqual({})
    expect(certificateErrors({ config: { ...config, domain: '*.example.com' } }, {})).toEqual({})
    expect(config).not.toHaveProperty('domain')
    expect(config.cert_mode).toBe('')
    expect(certificateErrors({ config: { cert_mode: 'self', domain: 'Private service name' } }, {})).toEqual({})
    expect(certificateErrors({ config: { cert_mode: 'self', domain: ' \t ' } }, {})).toHaveProperty('domain')
  })
  it.each(['proxy', 'example.com.', 'b\u00fccher.example', '\u4f8b\u5b50.\u6d4b\u8bd5', '09', '1234567890123456789012345'])('accepts native host %j without normalizing stored values', domain => {
    const config = { cert_mode: 'http', domain }
    expect(certificateErrors({ config }, {})).toEqual({})
    expect(certificateErrors({ provider: 'managed', config: { ...config, cert_mode: 'dns', domain: '*.' + domain } }, providers)).toEqual({})
    expect(config.domain).toBe(domain)
  })
  it.each(['https://proxy.example', 'proxy.example:443', 'proxy.example/path', 'proxy.example?query', 'proxy.example#fragment', 'user@proxy.example', 'proxy%2eexample', 'proxy\\example', ' proxy.example', 'proxy.example ', 'proxy\t.example', 'proxy\n.example', 'proxy\0.example', 'proxy\u007f.example', 'proxy..example', '-bad.example', 'bad-.example', 'a'.repeat(64) + '.example'])('rejects malformed ACME host %j', domain => {
    expect(certificateErrors({ config: { cert_mode: 'http', domain } }, {})).toHaveProperty('domain')
  })
  it.each([' operator@proxy.example', 'operator@proxy.example ', '   '])('rejects raw email whitespace %j without rewriting it', email => {
    const config = { cert_mode: 'http', domain: 'proxy.example', email }
    expect(certificateErrors({ config }, {})).toHaveProperty('email')
    expect(config.email).toBe(email)
  })

  it('retains existing inline DNS credentials without exposing or rewriting them', () => {
    const profile = { config: { cert_mode: 'dns', domain: 'proxy.example', dns_provider: 'cloudflare', dns_env: { CLOUDFLARE_DNS_API_TOKEN: 'retained-secret' } } }
    expect(certificateErrors(profile, {})).toEqual({})
    expect(profile.config.dns_env.CLOUDFLARE_DNS_API_TOKEN).toBe('retained-secret')
  })

  it('requires node paths only for file profiles and bounded PEM envelopes for supplied material', () => {
    expect(certificateErrors({ config: { cert_mode: 'file', cert_file: '/etc/node/cert.pem', key_file: '/etc/node/key.pem' } }, {})).toEqual({})
    expect(certificateErrors({ config: { cert_mode: 'file', cert_file: 'relative.pem', key_file: '' } }, {})).toEqual({ key_file: 'pathInvalid' })
    expect(certificateErrors({ config: { cert_mode: 'content', domain: 'proxy.example', cert_content: certificate, key_content: privateKey } }, {})).toEqual({})
    expect(certificateErrors({ config: { cert_mode: 'content', domain: 'proxy.example', cert_content: certificate, key_content: 'invalid' } }, {})).toHaveProperty('key_content')
    expect(certificateErrors({ config: { cert_mode: 'content', domain: 'proxy.example', cert_content: 'a'.repeat(maxCertificatePEMBytes + 1), key_content: privateKey } }, {})).toHaveProperty('cert_content')
    expect(certificateErrors({ config: { cert_mode: 'self', domain: 'localhost' } }, {})).toEqual({})
  })
  it.each(['cert.pem', './tls/server cert.pem', '../shared/cert.pem', '/etc/node/cert.pem', ' tls/cert.pem '])('preserves supported relative and absolute file path %j', cert_file => {
    const profile = { config: { cert_mode: 'file', cert_file, key_file: '../shared/private.key' } }
    expect(certificateErrors(profile, {})).toEqual({})
    expect(profile.config.cert_file).toBe(cert_file)
    expect(profile.config.key_file).toBe('../shared/private.key')
  })
  it.each(['', '   ', '\t', '\n', 'tls/\0cert.pem', 'tls/\rcert.pem', 'tls/\ncert.pem', 'a'.repeat(maxCertificatePathLength + 1)])('rejects blank, control or oversized file path %j', invalid => {
    expect(certificateErrors({ config: { cert_mode: 'file', cert_file: invalid, key_file: invalid } }, {})).toEqual({ cert_file: 'pathInvalid', key_file: 'pathInvalid' })
  })

  it('preserves explicit inbound references and identifies profiles that cannot be removed', () => {
    const settings = { certificates: { primary: { config: { cert_mode: 'self', domain: 'proxy.example' } } }, inboundCertificates: { paid: 'primary', alternate: 'primary' } }
    expect(certificateBoundInbounds(settings, 'primary')).toEqual(['paid', 'alternate'])
    expect(certificateBindingsValid(settings, ['paid', 'alternate'])).toBe(true)
    expect(certificateBindingsValid(settings, ['paid'])).toBe(false)
    expect(certificateBindingsValid({ ...settings, certificates: {} }, ['paid', 'alternate'])).toBe(false)
  })
})
