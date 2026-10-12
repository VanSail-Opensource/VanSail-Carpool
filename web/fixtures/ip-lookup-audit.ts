import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { ApiError } from '@/api/http'
import { ipLookupApi, type IPLookupAdminSettings, type IPLookupCheck, type IPLookupQuote, type IPLookupReport, type IPLookupState } from '@/api/ipLookup'
import { useSessionStore } from '@/stores/session'
import { setLocale, t } from '@/i18n'
import IPLookupPage from '@/components/ip-lookup/IPLookupPage.vue'
import AdminIPLookupPage from '@/components/admin/ip-lookup/AdminIPLookupPage.vue'
import IPLookupAudit from './IPLookupAudit.vue'
import { installLiveIPAudit } from './ip-lookup-live-audit'
import { installEnhancedIPAudit } from './ip-lookup-enhanced-audit'
import { decorateShodanAudit } from './ip-lookup-shodan-audit'
import { decorateRatingAudit } from './ip-lookup-rating-audit'

const params = new URLSearchParams(location.search)
const enhancedAudit = installEnhancedIPAudit(params)
const liveCounters = installLiveIPAudit(params)
const mode = params.get('mode') ?? 'empty'
const ids = ['abuseipdb', 'scamalytics', 'ipapi', 'maxmind', 'ipqs', 'ip2location'] as const
const money = (minor: number) => ({ currency: 'TXB' as const, minor: String(minor), display: `${(minor / 100).toFixed(2)} TXB` })
const now = () => new Date().toISOString()
const state: IPLookupState = { enabled: mode !== 'disabled', lookupFee: money(250), refreshFee: money(350), allowance: params.has('no-combo') ? null : { purchaseId: 'term', total: 2, remaining: 2, validUntil: '2026-12-01T00:00:00Z' } }
const settings: IPLookupAdminSettings = { proxySources: [{ id: 'named-source', name: 'Community proxy inventory', url: 'https://example.net/proxies/list.txt', enabled: true }, { id: 'disabled-source', name: 'Disabled inventory', url: 'https://example.net/proxies/disabled.txt', enabled: false }], dnsResolvers: ['1.1.1.1:53', '8.8.8.8:53'], probePorts: [443, 4430, 8443], enabled: false, lookupFeeTxb: '', refreshFeeTxb: '', providers: ids.map(id => ({ id, enabled: false, accountId: id === 'maxmind' ? '123456' : id === 'scamalytics' ? 'fixture-account' : '' })), geolocationOrder: ['ip2location', 'ipapi', 'maxmind', 'scamalytics', 'abuseipdb'], retry: { enabled: true, maxAttempts: 5, delaySeconds: 120 }, credentials: {}, configured: Object.fromEntries([...ids, 'cloudflare_radar'].map(id => [id, true])), comboQuotas: { standard: null, weekend: 0 } }
let cached = params.has('cached') || mode === 'cached' || mode === 'subnet'
let polls = 0
let submitCount = 0
let quoteCount = 0
let savedBody: IPLookupAdminSettings | null = null
let lastQuote: IPLookupQuote | null = null
let refreshedLive: Awaited<ReturnType<typeof liveCounters.refresh>> | null = null
let refreshedOnce = false
const receipt = () => ({ id: 'check-audit', kind: 'ip_reputation_lookup', status: 'queued' as const, errorCode: null, createdAt: now(), updatedAt: now(), completedAt: null })

function makeReport(ip: string): IPLookupReport {
  const report: IPLookupReport = {
    id: 'report-audit', ip, status: 'succeeded', refundRequired: false, verdict: 'suitable', reasons: [],
    facts: { country: 'JP', city: 'Tokyo', latitude: 35.6762, longitude: 139.6503, asn: '2527', asnName: 'Sony Network Communications Inc.', networkType: 'residential' },
    sources: { country: 'ip2location', city: 'ip2location', latitude: 'ip2location', longitude: 'ip2location', asn: 'ipapi', asnName: 'ipapi', networkType: 'ipapi,maxmind,ip2location', 'maxmind.ip_risk_snapshot': 'maxmind', 'maxmind.static_ip_score': 'maxmind', 'maxmind.user_count': 'maxmind', 'maxmind.user_type': 'maxmind' },
    maxmind: { ip_risk_snapshot: 0.4, static_ip_score: 0.95, user_count: 3, user_type: 'residential' },
    refusals: [], databases: ids.map(id => ({ id, status: 'success' })), checkedAt: now(), policyVersion: 'residential-v3', parserVersion: 'residential-v3',
  }
  if (params.get('live') !== 'legacy' || lastQuote?.refresh) {
    const validity = (params.get('rpki') ?? 'valid') as 'valid' | 'invalid_asn' | 'invalid_length' | 'not_found' | 'unavailable'
    const meta = { status: validity === 'unavailable' ? 'error' as const : 'success' as const, source: 'ripe:rpki-validation', observedAt: now(), errorCode: validity === 'unavailable' ? 'IP_DETAILS_UNAVAILABLE' : '' }
    report.rpki = { ...meta, prefix: ip.includes(':') ? '2606:4700:4700::/48' : `${ip.split('.').slice(0, 3).join('.')}.0/24`, routes: [{ ...meta, asn: 45671, validity, roas: validity === 'not_found' || validity === 'unavailable' ? [] : [{ asn: 45671, prefix: '118.127.0.0/18', maxLength: 24 }] }] }
    report.policyVersion = 'residential-v4'
    if (validity === 'invalid_asn' || validity === 'invalid_length') { report.verdict = 'unsuitable'; report.refusals = [{ kind: validity === 'invalid_asn' ? 'rpki_invalid_asn' : 'rpki_invalid_length', source: 'ripe_rpki', value: `AS45671 ${report.rpki.prefix}` }] }
    if (validity === 'unavailable') report.status = 'partial'
  }
  if (mode === 'risk' || mode === 'scamalytics-risk' || mode === 'maxmind-risk') {
    report.verdict = 'unsuitable'
    report.refusals = mode === 'risk' ? [{ kind: 'abuse_reports', source: 'abuseipdb', value: 1 }]
      : mode === 'scamalytics-risk' ? [{ kind: 'proxy', source: 'scamalytics:ip2proxy', value: 'Example proxy network' }]
        : [{ kind: 'user_count', source: 'maxmind', value: 6 }]
    if (mode === 'maxmind-risk') report.maxmind.user_count = 6
    const reached = mode === 'risk' ? 1 : mode === 'scamalytics-risk' ? 2 : 4
    report.databases = report.databases.slice(0, reached)
    const locationSource = mode === 'scamalytics-risk' ? 'scamalytics:maxmind_geolite2' : 'ipapi'
    for (const key of ['country', 'city', 'latitude', 'longitude']) report.sources[key] = locationSource
    report.sources.networkType = reached === 4 ? 'ipapi,maxmind' : locationSource
    if (reached < 4) report.maxmind = { ip_risk_snapshot: null, static_ip_score: null, user_count: null, user_type: null }
    if (reached === 1) {
      report.facts = { country: 'JP', city: '', latitude: null, longitude: null, asn: '', asnName: '', networkType: 'residential' }
      report.sources = { country: 'abuseipdb:ipinfo', networkType: 'abuseipdb:ipinfo' }
    }
  }
  if (mode === 'partial' || mode === 'all-failed') {
    report.status = 'partial'; report.verdict = report.refusals.length ? 'unsuitable' : 'inconclusive'
    report.databases = report.databases.map(database => ({ ...database, status: mode === 'all-failed' || database.id === 'maxmind' ? 'error' : database.status }))
    report.maxmind = { ip_risk_snapshot: null, static_ip_score: null, user_count: null, user_type: null }
    report.sources.networkType = 'ipapi,ip2location'
  }
  if (mode === 'nulls' || mode === 'all-failed') {
    report.facts = { country: '', city: '', latitude: null, longitude: null, asn: '', asnName: '', networkType: '' }
    report.maxmind = { ip_risk_snapshot: null, static_ip_score: null, user_count: null, user_type: null }
    report.sources = {}
    report.status = 'partial'; report.verdict = 'inconclusive'
  }
  if (mode === 'zero') {
    report.facts.latitude = 0; report.facts.longitude = 0
    report.maxmind = { ip_risk_snapshot: 0, static_ip_score: 0, user_count: 0, user_type: null }
  }
  if (mode === 'subsource') {
    report.sources.city = 'scamalytics:maxmind_geolite2'
    report.sources.latitude = report.sources.longitude = 'scamalytics:ipinfo'
  }
  if (params.has('many-refusals')) { report.verdict = 'unsuitable'; report.refusals = [{ kind: 'proxy', source: 'scamalytics:ip2proxy', value: 'Residential proxy provider with a long network name and multiple services' }, { kind: 'fraud_score', source: 'scamalytics', value: 73 }, { kind: 'vpn', source: 'ipapi', value: 'Commercial VPN organization and exit infrastructure' }, { kind: 'user_count', source: 'maxmind', value: 14 }] }
  enhancedAudit.decorate(report)
  decorateShodanAudit(report, params)
  decorateRatingAudit(report, params)
  return report
}
async function pause() {
  if (mode === 'loading') await new Promise(resolve => setTimeout(resolve, 5000))
  if (mode === 'error') throw new ApiError(502, { code: 'IP_LOOKUP_FAILED', message: 'Constructed unavailable application' })
}
ipLookupApi.progress = enhancedAudit.progress
ipLookupApi.state = async () => { await pause(); return structuredClone(state) }
ipLookupApi.settings = async () => { await pause(); return structuredClone(settings) }
ipLookupApi.saveSettings = async value => {
  savedBody = JSON.parse(JSON.stringify(value)) as IPLookupAdminSettings
  if (mode === 'save-error' || (value.enabled && (value.lookupFeeTxb === '' || value.refreshFeeTxb === '' || Object.values(value.comboQuotas).some(q => q === null)))) throw new ApiError(422, { code: 'IP_LOOKUP_CONFIGURATION_REQUIRED', message: 'Constructed missing configuration' })
  Object.assign(settings, JSON.parse(JSON.stringify(value)), { credentials: {} })
  state.enabled = value.enabled
  return structuredClone(settings)
}
ipLookupApi.quote = async (ip, refresh) => {
  quoteCount++
  const useQuota = !refresh && !cached && !!state.allowance?.remaining
  const cachedReport = cached ? makeReport(mode === 'subnet' ? '8.8.8.7' : ip) : null
  return { ip, refresh, charge: money(refresh ? 350 : cached || useQuota ? 0 : 250), useQuota, purchaseId: state.allowance?.purchaseId ?? '', remaining: state.allowance?.remaining ?? 0, cacheReportId: cached ? 'report-audit' : '', cacheMatch: cachedReport ? cachedReport.ip === ip ? 'exact' : 'subnet' : 'none', cachedReport, configHash: 'fixture-config', expiresAt: Math.floor(Date.now() / 1000) + 120, token: 'fixture-token' }
}
ipLookupApi.submit = async quote => {
  submitCount++; lastQuote = structuredClone(quote)
  if (mode === 'balance-error') throw new ApiError(409, { code: 'INSUFFICIENT_BALANCE', message: 'Constructed low balance' })
  if (quote.useQuota && state.allowance) state.allowance.remaining--
  polls = 0; refreshedLive = null; refreshedOnce = false
  return receipt()
}
ipLookupApi.check = async () => {
  polls++
  const requestedIP = lastQuote?.ip ?? '8.8.8.8'
  const report = makeReport(requestedIP)
  const pending = polls < 3
  if (!pending && lastQuote?.refresh && !refreshedOnce) { refreshedOnce = true; try { refreshedLive = await liveCounters.refresh(requestedIP) } catch { report.status = 'partial' } }
  const result: IPLookupCheck = { operation: { ...receipt(), status: pending ? 'processing' : report.status, completedAt: pending ? null : now() }, report: pending ? null : report, liveDetails: refreshedLive, requestedIP, cacheMatch: 'none', cached: false, charge: lastQuote?.charge ?? money(0), usedQuota: lastQuote?.useQuota ?? false, refunded: false, progress: { phase: pending ? 'checking' : 'complete', completed: pending ? polls : 6, total: 6, currentProvider: pending ? ids[polls]! : undefined } }
  if (!pending) cached = true
  return result
}
api.getCommunityAccess = async () => ({ activeCombo: false })
api.getAdminResource = async () => ({ items: ['standard', 'weekend'].map((id, index) => ({ id, name: index ? 'Weekend' : 'Standard', ipLookupQuota: settings.comboQuotas[id] })) }) as never
const app = createApp(IPLookupAudit)
app.config.globalProperties.$t = t
const pinia = createPinia()
app.use(pinia)
const session = useSessionStore(pinia)
session.session = { authenticated: true, user: { id: 'ip-audit-member', telegramId: '96001', firstName: 'Morgan', username: 'morgan', telegramUsername: 'morgan', role: params.has('admin') ? 'admin' : 'member', onboardingState: 'complete' } } as never
session.status = 'ready'
const router = createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: params.has('admin') ? AdminIPLookupPage : IPLookupPage }] })
app.use(router); app.use(ui)
setLocale(params.has('zh') ? 'zh-CN' : 'en')
document.title = t('ipLookup.title')
;(window as unknown as { __ipAudit: object }).__ipAudit = { state, settings, seedLive: liveCounters.seed, counters: () => ({ submitCount, quoteCount, polls, savedBody, live: liveCounters.counters() }), setCached: (value: boolean) => { cached = value } }
void router.isReady().then(() => app.mount('#app'))
