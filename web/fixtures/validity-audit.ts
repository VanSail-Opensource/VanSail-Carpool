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
const combos = [
  { id: '00000000-0000-4000-8000-000000000010', name: 'Pacific Monthly', description: '', price: money('1250'), validityDays: 30, trafficLimitBytes: '107374182400', resetStrategy: 'MONTH_ROLLING', includedSquads: [], active: true, createdAt: iso(-86_400_000), updatedAt: iso(0) },
  { id: '00000000-0000-4000-8000-000000000011', name: 'Pacific Quarterly', description: '', price: money('3400'), validityDays: 90, trafficLimitBytes: '322122547200', resetStrategy: 'MONTH_ROLLING', includedSquads: [], active: true, createdAt: iso(-86_400_000), updatedAt: iso(0) },
]
const squads = [
  { id: 'squad-tokyo', groupId: 'group-tokyo', name: 'Tokyo Direct', description: '', profile: null, price: money('500'), visible: true, activationRequired: false, accessibleNodes: [], upstreamPresent: true, stockHeldByCurrentUser: false, createdAt: iso(-86_400_000), updatedAt: iso(0) },
  { id: 'squad-singapore', groupId: 'group-singapore', name: 'Singapore Transit', description: '', profile: null, price: money('500'), visible: true, activationRequired: false, accessibleNodes: [], upstreamPresent: true, stockHeldByCurrentUser: false, createdAt: iso(-86_400_000), updatedAt: iso(0) },
]
const hourlyView = {
  policy: { version: 'fixture-policy-1', policy: { salesEnabled: true, dailyAllowanceBytes: '10000000000', reducedRateBytesPerSecond: '1250000', extraGBPriceTXBMinor: '125', activationHours: 168, cooldownSeconds: 86400 } },
  balance: money('2500'),
  extraPreference: { extraEnabled: true, acceptedExtraPriceTXBMinor: '125', updatedAt: iso(0) },
  usage: {
    dayUTC: new Date(now).toISOString().slice(0, 10), freeAllowanceBytes: '10000000000', freeUsedBytes: '2500000000', freeReservedBytes: '0',
    extraFundedBytes: '2000000000', extraUsedBytes: '1000000000', extraReservedBytes: '0', extraExpiresAt: iso(86_400_000),
    reducedRateBytesPerSecond: '1250000', accountingPending: false,
  },
  squads: [
    { enrollmentId: 'enrollment-tokyo', groupId: 'group-tokyo', name: 'Tokyo Direct', hourlyPrice: money('5'), offerEnabled: true, status: 'active', paidThrough: iso(3_600_000), cooldownUntil: null, activatedAt: iso(-3_600_000), disabledAt: null, createdAt: iso(-86_400_000) },
    { enrollmentId: 'enrollment-singapore', groupId: 'group-singapore', name: 'Singapore Transit', hourlyPrice: money('10'), offerEnabled: false, status: 'disabled', paidThrough: null, cooldownUntil: null, activatedAt: iso(-172_800_000), disabledAt: iso(-86_400_000), createdAt: iso(-172_800_000) },
  ],
}
const calls: string[] = []
Object.assign(window, { validityAuditCalls: calls })
window.fetch = async (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  const method = (init?.method ?? 'GET').toUpperCase()
  const body = typeof init?.body === 'string' && init.body ? JSON.parse(init.body) as Record<string, unknown> : {}
  const path = url.pathname
  calls.push(path)
  const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
  if (params.has('error') && path.endsWith('/dashboard')) return json({}, 503)
  if (params.has('hourly-error') && path.endsWith('/hourly') && method === 'GET') return json({ code: 'HOURLY_UNAVAILABLE', message: 'fixture' }, 503)
  if (path.endsWith('/hourly') && path.includes('/admin/users/99101') && method === 'GET') return json(hourlyView)
  if (path.endsWith('/admin/combos')) return json({ items: combos })
  if (path.endsWith('/admin/squad-products')) return json({ items: squads })
  if (/\/admin\/users\/99101\/entitlements\/[^/]+\/refund$/.test(path) && method === 'GET') {
    return json({ purchaseId: items[0]!.id, paid: money('1250'), allocatedTrafficBytes: '107374182400', usedTrafficBytes: '21474836480', suggestedRefund: money('1000'), quotedAt: iso(0), reasonCode: null })
  }
  if (path.endsWith('/admin/users/99101/entitlements') && method === 'POST') {
    if (params.has('overlap')) return json({ code: 'ENTITLEMENT_OVERLAP', message: 'The requested term overlaps an existing entitlement.' }, 409)
    const created: AdminEntitlement = {
      id: `00000000-0000-4000-8000-0000000000${20 + items.length}`, userId: '99101',
      comboId: String(body.comboId), comboName: combos.find(combo => combo.id === body.comboId)?.name ?? 'Pacific Monthly',
      price: money('0'), grossPrice: money('0'), couponDiscount: money('0'), couponGrantId: null,
      validFrom: String(body.validFrom), validUntil: String(body.validUntil), status: 'active',
      trafficLimitBytes: String(body.trafficLimitBytes), resetStrategy: body.resetStrategy as AdminEntitlement['resetStrategy'],
      squadUuids: (body.squadUuids as string[] | undefined) ?? [], createdAt: iso(0), updatedAt: iso(0),
    }
    items.push(created)
    return json(created, 201)
  }
  if (/\/admin\/users\/99101\/entitlements\/[^/]+$/.test(path) && method === 'PUT') {
    const id = path.split('/').at(-1)
    const target = items.find(item => item.id === id)
    if (!target) return json({ code: 'NOT_FOUND', message: 'fixture' }, 404)
    Object.assign(target, body, { updatedAt: iso(0) })
    return json(target)
  }
  if (/\/admin\/users\/99101\/entitlements\/[^/]+\/refund$/.test(path) && method === 'POST') {
    return json({ id: 'refund-operation', kind: 'entitlement_refund', status: 'queued', createdAt: iso(0), updatedAt: iso(0), completedAt: null }, 202)
  }
  if (/\/admin\/users\/99101\/hourly\/enrollments\/[^/]+\/disable$/.test(path) && method === 'POST') {
    if (params.has('conflict')) return json({ code: 'HOURLY_TERMS_CHANGED', message: 'fixture' }, 409)
    const id = path.split('/').at(-2)
    const squad = hourlyView.squads.find(value => value.enrollmentId === id)
    if (squad) { squad.status = 'disabled'; squad.disabledAt = iso(0); squad.paidThrough = null }
    return json({ id: 'hourly-disable-operation', kind: 'hourly_access', status: 'queued', createdAt: iso(0), updatedAt: iso(0), completedAt: null }, 202)
  }
  if (path.endsWith('/admin/users/99101')) {
    return json({
      user: { id: '99101', displayName: 'Marina', telegramId: '99101', role: 'member', createdAt: iso(-86_400_000), updatedAt: iso(0) },
      balance: money('2500'), synchronization: { status: 'synchronized', remoteUserId: null, lastError: null },
      activeCombo: items[0], entitlements: items, embyAccounts: [], payments: [], refunds: [], operations: [], ipBlocks: [], couponWallet: [],
      affiliateHistory: { items: [], page: { nextCursor: null } }, temporaryBan: null, pmConversation: null,
    })
  }
  if (path.endsWith('/automatic-renewal')) return json({ enabled: false, eligible: false, ineligibleReason: 'unavailable' })
  if (path.endsWith('/refund/eligibility')) return json({ eligible: false })
  const filter = url.searchParams.get('status')
  let value: unknown = { items: [] }
  if (path.endsWith('/dashboard')) value = { activePurchase: items[0], queuedPurchase: null, balance: money('2500'), user: { id: '99101', role: 'member' }, statistics: null, subscriptionUrl: null, statisticsStale: false, statisticsWarning: '', autoRenewalFailure: null, fetchedAt: iso(0) }
  else if (path.endsWith('/catalog')) value = { combos: [], addons: [], nodes: [] }
  else if (path.endsWith('/admin/entitlements')) value = { items: filter ? items.filter(item => effectivePurchaseStatus(item, Date.now()) === filter) : items, page: { nextCursor: null } }
  return json(value)
}
const app = createApp(ValidityAudit, { items, inventory: params.has('inventory'), profile: params.has('profile') })
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: ValidityAudit }] })).use(ui).mount('#app')
