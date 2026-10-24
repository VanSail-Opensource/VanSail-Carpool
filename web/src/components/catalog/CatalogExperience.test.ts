import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import CatalogExperience from './CatalogExperience.vue'
import CatalogPage from './CatalogPage.vue'
import PAYGPage from './PAYGPage.vue'
import CatalogModeTransition from './CatalogModeTransition.vue'

const button = { props: ['label'], emits: ['click'], template: '<div role="button" tabindex="0" @click="$emit(\'click\')">{{ label }}</div>' }

describe('CatalogExperience', () => {
  it('opens PAYG without mounting the gated combo flow and switches both ways', async () => {
    const wrapper = mount(CatalogExperience, { global: { stubs: { UButton: button, Button: button, CatalogPage: true, PAYGPage: true } } })
    expect(wrapper.findComponent(PAYGPage).exists()).toBe(true)
    expect(wrapper.findComponent(CatalogPage).exists()).toBe(false)
    expect(wrapper.findComponent(CatalogModeTransition).props('mode')).toBe('payg')
    await wrapper.findAll('[role=button]')[1]!.trigger('click')
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
