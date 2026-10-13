const telegramLaunchParameter = /^tgWebApp(?:Data|Version|Platform|ThemeParams|StartParam)$/i
const telegramPlatforms = new Set(['android', 'ios', 'macos', 'tdesktop', 'web', 'weba', 'webk', 'unigram'])

export function isTelegramUserAgent(userAgent: string): boolean {
  return /\bTelegram\b|Telegram(?:[-/]|Android|iOS|Desktop|Web)/i.test(userAgent)
}

function hasTelegramLaunchParameters(): boolean {
  const locationHasParameters = [window.location.hash, window.location.search].some((source) => {
    const normalized = source.replace(/^#/, '').replace(/^\?/, '')
    return [...new URLSearchParams(normalized).keys()].some((key) => telegramLaunchParameter.test(key))
  })
  if (locationHasParameters) return true
  return Object.keys(window.Telegram?.WebView?.initParams ?? {}).some((key) => telegramLaunchParameter.test(key))
}

function hasKnownTelegramPlatform(platform: string | undefined): boolean {
  return Boolean(platform && telegramPlatforms.has(platform.toLowerCase()))
}

function hasTelegramWebViewBridge(): boolean {
  return typeof window.TelegramWebviewProxy?.postEvent === 'function'
    || typeof window.TelegramWebviewProxyProto?.postEvent === 'function'
}

export function detectTelegramWebApp(app: TelegramWebApp | undefined): boolean {
  if (hasTelegramWebViewBridge()) return true
  if (app?.initData?.trim()) return true
  if (app?.initDataUnsafe?.user?.id) return true
  if (hasTelegramLaunchParameters()) return true
  if (hasKnownTelegramPlatform(app?.platform)) return true
  return typeof navigator !== 'undefined' && isTelegramUserAgent(navigator.userAgent)
}

export function hasTelegramLaunchContext(): boolean {
  return hasTelegramWebViewBridge() || hasTelegramLaunchParameters() || Boolean(window.Telegram?.WebApp?.initData?.trim())
}

export async function waitForTelegramContext(timeoutMs = 3000): Promise<boolean> {
  if (detectTelegramWebApp(window.Telegram?.WebApp)) return true
  const startedAt = Date.now()
  const hasSignal = () => Boolean(
    window.TelegramWebviewProxy?.postEvent
      || window.TelegramWebviewProxyProto?.postEvent
      || isTelegramUserAgent(navigator.userAgent)
      || hasTelegramLaunchParameters(),
  )
  // The public SDK installs a placeholder WebApp object in every browser.
  // Only wait when the host supplies a native or launch-context signal.
  if (!hasSignal()) return false

  while (Date.now() - startedAt < timeoutMs) {
    await new Promise<void>((resolve) => window.setTimeout(resolve, 50))
    if (detectTelegramWebApp(window.Telegram?.WebApp)) return true
  }
  return detectTelegramWebApp(window.Telegram?.WebApp)
}
