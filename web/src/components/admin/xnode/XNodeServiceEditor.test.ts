/* eslint-disable vue/one-component-per-file */

import { mount } from '@vue/test-utils'
import { defineComponent, h, reactive } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { NamedService, NamedServiceWrite } from '@/api/xnode'
import XNodeServiceEditor from './XNodeServiceEditor.vue'

vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const stubs = {
  // Nuxt UI's Vite auto-imports resolve to the unprefixed component names.
  Modal: { template: '<div><slot name="body" /></div>' },
  FormField: { props: { label: { type: String, default: '' } }, template: '<label :data-label="label"><slot /></label>' },
  Input: defineComponent({ props: { modelValue: { type: String, default: '' } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('input', { value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value) }) }),
  CheckboxGroup: defineComponent({ emits: ['update:modelValue'], setup: (_, { emit }) => () => h('button', { type: 'button', 'data-groups': '', onClick: () => emit('update:modelValue', ['group']) }) }),
  Switch: { template: '<div />' },
  Button: defineComponent({ props: { label: { type: String, default: '' } }, setup: props => () => h('button', props.label) }),
  Alert: { template: '<div />' },
}

function editor(item: NamedService | null) {
  return mount(XNodeServiceEditor, { props: { item, groups: [{ id: 'group', name: 'Group', enabled: true, bindings: [] }], busy: false }, global: { stubs } })
}

describe('named-service quota form', () => {
  it('creates a term without exposing or requiring a cadence', async () => {
    const wrapper = editor(null)
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('[role="combobox"]').exists()).toBe(false)
    expect(wrapper.find('[data-label="namedServices.reset"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('namedServices.reset')
    expect(wrapper.findAll('input')).toHaveLength(3)
    await wrapper.get('[data-label="common.name"] input').setValue('new-service')
    await wrapper.get('[data-label="namedServices.expiry"] input').setValue(new Date(Date.now() + 86400000).toISOString().slice(0, 16))
    await wrapper.get('[data-groups]').trigger('click')
    await wrapper.get('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as { access: Record<string, unknown> }
    expect(payload.access).not.toHaveProperty('strategy')
    wrapper.unmount()
  })

  it('preserves an existing legacy cadence internally while editing the quota', async () => {
    const item = reactive<NamedService>({ identity: { proxyUserId: '4503599627370496', userId: '', username: 'legacy-service', serviceName: 'legacy-service', currentPeriodId: 'period', accessDisabled: false, createdAt: '2026-01-01T00:00:00Z' }, access: { limitBytes: '1000', strategy: 'DAY', startedAt: '2026-01-01T00:00:00Z', expiresAt: new Date(Date.now() + 86400000).toISOString(), groupIds: ['group'], disabled: false } })
    const wrapper = editor(item)
    expect(wrapper.find('select').exists()).toBe(false)
    await wrapper.get('[data-label="namedServices.quota"] input').setValue('2000')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({ access: { strategy: 'DAY', limitBytes: '2000' } })
    expect((wrapper.emitted('save')?.[0]?.[0] as NamedServiceWrite).access.groupIds).not.toBe(item.access.groupIds)
    expect(item.access).toMatchObject({ strategy: 'DAY', limitBytes: '1000' })
    wrapper.unmount()
  })
})
