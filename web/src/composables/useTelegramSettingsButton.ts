import { onMounted, onScopeDispose, watch } from 'vue'
import { getTelegramWebApp, tryTelegramCall } from '@/utils/telegram'

/**
 * Wires the native Telegram Settings button. The optional visibility getter lets
 * the owning shell hide the button for members who cannot reach /settings and
 * show it again once they gain access. The getter is watched, so the button is
 * restored without a remount.
 */
export function useTelegramSettingsButton(navigate: () => void, isVisible: () => boolean = () => true): void {
  let button: TelegramWebApp['SettingsButton']
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  let attempts = 0
  let listening = false

  function shouldShow(): boolean {
    try {
      return isVisible()
    } catch {
      return false
    }
  }

  function applyVisibility(): void {
    if (disposed || !button) return
    if (shouldShow()) {
      if (listening) {
        tryTelegramCall(() => button?.show())
        return
      }
      listening = true
      tryTelegramCall(() => { button?.onClick(navigate); button?.show() })
      return
    }
    if (listening) tryTelegramCall(() => button?.offClick(navigate))
    listening = false
    tryTelegramCall(() => button?.hide())
  }

  function connect(): void {
    if (disposed) return
    button = getTelegramWebApp()?.SettingsButton
    if (button) applyVisibility()
    else if (attempts++ < 30) timer = setTimeout(connect, 100)
  }

  onMounted(connect)
  watch(isVisible, applyVisibility, { flush: 'post' })
  onScopeDispose(() => {
    disposed = true
    if (timer) clearTimeout(timer)
    if (button) tryTelegramCall(() => { button?.offClick(navigate); button?.hide() })
  })
}
