import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, shallowRef } from 'vue'
import MemberMetricValue from './MemberMetricValue.vue'

const reducedMotion = shallowRef(false)
vi.mock('@/composables/useMotionPreferences', () => ({ useMotionPreferences: () => ({ reducedMotion }) }))
vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

function mountValue(value: string | null = '3') {
  const wrapper = mount(MemberMetricValue, { props: { value, unit: 'MiB/s' } })
  const cancel = vi.fn()
  const animate = vi.fn(() => ({ cancel }))
  const number = wrapper.get('.member-metric-value__number').element
  Object.defineProperty(number, 'animate', { configurable: true, value: animate })
  return { wrapper, number, animate, cancel }
}

beforeEach(() => {
  reducedMotion.value = false
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
})
afterEach(() => vi.restoreAllMocks())

describe('member metric number changes', () => {
  it('keeps the number node and units stable, animating only a changed displayed number', async () => {
    const { wrapper, number, animate, cancel } = mountValue('9007199254740993')
    const unit = wrapper.get('.member-metric-value__unit').element
    expect(animate).not.toHaveBeenCalled()
    await wrapper.setProps({ value: '9007199254740993' })
    await wrapper.setProps({ unit: 'KiB/s' })
    expect(animate).not.toHaveBeenCalled()
    await wrapper.setProps({ value: '9007199254740994' })
    expect(animate).toHaveBeenCalledTimes(1)
    expect(wrapper.get('.member-metric-value__number').element).toBe(number)
    expect(wrapper.get('.member-metric-value__unit').element).toBe(unit)
    expect(wrapper.text()).toContain('9007199254740994')
    wrapper.unmount()
    expect(cancel).toHaveBeenCalledTimes(1)
  })

  it('shows placeholders and first observations without animation', async () => {
    const { wrapper, animate } = mountValue(null)
    expect(wrapper.text()).toBe('common.notAvailable')
    expect(wrapper.find('.member-metric-value__unit').exists()).toBe(false)
    await wrapper.setProps({ value: '0' })
    expect(wrapper.text()).toContain('0')
    await wrapper.setProps({ value: null })
    expect(animate).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('cancels motion immediately when the preference changes and keeps text current', async () => {
    const { wrapper, animate, cancel } = mountValue()
    await wrapper.setProps({ value: '4' })
    reducedMotion.value = true
    await nextTick()
    expect(cancel).toHaveBeenCalledTimes(1)
    await wrapper.setProps({ value: '5' })
    expect(wrapper.text()).toContain('5')
    expect(animate).toHaveBeenCalledTimes(1)
    reducedMotion.value = false
    await nextTick()
    expect(animate).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('does not start motion while hidden', async () => {
    const { wrapper, animate } = mountValue()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    await wrapper.setProps({ value: '9' })
    expect(wrapper.text()).toContain('9')
    expect(animate).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
