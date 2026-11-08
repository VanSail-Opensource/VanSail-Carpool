import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { adminBillingApi } from '@/api/adminBilling'
import { featuresApi } from '@/api/features'
import { embyConnectionApi, type EmbyConnectionWrite } from '@/api/embyConnection'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import EmbyConnectionAudit from './EmbyConnectionAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const state = { reads: 0, writes: [] as EmbyConnectionWrite[], genericWrites: [] as string[], version: 'a'.repeat(64) }
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
  ...['base_url', 'api_token'].map(key => ({ key: `emby.${key}`, value: '', encrypted: true, configured, category: 'emby', updatedAt: null })),
  { key: 'emby.setup_price_txb', value: '0', encrypted: false, configured: true, category: 'emby', updatedAt: null },
] }) as never
api.updateAdminSetting = async key => { state.genericWrites.push(key); return undefined as never }
const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
const limits = { minimum: money('100'), maximum: money('10000'), updatedAt: new Date().toISOString() }
adminBillingApi.getAmountLimits = async () => limits
adminBillingApi.updateAmountLimits = async () => limits
api.getAdminPaymentProfiles = async () => ({ items: [] })
const activity = { timezone: 'UTC', dailyRewardMinTxb: '1.00', dailyRewardMaxTxb: '2.00', groupMessageThreshold: 5, groupMessageRewardTxb: '1.00', updatedAt: null }
featuresApi.getAdminActivitySettings = async () => activity as never
featuresApi.saveAdminActivitySettings = async () => activity as never
Object.assign(window, { __embyConnectionAudit: state })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: EmbyConnectionAudit }] })
await router.push('/')
const app = createApp(EmbyConnectionAudit).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t = t
app.mount('#app')
