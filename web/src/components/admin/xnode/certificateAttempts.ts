import { z } from 'zod'
import { createUuid } from '@/utils/browserCompatibility'

const prefix = 'txc-certificate-attempt:'
const schema = z.object({ key: z.uuid(), action: z.enum(['issue', 'renew']), expectedRevision: z.number().int().positive().safe() })
export type CertificateAttempt = z.infer<typeof schema>
export type CertificateAction = CertificateAttempt['action']
const storageKey = (scope: string) => prefix + encodeURIComponent(scope)

export function loadCertificateAttempt(scope: string): CertificateAttempt | null {
  try {
    const value = schema.safeParse(JSON.parse(globalThis.sessionStorage?.getItem(storageKey(scope)) ?? 'null'))
    return value.success ? value.data : null
  } catch { return null }
}
export function saveCertificateAttempt(scope: string, attempt: CertificateAttempt | null): void {
  try {
    if (attempt) globalThis.sessionStorage?.setItem(storageKey(scope), JSON.stringify(attempt))
    else globalThis.sessionStorage?.removeItem(storageKey(scope))
  } catch { /* Recovery still uses the server's durable profile receipt. */ }
}
export function createCertificateAttempt(action: CertificateAction, expectedRevision: number): CertificateAttempt {
  return { key: createUuid(), action, expectedRevision }
}
export function clearCertificateAttempts(): void {
  try {
    const storage = globalThis.sessionStorage
    for (const key of Object.keys(storage ?? {})) if (key.startsWith(prefix)) storage.removeItem(key)
  } catch { /* Browser storage may be unavailable. */ }
}
if (typeof window !== 'undefined') {
  window.addEventListener('txc:session-cleared', clearCertificateAttempts)
  window.addEventListener('txc:browser-session-expired', clearCertificateAttempts)
}
