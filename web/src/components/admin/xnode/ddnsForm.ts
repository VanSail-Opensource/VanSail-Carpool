import { z } from 'zod'
import type { XNodeEndpoint, XNodeSettings } from '@/api/xnode'

export const builtinDDNSProvider = 'builtin:panel'
const jobSchema = z.object({
  id: z.string().regex(/^(?!_xnode_)[A-Za-z0-9._-]{1,128}$/),
  provider: z.string().min(1), zone: z.string(), name: z.string(),
  interval_seconds: z.number().int().min(0).max(86400).optional(),
  ttl_seconds: z.number().int().min(0).optional(),
})

function discoverable(value: string | undefined, nic: string | undefined, lookup: string | undefined): boolean {
  if (value !== 'auto' || nic?.trim()) return true
  try { const url = new globalThis.URL(lookup ?? ''); return url.protocol === 'https:' && !url.username && !url.password } catch { return false }
}

export function validDDNSSettings(settings: XNodeSettings, hosts: readonly XNodeEndpoint[]): boolean {
  const ids = new Set<string>()
  const used = new Set<string>()
  const available = new Set(hosts.map(host => host.id))
  for (const job of settings.ddns ?? []) {
    if (!jobSchema.safeParse(job).success || ids.has(job.id)) return false
    ids.add(job.id)
    const builtin = job.provider === builtinDDNSProvider
    if (!builtin && (!job.zone.trim() || !job.name.trim())) return false
    if (builtin && (!job.ipv4 && !job.ipv6 || job.ipv4 && job.ipv4 !== 'auto' || job.ipv6 && job.ipv6 !== 'auto')) return false
    if (!discoverable(job.ipv4, job.interface, job.ipv4_url) || !discoverable(job.ipv6, job.interface, job.ipv6_url)) return false
    const binding = settings.ddnsBindings?.[job.id]
    for (const [source, id] of [[job.ipv4, binding?.ipv4HostId], [job.ipv6, binding?.ipv6HostId]]) {
      if (builtin && source && !id || id && (!source || !available.has(id) || used.has(id))) return false
      if (id) used.add(id)
    }
  }
  return true
}
