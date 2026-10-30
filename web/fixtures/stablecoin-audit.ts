import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { usdtApi, type USDTConfig } from '@/api/usdt'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import StablecoinAudit from './StablecoinAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser'); initializeBrowserTheme(); setLocale(params.has('zh') ? 'zh-CN' : 'en')
const writes: USDTConfig[] = []
const copies: string[] = []
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { copies.push(value) } } })
const configuration: USDTConfig = { mode: 'both', enabled: false, name: 'VanSail', expiryMinutes: 30, concurrency: 1, networks: [] }
usdtApi.config = async () => {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 1800))
  if (params.has('error')) throw new ApiError(503, { code: 'USDT_RPC_UNAVAILABLE', message: '' })
  return structuredClone(configuration)
}
usdtApi.status = async () => ({ networks: [] })
usdtApi.review = async () => ({ items: params.has('purged-review') ? [{ id: 'purged-transfer', currency: 'USDT', network: 'ethereum', token: '0x1111111111111111111111111111111111111111', recipient: '0x2222222222222222222222222222222222222222', atomicAmount: '1001000', amount: '1.001', txHash: '0x3333333333333333333333333333333333333333', eventId: '1', occurredAt: new Date().toISOString(), orderId: 'retired-order', status: 'review', reason: 'ACCOUNT_PURGED' }] : [], nextCursor: '' })
usdtApi.save = async value => {
  if (params.has('save-error')) throw new ApiError(503, { code: 'USDT_RPC_UNAVAILABLE', message: '' })
  writes.push(structuredClone(value)); return structuredClone(value)
}
Object.assign(window, { __stablecoinAudit: { writes, copies } })
const app = createApp(StablecoinAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: StablecoinAudit }] })).use(ui).mount('#app')
