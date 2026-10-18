import { createApp, defineComponent, h, shallowRef } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import UApp from '@nuxt/ui/components/App.vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import AdminUSDTPage from '@/components/admin/usdt/AdminUSDTPage.vue'
import AdminPaymentCreditDialog from '@/components/admin/users/AdminPaymentCreditDialog.vue'
import type { AdminUserDetail } from '@/api/adminOperations'
import { setLocale, t } from '@/i18n'
import '@/styles/main.css'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
document.documentElement.dataset.experience = 'browser'
document.cookie = `txc_admin_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
document.documentElement.classList.toggle('dark', params.has('dark'))
if (params.has('zh')) setLocale('zh-CN')
const networks = ['tron', 'ethereum', 'bsc', 'polygon', 'arbitrum', 'xlayer', 'plasma', 'solana', 'aptos', 'ton']
const audit = { calls: [] as { path: string; body: unknown; signed: boolean }[], fail: params.has('error'), slow: params.has('slow'), empty: params.has('empty'), failSave: params.has('save-error'), dismissed: false }
Object.assign(window, { __stablecoinAudit: audit })
const config = { enabled: true, mode: 'both', name: 'VanSail', rate: '10', expiryMinutes: 30, concurrency: 2, networks: networks.map(network => ({ network, enabled: network === 'ethereum', addresses: network === 'ethereum' ? ['0x1111111111111111111111111111111111111111'] : [], endpoints: network === 'ethereum' ? [{ id: 'rpc', rpcUrl: '', wsUrl: '', indexerUrl: '', apiKey: '', useWs: false, rpcConfigured: true, wsConfigured: false, indexerConfigured: false, keyConfigured: true, host: 'rpc.example' }] : [] })) }
const transfer = { id: 'a'.repeat(64), currency: 'USDC', network: 'ethereum', token: '0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48', recipient: '0x1111111111111111111111111111111111111111', atomicAmount: '1000000', amount: '1.000000', txHash: `0x${'b'.repeat(64)}`, eventId: '0', occurredAt: new Date().toISOString(), orderId: 'payment', status: 'review', reason: 'USDT_LATE_PAYMENT' }
window.fetch = async (input, init) => {
  const outgoing = input instanceof Request ? input : new Request(new URL(String(input), location.origin), init)
  const path = new URL(outgoing.url).pathname
  if (!path.startsWith('/api/')) return new Response('', { status: 404 })
  const body = outgoing.method !== 'GET' ? await outgoing.clone().json() : null
  audit.calls.push({ path, body, signed: outgoing.headers.has('X-TXC-Signature') })
  if (audit.slow) await new Promise(resolve => setTimeout(resolve, 2500))
  const failed = audit.fail || (audit.failSave && outgoing.method !== 'GET')
  let result: unknown = {}
  if (failed) result = { code: 'USDT_RPC_UNAVAILABLE', message: 'fixture failure' }
  else if (path.endsWith('/status')) result = { networks: ['usdt.erc20', 'usdc.erc20'].map(rail => ({ network: 'ethereum', token: rail === 'usdc.erc20' ? transfer.token : '0xdac17f958d2ee523a2206206994597c13d831ec7', rail, state: 'ready', cursor: rail === 'usdc.erc20' ? '200' : '100', updatedAt: new Date().toISOString(), errorCode: '', connection: 'polling' })) }
  else if (path.endsWith('/resolution')) { audit.dismissed = body.action === 'dismiss'; result = { orderId: body.orderId ?? '', transferId: transfer.id, credited: !audit.dismissed, replayed: false } }
  else if (path.endsWith('/review')) result = { items: audit.empty || audit.dismissed ? [] : [transfer], nextCursor: '' }
  else if (path.endsWith('/check')) result = { network: body.network, token: transfer.token, rail: body.rail, success: true, errorCode: '' }
  else if (path.endsWith('/usdt')) result = outgoing.method === 'PUT' ? { ...body, networks: body.networks.map((n: typeof config.networks[number]) => ({ ...n, endpoints: n.endpoints.map(e => ({ ...e, rpcUrl: '', apiKey: '' })) })) } : config
  return new Response(JSON.stringify(result), { status: failed ? 502 : 200, headers: { 'Content-Type': 'application/json' } })
}
const payment = { id: 'payment', provider: 'builtin_usdt', status: 'expired' } as AdminUserDetail['payments'][number]
const root = defineComponent({ setup() {
  const current = shallowRef<typeof payment | null>(payment)
  return () => h(UApp, { locale: params.has('zh') ? zh_cn : en }, { default: () => h('main', { class: 'browser-admin', style: 'padding:24px;max-width:1200px;margin:auto' }, params.has('credit') ? h(AdminPaymentCreditDialog, { payment: current.value, busy: false, error: params.has('action-error') ? t('errors.adminAction') : null, onClose: () => { current.value = null }, onConfirm: (id: string, reason: string) => { audit.calls.push({ path: id ? 'settlement' : 'courtesy', body: { id, reason }, signed: true }) } }) : h(AdminUSDTPage)) })
} })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: root }] })
createApp(root).use(createPinia()).use(router).use(ui).mount('#app')
