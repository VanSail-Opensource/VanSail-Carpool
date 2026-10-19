import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { ApiError } from '@/api/http'
import { subscriptionApi, type MemberSubscription, type UptimeTimeline } from '@/api/subscription'
import { connectivityApi } from '@/api/connectivity'
import { xnodeMemberApi } from '@/api/xnodeMember'
import { ipLookupApi } from '@/api/ipLookup'
import { memberOperationsApi } from '@/api/memberOperations'
import { preferencesApi } from '@/api/preferences'
import { displayCurrencyApi } from '@/api/displayCurrency'
import { useSessionStore } from '@/stores/session'
import { setLocale, t } from '@/i18n'
import SubscriptionAudit from './SubscriptionAudit.vue'

const params = new URLSearchParams(location.search)
ipLookupApi.state = async () => ({ enabled: false }) as never
xnodeMemberApi.subscription = async () => ({ username: 'native-member', active: true, incomplete: false, url: 'https://panel.example/subscription/' + 'a'.repeat(43), downloadUrl: 'https://panel.example/api/v1/subscriptions/' + 'a'.repeat(43), usedBytes: '0', limitBytes: '1000', expiresAt: null, formats: { native: true, base64: true, xray: true, 'sing-box': true, clash: true, stash: !params.has('diagnostics') }, formatDiagnostics: params.has('diagnostics') ? { 'sing-box': { code: 'SUBSCRIPTION_ENDPOINTS_EXCLUDED', excludedEndpoints: 2 }, stash: { code: 'SUBSCRIPTION_FORMAT_RENDER_FAILED', excludedEndpoints: 0 } } : undefined })
xnodeMemberApi.routing = async () => ({ plan: { revision: 0, choices: [], phase: 'active', reason: '', operationId: '' }, nodes: [], templates: [], paths: [] })
xnodeMemberApi.previewRouting = async () => ({ paths: [], trace: null })
xnodeMemberApi.metrics = async () => {
  if (params.has('metrics-error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 1200))
  return { activityStatus: params.has('unknown') ? 'unavailable' : params.has('offline') ? 'offline' : 'online', activityExpiresAt: params.has('unknown') ? null : new Date(Date.now() + (params.has('expire') ? 1000 : 15000)).toISOString(), connections: params.has('offline') ? 0 : 3, uploadBytesPerSecond: params.has('offline') ? 0 : 1000, downloadBytesPerSecond: 0, limited: false, incomplete: params.has('unknown'), reasons: [], fetchedAt: new Date().toISOString() }
}
if (params.has('telegram')) Object.assign(window, { Telegram: { WebApp: { platform: 'ios', version: '9.0', initData: '', initDataUnsafe: {}, BackButton: { show() {}, hide() {}, onClick() {}, offClick() {} } } } })
const state = { reads: 0, summaryReads: 0, revocations: 0, saves: 0, checks: 0, resolves: [] as string[], savedConfig: null as unknown, failure: params.has('error') }
const now = Date.now()
const at = (offset: number) => new Date(now + offset).toISOString()
const day = 86_400_000
const timeline = (status: UptimeTimeline['state']): UptimeTimeline => ({ from: at(-day), to: at(0), state: status, segments: [
  { from: at(-day), to: at(-day * 0.75), state: 'unknown' },
  { from: at(-day * 0.75), to: at(-day * 0.25), state: status === 'outage' ? 'operational' : status },
  { from: at(-day * 0.25), to: at(0), state: status },
] })
const data = (): MemberSubscription => ({
  activeCombo: !params.has('inactive'), subscriptionUrl: params.has('inactive') ? null : 'https://subscription.example/import/revision-' + state.revocations,
  summary: { monitoringEnabled: !params.has('monitor-off'), activeCombo: !params.has('inactive'), ...timeline(params.has('unknown') ? 'unknown' : 'partial'), errorCode: params.has('unknown') ? 'CONNECTIVITY_NOT_CONFIGURED' : '' },
  hosts: params.has('empty') || params.has('inactive') ? [] : [
    { uuid: 'tokyo-edge', name: 'Tokyo transit', countryCodes: ['JP'], squadUuids: ['international'], link: 'vless://fixture-' + state.revocations + '@tokyo.example:443#Tokyo%20transit', linkErrorCode: '', timeline: timeline('operational') },
    { uuid: 'shared-edge', name: 'Shared transit with a deliberately long host name to check narrow layouts', countryCodes: ['DE', 'NL'], squadUuids: ['international', 'europe'], link: 'trojan://fixture-' + state.revocations + '@europe.example:443#Shared', linkErrorCode: '', timeline: timeline('outage') },
    { uuid: 'unknown-edge', name: 'Unmeasured host', countryCodes: [], squadUuids: ['europe'], link: null, linkErrorCode: 'CONNECTIVITY_LINK_AMBIGUOUS', timeline: timeline('unknown') },
  ],
  squads: params.has('inactive') ? [] : [
    { uuid: 'international', name: 'International network', hostUuids: params.has('empty') ? [] : ['tokyo-edge', 'shared-edge'], timeline: timeline('partial') },
    { uuid: 'europe', name: 'European transit', hostUuids: params.has('empty') ? [] : ['shared-edge', 'unknown-edge'], timeline: timeline('unknown') },
  ],
})
async function delay(): Promise<void> {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 10_000))
  if (state.failure) throw new ApiError(502, { code: 'CONNECTIVITY_SUBSCRIPTION_UNAVAILABLE', message: '' })
}
subscriptionApi.subscription = async () => { state.reads++; await delay(); return data() }
subscriptionApi.summary = async () => { state.summaryReads++; await delay(); return data().summary }
api.getCommunityAccess = async () => ({ activeCombo: !params.has('inactive') })
api.getDashboard = async () => ({ user: {}, balance: { currency: 'TXB', minor: '12345', display: '123.45 TXB' }, activePurchase: null, queuedPurchase: null, statisticsStale: false, fetchedAt: at(0), subscriptionUrl: 'https://subscription.example/import', statistics: { usedTrafficBytes: '12345678900', lifetimeTrafficBytes: '22345678900', trafficLimitBytes: '100000000000', onlineAt: at(0), lastTrafficResetAt: params.has('noreset') ? null : at(-day), categories: [], sparklineData: [], topNodes: [] } }) as never
api.getCatalog = async () => ({ combos: [], addons: [], nodes: [] })
api.revokeSubscription = async () => { state.revocations++; return { id: 'audit-revoke', kind: 'subscription_revoke', status: 'queued', errorCode: null, createdAt: at(0), updatedAt: at(0), completedAt: null } }
memberOperationsApi.getOperation = async () => ({ id: 'audit-revoke', kind: 'subscription_revoke', status: 'succeeded', errorCode: null, createdAt: at(0), updatedAt: at(0), completedAt: at(0) })
const config = { scheduledEnabled: true, remnawaveUserId: 42, intervalSeconds: 300, timeoutSeconds: 15, maxRetries: 10, retryIntervalSeconds: 1, probeUrl: 'https://probe.example/status' }
connectivityApi.resolve = async username => { state.resolves.push(username); await delay(); if (params.has('resolve-error')) throw new ApiError(422, { code: 'CONNECTIVITY_ACCOUNT_UNAVAILABLE', message: '' }); return { id: 1099511627776, username, status: 'ACTIVE' } }
connectivityApi.snapshot = async () => { await delay(); return { config: { ...config }, user: { id: config.remnawaveUserId, username: config.remnawaveUserId === 42 ? 'monitor' : 'native-monitor', status: 'ACTIVE' }, run: null, hosts: params.has('empty') ? [] : [{ hostUuid: 'tokyo-edge', remark: 'Tokyo transit', address: 'tokyo.example', port: 443, latest: { id: 'result', runId: 'run', hostUuid: 'tokyo-edge', remnawaveUserId: 42, trigger: 'scheduled', startedAt: at(-30_000), finishedAt: at(-29_000), status: 'connected', latencyMs: 123, httpStatus: 204, errorCode: '' } }], stale: false, errorCode: '' } }
connectivityApi.history = async () => { await delay(); return { items: [], nextCursor: null } }
connectivityApi.save = async next => { state.saves++; state.savedConfig = structuredClone(next); Object.assign(config, next) }
connectivityApi.check = async () => { state.checks++; return { id: 'run', status: 'completed', trigger: 'manual', startedAt: at(0), finishedAt: at(0), total: 1, completed: 1, errorCode: '' } }
Object.assign(window, { __subscriptionAudit: state, __subscriptionAuditLocale: setLocale, __subscriptionSnapshot: data })
const mode = params.get('mode') ?? 'subscription'
const shell = params.has('shell')
const app = shell ? createApp((await import('@/App.vue')).default) : createApp(SubscriptionAudit, { mode })
app.mixin({ errorCaptured(error) { console.error(error) } })
app.config.globalProperties.$t = t
const pinia = createPinia()
app.use(pinia)
useSessionStore(pinia).session = { authenticated: true, user: { id: 'audit-member', telegramId: '42', firstName: 'Mira', role: 'user', onboardingState: 'complete' } } as never
useSessionStore(pinia).status = 'ready'
preferencesApi.get = async () => ({ activeCombo: true, showAroundTx: false, showActivity: false, includeNodePrices: true, notifications: {} }) as never
displayCurrencyApi.get = async () => ({ currency: 'TXB', rates: null }) as never
const router = shell ? (await import('@/router')).default : createRouter({ history: createWebHistory(), routes: [
  { path: '/fixtures/subscription-audit.html', component: SubscriptionAudit },
  { path: '/subscription', name: 'subscription', component: SubscriptionAudit },
  { path: '/connections', component: SubscriptionAudit },
  { path: '/catalog', component: SubscriptionAudit },
] })
if (shell) await router.replace(mode === 'home' ? '/home' : '/subscription')
app.use(router); app.use(ui)
void router.isReady().then(() => app.mount('#app'))
