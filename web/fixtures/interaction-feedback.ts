import { isBrowserExperience } from '@/utils/experience'
import { installHapticClickFeedback, type HapticImpact } from '@/utils/telegram'

// Records calls to a constructed SDK; this never invokes a Telegram host.
export function installInteractionFeedback() {
  const haptics: string[] = [], closing: boolean[] = [], swipes: boolean[] = []
  const feedback = { haptics, closing, swipes }
  if (isBrowserExperience()) return feedback
  const app: TelegramWebApp = {
    version: '9.0', initData: '', initDataUnsafe: {}, colorScheme: 'dark',
    ready: () => undefined, expand: () => undefined, close: () => undefined,
    openLink: () => undefined, openTelegramLink: () => undefined, openInvoice: () => undefined,
    enableClosingConfirmation: () => closing.push(true),
    disableClosingConfirmation: () => closing.push(false),
    enableVerticalSwipes: () => swipes.push(true),
    disableVerticalSwipes: () => swipes.push(false),
    HapticFeedback: {
      impactOccurred: (value: HapticImpact) => haptics.push(`impact:${value}`),
      notificationOccurred: (value: 'error' | 'success' | 'warning') => haptics.push(`notification:${value}`),
      selectionChanged: () => haptics.push('selection'),
    },
  }
  window.Telegram = { WebApp: app }
  const dispose = installHapticClickFeedback()
  window.addEventListener('pagehide', dispose, { once: true })
  return feedback
}
