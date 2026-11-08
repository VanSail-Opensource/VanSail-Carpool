import { defineComponent, h, nextTick, shallowRef, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { setExperience } from '@/utils/experience'
import { useTelegramProtection } from './useTelegramProtection'

function telegram(version = '9.0') {
  const app = { version, enableClosingConfirmation: vi.fn(), disableClosingConfirmation: vi.fn(),
    disableVerticalSwipes: vi.fn(), enableVerticalSwipes: vi.fn() }
  window.Telegram = { WebApp: app as unknown as TelegramWebApp }
  return app
}
function protect(active: Ref<boolean>) {
  return mount(defineComponent({ setup() { useTelegramProtection(active); return () => h('div') } }))
}
afterEach(() => { setExperience('miniapp'); window.Telegram = undefined })

describe('Telegram closing protection', () => {
  it('retains overlapping owners without changing host swipe gestures', async () => {
    setExperience('miniapp')
    const app = telegram()
    const first = protect(shallowRef(true))
    const active = shallowRef(true)
    const second = protect(active)
    expect(app.enableClosingConfirmation).toHaveBeenCalledOnce()
    first.unmount()
    expect(app.disableClosingConfirmation).not.toHaveBeenCalled()
    active.value = false; await nextTick()
    expect(app.disableClosingConfirmation).toHaveBeenCalledOnce()
    active.value = true; await nextTick()
    expect(app.enableClosingConfirmation).toHaveBeenCalledTimes(2)
    second.unmount()
    expect(app.disableClosingConfirmation).toHaveBeenCalledTimes(2)
    expect(app.disableVerticalSwipes).not.toHaveBeenCalled()
    expect(app.enableVerticalSwipes).not.toHaveBeenCalled()
  })
  it('does not invoke unsupported methods or browser administrator host controls', () => {
    setExperience('miniapp')
    const old = telegram('6.1')
    const unsupported = protect(shallowRef(true))
    expect(old.enableClosingConfirmation).not.toHaveBeenCalled()
    unsupported.unmount()
    setExperience('browser')
    const app = telegram()
    const browser = protect(shallowRef(true))
    browser.unmount()
    expect(app.enableClosingConfirmation).not.toHaveBeenCalled()
    expect(app.disableClosingConfirmation).not.toHaveBeenCalled()
    expect(app.disableVerticalSwipes).not.toHaveBeenCalled()
  })
})
