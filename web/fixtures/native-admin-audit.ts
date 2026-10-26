import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme, setBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import NativeAdminAudit from './NativeAdminAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser'); initializeBrowserTheme(); setBrowserTheme(params.has('dark') ? 'dark' : 'light'); setLocale(params.has('zh') ? 'zh-CN' : 'en')
document.cookie = `txc_admin_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
const audit = { fail: params.has('error'), slow: params.has('slow'), empty: params.has('empty'), failSave: params.has('save-error'), calls: [] as { path: string; body: unknown; signed: boolean; scope: string | null }[] }
Object.assign(window, { __nativeAdminAudit: audit })
const stamp = new Date().toISOString()
const days = Array.from({ length: 7 }, (_, i) => new Date(Date.now() - (6 - i) * 86400000).toISOString().slice(0, 10))
const nodes = [{ nodeId: 'tokyo', name: 'Tokyo native', countryCode: 'JP', available: true, currentUse: true, connections: '3', uploadBytesPerSecond: 130000, downloadBytesPerSecond: 2100000, uploadBytes: '9007199254740993', downloadBytes: '14411518807585587', weightedBytes: '27021597764222979' }, { nodeId: 'offline', name: 'Frankfurt unavailable', countryCode: 'DE', available: false, currentUse: false, connections: '0', uploadBytesPerSecond: 0, downloadBytesPerSecond: 0, uploadBytes: '1024', downloadBytes: '2048', weightedBytes: '3072' }]
const profile = { connections: 3, uploadBytesPerSecond: 130000, downloadBytesPerSecond: 2100000, incomplete: true, limited: false, reasons: [], fetchedAt: stamp, nodes: audit.empty ? [] : nodes, usage: days.map((date, i) => ({ date, uploadBytes: String(1000000 * (i + 1)), downloadBytes: String(3000000 * (i + 1)), weightedBytes: String(4000000 * (i + 1) + 10000), legacyBytes: '10000' })), usedTrafficBytes: '9007199254740993', lifetimeTrafficBytes: '90071992547409930' }
const machine = { nodes: audit.empty ? [] : [{ nodeId: 'tokyo', available: true, connected: true, ready: true, synchronized: true, stale: false, version: 'v2.0.7', xrayVersion: '26.10.10', capabilities: ['sampled_activity_v1'], reportedAt: stamp, machine: { cpu_usage: [12.4, 18.2], memory: { total: '9007199254740993', used: '1351079888211149', usedPercent: 15 }, uptime_seconds: '1000000' }, certificates: {}, activeConnections: 3, usersOnline: 1, uploadBytesPerSecond: 130000, downloadBytesPerSecond: 2100000, logs: [] }], onlineNodes: audit.empty ? 0 : 1, totalNodes: audit.empty ? 0 : 1, activeConnections: audit.empty ? 0 : 3, uploadBytesPerSecond: audit.empty ? 0 : 130000, downloadBytesPerSecond: audit.empty ? 0 : 2100000, fetchedAt: stamp }
const money = { currency: 'TXB', minor: '1000', display: '10.00 TXB' }
const statistics = { generatedAt: stamp, remoteGeneratedAt: stamp, databaseGeneratedAt: stamp, stalePartitions: [], remote: { weeklyUserIncrease: 4, monthlyAverageUsagePercent: 37, trafficDates: days, trafficSeries: audit.empty ? [] : [{ uuid: 'tokyo', name: 'Tokyo native', countryCode: 'JP', dailyBytes: days.map((_, i) => String((i + 1) * 1000000)) }] }, database: { newUserConversionPercent: 50, averageSpend: money, spendMinimum: money, spendMaximum: money, subscriptionStates: audit.empty ? [] : [{ id: 'active', label: 'Active', value: 10 }], averageCheckInReward: money, comboShares: [], groupMessagesTotal: 100, averageOptionalSquads: 1, paymentStatuses: [], databaseBytes: '1000000', squadByCombo: [], comboBySquad: [] } }
window.fetch = async (input, init) => {
  const request = input instanceof Request ? input : new Request(new URL(String(input), location.origin), init)
  const url = new URL(request.url)
  const rawBody = request.method === 'GET' ? '' : await request.clone().text()
  const body = rawBody ? JSON.parse(rawBody) : null
  audit.calls.push({ path: url.pathname, body, signed: request.headers.has('X-TXC-Signature'), scope: request.headers.get('X-TXC-Session-Scope') })
  const old = url.pathname.includes('/old/')
  if (audit.slow || old) await new Promise(resolve => setTimeout(resolve, old ? 2200 : 1200))
  const failed = audit.fail || audit.failSave && request.method !== 'GET'
  let value: unknown = {}
  if (failed) value = { code: 'XNODE_UNAVAILABLE', message: 'fixture' }
  else if (url.pathname.endsWith('/metrics')) value = { ...profile, fetchedAt: new Date().toISOString(), nodes: old ? [{ ...nodes[0], name: 'Obsolete profile' }] : profile.nodes }
  else if (url.pathname.endsWith('/statistics/nodes')) value = { generatedAt: stamp, stale: false, nodes: audit.empty ? [] : [{ uuid: 'tokyo', name: 'Tokyo native', countryCode: 'JP', online: true, usersOnline: 1, rxBytesPerSec: '2100000', txBytesPerSec: '130000', xrayVersion: '26.10.10', multiplier: 1 }] }
  else if (url.pathname.endsWith('/statistics')) value = statistics
  else if (url.pathname.includes('/connections')) value = { id: 'scan', isCompleted: request.method === 'GET', isFailed: params.has('scan-failed'), errorCode: params.has('scan-failed') ? 'CONNECTION_SCAN_START_AMBIGUOUS' : undefined, progressPercent: request.method === 'GET' ? 100 : 0, nodes: audit.empty ? [] : [{ uuid: 'tokyo', name: 'Tokyo native', countryCode: 'JP', unavailable: false, ips: [{ ip: '203.0.113.10', lastSeen: stamp, handle: 'fixture' }] }, { uuid: 'offline', name: 'Frankfurt unavailable', countryCode: 'DE', unavailable: true, ips: [] }], createdAt: stamp, expiresAt: new Date(Date.now() + 1800000).toISOString() }
  else if (url.pathname.endsWith('/ip-blocks')) value = { items: [] }
  else if (url.pathname.endsWith('/firewall/exemption')) value = { enabled: false, price: '0', hours: 24 }
  else if (url.pathname.endsWith('/balance-adjustments')) value = { id: 'ledger', delta: money, balanceAfter: money, kind: 'admin_adjustment', referenceId: '', note: body.reason, createdAt: stamp }
  return new Response(JSON.stringify(value), { status: failed ? 503 : 200, headers: { 'Content-Type': 'application/json' } })
}
const app = createApp(NativeAdminAudit, { machine: machine as any, view: params.get('view') ?? 'profile' })
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: NativeAdminAudit }] })).use(ui).mount('#app')
