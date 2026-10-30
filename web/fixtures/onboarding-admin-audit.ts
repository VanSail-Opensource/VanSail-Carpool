import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import UApp from '@nuxt/ui/components/App.vue'
import { featuresApi, type OnboardingBundle } from '@/api/features'
import { ApiError } from '@/api/http'
import AdminOnboardingPanel from '@/components/admin/AdminOnboardingPanel.vue'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser'); initializeBrowserTheme(); setLocale(params.has('zh') ? 'zh-CN' : 'en')
let draftRevision = 2, publishedRevision = 1
const trace = { publishes: 0 }
const bundle = (kind: OnboardingBundle['kind']): OnboardingBundle => {
  const content = kind === 'welcome' ? [{ id: 'welcome', text: t('adminOnboarding.welcome'), durationMs: 1000 }] : [{ id: 'terms', title: t('adminOnboarding.agreements'), body: t('adminOnboarding.agreementHint'), icon: 'shield-check' as const }]
  return { kind, draftRevision, publishedRevision, draft: { en: structuredClone(content), 'zh-CN': structuredClone(content) }, published: { en: structuredClone(content), 'zh-CN': structuredClone(content) } }
}
featuresApi.getAdminOnboardingBundle = async kind => {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 1800))
  if (params.has('error')) throw new ApiError(503, { code: 'ONBOARDING_CONTENT_UNAVAILABLE', message: '' })
  return bundle(kind)
}
featuresApi.saveAdminOnboardingDraft = async kind => { draftRevision++; return bundle(kind) }
featuresApi.publishAdminOnboarding = async kind => {
  if (params.has('publish-error')) throw new ApiError(409, { code: 'ONBOARDING_STATE_CONFLICT', message: '' })
  trace.publishes++; publishedRevision++; return bundle(kind)
}
Object.assign(window, { __agreementAdminAudit: trace })
const root = { render: () => h(UApp, null, { default: () => h('main', { style: 'max-width:1100px;margin:auto;padding:20px' }, h(AdminOnboardingPanel)) }) }
const app = createApp(root)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: root }] })).use(ui).mount('#app')
