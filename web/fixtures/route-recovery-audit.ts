import { createApp, h } from 'vue'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import BrowserLoading from '@/browser/BrowserLoading.vue'
import { installBrowserRouteRecovery } from '@/browser/routeRecovery'
import { initializeBrowserTheme } from '@/browser/theme'
import { setExperience } from '@/utils/experience'
import { setLocale, t } from '@/i18n'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
sessionStorage.removeItem('txc-route-recovery')
const state = { attempts: 0, replacements: [] as string[], recover: false, errors: [] as string[] }
const target = '/admin/users/synthetic-member?tab=history#billing'
const page = { render: () => h('main', { class: 'browser-entry', 'data-recovered-route': true }, t('adminUserProfile.open')) }
const router = createRouter({ history: createMemoryHistory(), routes: [
  { path: '/', component: { render: () => h('button', { onClick: () => void navigate() }, t('adminUserProfile.open')) } },
  { path: '/admin/users/:id', component: async () => {
    state.attempts++
    if (!state.recover && (params.has('permanent') || params.has('runtime') || state.attempts === 1)) {
      throw params.has('runtime') ? new ReferenceError('synthetic route evaluation failure') : new TypeError('Failed to fetch dynamically imported module')
    }
    return page
  } },
] })
const recovery = installBrowserRouteRecovery(router, failedTarget => {
  state.replacements.push(failedTarget)
  // Model a new document retry using the same destination without leaving the fixture.
  queueMicrotask(() => { void router.replace(failedTarget).catch(() => undefined) })
})
router.onError(error => { state.errors.push(String(error)) })
async function navigate(): Promise<void> { await router.push(target).catch(() => undefined) }
async function retry(): Promise<void> {
  state.recover = true
  await router.replace(recovery.target.value ?? target).catch(() => undefined)
}
Object.assign(window, { __routeRecoveryAudit: state, __routeRecoveryRouter: router })
const app = createApp({ render: () => recovery.error.value || !router.currentRoute.value.matched.length
  ? h(BrowserLoading, { error: recovery.error.value ? t('browser.routeLoadFailed') : null }, { default: () => h('button', { onClick: retry }, t('common.tryAgain')) })
  : h(RouterView) })
app.config.globalProperties.$t = t
app.use(router).use(ui).mount('#app')
if (params.has('direct')) void navigate()
