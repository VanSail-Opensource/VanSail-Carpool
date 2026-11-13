import { computed, defineComponent, h, nextTick, shallowRef } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { setExperience } from '@/utils/experience'
import { useTelegramBackButton } from './useTelegramBackButton'

afterEach(() => { window.Telegram = undefined; setExperience('miniapp') })

describe('Telegram back button ownership', () => {
  it('restores the remaining owner and hides the host button when the last owner leaves', async () => {
    const button = { show: vi.fn(), hide: vi.fn(), onClick: vi.fn(), offClick: vi.fn() }
    window.Telegram = { WebApp: { version: '9.0', BackButton: button } as unknown as TelegramWebApp }
    const firstAction = vi.fn()
    const secondAction = vi.fn()
    const visible = shallowRef(true)
    function owner(action: () => void) {
      return mount(defineComponent({ setup() {
        useTelegramBackButton(computed(() => visible.value), action)
        return () => h('div')
      } }))
    }
    const first = owner(firstAction)
    const second = owner(secondAction)
    const handle = button.onClick.mock.calls[0]![0] as () => void
    handle()
    expect(secondAction).toHaveBeenCalledOnce()
    second.unmount()
    handle()
    expect(firstAction).toHaveBeenCalledOnce()
    expect(button.hide).not.toHaveBeenCalled()
    visible.value = false
    await nextTick()
    expect(button.hide).toHaveBeenCalledOnce()
    visible.value = true
    await nextTick()
    first.unmount()
    expect(button.offClick).toHaveBeenCalledWith(handle)
    expect(button.hide).toHaveBeenCalledTimes(2)
  })
})
