import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { selectionHaptic } from '@/utils/telegram'
import CatalogExperience from './CatalogExperience.vue'

const button = { props: ['label'], emits: ['click'], template: '<div role="button" tabindex="0" @click="$emit(\'click\')">{{ label }}</div>' }
// Async wrappers cannot infer props for automatic boolean stubs. Declare the
// child contracts explicitly and select the uniquely named stub components.
const HourlyPage = { name: 'HourlyContentStub', template: '<div />' }
const PAYGPage = { name: 'PAYGContentStub', props: { entranceReady: Boolean }, emits: ['ready'], template: '<div />' }
const stubs = { UButton: button, Button: button, HourlyPage, PAYGPage }
vi.mock('@/utils/telegram', () => ({ selectionHaptic: vi.fn() }))
vi.mock('@/composables/useMotionPreferences', async () => {
  const { shallowRef } = await import('vue')
  return { useMotionPreferences: () => ({ reducedMotion: shallowRef(false) }) }
})

describe('CatalogExperience', () => {
  it('opens traffic billing and mounts hourly access only when selected', async () => {
    vi.mocked(selectionHaptic).mockClear()
    const wrapper = mount(CatalogExperience, { global: { stubs } })
    await flushPromises()
    expect(wrapper.findComponent(PAYGPage).exists()).toBe(true)
    expect(wrapper.findComponent(HourlyPage).exists()).toBe(false)
    await wrapper.findAll('[role=button]')[0]!.trigger('click')
    expect(selectionHaptic).not.toHaveBeenCalled()
    await wrapper.findAll('[role=button]')[1]!.trigger('click')
    await flushPromises()
    expect(selectionHaptic).toHaveBeenCalledTimes(1)
    expect(wrapper.findComponent(HourlyPage).exists()).toBe(true)
    expect(wrapper.get('.catalog-experience__content').attributes()).not.toHaveProperty('inert')
    expect(wrapper.find('.mode-transition').exists()).toBe(false)
    await wrapper.findAll('[role=button]')[0]!.trigger('click')
    expect(wrapper.findComponent(PAYGPage).exists()).toBe(true)
    for (const index of [1, 0, 1, 0, 1, 0]) await wrapper.findAll('[role=button]')[index]!.trigger('click')
    wrapper.unmount()
  })
})
