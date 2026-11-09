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
    const wrapper = editor({ mode: 'smartdns', upstreams: ['1.1.1.1'], redirect: false })
    expect(wrapper.findAll('[role="switch"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('nodeForms.nativeXrayDnsHint')
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })

  it('preserves original legacy migration evidence when toggling redirect', async () => {
    const settings = { mode: 'smartdns', upstreams: ['1.1.1.1', 'https://dns.example/dns-query'], redirect: false }
    const wrapper = editor(settings)
    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual({ ...settings, redirect: true })
    expect(settings).toEqual({ mode: 'smartdns', upstreams: ['1.1.1.1', 'https://dns.example/dns-query'], redirect: false })
    wrapper.unmount()
  })

  it('does not invent resolver settings for defaults or overwrite a loaded Xray policy', async () => {
    for (const settings of [{}, { mode: 'xray', redirect: true, upstreams: ['9.9.9.9'] }]) {
      const wrapper = editor(settings)
      await wrapper.get('[role="switch"]').trigger('click')
      expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual({ ...settings, redirect: !settings.redirect })
      wrapper.unmount()
    }
  })
})
