import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import type { AdminEntitlement } from '@/api/types'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme, setBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import { effectivePurchaseStatus } from '@/utils/purchaseValidity'
import ValidityAudit from './ValidityAudit.vue'
import '@/styles/main.css'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme(); setBrowserTheme(params.has('light') ? 'light' : 'dark')
setLocale(params.has('zh') ? 'zh-CN' : 'en')
document.cookie = `txc_admin_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
document.cookie = `txc_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
const now = Date.now()
const iso = (offset: number) => new Date(now + offset).toISOString()
const money = (minor: string) => ({ currency: 'TXB', minor, display: `${Number(minor) / 100} TXB` })
const statuses = ['active', 'active', 'cancelled', 'failed', 'queued'] as const
const items: AdminEntitlement[] = statuses.map((status, index) => ({
  id: `00000000-0000-4000-8000-00000000000${index}`, userId: '99101', comboId: '00000000-0000-4000-8000-000000000010',
  comboName: ['Pacific Monthly', 'Next Monthly', 'Cancelled Monthly', 'Failed Monthly', 'Old Queued Monthly'][index]!, status,
  price: money('1250'), grossPrice: money('1250'), couponDiscount: money('0'), couponGrantId: null,
  validFrom: iso(index === 1 ? 3_600_000 : -86_400_000),
  validUntil: iso(index === 1 ? 86_400_000 : index === 0 && params.has('boundary') ? 15_000 : -1000),
  autoRenewEnabled: false, trafficLimitBytes: '107374182400', resetStrategy: 'MONTH_ROLLING',
  squadUuids: [], createdAt: iso(-86_400_000), updatedAt: iso(-86_400_000),
}))
const calls: string[] = []
Object.assign(window, { validityAuditCalls: calls })
window.fetch = async input => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  calls.push(url.pathname)
  if (params.has('error') && url.pathname.endsWith('/dashboard')) return new Response('{}', { status: 503 })
  const filter = url.searchParams.get('status')
  let value: unknown = { items: [] }
  if (url.pathname.endsWith('/dashboard')) value = { activePurchase: items[0], queuedPurchase: null, balance: money('2500'), user: { id: '99101', role: 'member' }, statistics: null, subscriptionUrl: null, statisticsStale: false, statisticsWarning: '', autoRenewalFailure: null, fetchedAt: iso(0) }
  else if (url.pathname.endsWith('/catalog')) value = { combos: [], addons: [], nodes: [] }
  else if (url.pathname.endsWith('/admin/entitlements')) value = { items: filter ? items.filter(item => effectivePurchaseStatus(item, Date.now()) === filter) : items, page: { nextCursor: null } }
  else if (url.pathname.endsWith('/admin/users/99101')) value = {
    user: { id: '99101', displayName: 'Marina', telegramId: '99101', role: 'member', createdAt: iso(-86_400_000), updatedAt: iso(0) },
    balance: money('2500'), synchronization: { status: 'synchronized', remoteUserId: null, lastError: null },
    activeCombo: items[0], entitlements: items, embyAccounts: [], payments: [], refunds: [], operations: [], ipBlocks: [], couponWallet: [],
    affiliateHistory: { items: [], page: { nextCursor: null } }, temporaryBan: null, autoTrafficResetEnabled: false, pmConversation: null,
  }
  else if (url.pathname.endsWith('/automatic-renewal')) value = { enabled: false, eligible: false, ineligibleReason: 'unavailable' }
  else if (url.pathname.endsWith('/refund/eligibility')) value = { eligible: false }
  return new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } })
}
const app = createApp(ValidityAudit, { items, inventory: params.has('inventory'), profile: params.has('profile') })
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: ValidityAudit }] })).use(ui).mount('#app')
