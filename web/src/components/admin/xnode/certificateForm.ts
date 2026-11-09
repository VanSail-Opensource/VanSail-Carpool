import { z } from 'zod'
import type { XNodeGlobal, XNodeSettings } from '@/api/xnode'

export type CertificateProfile = NonNullable<XNodeSettings['certificates']>[string]
export type CertificateProviders = XNodeGlobal['providers']
export type CertificateErrors = Partial<Record<keyof CertificateProfile['config'] | 'provider', string>>
export const certificateModes = ['http', 'dns', 'file', 'content', 'self'] as const
export const maxCertificatePEMBytes = 1024 * 1024
const domain = z.string().max(253).regex(/^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i)
const address = z.union([z.ipv4(), z.ipv6()])
const path = z.string().regex(/^\/[^\0\r\n]+$/)

function pem(value: string | undefined, kind: 'certificate' | 'key'): boolean {
  if (!value || new TextEncoder().encode(value).length > maxCertificatePEMBytes) return false
  const pattern = kind === 'certificate' ? /-----BEGIN CERTIFICATE-----\s+[A-Za-z0-9+/=\s]+-----END CERTIFICATE-----/
    : /-----BEGIN (PRIVATE KEY|RSA PRIVATE KEY|EC PRIVATE KEY)-----\s+[A-Za-z0-9+/=\s]+-----END \1-----/
  return pattern.test(value)
}

export function certificateErrors(profile: CertificateProfile, providers: CertificateProviders): CertificateErrors {
  const errors: CertificateErrors = {}, config = profile.config
  const mode = config.cert_mode
  if (!certificateModes.some(value => value === mode)) errors.cert_mode = 'modeInvalid'
  const name = config.domain?.trim() ?? ''
  if (mode !== 'file') {
    const valid = mode === 'dns' ? domain.safeParse(name.replace(/^\*\./, '')).success
      : mode === 'http' ? domain.safeParse(name).success : domain.safeParse(name).success || address.safeParse(name).success || name === 'localhost'
    if (!valid) errors.domain = 'domainInvalid'
  }
  if (['http', 'dns'].includes(mode) && config.email?.trim() && !z.email().safeParse(config.email.trim()).success) errors.email = 'emailInvalid'
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
