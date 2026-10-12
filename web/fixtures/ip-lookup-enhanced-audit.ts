import { ipLookupApi, type IPLookupReport, type IPLookupSharedProgress } from '@/api/ipLookup'
import { ApiError } from '@/api/http'

export function installEnhancedIPAudit(params: URLSearchParams) {
  const mode = params.get('enhanced') ?? ''
  let active = params.has('checking')
  let activeIP = '8.8.8.8'
  const task = (key: string, status = 'success', milestone = 0) => ({ key, status: status as 'success' | 'active', startedAt: new Date(Date.now() - 170000).toISOString(), nextMilestone: milestone, errorCode: '', ...(params.has('trojan-unavailable') && key.startsWith('trojan_') ? { result: 'unavailable' as const } : {}) })
  function ports(): NonNullable<IPLookupReport['ports']> {
    return [{ port: 443, status: active ? 'processing' : 'success', tasks: [task('tcp'), task('tls'), task('trojan_short'), task('trojan_incomplete', active ? 'active' : 'success', active ? 595 : 0)], findings: [{ protocol: mode === 'web-server' ? 'web_server' : 'reality', confidence: mode === 'web-server' ? 'observed' : 'suspected', evidence: mode === 'web-server' ? 'dns_matches_ip' : 'feasible_tls_dns_mismatch', dns: { domain: 'edge.example.net', resolvers: [{ server: '1.1.1.1:53', status: 'success', addresses: [mode === 'web-server' ? '8.8.8.8' : '1.1.1.1'] }, { server: '8.8.8.8:53', status: 'success', addresses: [mode === 'web-server' ? '8.8.8.8' : '1.1.1.1'] }] } }] }, { port: 4430, status: 'closed', tasks: [task('tcp')], findings: [] }, { port: 8443, status: 'success', tasks: [task('tcp'), task('trojan_overbuffer')], findings: mode === 'risk' ? [{ protocol: 'socks5', confidence: 'confirmed', evidence: 'authentication_required' }] : [] }]
  }
  function subnet(ip: string): NonNullable<IPLookupReport['subnet']> {
    if (mode === 'skipped' || mode === 'risk') return { prefix: '8.8.8.0/24', status: 'skipped', reason: 'ip_not_passed', addresses: [] }
    if (ip.includes(':')) return { prefix: '2606:4700:4700::/64', status: 'success', reason: 'ipv6', addresses: [] }
    const prefix = ip.split('.').slice(0, 3).join('.')
    return { prefix: `${prefix}.0/24`, status: mode === 'partial' ? 'partial' : 'success', reason: '', addresses: Array.from({ length: 256 }, (_, last) => ({ ip: `${prefix}.${last}`, status: last % 17 === 0 ? 'flagged' as const : mode === 'partial' && last % 7 === 0 ? 'unknown' as const : 'clean' as const, proxy: last % 17 === 0 ? 'flagged' : 'success', abuse: last % 17 === 0 ? 'skipped' : 'success', ipapi: last % 17 === 0 ? 'skipped' : mode === 'partial' && last % 7 === 0 ? 'error' : 'success', sources: last % 17 === 0 ? ['named-source'] : [] })) }
  }
  function decorate(report: IPLookupReport): IPLookupReport {
    if (!mode) return report
    report.proxy = { status: mode === 'partial' ? 'partial' : 'success', matches: mode === 'risk' ? ['named-source'] : [], sources: [{ id: 'named-source', name: 'Community proxy inventory', status: mode === 'partial' ? 'partial' : 'success', observedAt: new Date().toISOString(), count: 24018, url: 'https://example.net/proxies/list.txt', hosts: 200, processed: 200, skipped: 12, dnsFailed: mode === 'partial' ? 5 : 0, errorCode: mode === 'partial' ? 'IP_DNS_UNAVAILABLE' : '' }] }
    report.ports = ports()
    report.subnet = subnet(report.ip)
    if (mode === 'risk') { report.verdict = 'unsuitable'; report.refusals.push({ kind: 'active_proxy', source: 'socks5', value: 8443 }) }
    return report
  }
  async function progress(ip: string): Promise<IPLookupSharedProgress | null> {
    if (params.has('progress-error')) throw new ApiError(502, { code: 'IP_LOOKUP_FAILED', message: 'Constructed progress outage' })
    return active && ip === activeIP ? { ip, status: 'processing', progress: { phase: 'probes', completed: 8, total: 11, currentProvider: '443:trojan_incomplete', tasks: [{ key: 'proxy', status: 'success' }, { key: 'probes', status: 'active' }, { key: 'subnet', status: 'pending' }] }, ports: ports(), subnet: null, updatedAt: new Date().toISOString() } : null
  }
  let sourcePolls = 0
  let refreshingSource = params.has('source-warming') ? 'named-source' : ''
  ipLookupApi.sourcesStatus = async () => {
    if (params.has('source-status-error')) throw new ApiError(502, { code: 'IP_LOOKUP_FAILED', message: 'Constructed source status outage' })
    if (refreshingSource) sourcePolls++
    const processing = refreshingSource !== '' && sourcePolls < 3
    const failed = params.has('source-partial') && !processing
    const now = new Date().toISOString()
    const sources = ['named-source', 'disabled-source'].map(id => ({ id, name: id === 'named-source' ? 'Community proxy inventory' : 'Disabled inventory', enabled: id === 'named-source', status: processing && refreshingSource === id ? 'processing' : id === 'disabled-source' ? 'disabled' : failed ? 'error' : 'success', refreshing: processing && refreshingSource === id, stale: failed, observedAt: now, count: id === 'named-source' ? 188 : 0, phase: processing ? 'dns' : 'complete', errorCode: failed && id === 'named-source' ? 'IP_PROXY_DOWNLOAD' : '', expiresAt: new Date(Date.now() + 21600000).toISOString(), nextRefreshAt: new Date(Date.now() + 21600000).toISOString() }))
    return { sources, uniqueIPs: 188 }
  }
  ipLookupApi.testSource = async id => {
    if (params.has('source-error')) throw new ApiError(502, { code: 'IP_LOOKUP_FAILED', message: 'Constructed source outage' })
    sourcePolls = 0
    refreshingSource = id
    return { id: 'source-test', status: 'queued', result: { id, name: 'Community proxy inventory', status: 'pending', observedAt: '', count: 0, phase: 'download', hosts: 200, processed: 0 } }
  }
  ipLookupApi.sourceTest = async () => {
    sourcePolls++
    const done = sourcePolls >= 2
    return { id: 'source-test', status: done ? 'completed' : 'processing', result: { id: 'named-source', name: 'Community proxy inventory', status: done ? params.has('source-partial') ? 'partial' : 'success' : 'processing', phase: done ? 'complete' : 'dns', observedAt: done ? new Date().toISOString() : '', count: done ? 188 : 82, url: 'https://example.net/proxies/list.txt', hosts: 200, processed: done ? 200 : 100, skipped: 12, dnsFailed: params.has('source-partial') ? 5 : 0, errorCode: done && params.has('source-partial') ? 'IP_DNS_UNAVAILABLE' : '' } }
  }
  const controls = { start: (ip: string) => { active = true; activeIP = ip }, finish: () => { active = false }, progress, decorate }
  ;(window as unknown as { __ipEnhancedAudit: typeof controls }).__ipEnhancedAudit = controls
  return controls
}
