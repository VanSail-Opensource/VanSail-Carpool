/* eslint-disable vue/one-component-per-file */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import { configureFeedback } from '@/utils/feedback'
import { setExperience } from '@/utils/experience'
import DIYFieldControl from './DIYFieldControl.vue'

const field = { key: 'tlsAllowInsecure', group: 'tls', kind: 'boolean' as const, minimum: 0, maximum: 0, unit: '', advanced: false }
const Select = defineComponent({ emits: ['update:modelValue'], setup: (_props, { emit }) => () => h('button', { onClick: () => emit('update:modelValue', 'true') }) })
const FormField = { template: '<div><slot /></div>' }, Input = { template: '<span />' }
// Match both Nuxt's imported SFC names and registered U-prefixed aliases.
const global = { stubs: { FormField, UFormField: FormField, Select, USelect: Select, Input, UInput: Input } }
it('reuses selection controls in browser admin without Telegram feedback', async () => {
  const feedback = vi.fn(); configureFeedback({ selectionHaptic: feedback }); setExperience('browser')
  const wrapper = mount(DIYFieldControl, { props: { field, modelValue: undefined }, global })
  await wrapper.get('button').trigger('click')
  expect(wrapper.emitted('update:modelValue')).toEqual([[true]])
  expect(feedback).not.toHaveBeenCalled()
  setExperience('miniapp'); await wrapper.get('button').trigger('click')
  expect(feedback).toHaveBeenCalledOnce()
  wrapper.unmount(); configureFeedback({ selectionHaptic: () => undefined })
})
it('honors pinned fixed-off field metadata without offering an enable control', () => {
  const wrapper = mount(DIYFieldControl, { props: { field: { ...field, restrictions: ['fixedOff'] }, modelValue: undefined }, global })
  expect(wrapper.find('button').exists()).toBe(false)
  wrapper.unmount()
})
it('adds custom ALPN entries with a structured string-array control', async () => {
  const Menu = defineComponent({ emits: ['create'], setup: (_props, { emit }) => () => h('button', { onClick: () => emit('create', 'h2') }) })
  const wrapper = mount(DIYFieldControl, { props: { field: { ...field, key: 'tlsAlpn', kind: 'strings', enum: [] }, modelValue: ['http/1.1'] }, global: { stubs: { ...global.stubs, SelectMenu: Menu, USelectMenu: Menu } } })
  await wrapper.get('button').trigger('click')
  expect(wrapper.emitted('update:modelValue')).toEqual([[['http/1.1', 'h2']]])
  wrapper.unmount()
})
