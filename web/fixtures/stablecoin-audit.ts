import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { usdtApi, type USDTConfig, type USDTNetworkID } from '@/api/usdt'
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
const dismissed = new Set<string>()
const state = { writes, copies, recover: false, checks: [] as { network: USDTNetworkID; rail?: string }[], dismissals: [] as { id: string; reason: string }[] }
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { copies.push(value) } } })
const configuration: USDTConfig = { mode: 'both', enabled: false, name: 'VanSail', expiryMinutes: 30, concurrency: 1, networks: [] }
if (params.has('stored')) configuration.networks = [{ network: 'ethereum', enabled: true, currencies: ['USDT'], addresses: ['0x1111111111111111111111111111111111111111'], endpoints: [{ id: 'stored-endpoint', rpcUrl: '', wsUrl: '', indexerUrl: '', apiKey: '', useWs: true, rpcConfigured: true, wsConfigured: true, indexerConfigured: false, keyConfigured: true, host: 'rpc.example' }] }]
usdtApi.config = async () => {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 1800))
  if (params.has('error')) throw new ApiError(503, { code: 'USDT_RPC_UNAVAILABLE', message: '' })
  return structuredClone(configuration)
}
usdtApi.status = async () => ({ networks: [] })
usdtApi.check = async (network, rail) => {
  state.checks.push({ network, rail })
  return { network, token: 'constructed-token', rail: rail ?? network, success: !params.has('check-error'), errorCode: params.has('check-error') ? 'USDT_RPC_UNAVAILABLE' : '' }
}
usdtApi.dismiss = async (id, reason) => {
  state.dismissals.push({ id, reason })
  if (params.has('action-error') && !state.recover) throw new ApiError(503, { code: 'USDT_RPC_UNAVAILABLE', message: '' })
  const replayed = dismissed.has(id)
  dismissed.add(id)
  return { orderId: 'retired-order', transferId: id, credited: false, replayed }
}
usdtApi.review = async () => ({ items: params.has('purged-review') && !dismissed.has('purged-transfer') ? [{ id: 'purged-transfer', currency: 'USDT', network: 'ethereum', token: '0x1111111111111111111111111111111111111111', recipient: '0x2222222222222222222222222222222222222222', atomicAmount: '1001000', amount: '1.001', txHash: '0x3333333333333333333333333333333333333333', eventId: '1', occurredAt: new Date().toISOString(), orderId: 'retired-order', status: 'review', reason: 'ACCOUNT_PURGED' }] : [], nextCursor: '' })
usdtApi.save = async value => {
  if (params.has('secret-rejected')) throw new ApiError(422, { code: 'USDT_CONFIGURATION_INVALID', message: '' })
  if (params.has('save-error')) throw new ApiError(503, { code: 'USDT_RPC_UNAVAILABLE', message: '' })
  writes.push(structuredClone(value))
  return { ...structuredClone(value), networks: value.networks.map(network => ({ ...network, endpoints: network.endpoints.map(endpoint => ({ ...endpoint,
    rpcConfigured: endpoint.rpcConfigured || !!endpoint.rpcUrl, wsConfigured: endpoint.wsConfigured || !!endpoint.wsUrl,
    indexerConfigured: endpoint.indexerConfigured || !!endpoint.indexerUrl, keyConfigured: endpoint.keyConfigured || !!endpoint.apiKey,
    host: endpoint.rpcUrl ? new URL(endpoint.rpcUrl).hostname : endpoint.host, rpcUrl: '', wsUrl: '', indexerUrl: '', apiKey: '',
  })) })) }
}
Object.assign(window, { __stablecoinAudit: state })
const app = createApp(StablecoinAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: StablecoinAudit }] })).use(ui).mount('#app')
