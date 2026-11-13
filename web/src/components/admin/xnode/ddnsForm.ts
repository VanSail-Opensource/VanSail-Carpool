import type { XNodeEndpoint, XNodeSettings } from '@/api/xnode'
import { builtinDDNSProvider, ddnsErrors } from './ddnsValidation'
export { builtinDDNSProvider } from './ddnsValidation'

export function validDDNSSettings(settings: XNodeSettings, hosts: readonly XNodeEndpoint[]): boolean {
  const ids = new Set<string>()
  const used = new Set<string>()
  const available = new Set(hosts.map(host => host.id))
  for (const job of settings.ddns ?? []) {
    if (Object.keys(ddnsErrors(job)).length > 0 || ids.has(job.id)) return false
    ids.add(job.id)
    const builtin = job.provider === builtinDDNSProvider
    const binding = settings.ddnsBindings?.[job.id]
    for (const [source, hostIds] of [[job.ipv4, binding?.ipv4HostIds ?? []], [job.ipv6, binding?.ipv6HostIds ?? []]] as const) {
      if (!builtin && hostIds.length > 1) return false
      if (builtin && source && !hostIds.length || !source && hostIds.length) return false
      for (const id of hostIds) {
        if (!available.has(id) || used.has(id)) return false
        used.add(id)
      }
    }
  }
  return true
}
