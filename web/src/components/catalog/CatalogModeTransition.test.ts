import { flushPromises, mount } from '@vue/test-utils'
import { shallowRef } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import CatalogModeTransition from './CatalogModeTransition.vue'

vi.mock('@/composables/useMotionPreferences', () => ({ useMotionPreferences: vi.fn() }))
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

function preferences(reduced = false) {
  const reducedMotion = shallowRef(reduced)
  vi.stubGlobal('CSS', { supports: () => true })
  vi.mocked(useMotionPreferences).mockReturnValue({ reducedMotion, offset: value => value, transition: () => ({ duration: 0, ease: 'easeOut' }) })
  return reducedMotion
}

describe('full-screen mode introduction', () => {
  it('replaces the scene on rapid switching and disposes it within 800ms', async () => {
    vi.useFakeTimers()
    preferences()
    const wrapper = mount(CatalogModeTransition, { props: { mode: 'payg' }, global: { stubs: { Teleport: true } } })
    await flushPromises()
    expect(wrapper.get('.mode-transition').text()).toContain('Your connection. Your pace.')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.setProps({ mode: 'combo' })
    await flushPromises()
    expect(wrapper.findAll('.mode-transition')).toHaveLength(1)
    expect(wrapper.get('.mode-transition').text()).toContain('Make every journey yours.')
    await vi.advanceTimersByTimeAsync(800)
    expect(wrapper.find('.mode-transition').exists()).toBe(false)
    await wrapper.setProps({ mode: 'payg' })
    await flushPromises()
    wrapper.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('omits the full-screen scene for reduced motion', () => {
    preferences(true)
    const wrapper = mount(CatalogModeTransition, { props: { mode: 'payg' }, global: { stubs: { Teleport: true } } })
    expect(wrapper.find('.mode-transition').exists()).toBe(false)
    wrapper.unmount()
  })
})
