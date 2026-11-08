import { getTelegramWebApp, supportsTelegramVersion, tryTelegramCall } from './telegram'

export type HapticImpact = 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'
export type HapticIntent =
  | 'action'
  | 'confirm'
  | 'copy'
  | 'destructive'
  | 'dismiss'
  | 'navigate'
  | 'open'
  | 'refresh'
  | 'retry'
  | 'zoom'

const impactByIntent: Readonly<Record<HapticIntent, HapticImpact>> = {
  action: 'medium',
  confirm: 'rigid',
  copy: 'light',
  destructive: 'heavy',
  dismiss: 'soft',
  navigate: 'soft',
  open: 'soft',
  refresh: 'light',
  retry: 'light',
  zoom: 'light',
}

export function haptic(intent: HapticIntent = 'action'): void {
  const app = getTelegramWebApp()
  if (app?.HapticFeedback && supportsTelegramVersion('6.1')) {
    tryTelegramCall(() => app.HapticFeedback?.impactOccurred(impactByIntent[intent]))
  }
}

export function selectionHaptic(): void {
  const app = getTelegramWebApp()
  if (!app?.HapticFeedback || !supportsTelegramVersion('6.1')) return
  tryTelegramCall(() => {
    if (app.HapticFeedback?.selectionChanged) app.HapticFeedback.selectionChanged()
    else app.HapticFeedback?.impactOccurred('light')
  })
}

function hapticIntentFor(element: Element): HapticIntent | undefined {
  const value = element.getAttribute('data-haptic') as HapticIntent | null
  return value && value in impactByIntent ? value : undefined
}

function handleHapticClick(event: MouseEvent): void {
  if (!(event.target instanceof Element)) return
  const markedTarget = event.target.closest('[data-haptic]')
  if (markedTarget) {
    if (isDisabled(markedTarget)) return
    const intent = hapticIntentFor(markedTarget)
    if (intent) haptic(intent)
    return
  }

  const actionTarget = event.target.closest('button, a[href], [role="button"]')
  if (!actionTarget || actionTarget.closest('[data-haptic-skip]') || isDisabled(actionTarget) || isSelectionControl(actionTarget)) return
  haptic(actionTarget.matches('a[href]') ? 'navigate' : 'action')
}

function isDisabled(element: Element): boolean {
  return element.hasAttribute('disabled') || element.getAttribute('aria-disabled') === 'true'
}

function isSelectionControl(element: Element): boolean {
  const role = element.getAttribute('role')
  return element.hasAttribute('aria-pressed')
    || element.hasAttribute('aria-expanded')
    || role === 'checkbox'
    || role === 'option'
    || role === 'radio'
    || role === 'switch'
    || role === 'tab'
}

export function installHapticClickFeedback(): () => void {
  document.addEventListener('click', handleHapticClick, true)
  return () => document.removeEventListener('click', handleHapticClick, true)
}

export function notifyHaptic(type: 'error' | 'success' | 'warning'): void {
  const app = getTelegramWebApp()
  if (app?.HapticFeedback && supportsTelegramVersion('6.1')) tryTelegramCall(() => app.HapticFeedback?.notificationOccurred(type))
}

export function notifyBetOutcome(outcome: 'win' | 'loss'): void {
  notifyHaptic(outcome === 'win' ? 'success' : 'warning')
}
