import { afterEach, describe, expect, it, vi } from 'vitest'
import { installHapticClickFeedback, notifyBetOutcome } from './telegramHaptics'

afterEach(() => { window.Telegram = undefined })

describe('activity action and outcome feedback', () => {
  it('emits one delegated confirmation impact and one notification for each bet', () => {
    const impactOccurred = vi.fn(), notificationOccurred = vi.fn()
    window.Telegram = { WebApp: { version: '9.0', HapticFeedback: { impactOccurred, notificationOccurred } } as unknown as TelegramWebApp }
    const button = document.createElement('button')
    button.dataset.haptic = 'confirm'
    document.body.append(button)
    const dispose = installHapticClickFeedback()
    try {
      button.click(); notifyBetOutcome('win')
      expect(impactOccurred.mock.calls).toEqual([['rigid']])
      expect(notificationOccurred.mock.calls).toEqual([['success']])
      button.click(); notifyBetOutcome('loss')
      expect(impactOccurred.mock.calls).toEqual([['rigid'], ['rigid']])
      expect(notificationOccurred.mock.calls).toEqual([['success'], ['warning']])
    } finally { dispose(); button.remove() }
  })
  it('keeps outcome feedback capability-aware and tolerates host failures', () => {
    const notificationOccurred = vi.fn()
    const app = { version: '6.0', HapticFeedback: { notificationOccurred } }
    window.Telegram = { WebApp: app as unknown as TelegramWebApp }
    notifyBetOutcome('win')
    expect(notificationOccurred).not.toHaveBeenCalled()
    app.version = '9.0'
    notificationOccurred.mockImplementation(() => { throw new Error('Host unavailable') })
    expect(() => notifyBetOutcome('loss')).not.toThrow()
  })
})
