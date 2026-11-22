import { showBootstrapFailure } from './utils/bootstrapFallback'
import { installBrowserCompatibility, missingBrowserCapabilities } from './utils/browserCompatibility'
import { configureFeedback } from './utils/feedback'

export async function mountMiniApp(sdkReady: Promise<void>): Promise<void> {
  installBrowserCompatibility()
  const stylesReady = import('./styles/main.css')

  if (missingBrowserCapabilities().length > 0) {
    const [{ createApp }, { default: BrowserCapabilityGate }, { t }] = await Promise.all([
      import('vue'),
      import('./components/session/BrowserCapabilityGate.vue'),
      import('./i18n'),
      stylesReady,
      sdkReady,
    ])

    const app = createApp(BrowserCapabilityGate)
    app.config.globalProperties.$t = t
    app.config.errorHandler = () => showBootstrapFailure()
    app.mount('#app')
    return
  }

  let abandoned = false
  let disposeTelegram: (() => void) | undefined
  const hostReady = Promise.all([sdkReady, import('./utils/telegram')]).then(async ([, telegram]) => {
    await telegram.waitForTelegramContext()
    if (!abandoned) {
      configureFeedback(telegram)
      disposeTelegram = telegram.initializeTelegram()
    }
    return telegram
  })
  void hostReady.catch(() => undefined)

  const piniaReady = import('pinia').then(({ createPinia }) => createPinia())
  const sessionStarted = Promise.all([piniaReady, import('./stores/session')])
    .then(([pinia, { useSessionStore }]) => {
      // Run the existing cookie probe without waiting for the SDK or shell.
      // The same store gates Telegram login and navigation on host readiness.
      if (!abandoned) void useSessionStore(pinia).bootstrap(false, hostReady)
    })

  try {
    const [pinia, { default: ui }, { createApp }, { default: App }, { createMemberRouter }, { t }] = await Promise.all([
      piniaReady,
      import('@nuxt/ui/vue-plugin'),
      import('vue'),
      import('./App.vue'),
      import('./router'),
      import('./i18n'),
      stylesReady,
      sessionStarted,
    ])

    const app = createApp(App)
    app.config.globalProperties.$t = t
    app.config.errorHandler = () => showBootstrapFailure()

    app.use(pinia)
    app.use(createMemberRouter(hostReady))
    app.use(ui)
    app.mount('#app')

    try {
      const telegram = await hostReady
      telegram.markTelegramReady()
      if (disposeTelegram) window.addEventListener('pagehide', disposeTelegram, { once: true })
    } catch (error) {
      app.unmount()
      throw error
    }
  } catch (error) {
    abandoned = true
    disposeTelegram?.()
    throw error
  }
}
