import type { IPLookupReport } from '@/api/ipLookup'
import { sourceText } from './presentation'

const icons: Record<string, string> = { abuse: 'i-ph-shield-warning', datacenter: 'i-ph-buildings', vpn: 'i-ph-lock-key', proxy: 'i-ph-arrows-left-right', tor: 'i-ph-eye-slash', user_count: 'i-ph-users' }

export function refusalGroups(refusals: IPLookupReport['refusals']) {
  const groups = new Map<string, { kind: string; icon: string; items: Array<{ value: string | number | boolean; source: string }> }>()
  const seen = new Set<string>()
  for (const item of refusals) {
    const key = JSON.stringify([item.kind, item.source, item.value])
    if (seen.has(key)) continue
    seen.add(key)
    const group = groups.get(item.kind) ?? { kind: item.kind, icon: icons[item.kind] ?? 'i-ph-warning-circle', items: [] }
    group.items.push({ value: item.value, source: sourceText(item.source) })
    groups.set(item.kind, group)
  }
  return [...groups.values()]
}
