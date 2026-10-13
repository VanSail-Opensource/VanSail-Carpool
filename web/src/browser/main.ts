import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ui from '@nuxt/ui/vue-plugin'
import { setExperience } from '@/utils/experience'
import { showBootstrapFailure } from '@/utils/bootstrapFallback'
import { initializeBrowserTheme } from './theme'

setExperience('browser')
const disposeTheme = initializeBrowserTheme()
void Promise.all([import('./BrowserApp.vue'), import('./router'), import('@/i18n'), import('./styles.css')])
  .then(([{ default: BrowserApp }, { default: router }, { t }]) => {
    const app = createApp(BrowserApp)
    app.config.globalProperties.$t = t
    app.config.errorHandler = (error) => { console.error(error); showBootstrapFailure() }
    app.use(createPinia()).use(router).use(ui).mount('#app')
    window.addEventListener('pagehide', disposeTheme, { once: true })
  }).catch(showBootstrapFailure)
