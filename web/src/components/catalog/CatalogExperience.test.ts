import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { selectionHaptic } from '@/utils/telegram'
import CatalogExperience from './CatalogExperience.vue'
import CatalogModeTransition from './CatalogModeTransition.vue'

const button = { props: ['label'], emits: ['click'], template: '<div role="button" tabindex="0" @click="$emit(\'click\')">{{ label }}</div>' }
// Async wrappers cannot infer props for automatic boolean stubs. Declare the
// child contracts explicitly and select the uniquely named stub components.
const CatalogPage = { name: 'ComboContentStub', props: { embedded: Boolean }, emits: ['ready'], template: '<div />' }
const PAYGPage = { name: 'PAYGContentStub', props: { entranceReady: Boolean }, emits: ['ready'], template: '<div />' }
const stubs = { UButton: button, Button: button, CatalogPage, PAYGPage }
vi.mock('@/utils/telegram', () => ({ selectionHaptic: vi.fn() }))
vi.mock('@/composables/useMotionPreferences', async () => {
  const { shallowRef } = await import('vue')
  return { useMotionPreferences: () => ({ reducedMotion: shallowRef(false) }) }
})

describe('CatalogExperience', () => {
  it('holds fast PAYG data for scene completion without delaying readiness', async () => {
    const wrapper = mount(CatalogExperience, { global: { stubs } })
    await flushPromises()
    const page = wrapper.findComponent(PAYGPage)
    expect(page.props('entranceReady')).toBe(false)
    page.vm.$emit('ready', true)
    await wrapper.vm.$nextTick()
    expect(page.props('entranceReady')).toBe(false)
    wrapper.findComponent(CatalogModeTransition).vm.$emit('active', false)
    await wrapper.vm.$nextTick()
    expect(page.props('entranceReady')).toBe(true)
    wrapper.unmount()
  })
  it('keeps a ready Combo inert until its introduction completes', async () => {
    const wrapper = mount(CatalogExperience, { global: { stubs } })
    await flushPromises()
    await wrapper.findAll('[role=button]')[1]!.trigger('click')
    await flushPromises()
    const content = () => wrapper.get('.catalog-experience__content')
    expect(content().attributes('aria-hidden')).toBe('true')
    expect(content().attributes()).toHaveProperty('inert')
    wrapper.findComponent(CatalogPage).vm.$emit('ready', true)
    await wrapper.vm.$nextTick()
    expect(content().classes()).toContain('catalog-experience__content--hidden')
    wrapper.findComponent(CatalogModeTransition).vm.$emit('active', false)
    await wrapper.vm.$nextTick()
    expect(content().attributes('aria-hidden')).toBeUndefined()
    expect(content().attributes()).not.toHaveProperty('inert')
    wrapper.unmount()
  })
  it('opens PAYG without mounting the gated combo flow and switches both ways', async () => {
    vi.mocked(selectionHaptic).mockClear()
    const wrapper = mount(CatalogExperience, { global: { stubs } })
    await flushPromises()
    expect(wrapper.findComponent(PAYGPage).exists()).toBe(true)
    expect(wrapper.findComponent(CatalogPage).exists()).toBe(false)
    await wrapper.findAll('[role=button]')[0]!.trigger('click')
    expect(selectionHaptic).not.toHaveBeenCalled()
    expect(wrapper.findComponent(CatalogModeTransition).props('mode')).toBe('payg')
    await wrapper.findAll('[role=button]')[1]!.trigger('click')
    await flushPromises()
    expect(selectionHaptic).toHaveBeenCalledTimes(1)
    expect(wrapper.findComponent(CatalogPage).exists()).toBe(true)
    expect(wrapper.findComponent(CatalogPage).props('embedded')).toBe(true)
    expect(wrapper.findComponent(CatalogModeTransition).props('mode')).toBe('combo')
    await wrapper.findAll('[role=button]')[0]!.trigger('click')
    expect(wrapper.findComponent(PAYGPage).exists()).toBe(true)
    for (const index of [1, 0, 1, 0, 1, 0]) await wrapper.findAll('[role=button]')[index]!.trigger('click')
    expect(wrapper.findComponent(CatalogModeTransition).props('mode')).toBe('payg')
    expect(wrapper.findAllComponents(CatalogModeTransition)).toHaveLength(1)
    wrapper.unmount()
  })
})
