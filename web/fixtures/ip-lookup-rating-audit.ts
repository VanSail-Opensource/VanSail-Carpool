import type { IPLookupReport } from '@/api/ipLookup'

/** Construct backend policy outcomes for visual audits without collection. */
export function decorateRatingAudit(report: IPLookupReport, params: URLSearchParams) {
  const requested = params.get('rating') as IPLookupReport['rating']
  report.policyVersion = 'ai-suitability-v1'
  report.warnings ??= []
  if (!report.proxy) report.proxy = { status: 'success', matches: [], sources: [] }
  if (!report.ports) report.ports = [{ port: 443, status: 'closed', tasks: [], findings: [] }]
  if (requested === 'average') {
    report.facts.networkType = 'datacenter'
    report.warnings.push({ kind: 'datacenter', source: 'ipapi', value: 'Hosting provider' })
  }
  if (requested === 'good') report.maxmind.user_type = 'business'
  if (requested === 'poor') report.refusals.push({ kind: 'proxy_list', source: 'proxy_lists', value: 'Community proxy inventory' })
  if (requested === 'incomplete') report.proxy.status = 'partial'
  report.rating = report.refusals.some(item => item.kind !== 'datacenter') ? 'poor'
    : report.status === 'partial' || report.proxy.status !== 'success' || params.get('live') === 'legacy' ? 'incomplete'
      : requested ?? (report.ports.some(port => port.status !== 'closed') ? 'good' : 'perfect')
}
