import '@/styles/main.css'
import { createApp, defineComponent, h } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api, ApiError } from '@/api/client'
import { featuresApi, type PublishedOnboarding } from '@/api/features'
import type { Session } from '@/api/types'
import { useSessionStore } from '@/stores/session'
import { setLocale, t } from '@/i18n'
import OnboardingFlow from '@/components/onboarding/OnboardingFlow.vue'
import OnboardingAudit from './OnboardingAudit.vue'

const params = new URLSearchParams(location.search)
if (params.has('zh')) setLocale('zh-CN')
const now = new Date().toISOString()
const initial: Session = { authenticated: true, user: { id: '99101', telegramId: '99101', firstName: 'Mira', lastName: 'Chen', telegramUsername: params.has('nickname') ? '' : 'mirachen', username: params.has('nickname') ? 'Mira Chen' : '@mirachen', role: 'user', onboardingState: params.has('intro') ? 'intro' : params.has('legacy') ? 'username' : 'agreement', groupJoined: false, channelJoined: false, policyAcceptedAt: null, agreementRevision: 0, recoveryReason: '', displayCurrency: 'TXB', createdAt: now, updatedAt: now } }
const trace = { calls: [] as string[], fail: params.has('error'), writes: [] as unknown[] }
Object.assign(window, { __onboardingAudit: trace })
const pause = () => new Promise(resolve => setTimeout(resolve, params.has('slow') ? 1800 : 30))
featuresApi.getPublishedOnboarding = async locale => {
  trace.calls.push('content'); await pause()
  if (trace.fail) throw new ApiError(503, { code: 'ONBOARDING_CONTENT_UNAVAILABLE', message: '' })
  return { locale, welcomeRevision: 1, agreementRevision: 2, welcome: [{ id: 'welcome', text: t('diy.title'), durationMs: 100 }], agreements: params.has('empty') ? [] : [{ id: 'terms', icon: 'shield-check', title: t('onboarding.agreementTitle'), body: t('onboarding.agreementTitle') }] } satisfies PublishedOnboarding
}
const pinia = createPinia()
const session = useSessionStore(pinia)
session.session = initial; session.status = 'ready'
session.updateSession = value => { session.session = value; session.status = 'ready' }
api.continueOnboarding = async () => { trace.calls.push('continue'); await pause(); return { ...initial, user: { ...initial.user, onboardingState: 'agreement' } } }
api.acceptAgreement = async (revision, agreementIds) => {
  trace.calls.push('agreement'); trace.writes.push({ revision, agreementIds }); await pause()
  if (params.has('save-error')) throw new ApiError(409, { code: 'ONBOARDING_STATE_CONFLICT', message: '' })
  return { ...initial, user: { ...initial.user, onboardingState: 'complete' } }
}
api.getMe = async () => session.session!
const router = createRouter({ history: createWebHistory(), routes: [{ path: '/fixtures/onboarding-audit.html', component: OnboardingFlow }, { path: '/home', component: defineComponent(() => () => h('div', t('onboarding.finish'))) }] })
const app = createApp(OnboardingAudit)
app.config.globalProperties.$t = t
app.use(pinia); app.use(router); app.use(ui); app.mount('#app')
