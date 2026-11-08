import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { featuresApi } from '@/api/features'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import ResetRewardAudit from './ResetRewardAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
featuresApi.getAdminLuckyDrawForecast = async () => {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 3000))
  if (params.has('error')) throw new Error('Constructed forecast failure')
  return { entries: 0, incomeMinor: '0', expectedExpenseMinMinor: null, expectedExpenseMaxMinor: null, possibleExpenseMinMinor: null, possibleExpenseMaxMinor: null, breakEvenMinMinor: null, breakEvenMaxMinor: null, averageBalanceMinor: null, multiplierUnavailable: false, couponOneTermOnly: true }
}
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: ResetRewardAudit }] })
await router.push('/')
const app = createApp(ResetRewardAudit).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t = t
app.mount('#app')
