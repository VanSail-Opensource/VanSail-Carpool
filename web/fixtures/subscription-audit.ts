import '@/styles/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { usageBreakdownApi } from '@/api/usageBreakdown'
import { usageBreakdownFixture } from './usage-breakdown-data'
import { pricingApi } from '@/api/pricing'
import { ApiError } from '@/api/http'
import { subscriptionApi, type MemberSubscription } from '@/api/subscription'
import { xnodeMemberApi } from '@/api/xnodeMember'
import { ipLookupApi } from '@/api/ipLookup'
import { memberOperationsApi } from '@/api/memberOperations'
import { preferencesApi } from '@/api/preferences'
import { displayCurrencyApi } from '@/api/displayCurrency'
import { useSessionStore } from '@/stores/session'
import { usePreferencesStore } from '@/stores/preferences'
import { setLocale, t } from '@/i18n'
import SubscriptionAudit from './SubscriptionAudit.vue'

const params = new URLSearchParams(location.search)
ipLookupApi.state = async () => ({ enabled: false }) as never
xnodeMemberApi.subscription = async () => ({ ready: !params.has('first-sync'), firstSyncedAt: params.has('first-sync') ? null : new Date().toISOString(), username: 'native-member', active: true, incomplete: false, url: 'https://panel.example/subscription/' + 'a'.repeat(43), downloadUrl: 'https://panel.example/api/v1/subscriptions/' + 'a'.repeat(43), usedBytes: '0', limitBytes: '1000', expiresAt: null, formats: { native: true, base64: true, xray: true, 'sing-box': true, clash: true, stash: !params.has('diagnostics') }, formatDiagnostics: params.has('diagnostics') ? { 'sing-box': { code: 'SUBSCRIPTION_ENDPOINTS_EXCLUDED', excludedEndpoints: 2 }, stash: { code: 'SUBSCRIPTION_FORMAT_RENDER_FAILED', excludedEndpoints: 0 } } : undefined })
xnodeMemberApi.routing = async () => ({ plan: { revision: 0, choices: [], phase: 'active', reason: '', operationId: '' }, nodes: [], templates: [], paths: [] })
xnodeMemberApi.previewRouting = async () => ({ paths: [], trace: null })
xnodeMemberApi.metrics = async () => {
  if (params.has('metrics-error')) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 1200))
  return { activityStatus: params.has('unknown') ? 'unavailable' : params.has('offline') ? 'offline' : 'online', activityExpiresAt: params.has('unknown') ? null : new Date(Date.now() + (params.has('expire') ? 1000 : 15000)).toISOString(), connections: params.has('offline') ? 0 : 3, uploadBytesPerSecond: params.has('offline') ? 0 : 1000, downloadBytesPerSecond: 0, limited: params.has('policy-expired'), incomplete: params.has('unknown'), reasons: params.has('policy-expired') ? [{ nodeId: 'hdnl', reason: 'expired', scope: 'new_connection' }] : [], fetchedAt: new Date().toISOString() }
}
let usageReads = 0
usageBreakdownApi.get = async () => {
 usageReads++
 if (params.has('history-slow')) await new Promise(resolve => setTimeout(resolve,Number(params.get('history-slow')) || 1800))
 if (params.has('history-error')) throw new ApiError(503,{code:'XNODE_UNAVAILABLE',message:''})
 const flags = new URLSearchParams(params.has('history-shares') ? 'grouping' : 'large')
 for (const [previous, next] of [['history-empty','empty'],['history-stale','stale'],['history-truncated','omitted']] as const) if (params.has(previous)) flags.set(next, '')
 return usageBreakdownFixture(flags)
}
if (params.has('telegram')) Object.assign(window, { Telegram: { WebApp: { platform: 'ios', version: '9.0', initData: '', initDataUnsafe: {}, BackButton: { show() {}, hide() {}, onClick() {}, offClick() {} } } } })
const hostNames: Record<string, string> = {}
const state = { reads: 0, summaryReads: 0, revocations: 0, saves: 0, checks: 0, resolves: [] as string[], savedConfig: null as unknown, failure: params.has('error'), get usageReads() { return usageReads } }
const now = Date.now()
const at = (offset: number) => new Date(now + offset).toISOString()
const day = 86_400_000
const data = (): MemberSubscription => ({
  ready: !params.has('first-sync'), firstSyncedAt: params.has('first-sync') ? null : at(-day),
  activeCombo: !params.has('inactive'), subscriptionUrl: params.has('first-sync') ? null : params.has('inactive') && !params.has('payg') ? null : 'https://subscription.example/import/revision-' + state.revocations,
  hosts: params.has('first-sync') || params.has('empty') || params.has('inactive') && !params.has('payg') ? [] : [
    { uuid: 'tokyo-edge', publishedName: 'Tokyo {price}', nameOverride: hostNames['tokyo-edge'] || '', priceLabel:'1.0x',customRoute:true,splitXhttp:false, name: (hostNames['tokyo-edge'] || 'Tokyo {price}').replaceAll('{price}', '1.0x'), countryCodes: ['JP'], squadUuids: ['international'], link: 'vless://fixture-' + state.revocations + '@tokyo.example:443#'+encodeURIComponent((hostNames['tokyo-edge'] || 'Tokyo {price}').replaceAll('{price}', '1.0x')), linkErrorCode: '' },
    { uuid: 'shared-edge',customRoute:false,splitXhttp:true, name: 'Shared transit with a deliberately long host name to check narrow layouts', countryCodes: ['DE', 'NL'], squadUuids: ['international', 'europe'], link: 'trojan://fixture-' + state.revocations + '@europe.example:443#Shared', linkErrorCode: '' },
    { uuid: 'unknown-edge', name: 'Unmeasured host', countryCodes: [], squadUuids: ['europe'], link: null, linkErrorCode: 'SUBSCRIPTION_LINK_UNAVAILABLE' },
  ],

})
async function delay(): Promise<void> {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 10_000))
  if (state.failure) throw new ApiError(502, { code: 'SUBSCRIPTION_UNAVAILABLE', message: '' })
}
subscriptionApi.saveHostName = async (id, name) => { await delay(); if (params.has('save-error')) throw new ApiError(409,{code:'XNODE_UNAVAILABLE',message:''}); hostNames[id]=name.trim(); state.saves++; return {hostId:id,name:name.replaceAll('{price}','1.0x'),nameOverride:name,publishedName:'Tokyo {price}',priceLabel:'1.0x'} }
subscriptionApi.subscription = async () => { state.reads++; await delay(); return data() }
memberOperationsApi.getPurchaseRefundQuote = async () => ({ eligible:false,reason:'FIXTURE',amount:{currency:'TXB',minor:'0',display:'0.00 TXB'} }) as never
api.getAutoRenewal = async () => ({ purchaseId:'fixture-purchase',enabled:true,canEnable:true,ineligibleReason:null,grossPrice:{currency:'TXB',minor:'1000',display:'10.00 TXB'},discount:{currency:'TXB',minor:'0',display:'0.00 TXB'},netPrice:{currency:'TXB',minor:'1000',display:'10.00 TXB'},scheduledAt:at(23*day),nextCycleEndsAt:at(53*day) })
pricingApi.renewalInfo = async () => ({ policy: { dynamicEnabled: false }, quote: null, graceSeconds: 0 }) as never
api.getCommunityAccess = async () => ({ activeCombo: !params.has('inactive') && (!params.has('zero-balance') || params.has('combo')) })
const fixturePurchase = { id:'fixture-purchase',comboId:'fixture-combo',comboName:'Tokyo monthly',price:{currency:'TXB',minor:'1000',display:'10.00 TXB'},grossPrice:{currency:'TXB',minor:'1000',display:'10.00 TXB'},couponDiscount:{currency:'TXB',minor:'0',display:'0.00 TXB'},couponGrantId:null,validFrom:at(-7*day),validUntil:at(23*day),status:'active',autoRenewEnabled:true,trafficLimitBytes:'100000000000',resetStrategy:'MONTH_ROLLING',squadUuids:['international'],createdAt:at(-7*day),updatedAt:at(0) }
api.getDashboard = async () => {
  if (params.has('home-slow')) await new Promise(resolve => setTimeout(resolve, 10_000))
  if (params.has('home-error')) throw new ApiError(502, { code: 'DASHBOARD_UNAVAILABLE', message: '' })
  return ({ user: {}, balance: { currency: 'TXB', minor: params.has('zero-balance') ? '0' : '12345', display: params.has('zero-balance') ? '0.00 TXB' : '123.45 TXB' }, activePurchase: params.has('combo') ? fixturePurchase : null, queuedPurchase: null, statisticsStale: false, fetchedAt: at(0), subscriptionUrl: 'https://subscription.example/import', statistics: { usedTrafficBytes: '12345678900', lifetimeTrafficBytes: '22345678900', trafficLimitBytes: '100000000000', onlineAt: at(0), lastTrafficResetAt: params.has('noreset') ? null : at(-day), categories: [], sparklineData: [], topNodes: [] } }) as never
}
api.getCatalog = async () => ({ combos: [], addons: [], nodes: [] })
api.getBalance = async () => {
  if (params.has('balance-error')) throw new ApiError(503, { code: 'UNAVAILABLE', message: '' })
  const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
  return { balance: money(params.has('zero-balance') ? '0' : '12345'), paymentMethods: [], pendingPaymentOrder: null, addAmountLimits: { minimum: money('100'), maximum: money('1000000'), updatedAt: at(0) } }
}
api.revokeSubscription = async () => { state.revocations++; return { id: 'audit-revoke', kind: 'subscription_revoke', status: 'queued', errorCode: null, createdAt: at(0), updatedAt: at(0), completedAt: null } }
memberOperationsApi.getOperation = async () => ({ id: 'audit-revoke', kind: 'subscription_revoke', status: 'succeeded', errorCode: null, createdAt: at(0), updatedAt: at(0), completedAt: at(0) })
Object.assign(window, { __subscriptionAudit: state, __subscriptionAuditLocale: setLocale, __subscriptionSnapshot: data })
const mode = params.get('mode') ?? 'subscription'
const shell = params.has('shell')
const app = shell ? createApp((await import('@/App.vue')).default) : createApp(SubscriptionAudit, { mode })
app.mixin({ errorCaptured(error) { console.error(error) } })
app.config.globalProperties.$t = t
const pinia = createPinia()
app.use(pinia)
usePreferencesStore(pinia).value = { paygEnabled: true, paygSquads: {}, activeCombo: true, showAroundTx: true, showActivity: true, includeNodePrices: true, notifications: {} } as never
useSessionStore(pinia).session = { authenticated: true, user: { id: 'audit-member', telegramId: '42', firstName: 'Mira', role: 'user', onboardingState: 'complete' } } as never
useSessionStore(pinia).status = 'ready'
preferencesApi.get = async () => ({ activeCombo: true, showAroundTx: false, showActivity: false, includeNodePrices: true, notifications: {} }) as never
displayCurrencyApi.get = async () => ({ currency: 'TXB', rates: null }) as never
const router = shell ? (await import('@/router')).default : createRouter({ history: createWebHistory(), routes: [
  { path: '/fixtures/subscription-audit.html', component: SubscriptionAudit },
  { path: '/subscription', name: 'subscription', component: SubscriptionAudit },
  { path: '/connections', component: SubscriptionAudit },
  { path: '/catalog', component: SubscriptionAudit },
  { path: '/:pathMatch(.*)*', component: SubscriptionAudit },
] })
if (shell) await router.replace(mode === 'home' ? '/home' : '/subscription')
app.use(router); app.use(ui)
void router.isReady().then(() => app.mount('#app'))
