import { t } from '@/i18n'
import { sourceText } from './presentation'

const icons: Record<string, string> = { abuse: 'i-ph-shield-warning', abuse_list: 'i-ph-shield-warning', proxy_list: 'i-ph-arrows-left-right', proxy_database: 'i-ph-database', datacenter: 'i-ph-buildings', vpn: 'i-ph-lock-key', proxy: 'i-ph-arrows-left-right', tor: 'i-ph-eye-slash', user_count: 'i-ph-users' }

export type DisplayEvidence = { kind: string; source: string; value: string | number | boolean }

function evidenceSource(item: DisplayEvidence): string {
  if (item.kind === 'port_observation') return item.source.split(',').map(source => {
    const [protocol, confidence] = source.split(':')
    const label = t(`ipLookup.protocols.${protocol}`)
    return confidence ? t('ipLookup.portService', { protocol: label, confidence: t(`ipLookup.confidence.${confidence}`) }) : label
  }).join(', ')
  if (item.kind === 'active_proxy') return item.source.split(',').map(protocol => t(`ipLookup.protocols.${protocol.trim()}`)).join(', ')
  return sourceText(item.source)
}

export function refusalGroups(refusals: DisplayEvidence[]) {
  const groups = new Map<string, { kind: string; icon: string; items: Array<{ value: string | number | boolean; source: string }> }>()
  const seen = new Set<string>()
  for (const item of refusals) {
    const key = JSON.stringify([item.kind, item.source, item.value])
    if (seen.has(key)) continue
    seen.add(key)
    const group = groups.get(item.kind) ?? { kind: item.kind, icon: icons[item.kind] ?? 'i-ph-warning-circle', items: [] }
    const source = evidenceSource(item)
    const existing = ['port_observation', 'active_proxy', 'cloudflare_cdn'].includes(item.kind) ? group.items.find(row => row.value === item.value) : undefined
    if (existing) existing.source = [...new Set([...existing.source.split(', '), ...source.split(', ')])].join(', ')
    else group.items.push({ value: item.value, source })
    groups.set(item.kind, group)
  }
  return [...groups.values()]
}
