import type { IPLookupReport } from '@/api/ipLookup'
import { t } from '@/i18n'
import { sourceText } from './presentation'

export function reportRows(report: IPLookupReport) {
  const facts = report.facts
  const rows = [
    { key: 'networkType', value: facts.networkType ? t(`ipLookup.networkTypes.${facts.networkType}`) : null, icon: 'i-ph-network' },
    { key: 'country', value: facts.country, icon: 'i-ph-globe' },
    { key: 'city', value: facts.city, icon: 'i-ph-map-pin' },
    { key: 'asn', value: facts.asn ? `AS${facts.asn.replace(/^AS/i, '')}` : null, icon: 'i-ph-hash' },
    { key: 'asnName', value: facts.asnName, icon: 'i-ph-buildings' },
    ...(['ip_risk_snapshot', 'static_ip_score', 'user_count', 'user_type'] as const).map(key => ({ key: `maxmind.${key}`, value: report.maxmind[key], icon: key === 'user_count' ? 'i-ph-users' : key === 'user_type' ? 'i-ph-user' : 'i-ph-chart-line' })),
  ]
  return rows.filter(row => row.value !== null && row.value !== undefined && row.value !== '').map(row => ({
    ...row, label: t(`ipLookup.facts.${row.key}`), source: sourceText(report.sources[row.key]),
  }))
}
