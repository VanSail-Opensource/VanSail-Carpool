import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api, type AdminPaymentProfile, type AdminPaymentProfileWrite } from '@/api/client'
import { adminBillingApi } from '@/api/adminBilling'
import { featuresApi, type ActivitySettings, type ActivitySettingsWrite } from '@/api/features'
import { embyConnectionApi, type EmbyConnectionWrite } from '@/api/embyConnection'
import { cacheGeneration, setCacheSession, writeResponseCache } from '@/api/cache/session'
import { responseCacheKey } from '@/api/cache/policy'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { moneyFromTxbInput } from '@/utils/format'
import { setLocale, t } from '@/i18n'
import EmbyConnectionAudit from './EmbyConnectionAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const state = { reads: 0, writes: [] as EmbyConnectionWrite[], genericWrites: [] as string[], version: 'a'.repeat(64),
  pendingReads: 0, profileWrites: [] as AdminPaymentProfileWrite[], profileDeletes: [] as string[],
  amountWrites: [] as { minimum: string; maximum: string }[], activityWrites: [] as ActivitySettingsWrite[] }
const pending: Array<{ resolve: () => void; reject: (error: Error) => void }> = []
async function hold<T>(value: T): Promise<T> {
  if (params.has('cached-refresh')) {
    state.pendingReads++
    await new Promise<void>((resolve, reject) => { pending.push({ resolve, reject }) })
  }
  return value
}
Object.assign(state, { releaseReads: async (fail = false) => {
  for (const read of pending.splice(0)) { if (fail) read.reject(new Error('Constructed retired settings read')); else read.resolve() }
  state.pendingReads = 0
  await new Promise(resolve => setTimeout(resolve, 0))
} })
let configured = !params.has('empty')
embyConnectionApi.get = async () => {
  state.reads++
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 3500))
  if (params.has('error') && state.reads === 1) throw new Error('Constructed connection load failure')
  return { version: state.version, urlConfigured: configured, tokenConfigured: configured }
}
embyConnectionApi.save = async body => {
  if (params.has('conflict')) throw new ApiError(409, { code: 'EMBY_CONNECTION_CHANGED', message: '' })
  if (params.has('save-error')) throw new ApiError(422, { code: 'EMBY_FRESH_TOKEN_REQUIRED', message: '' })
  state.writes.push(body)
  state.version = 'b'.repeat(64)
  configured = true
  return { version: state.version, urlConfigured: true, tokenConfigured: true }
}
api.getAdminResource = async () => ({ items: [
  ...['base_url', 'api_token'].map(key => ({ key: 'emby.' + key, value: '', encrypted: true, configured, category: 'emby', updatedAt: new Date().toISOString() })),
  { key: 'emby.setup_price_txb', value: '0', encrypted: false, configured: true, category: 'emby', updatedAt: new Date().toISOString() },
] }) as never
api.updateAdminSetting = async key => { state.genericWrites.push(key); return undefined as never }
const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
let limits = { minimum: money('100'), maximum: money('10000'), updatedAt: new Date().toISOString() }
adminBillingApi.getAmountLimits = async () => hold(params.has('cached-refresh') ? { ...structuredClone(limits), maximum: money('20000') } : structuredClone(limits))
adminBillingApi.updateAmountLimits = async (minimum, maximum) => {
  state.amountWrites.push({ minimum, maximum })
  limits = { minimum: money(minimum), maximum: money(maximum), updatedAt: new Date().toISOString() }
  return structuredClone(limits)
}
let profiles: AdminPaymentProfile[] = params.has('cached-refresh') ? [{ id: 'constructed-profile', provider: 'ezpay',
  providerName: 'Constructed provider', enabledChannels: ['alipay'], endpoint: 'https://payment.example', merchantId: 'constructed',
  credential: '', acknowledgement: 'ok', enabled: true, configured: true }] : []
api.getAdminPaymentProfiles = async () => hold({ items: structuredClone(profiles) })
api.updateAdminPaymentProfile = async (id, body) => {
  state.profileWrites.push(structuredClone(body))
  const next = { ...body, id, credential: '', configured: true }
  profiles = profiles.map(profile => profile.id === id ? next : profile)
  return structuredClone(next)
}
api.createAdminPaymentProfile = async body => {
  state.profileWrites.push(structuredClone(body))
  const next = { ...body, id: 'constructed-created-' + state.profileWrites.length, credential: '', configured: !!body.credential }
  profiles.push(next)
  return structuredClone(next)
}
api.deleteAdminPaymentProfile = async id => { state.profileDeletes.push(id); profiles = profiles.filter(profile => profile.id !== id) }
let activity: ActivitySettings = { timezone: 'UTC', dailyRewardMinTxb: '1.00', dailyRewardMinTxbMinor: '100',
  dailyRewardMaxTxb: '2.00', dailyRewardMaxTxbMinor: '200', groupMessageThreshold: 5,
  groupMessageRewardTxb: '1.00', groupMessageRewardTxbMinor: '100' }
featuresApi.getAdminActivitySettings = async () => hold(structuredClone(activity))
featuresApi.saveAdminActivitySettings = async body => {
  state.activityWrites.push(structuredClone(body))
  activity = { ...body, dailyRewardMinTxbMinor: moneyFromTxbInput(body.dailyRewardMinTxb),
    dailyRewardMaxTxbMinor: moneyFromTxbInput(body.dailyRewardMaxTxb), groupMessageRewardTxbMinor: moneyFromTxbInput(body.groupMessageRewardTxb) }
  return structuredClone(activity)
}
if (params.has('cached-refresh')) {
  setCacheSession('constructed-settings-admin')
  for (const [path, value] of [
    ['/api/v1/admin/billing/amount-limits', limits],
    ['/api/v1/admin/payment-profiles', { items: profiles }],
    ['/api/v1/admin/activity-settings', activity],
  ] as const) {
    const key = responseCacheKey(path)
    if (key) writeResponseCache(key, value, cacheGeneration())
  }
}
Object.assign(window, { __embyConnectionAudit: state })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: EmbyConnectionAudit }] })
await router.push('/')
const app = createApp(EmbyConnectionAudit).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t = t
app.mount('#app')
