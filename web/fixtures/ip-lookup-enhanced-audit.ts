import type { IPLookupReport, IPLookupSharedProgress } from '@/api/ipLookup'
import { ApiError } from '@/api/http'

export function installEnhancedIPAudit(params: URLSearchParams) {
  const mode = params.get('enhanced') ?? ''
  let active = params.has('checking')
  let activeIP = '8.8.8.8'
  const task = (key: string, status = 'success', milestone = 0) => ({ key, status: status as 'success' | 'active', startedAt: new Date(Date.now() - 170000).toISOString(), nextMilestone: milestone, errorCode: '' })
  function ports(): NonNullable<IPLookupReport['ports']> {
    return [{ port: 443, status: active ? 'processing' : 'success', tasks: [task('tcp'), task('tls'), task('trojan_short'), task('trojan_incomplete', active ? 'active' : 'success', active ? 595 : 0)], findings: [{ protocol: 'reality', confidence: 'suspected', evidence: 'feasible_tls_dns_mismatch' }] }, { port: 4430, status: 'closed', tasks: [task('tcp')], findings: [] }, { port: 8443, status: 'success', tasks: [task('tcp'), task('trojan_overbuffer')], findings: mode === 'risk' ? [{ protocol: 'socks5', confidence: 'confirmed', evidence: 'authentication_required' }] : [] }]
  }
  function subnet(ip: string): NonNullable<IPLookupReport['subnet']> {
    if (mode === 'skipped' || mode === 'risk') return { prefix: '8.8.8.0/24', status: 'skipped', reason: 'ip_not_passed', addresses: [] }
    if (ip.includes(':')) return { prefix: '2606:4700:4700::/64', status: 'success', reason: 'ipv6', addresses: [] }
    const prefix = ip.split('.').slice(0, 3).join('.')
    return { prefix: `${prefix}.0/24`, status: mode === 'partial' ? 'partial' : 'success', reason: '', addresses: Array.from({ length: 256 }, (_, last) => ({ ip: `${prefix}.${last}`, status: last % 17 === 0 ? 'flagged' as const : mode === 'partial' && last % 7 === 0 ? 'unknown' as const : 'clean' as const, proxy: last % 17 === 0 ? 'flagged' : 'success', abuse: last % 17 === 0 ? 'skipped' : 'success', ipapi: last % 17 === 0 ? 'skipped' : mode === 'partial' && last % 7 === 0 ? 'error' : 'success', sources: last % 17 === 0 ? ['named-source'] : [] })) }
  }
  function decorate(report: IPLookupReport): IPLookupReport {
    if (!mode) return report
    report.proxy = { status: mode === 'partial' ? 'partial' : 'success', matches: mode === 'risk' ? ['named-source'] : [], sources: [{ id: 'named-source', name: 'Community proxy inventory', status: 'success', observedAt: new Date().toISOString(), count: 24018 }] }
    report.ports = ports()
    report.subnet = subnet(report.ip)
    if (mode === 'risk') { report.verdict = 'unsuitable'; report.refusals.push({ kind: 'active_proxy', source: 'socks5', value: 8443 }) }
    return report
  }
  async function progress(ip: string): Promise<IPLookupSharedProgress | null> {
    if (params.has('progress-error')) throw new ApiError(502, { code: 'IP_LOOKUP_FAILED', message: 'Constructed progress outage' })
    return active && ip === activeIP ? { ip, status: 'processing', progress: { phase: 'probes', completed: 8, total: 11, currentProvider: '443:trojan_incomplete', tasks: [{ key: 'proxy', status: 'success' }, { key: 'probes', status: 'active' }, { key: 'subnet', status: 'pending' }] }, ports: ports(), subnet: null, updatedAt: new Date().toISOString() } : null
  }
  const controls = { start: (ip: string) => { active = true; activeIP = ip }, finish: () => { active = false }, progress, decorate }
  ;(window as unknown as { __ipEnhancedAudit: typeof controls }).__ipEnhancedAudit = controls
  return controls
}
