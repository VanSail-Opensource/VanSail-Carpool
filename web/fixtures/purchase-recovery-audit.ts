import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme, setBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import PurchaseRecoveryAudit from './PurchaseRecoveryAudit.vue'
import '@/styles/main.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setBrowserTheme(params.has('light') ? 'light' : 'dark')
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const app = createApp(PurchaseRecoveryAudit).use(createPinia()).use(ui)
app.config.globalProperties.$t = t
app.mount('#app')
