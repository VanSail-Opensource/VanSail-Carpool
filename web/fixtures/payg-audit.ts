import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { installHapticClickFeedback } from '@/utils/telegram'
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
const profiles: SquadProduct['profile'][] = [
  profile as SquadProduct['profile'],
  { type: 'china_optimized', portMbps: 500, countryCode: 'SG', ct: 'CN2 GIA', cu: '9929', cm: 'CMIN2' },
  { type: 'broadband', portMbps: 300, isp: 'Harbor ISP', dynamic: true, location: 'Los Angeles' },
  null,
  { type: 'international_network', portMbps: null, countryCode: 'HK', upstreamCarriers: ['NTT Communications International Transit', 'Hurricane Electric'] },
  profile as SquadProduct['profile'],
]
const squad = (index: number): SquadProduct => ({
  id: `00000000-0000-4000-8000-00000000000${index}`, groupId: `00000000-0000-4000-8000-00000000000${index}`,
  name: ['Tokyo Direct', 'Singapore Transit', 'Los Angeles', 'Frankfurt', 'Hong Kong International Transit', 'London'][index]!,
  description: '', profile: profiles[index] ?? null, billingMode: 'payg', paygPrice: money('125'), price: money('0'),
  visible: true, upstreamPresent: true, stockHeldByCurrentUser: false, stockLimit: 100, stockRemaining: 80,
  activationRequired: false, accessibleNodes: [{ uuid: '00000000-0000-4000-8000-000000000099', name: 'Internal Tokyo server', countryCode: 'JP', consumptionMultiplier: 1.5, providerName: null }],
  createdAt: stamp, updatedAt: stamp,
})
const statuses = ['available', 'insufficient_balance', 'combo_covered', 'queued_combo', 'unavailable', 'sync_pending'] as const
const haptics: string[] = []
window.Telegram = { WebApp: { version: '9.0', HapticFeedback: { impactOccurred: value => haptics.push(value), notificationOccurred: value => haptics.push(value), selectionChanged: () => haptics.push('selection') } } } as unknown as typeof window.Telegram
if (!admin) installHapticClickFeedback()
Object.assign(window, { paygAuditHaptics: haptics })
const calls: string[] = []
Object.assign(window, { paygAuditCalls: calls })
let paygReads = 0
window.fetch = async (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  calls.push(url.pathname)
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 2500))
  if (url.pathname === '/api/v1/payg') {
    paygReads++
    const recovered = (window as unknown as { paygAuditRecover?: boolean }).paygAuditRecover
    if (!recovered && (params.has('error') || params.has('refresh-error') && paygReads > 1)) return new Response(JSON.stringify({ code: 'XNODE_UNAVAILABLE', message: 'fixture unavailable' }), { status: 503 })
  }
  let value: unknown = { items: [] }
  if (url.pathname === '/api/v1/payg') value = { balance: money('2500'), asOf: stamp, windowStart: '2026-10-21T00:00:00Z', squads: params.has('empty') ? [] : statuses.map((status, index) => ({ squad: squad(index), status, trafficBytes: '1610612736', spent: money('250') })), trafficBytes: params.has('empty') ? '0' : '9663676416', spent: money(params.has('empty') ? '0' : '1500') }
  else if (url.pathname === '/api/v1/catalog') {
    if (params.has('combo-error')) return new Response(JSON.stringify({ code: 'XNODE_UNAVAILABLE', message: 'fixture unavailable' }), { status: 503 })
    value = { combos: params.has('combo-empty') ? [] : [{ id: 'audit-combo', name: 'Standard', description: '', price: money('1000'), validityDays: 30, trafficLimitBytes: '107374182400', resetStrategy: 'MONTH_ROLLING', includedSquads: [], active: true, rolloverMinRemainingBps: 1000, createdAt: stamp, updatedAt: stamp }], addons: params.has('combo-empty') ? [] : statuses.map((_, index) => ({ ...squad(index), billingMode: 'combo' })), nodes: [] }
  }
  else if (url.pathname === '/api/v1/dashboard') value = { activePurchase: null, queuedPurchase: null, balance: money('2500'), user: { id: '99101', role: 'member' } }
  else if (url.pathname === '/api/v1/balance') value = { balance: money('2500'), methods: [] }
  void init
  return new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } })
}
const props = { admin, cards: params.has('cards'), squads: statuses.map((_, index) => squad(index)), squad: squad(0) }
const app = createApp(PAYGAudit, props)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: PAYGAudit }] })).use(ui).mount('#app')
