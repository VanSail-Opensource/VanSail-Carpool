import type { IPLookupReport } from '@/api/ipLookup'

/** Construct backend policy outcomes for visual audits without collection. */
export function decorateRatingAudit(report: IPLookupReport, params: URLSearchParams) {
  const requested = params.get('rating') as IPLookupReport['rating']
  report.policyVersion = 'ai-suitability-v2'
  report.warnings ??= []
  if (requested === 'average') {
    report.facts.networkType = 'datacenter'
    report.warnings.push({ kind: 'datacenter', source: 'ipapi', value: 'Hosting provider' })
  }
  if (requested === 'good') report.maxmind.user_type = 'business'
  if (requested === 'poor') report.refusals.push({ kind: 'proxy', source: 'ipapi', value: 'Proxy provider' })
  report.rating = report.refusals.some(item => item.kind !== 'datacenter') ? 'poor'
    : report.status === 'partial' || params.get('live') === 'legacy' ? 'incomplete'
      : requested ?? 'excellent'
}
