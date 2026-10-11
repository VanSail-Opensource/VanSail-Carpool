import type { IPLookupLiveDetails, IPLookupReport } from '@/api/ipLookup'

export function shodanAuditDetails(params: URLSearchParams, original: IPLookupLiveDetails['shodan']): IPLookupLiveDetails['shodan'] {
  const mode = params.get('shodan')
  if (mode === 'legacy') return original
  const result = { ...original, cpes: [] as string[], tags: [] as string[], vulns: [] as string[] }
  if (['tags', 'both', 'partial'].includes(mode ?? '')) result.tags = ['self-signed', 'vpn']
  if (['vulns', 'both'].includes(mode ?? '')) result.vulns = ['CVE-2017-15906', 'CVE-2018-15473']
  if (mode && mode !== 'empty') result.cpes = ['cpe:/a:igor_sysoev:nginx:1.24.0', 'cpe:/a:openbsd:openssh:7.4', `cpe:2.3:a:vendor:${'long_software_identifier_'.repeat(8)}:1.0:*:*:*:*:*:*:*`]
  if (mode === 'many-cpes') result.cpes = Array.from({ length: 300 }, (_, i) => `cpe:/a:vendor:product_${String(i).padStart(3, '0')}:1.0`)
  if (mode === 'partial') { result.status = 'partial'; result.errorCode = 'IP_DETAILS_INVALID_RESPONSE' }
  return result
}

export function decorateShodanAudit(report: IPLookupReport, params: URLSearchParams): void {
  const result = shodanAuditDetails(params, { status: 'success', source: 'shodan:internetdb', observedAt: '', errorCode: '', hostnames: [], ports: [] })
  for (const tag of result.tags ?? []) report.refusals.push({ kind: 'shodan_tag', source: result.source, value: tag })
  for (const vuln of result.vulns ?? []) report.refusals.push({ kind: 'shodan_vuln', source: result.source, value: vuln })
  if ((result.tags?.length ?? 0) + (result.vulns?.length ?? 0) > 0) report.verdict = 'unsuitable'
  if (params.has('shodan')) report.policyVersion = 'residential-v6'
  if (result.status === 'partial') report.status = 'partial'
  if (report.verdict === 'unsuitable' && report.subnet) report.subnet = { ...report.subnet, status: 'skipped', reason: 'ip_not_passed', addresses: [] }
}
