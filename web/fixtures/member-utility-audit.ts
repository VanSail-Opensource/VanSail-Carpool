import '@/styles/main.css'
import { createApp, h, reactive } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import { featuresApi } from '@/api/features'
import { ApiError } from '@/api/http'
import { browserAccessApi } from '@/api/browserAccess'
import { ipLookupApi } from '@/api/ipLookup'
import { preferencesApi } from '@/api/preferences'
import { memberOperationsApi } from '@/api/memberOperations'
import type { OperationReceipt } from '@/api/types'
import { useSessionStore, type SessionStatus } from '@/stores/session'
import { usePreferencesStore } from '@/stores/preferences'
import { setExperience } from '@/utils/experience'
import { setLocale, t } from '@/i18n'
import EmbyView from '@/views/EmbyView.vue'
import AffiliatesView from '@/views/AffiliatesView.vue'
import AdminAccessPanel from '@/components/settings/AdminAccessPanel.vue'
import MemberUtilityAudit from './MemberUtilityAudit.vue'
import { memberUtilityData, utilityMoney } from './member-utility-data'

const params = new URLSearchParams(location.search)
const page = params.get('page') ?? 'emby'
setExperience('miniapp')
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const fixture = memberUtilityData(params)
const state = reactive({
  recover: false, reads: [] as string[], writes: [] as string[], unexpected: [] as string[], copies: [] as string[],
  retries: 0, links: 0, phase: (params.has('loading') ? 'loading' : 'error') as SessionStatus,
})
Object.assign(window, { __memberUtilityAudit: state })
const nativeFetch = window.fetch.bind(window)
window.fetch = (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  if (url.pathname.startsWith('/api/')) {
    state.unexpected.push(url.pathname)
    return Promise.reject(new Error('Unstubbed utility-fixture API request'))
  }
  return nativeFetch(input, init)
}
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { state.copies.push(value) } } })
if (page === 'entry' && params.has('error')) {
  Object.assign(window, { Telegram: { WebApp: { platform: 'desktop', version: '9.0', initData: 'constructed-only', initDataUnsafe: {} } } })
}
async function read(name: string): Promise<void> {
  state.reads.push(name)
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 1200))
  if (params.has('error') && !state.recover) throw new ApiError(503, { code: 'UNAVAILABLE', message: '' })
}
const receipt = (kind: string): OperationReceipt => ({ id: 'constructed-utility-operation', kind, status: 'succeeded', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(), completedAt: new Date().toISOString() })
featuresApi.getEmby = async () => { await read('emby'); return structuredClone(fixture.emby) }
featuresApi.getAffiliates = async () => { await read('affiliates'); return structuredClone(fixture.affiliates) }
featuresApi.getAffiliateReferrals = async value => { await read(`referrals:${value}`); return fixture.referrals(value) }
featuresApi.updateEmbyPreferences = async value => {
  state.writes.push('emby-preferences')
  if (fixture.emby.account) Object.assign(fixture.emby.account, value)
  return receipt('emby_preferences')
}
featuresApi.changeEmbyPassword = async () => { state.writes.push('emby-password'); return receipt('emby_password') }
featuresApi.setupEmby = async value => {
  state.writes.push('emby-setup')
  fixture.emby.account = { id: 'constructed-emby', username: 'sail_traveler', status: 'active', maxParentalRating: value.maxParentalRating, disabledLibraryIds: value.disabledLibraryIds, retryable: false, updatedAt: new Date().toISOString() }
  return receipt('emby_setup')
}
memberOperationsApi.getOperation = async () => receipt('emby_preferences')
browserAccessApi.createLink = async () => {
  await read('admin-link'); state.links++
  if (params.has('empty')) throw new ApiError(503, { code: 'UNAVAILABLE', message: '' })
  return { url: `https://panel.example/admin/access#token=constructed-preview-${state.links}`, expiresAt: new Date(Date.now() + 120_000).toISOString() }
}
api.getCommunityAccess = async () => ({ activeCombo: true })
api.getBalance = async () => ({ balance: utilityMoney('98765'), paymentMethods: [], pendingPaymentOrder: null, addAmountLimits: { minimum: utilityMoney('100'), maximum: utilityMoney('1000000'), updatedAt: new Date().toISOString() } })
ipLookupApi.state = async () => ({ enabled: false }) as Awaited<ReturnType<typeof ipLookupApi.state>>
preferencesApi.get = async () => structuredClone(fixture.preferences)
preferencesApi.update = async value => ({ ...fixture.preferences, ...value, paygSquads: { ...fixture.preferences.paygSquads, ...value.paygSquads }, notifications: { ...fixture.preferences.notifications, ...value.notifications } })
const router = createRouter({ history: createMemoryHistory(), routes: [
  { path: '/emby', component: EmbyView },
  { path: '/affiliates', component: AffiliatesView },
  { path: '/settings/admin-access', component: AdminAccessPanel },
  { path: '/:pathMatch(.*)*', component: { template: '<div />' } },
] })
function retry(): void {
  state.retries++; state.phase = 'loading'
  globalThis.setTimeout(() => { state.phase = 'error' }, 300)
}
const app = createApp({ setup: () => () => h(MemberUtilityAudit, {
  page, phase: state.phase, message: t(params.has('error') ? 'auth.authenticationFailed' : 'auth.openInTelegram'),
  code: params.has('recovery') ? 'ACCOUNT_RECOVERY_PENDING' : undefined, onRetry: retry,
}) })
app.config.globalProperties.$t = t
app.use(createPinia())
const session = useSessionStore()
session.session = fixture.session
session.status = 'ready'
usePreferencesStore().bind(fixture.session.user.id)
app.use(router).use(ui)
await router.push(page === 'admin-access' ? '/settings/admin-access' : page === 'affiliates' ? '/affiliates' : '/emby')
await router.isReady()
app.mount('#app')
