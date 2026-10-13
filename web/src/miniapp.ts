import { showBootstrapFailure } from './utils/bootstrapFallback'

import { configureFeedback } from './utils/feedback'

void bootstrap().catch(showBootstrapFailure)

async function bootstrap(): Promise<void> {
  const compatibility = await import('./utils/browserCompatibility')
  compatibility.installBrowserCompatibility()
  await import('./styles/main.css')

  if (compatibility.missingBrowserCapabilities().length > 0) {
    const [{ createApp }, { default: BrowserCapabilityGate }, { t }] = await Promise.all([
      import('vue'),
      import('./components/session/BrowserCapabilityGate.vue'),
      import('./i18n'),
    ])

    const app = createApp(BrowserCapabilityGate)
    app.config.globalProperties.$t = t
    app.config.errorHandler = () => showBootstrapFailure()
    app.mount('#app')
    return
  }

  const telegram = await import('./utils/telegram')
  configureFeedback(telegram)
  const { initializeTelegram, markTelegramReady, waitForTelegramContext } = telegram
  await waitForTelegramContext()
  const disposeTelegram = initializeTelegram()

  const [
    { createPinia },
    { default: ui },
    { createApp },
    { default: App },
    { default: router },
    { t },
  ] = await Promise.all([
    import('pinia'),
    import('@nuxt/ui/vue-plugin'),
    import('vue'),
    import('./App.vue'),
    import('./router'),
    import('./i18n'),
  ])

  const app = createApp(App)
  app.config.globalProperties.$t = t
  app.config.errorHandler = () => showBootstrapFailure()

  app.use(createPinia())
  app.use(router)
  app.use(ui)

  app.mount('#app')
  markTelegramReady()

  window.addEventListener('pagehide', disposeTelegram, { once: true })
}

