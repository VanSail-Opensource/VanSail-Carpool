import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme, setBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import type { SquadProduct } from '@/api/types'
import PAYGAudit from './PAYGAudit.vue'
import '@/styles/main.css'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
const admin = params.has('admin')
setExperience(admin ? 'browser' : 'miniapp')
initializeBrowserTheme(); setBrowserTheme(params.has('light') ? 'light' : 'dark')
setLocale(params.has('zh') ? 'zh-CN' : 'en')
document.cookie = `txc_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
document.cookie = `txc_admin_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
const money = (minor: string) => ({ currency: 'TXB', minor, display: `${(Number(minor) / 100).toFixed(2)} TXB` })
const stamp = '2026-10-24T00:00:00Z'
const profile = { type: 'international_network', portMbps: 1000, countryCode: 'JP', upstreamCarriers: ['KDDI'] }
const squad = (index: number): SquadProduct => ({ id: `00000000-0000-4000-8000-00000000000${index}`, groupId: `00000000-0000-4000-8000-00000000000${index}`, name: ['Tokyo Direct', 'Singapore Transit', 'Los Angeles', 'Frankfurt', 'Hong Kong', 'London'][index]!, description: '', profile: profile as unknown as SquadProduct['profile'], billingMode: 'payg', paygPrice: money('125'), price: money('0'), visible: false, upstreamPresent: true, stockHeldByCurrentUser: false, stockLimit: 0, stockRemaining: 0, activationRequired: true, accessibleNodes: [], createdAt: stamp, updatedAt: stamp })
const statuses = ['available', 'insufficient_balance', 'combo_covered', 'queued_combo', 'unavailable', 'sync_pending'] as const
const calls: string[] = []
Object.assign(window, { paygAuditCalls: calls })
window.fetch = async (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  calls.push(url.pathname)
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 2500))
  if (params.has('error') && url.pathname === '/api/v1/payg') return new Response(JSON.stringify({ code: 'XNODE_UNAVAILABLE', message: 'fixture unavailable' }), { status: 503 })
  let value: unknown = { items: [] }
  if (url.pathname === '/api/v1/payg') value = { balance: money('2500'), asOf: stamp, windowStart: '2026-10-21T00:00:00Z', squads: params.has('empty') ? [] : statuses.map((status, index) => ({ squad: squad(index), status, trafficBytes: '1610612736', spent: money('250') })), trafficBytes: params.has('empty') ? '0' : '9663676416', spent: money(params.has('empty') ? '0' : '1500') }
  else if (url.pathname === '/api/v1/catalog') value = { combos: [], addons: [], nodes: [] }
  else if (url.pathname === '/api/v1/dashboard') value = { activePurchase: null, queuedPurchase: null, balance: money('2500'), user: { id: '99101', role: 'member' } }
  else if (url.pathname === '/api/v1/balance') value = { balance: money('2500'), methods: [] }
  void init
  return new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } })
}
const props = { admin, squad: { ...squad(0), profile: profile as unknown as SquadProduct['profile'] } }
const app = createApp(PAYGAudit, props)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: PAYGAudit }] })).use(ui).mount('#app')
