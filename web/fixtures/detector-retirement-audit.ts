import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import DetectorRetirementAudit from './DetectorRetirementAudit.vue'

const params = new URLSearchParams(location.search)
setExperience(params.has('member') ? 'miniapp' : 'browser')
if (params.has('member')) await import('@/styles/main.css')
else { await import('@/browser/styles.css'); initializeBrowserTheme() }
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const app = createApp(DetectorRetirementAudit)
app.config.globalProperties.$t = t
app.use(createPinia()).use(createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: DetectorRetirementAudit, meta: { adminSection: 'users' } }] })).use(ui).mount('#app')
