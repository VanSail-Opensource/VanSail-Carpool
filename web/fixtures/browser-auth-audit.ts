import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory, RouterView } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { browserAccessApi, type BrowserSession } from '@/api/browserAccess'
import { ApiError } from '@/api/http'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import BrowserAuthAudit from './BrowserAuthAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience(params.has('link') ? 'miniapp' : 'browser')
if (params.has('link')) await import('@/styles/main.css')
else initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const state = { configReads: 0, links: 0, exchanges: 0, copied: '', usedTokens: [] as string[] }
const config = { miniAppUrl: params.has('empty') ? '' : 'https://t.me/vansail_carpool_bot?startapp' }
const admin: BrowserSession = {
  authenticated: true, kind: 'admin', expiresAt: new Date(Date.now() + 86400000).toISOString(),
  user: { id: 'audit-admin', telegramId: '42', firstName: 'Ming', lastName: '', telegramUsername: 'vansail',
    username: null, role: 'admin', onboardingState: 'intro', groupJoined: false, channelJoined: false,
    policyAcceptedAt: null, agreementRevision: 0, recoveryReason: '',
    createdAt: '2026-10-14T00:00:00Z', updatedAt: '2026-10-14T00:00:00Z' },
}
const pause = async () => { if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, Number(params.get('slow')) || 3000)) }
browserAccessApi.config = async () => {
  state.configReads++
  await pause()
  if (params.has('error') && state.configReads === 1) throw new ApiError(503, { code: 'BROWSER_CONFIG_UNAVAILABLE', message: '' })
  return config
}
browserAccessApi.session = async () => ({ authenticated: false, kind: 'anonymous', miniAppUrl: config.miniAppUrl })
browserAccessApi.renew = async () => admin
browserAccessApi.logout = async () => undefined
browserAccessApi.createLink = async () => {
  state.links++
  await pause()
  if (params.has('error') && state.links === 1) throw new ApiError(503, { code: 'BROWSER_LINK_FAILED', message: '' })
  return { url: `https://panel.example/admin/access#token=audit-link-${state.links}`,
    expiresAt: new Date(Date.now() + (params.has('expired') && state.links === 1 ? -1000 : 300000)).toISOString() }
}
browserAccessApi.exchange = async token => {
  state.exchanges++
  await pause()
  if (params.has('invalid') || state.usedTokens.includes(token)) throw new ApiError(401, { code: 'BROWSER_LINK_INVALID', message: '' })
  state.usedTokens.push(token)
  return { ...admin, returnTo: '/audit-complete' }
}
Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (text: string) => { state.copied = text } } })
Object.assign(window, { __browserAuthAudit: state })
const router = createRouter({ history: createWebHistory(), routes: [
  { path: '/audit-complete', component: { render: () => h('p', t('browser.console')) } },
  { path: '/:pathMatch(.*)*', component: BrowserAuthAudit },
] })
const app = createApp({ render: () => h(RouterView) })
app.config.globalProperties.$t = t
app.use(createPinia()).use(router).use(ui).mount('#app')
