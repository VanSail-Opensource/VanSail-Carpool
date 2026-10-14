import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { t } from '@/i18n'
import PublicSubscriptionPage from './PublicSubscriptionPage.vue'
import '@/browser/styles.css'

setExperience('browser')
const disposeTheme = initializeBrowserTheme()
const router = createRouter({ history: createWebHistory(), routes: [{ path: '/subscription/:token', component: PublicSubscriptionPage }] })
const app = createApp(PublicSubscriptionPage)
app.config.globalProperties.$t = t
app.use(router).use(ui).mount('#app')
window.addEventListener('pagehide', disposeTheme, { once: true })
