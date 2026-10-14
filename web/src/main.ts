import { showBootstrapFailure } from './utils/bootstrapFallback'
import { detectTelegramWebApp, hasTelegramLaunchContext } from './utils/telegramContext'
import { setExperience } from './utils/experience'

void bootstrap().catch(showBootstrapFailure)

async function bootstrap(): Promise<void> {
  if (location.pathname.startsWith('/subscription/')) {
    setExperience('browser')
    await import('./publicSubscription/main')
    return
  }
  const adminPath = location.pathname === '/admin' || location.pathname.startsWith('/admin/')
  const browserAuthPath = location.pathname === '/admin/access' || location.pathname === '/admin/login'
  if (adminPath && !browserAuthPath && hasTelegramLaunchContext()) {
    const source = new URL(location.href)
    const business = new URLSearchParams(source.search)
    const destination = new URL('/settings/admin-access', source.origin)
    for (const [key, value] of source.searchParams) if (/^tgWebApp/i.test(key)) {
      business.delete(key)
      destination.searchParams.append(key, value)
    }
    destination.searchParams.set('returnTo', `${source.pathname}${business.size ? `?${business}` : ''}`)
    destination.hash = source.hash
    location.replace(destination.href)
    return
  }
  const miniapp = !location.pathname.startsWith('/admin') && detectTelegramWebApp(window.Telegram?.WebApp)
  setExperience(miniapp ? 'miniapp' : 'browser')
  if (!miniapp) {
    await import('./browser/main')
    return
  }
  if (!window.Telegram?.WebApp) await new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    const timer = setTimeout(() => reject(new Error('SDK_TIMEOUT')), 10_000)
    script.src = 'https://telegram.org/js/telegram-web-app.js?63'
    script.onload = () => { clearTimeout(timer); resolve() }
    script.onerror = () => { clearTimeout(timer); reject(new Error('SDK_UNAVAILABLE')) }
    document.head.append(script)
  })
  await import('./miniapp')
}
