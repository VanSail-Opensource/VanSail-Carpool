import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import UsageBreakdownAudit from './UsageBreakdownAudit.vue'
import { usageBreakdownFixture } from './usage-breakdown-data'
import { installInteractionFeedback } from './interaction-feedback'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme, setBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import '@/styles/main.css'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience(params.has('browser') ? 'browser' : 'miniapp')
initializeBrowserTheme(); setBrowserTheme(params.has('light') ? 'light' : 'dark')
setLocale(params.has('zh') ? 'zh-CN' : 'en')
document.cookie = `txc_request_key=${'A'.repeat(43)}; path=/; SameSite=Strict`
const feedback = installInteractionFeedback()
const state = { params, recover: false, calls: [] as string[], haptics: feedback.haptics, feedback, snapshot: usageBreakdownFixture(params) }
const originalFetch = window.fetch.bind(window)
window.fetch = async (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
  if (url.pathname !== '/api/v1/usage/breakdown') return originalFetch(input, init)
  state.calls.push(url.pathname)
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 1500))
  const failed = params.has('error') && !state.recover
  state.snapshot = usageBreakdownFixture(params)
  let value = state.snapshot
  if (params.has('invalid')) value = { ...value, cost: { ...value.cost, knownTotalMinor: { numerator: '1', denominator: '0' } } }
  if (params.has('invalid-total')) value = { ...value, cost: { ...value.cost, actualMinor: '0' } }
  if (params.has('invalid-positive-count')) value = { ...value, positiveEntries: { ...value.positiveEntries, cost: (BigInt(value.positiveEntries.cost) + 1n).toString() } }
  return new Response(JSON.stringify(failed ? { code: 'XNODE_UNAVAILABLE', message: 'Synthetic history failure' } : value), { status: failed ? 503 : 200, headers: { 'Content-Type': 'application/json' } })
}
Object.assign(window, { __usageBreakdownAudit: state })
const app = createApp(UsageBreakdownAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: UsageBreakdownAudit }] })).use(ui).mount('#app')
