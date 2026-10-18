import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import PublicSubscriptionPage from '@/publicSubscription/PublicSubscriptionPage.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
const token = 'a'.repeat(43)
setExperience('browser'); initializeBrowserTheme(); setLocale(params.has('zh') ? 'zh-CN' : 'en')
const trace = { calls: 0, credentials: [] as unknown[], copied: '', urls: [] as string[] }
const original = window.fetch.bind(window)
window.fetch = async (input, init) => {
  const url = String(input)
  if (!url.includes('/api/v1/subscriptions/')) return original(input, init)
  trace.calls++; trace.credentials.push(init?.credentials); trace.urls.push(url)
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 2000))
  if (params.has('error') && trace.calls === 1) return new Response('{}', { status: 404, headers: { 'Content-Type': 'application/json' } })
  return new Response(JSON.stringify({ username: 'alice', url: `${location.origin}/subscription/${token}`, usedBytes: '1610612736', limitBytes: params.has('unlimited') ? '0' : '10737418240', expiresAt: '2026-11-15T00:00:00Z', active: !params.has('empty'), incomplete: params.has('partial'), formats: { base64: !params.has('empty') && !params.has('formats-none'), stash: !params.has('empty') && !params.has('formats-none'), native: !params.has('empty') && !params.has('formats-none'), xray: !params.has('empty') && !params.has('formats-none'), 'sing-box': !params.has('empty') && !params.has('formats-none'), clash: !params.has('empty') && !params.has('formats-none') } }), { headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } })
}
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { trace.copied = value } } })
Object.assign(window, { __publicSubscriptionAudit: trace })
const app = createApp(PublicSubscriptionPage, { token: params.has('invalid') ? 'invalid' : token })
app.config.globalProperties.$t = t
app.use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: PublicSubscriptionPage }] })).use(ui).mount('#app')
