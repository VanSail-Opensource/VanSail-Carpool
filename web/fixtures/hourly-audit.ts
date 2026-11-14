import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import HourlyAudit from './HourlyAudit.vue'
import { hourlyFixture, hourlyMoney } from './hourly-data'
import { setExperience } from '@/utils/experience'
import { installHapticClickFeedback } from '@/utils/telegram'
import { initializeBrowserTheme, setBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import type { HourlyPolicy, HourlyExtraPreference } from '@/api/hourly'
import '@/styles/main.css'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
if (params.has('reduced') || params.has('reduced-motion')) {
  const nativeMatchMedia = window.matchMedia.bind(window)
  window.matchMedia = query => {
    const media = nativeMatchMedia(query)
    if (query === '(prefers-reduced-motion: reduce)' && !media.matches) Object.defineProperty(media, 'matches', { value: true })
    return media
  }
}
const admin = params.has('admin')
setExperience(admin ? 'browser' : 'miniapp')
initializeBrowserTheme(); setBrowserTheme(params.has('light') ? 'light' : 'dark')
setLocale(params.has('zh') ? 'zh-CN' : 'en')
document.cookie = `txc_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
document.cookie = `txc_admin_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
const fixture = hourlyFixture(params)
const calls: { path: string; method: string; body: unknown; key: string | null }[] = []
const haptics: string[] = []
window.Telegram = { WebApp: { version: '9.0', HapticFeedback: { impactOccurred: (value: string) => haptics.push(value), notificationOccurred: (value: string) => haptics.push(value), selectionChanged: () => haptics.push('selection') } } } as unknown as typeof window.Telegram
if (!admin) installHapticClickFeedback()
const audit = { calls, haptics, recover: false, fixture }
Object.assign(window, { __hourlyAudit: audit })
let action: { groupId: string; disable: boolean } | undefined
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
function receipt(status: string) { return { id: 'hourly-operation', kind: 'hourly_access', status, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(), completedAt: null, errorCode: status === 'failed' ? 'INSUFFICIENT_BALANCE' : null } }
window.fetch = async (input, init) => {
  const request = input instanceof Request ? input : undefined
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  const method = init?.method ?? request?.method ?? 'GET'
  const raw = typeof init?.body === 'string' ? init.body : request ? await request.text() : ''
  const body = raw ? JSON.parse(raw) as Record<string, unknown> : {}
  calls.push({ path: url.pathname, method, body, key: new Headers(init?.headers ?? request?.headers).get('Idempotency-Key') })
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 2000))
  if (!audit.recover && params.has('error') && method === 'GET') return response({ code: 'XNODE_UNAVAILABLE', message: 'fixture' }, 503)
  if (params.has('conflict') && method !== 'GET') return response({ code: 'CONFLICT', message: 'fixture' }, 409)
  if (url.pathname === '/api/v1/hourly' && method === 'GET') return response(fixture.view)
  if (url.pathname === '/api/v1/admin/hourly-policy') {
    if (method === 'PUT') { fixture.policy.policy = body.policy as HourlyPolicy; fixture.policy.version += '-next'; fixture.view.policy = fixture.policy }
    return response(fixture.policy)
  }
  if (url.pathname === '/api/v1/admin/hourly-offers') return response(fixture.offers)
  if (url.pathname === '/api/v1/admin/squad-products') return response({ items: fixture.squads })
  if (url.pathname.startsWith('/api/v1/admin/hourly-offers/') && method === 'PUT') {
    const groupId = decodeURIComponent(url.pathname.split('/').at(-1)!)
    const offer = { groupId, enabled: Boolean(body.enabled), hourlyPrice: hourlyMoney(String(body.hourlyPriceTXBMinor)), requiredHours: Number(body.requiredHours), revision: String(BigInt(String(body.revision)) + 1n), createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
    fixture.offers = [...fixture.offers.filter(value => value.groupId !== groupId), offer]
    return response(offer)
  }
  if (url.pathname === '/api/v1/hourly/extra-traffic') {
    fixture.view.extraPreference = { extraEnabled: Boolean(body.enabled), acceptedExtraPriceTXBMinor: body.acceptedPriceTXBMinor as string | undefined, updatedAt: new Date().toISOString() } satisfies HourlyExtraPreference
    return response(fixture.view.extraPreference)
  }
  if (url.pathname === '/api/v1/hourly/activate' || /\/enrollments\/[^/]+\/disable$/.test(url.pathname)) {
    const id = url.pathname.split('/').at(-2)
    action = { groupId: body.groupId as string ?? fixture.view.squads.find(value => value.enrollment?.id === id)?.offer.groupId ?? '', disable: url.pathname.endsWith('/disable') }
    return response(receipt('queued'), 202)
  }
  if (url.pathname === '/api/v1/operations/hourly-operation') {
    if (params.has('queue')) return response(receipt('processing'))
    if (params.has('failed')) return response(receipt('failed'))
    const item = fixture.view.squads.find(value => value.offer.groupId === action?.groupId)
    if (item && action) {
      const now = new Date().toISOString()
      item.enrollment = { id: `enrollment-${item.offer.groupId}`, groupId: item.offer.groupId, status: action.disable ? 'disabled' : 'active', activatedAt: now, paidThrough: new Date(Date.now() + 3600000).toISOString(), disabledAt: action.disable ? now : null, cooldownUntil: action.disable ? new Date(Date.now() + 86400000).toISOString() : null, createdAt: now, updatedAt: now }
      item.cooldownUntil = item.enrollment.cooldownUntil; item.ready = !action.disable
    }
    return response(receipt('succeeded'))
  }
  if (url.pathname === '/api/v1/payg') return response({
    balance: hourlyMoney('100000'), asOf: new Date().toISOString(), windowStart: new Date(Date.now() - 72 * 3600000).toISOString(),
    squads: fixture.squads.map((squad, index) => ({
      squad: { ...squad, billingMode: 'payg', paygState: 'normal', features: index === 1
        ? [{ icon: 'route', title: 'Regional transit', detail: 'Direct access across supported destinations' }]
        : [{ icon: 'rocket-launch', title: 'Fast access' }, { icon: 'shield', title: 'Private routing', detail: 'Owner-scoped configuration' }] },
      status: index === 1 ? 'unavailable' : 'available', trafficBytes: String((index + 1) * 1073741824), spent: hourlyMoney(String((index + 1) * 100)),
    })),
    trafficBytes: fixture.squads.length ? '6442450944' : '0', spent: hourlyMoney(fixture.squads.length ? '600' : '0'),
  })
  return response({ items: [] })
}
const app = createApp(HourlyAudit, { admin, catalog: params.has('catalog') })
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: HourlyAudit }] })).use(ui).mount('#app')
