import { flushPromises, mount } from '@vue/test-utils'
import { shallowRef } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import CatalogModeTransition from './CatalogModeTransition.vue'

vi.mock('@/composables/useMotionPreferences', () => ({ useMotionPreferences: vi.fn() }))
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

function preferences(reduced = false) {
  const reducedMotion = shallowRef(reduced)
  vi.mocked(useMotionPreferences).mockReturnValue({ reducedMotion, offset: value => value, transition: () => ({ duration: 0, ease: 'easeOut' }) })
  return reducedMotion
}

describe('full-screen mode introduction', () => {
  it('replaces the scene on rapid switching and disposes it within 800ms', async () => {
    vi.useFakeTimers()
    const schedule = vi.spyOn(globalThis, 'setTimeout')
    preferences()
    const wrapper = mount(CatalogModeTransition, { props: { mode: 'payg' }, global: { stubs: { Teleport: true } } })
    await flushPromises()
    expect(wrapper.get('.mode-transition strong').text()).toBe('Connect freely. Pay as you go.')
    expect(wrapper.find('.mode-transition p').exists()).toBe(false)
    expect(wrapper.find('.mode-transition__icons').exists()).toBe(true)
    expect(wrapper.get('.mode-transition').attributes('style')).not.toContain('clip-path')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.setProps({ mode: 'combo' })
    await flushPromises()
    expect(wrapper.findAll('.mode-transition')).toHaveLength(1)
    expect(wrapper.get('.mode-transition strong').text()).toBe('One plan. More room to explore.')
    await vi.advanceTimersByTimeAsync(800)
    expect(wrapper.find('.mode-transition').exists()).toBe(false)
    await wrapper.setProps({ mode: 'payg' })
    await flushPromises()
    const ownedIndex = schedule.mock.calls.map((call, index) => call[1] === 800 ? index : -1).filter(index => index >= 0).pop()!
    const ownedTimer = schedule.mock.results[ownedIndex]!.value
    const clear = vi.spyOn(globalThis, 'clearTimeout')
    wrapper.unmount()
    expect(clear).toHaveBeenCalledWith(ownedTimer)
  })

  it('omits the full-screen scene for reduced motion', () => {
    preferences(true)
    const wrapper = mount(CatalogModeTransition, { props: { mode: 'payg' }, global: { stubs: { Teleport: true } } })
    expect(wrapper.find('.mode-transition').exists()).toBe(false)
    wrapper.unmount()
  })

  it('reserves the toolbar and visible checkout controls without cutouts', async () => {
    vi.useFakeTimers()
    preferences()
    const controls = [
      { className: 'catalog-experience__header', top: 0, bottom: 60 },
      { className: 'payg-summary__wallet', top: 100, bottom: 160 },
      { className: 'catalog-flow-controls', top: 500, bottom: 560 },
    ].map(({ className, top, bottom }) => {
      const element = document.createElement('div')
      element.className = className
      vi.spyOn(element, 'getBoundingClientRect').mockReturnValue({ top, bottom, left: 0, right: 320, width: 320, height: bottom - top } as DOMRect)
      document.body.append(element)
      return element
    })
    const wrapper = mount(CatalogModeTransition, { props: { mode: 'payg' }, global: { stubs: { Teleport: true } } })
    await flushPromises()
    const style = wrapper.get('.mode-transition').attributes('style')
    expect(style).toContain('top: 160px')
    expect(style).toContain(`bottom: ${window.innerHeight - 500}px`)
    window.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.find('.mode-transition').exists()).toBe(false)
    wrapper.unmount()
    for (const element of controls) element.remove()
  })

  it('waits for the new view before measuring its controls', async () => {
    preferences()
    const wrapper = mount(CatalogModeTransition, { props: { mode: 'combo', ready: false }, global: { stubs: { Teleport: true } } })
    await flushPromises()
    expect(wrapper.find('.mode-transition').exists()).toBe(false)
    await wrapper.setProps({ ready: true })
    await flushPromises()
    expect(wrapper.find('.mode-transition').exists()).toBe(true)
    wrapper.unmount()
  })
})
