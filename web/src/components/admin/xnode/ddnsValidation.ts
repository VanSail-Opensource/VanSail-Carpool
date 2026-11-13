import { z } from 'zod'
import type { XNodeSettings } from '@/api/xnode'

export const builtinDDNSProvider = 'builtin:panel'
export const maxDDNSTTLSeconds = 2147483647
export type DDNSJob = NonNullable<XNodeSettings['ddns']>[number]
export type DDNSErrors = Partial<Record<keyof DDNSJob | 'source', string>>
const identity = z.string().regex(/^(?!_xnode_)[A-Za-z0-9._-]{1,128}$/)
const controls = (value: string) => [...value].some(character => { const code = character.charCodeAt(0); return code < 32 || code >= 127 && code <= 159 })

// DDNS owner names support underscores. Certificate hostnames use a stricter policy.
function domain(value: string, wildcard = false): string | null {
  if (!value || controls(value) || /[\s\\/:?#@%[\]]/u.test(value)) return null
  const normalized = value.replace(/[\u3002\uff0e\uff61]/g, '.')
  const labels = (normalized.endsWith('.') ? normalized.slice(0, -1) : normalized).toLowerCase().split('.')
  for (let index = 0; index < labels.length; index++) {
    let label = labels[index]!
    if (label === '*' && wildcard && index === 0 && labels.length > 1) continue
    if (!label.includes('_') && [...label].some(character => character.charCodeAt(0) >= 128)) {
      try { label = new URL(`https://${label}.invalid`).hostname.slice(0, -8) } catch { return null }
    }
    if (!/^[a-z0-9_-]{1,63}$/i.test(label)) return null
    labels[index] = label
  }
  const name = labels.join('.')
  return name.length <= 253 ? name : null
}
function lookup(value: string): boolean {
  if (!value || controls(value) || /[\s\\#]/u.test(value) || !/^https:\/\//i.test(value)) return false
  try {
    const url = new URL(value)
    return !!url.hostname && !(value.split('/')[2] ?? '').includes('@') && !url.username && !url.password && url.port !== '0' && !/:$/.test(value.split('/')[2] ?? '') && !controls(decodeURIComponent(url.pathname + url.search))
  } catch { return false }
}
function ipv4(value: string): boolean {
  const bytes = value.split('.').map(Number)
  return value !== '0.0.0.0' && value !== '255.255.255.255' && bytes[0] !== 127 && !(bytes[0]! >= 224 && bytes[0]! <= 239) && !(bytes[0] === 169 && bytes[1] === 254)
}
function address(value: string, family: 'ipv4' | 'ipv6'): boolean {
  if (z.ipv4().safeParse(value).success) return family === 'ipv4' && ipv4(value)
  if (!value.includes(':') || /[^0-9a-f:.]/i.test(value)) return false
  let normalized: string
  try { normalized = new URL(`https://[${value}]`).hostname.slice(1, -1) } catch { return false }
  const mapped = /^::ffff:([0-9a-f]+):([0-9a-f]+)$/.exec(normalized)
  if (mapped) {
    const high = parseInt(mapped[1]!, 16), low = parseInt(mapped[2]!, 16)
    return family === 'ipv4' && ipv4([high >>> 8, high & 255, low >>> 8, low & 255].join('.'))
  }
  return family === 'ipv6' && normalized !== '::' && normalized !== '::1' && !/^ff/i.test(normalized) && !/^fe[89ab]/i.test(normalized)
}
export function ddnsErrors(job: DDNSJob): DDNSErrors {
  const errors: DDNSErrors = {}
  const builtin = job.provider === builtinDDNSProvider
  if (!identity.safeParse(job.id).success) errors.id = 'ddnsIdInvalid'
  if (!job.provider.trim()) errors.provider = 'ddnsProviderInvalid'
  if (!z.number().int().min(0).max(86400).optional().safeParse(job.interval_seconds).success) errors.interval_seconds = 'ddnsIntervalInvalid'
  if (!z.number().int().min(0).max(maxDDNSTTLSeconds).optional().safeParse(job.ttl_seconds).success) errors.ttl_seconds = 'ddnsTTLInvalid'
  if (!builtin) {
    const zone = domain(job.zone), name = job.name === '@' ? zone : domain(job.name, true)
    if (!zone) errors.zone = 'ddnsZoneInvalid'
    if (!name || zone && name !== zone && !name.endsWith(`.${zone}`)) errors.name = 'ddnsNameInvalid'
  }
  if (!job.ipv4 && !job.ipv6) errors.source = 'ddnsSourceRequired'
  if (job.interface && (controls(job.interface) || /[\s/\\]/u.test(job.interface))) errors.interface = 'ddnsInterfaceInvalid'
  for (const family of ['ipv4', 'ipv6'] as const) {
    const source = job[family], urlKey = `${family}_url` as const, url = job[urlKey]
    if (url && !lookup(url)) errors[urlKey] = 'ddnsLookupInvalid'
    if (!source) continue
    if (source === 'auto') { if (!job.interface && !url) errors[urlKey] = 'ddnsDiscoveryRequired' }
    else if (builtin || !address(source, family)) errors[family] = 'ddnsAddressInvalid'
  }
  return errors
}
