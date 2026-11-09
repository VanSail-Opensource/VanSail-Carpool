import { z } from 'zod'
import type { XNodeGlobal, XNodeSettings } from '@/api/xnode'

export type CertificateProfile = NonNullable<XNodeSettings['certificates']>[string]
export type CertificateProviders = XNodeGlobal['providers']
export type CertificateErrors = Partial<Record<keyof CertificateProfile['config'] | 'provider', string>>
export const certificateModes = ['http', 'dns', 'file', 'content', 'self'] as const
export const maxCertificatePEMBytes = 1024 * 1024
export const maxCertificatePathLength = 4096
const address = z.union([z.ipv4(), z.ipv6()])
const path = z.string().max(maxCertificatePathLength).refine(value => value.trim().length > 0 && !/[\0\r\n]/.test(value))

export function certificateMode(config: CertificateProfile['config']): string {
  const explicit = config.cert_mode?.trim().toLowerCase()
  if (explicit) return explicit
  if (config.auto_tls) return 'http'
  if (config.cert_content && config.key_content) return 'content'
  if (config.cert_file && config.key_file) return 'file'
  return 'none'
}
function validDomain(value: string, wildcard = false): boolean {
  let name = value.endsWith('.') ? value.slice(0, -1) : value
  if (wildcard && name.startsWith('*.')) name = name.slice(2)
  const controls = [...name].some(character => { const code = character.charCodeAt(0); return code < 32 || code >= 127 && code <= 159 })
  if (!name || controls || /[\s\\/:?#@%]/u.test(name) || name.includes('[') || name.includes(']')) return false
  const validASCII = (host: string) => host.length <= 253 && host.split('.').every(label => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label))
  // Numeric ASCII labels are native-valid DNS names; URL may reinterpret them
  // as IPv4. Only IDNA conversion needs the platform hostname parser.
  if ([...name].every(character => character.charCodeAt(0) < 128)) return validASCII(name)
  try { return validASCII(new URL(`https://${name}`).hostname) } catch { return false }
}

function pem(value: string | undefined, kind: 'certificate' | 'key'): boolean {
  if (!value || new TextEncoder().encode(value).length > maxCertificatePEMBytes) return false
  const pattern = kind === 'certificate' ? /-----BEGIN CERTIFICATE-----\s+[A-Za-z0-9+/=\s]+-----END CERTIFICATE-----/
    : /-----BEGIN (PRIVATE KEY|RSA PRIVATE KEY|EC PRIVATE KEY)-----\s+[A-Za-z0-9+/=\s]+-----END \1-----/
  return pattern.test(value)
}

export function certificateErrors(profile: CertificateProfile, providers: CertificateProviders): CertificateErrors {
  const errors: CertificateErrors = {}, config = profile.config
  const mode = certificateMode(config)
  if (!certificateModes.some(value => value === mode)) errors.cert_mode = 'modeInvalid'
  const name = config.domain ?? ''
  if (['http', 'dns'].includes(mode) && !validDomain(name, mode === 'dns')) errors.domain = 'domainInvalid'
  if (mode === 'self' && !name.trim()) errors.domain = 'domainInvalid'
  if (mode === 'content' && name && !validDomain(name, true) && !address.safeParse(name.startsWith('[') && name.endsWith(']') ? name.slice(1, -1) : name).success) errors.domain = 'domainInvalid'
  if (['http', 'dns'].includes(mode) && config.email && !z.email().safeParse(config.email).success) errors.email = 'emailInvalid'
  if (mode === 'dns' && (profile.provider ? !providers[profile.provider] : !config.dns_provider)) errors.provider = 'providerInvalid'
  if (mode === 'file') {
    if (!path.safeParse(config.cert_file).success) errors.cert_file = 'pathInvalid'
    if (!path.safeParse(config.key_file).success) errors.key_file = 'pathInvalid'
  }
  if (mode === 'content') {
    if (!pem(config.cert_content, 'certificate')) errors.cert_content = 'certificatePEMInvalid'
    if (!pem(config.key_content, 'key')) errors.key_content = 'keyPEMInvalid'
  }
  return errors
}

export function certificateBindingsValid(settings: XNodeSettings, inbounds: readonly string[]): boolean {
  return Object.entries(settings.inboundCertificates ?? {}).every(([tag, id]) => inbounds.includes(tag) && !!settings.certificates?.[id])
}

export function certificateBoundInbounds(settings: XNodeSettings, id: string): string[] {
  return Object.entries(settings.inboundCertificates ?? {}).filter(([, profile]) => profile === id).map(([tag]) => tag)
}
