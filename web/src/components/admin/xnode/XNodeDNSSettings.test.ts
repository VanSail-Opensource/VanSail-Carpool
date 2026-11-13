import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { XNodeSettings } from '@/api/xnode'
import XNodeDNSSettings from './XNodeDNSSettings.vue'

vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const switchStub = defineComponent({
  props: { modelValue: Boolean, label: { type: String, default: '' }, help: { type: String, default: '' } }, emits: ['update:modelValue'],
  setup: (props, { emit }) => () => h('button', {
    type: 'button', role: 'switch', 'aria-checked': props.modelValue, 'aria-label': props.label,
    onClick: () => emit('update:modelValue', !props.modelValue),
  }, props.help),
})
function editor(settings: NonNullable<XNodeSettings['dns']>) {
  return mount(XNodeDNSSettings, { props: { modelValue: settings }, global: { stubs: { SwitchField: switchStub } } })
}

describe('native Xray DNS editing', () => {
  it('shows native configuration guidance and only the redirect control', () => {
    const wrapper = editor({ redirect: false })
    expect(wrapper.findAll('[role="switch"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('nodeForms.nativeXrayDnsHint')
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })

  it('emits only the editable redirect field without mutating the loaded policy', async () => {
    const settings = { redirect: false }
    const wrapper = editor(settings)
    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual({ ...settings, redirect: true })
    expect(settings).toEqual({ redirect: false })
    wrapper.unmount()
  })

  it('does not invent resolver settings for defaults or a loaded redirect policy', async () => {
    for (const settings of [{}, { redirect: true }]) {
      const wrapper = editor(settings)
      await wrapper.get('[role="switch"]').trigger('click')
      expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual({ ...settings, redirect: !settings.redirect })
      wrapper.unmount()
    }
  })
})
