/* eslint-disable vue/one-component-per-file */

import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { XNodeSettings } from '@/api/xnode'
import XNodeCertificateSettings from './XNodeCertificateSettings.vue'

vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const stubs = {
  XNodeCertificateProfile: { template: '<div />' },
  XNodeCertificateStatus: defineComponent({ props: { value: { type: Object, default: () => ({}) } }, setup: props => () => h('div', { 'data-status-phase': props.value.phase ?? '' }) }),
  XNodeCertificateLifecycle: defineComponent({ props: { canRun: Boolean }, setup: props => () => h('div', { 'data-lifecycle': '', 'data-can-run': String(props.canRun) }) }),
  FormField: { props: { label: { type: String, default: '' }, error: { type: String, default: '' } }, template: '<label :data-error="error"><slot /></label>' },
  Select: { render: () => h('select') },
  Button: defineComponent({ props: { label: { type: String, default: '' }, disabled: Boolean }, setup: props => () => h('button', { type: 'button', disabled: props.disabled }, props.label) }),
}
function editor(settings: XNodeSettings, nodeId = '') {
  return mount(XNodeCertificateSettings, { props: { modelValue: settings, providers: {}, inbounds: ['paid'], nodeId }, global: { stubs } })
}

describe('certificate profile references', () => {
  it('prevents bound profile deletion until the administrator explicitly unbinds it', async () => {
    const profile = { config: { cert_mode: 'self', domain: 'proxy.example' } }
    const wrapper = editor({ certificates: { primary: profile }, inboundCertificates: { paid: 'primary' } })
    const remove = wrapper.get('[aria-label="certificates.removeProfile"]')
    expect(remove.attributes('disabled')).toBeDefined()
    await remove.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.setProps({ modelValue: { certificates: { primary: profile }, inboundCertificates: {} } })
    await wrapper.get('[aria-label="certificates.removeProfile"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual({ certificates: {}, inboundCertificates: {} })
    wrapper.unmount()
  })

  it('surfaces broken retained references rather than silently removing them', () => {
    const settings = { certificates: {}, inboundCertificates: { paid: 'missing' } }
    const wrapper = editor(settings)
    expect(wrapper.emitted('valid')?.at(-1)).toEqual([false])
    expect(wrapper.find('[data-error="certificates.bindingInvalid"]').exists()).toBe(true)
    expect(settings.inboundCertificates.paid).toBe('missing')
    wrapper.unmount()
  })

  it('keeps new unsaved profiles editable without requesting nonexistent command history', async () => {
    const wrapper = editor({ certificates: {} }, 'node')
    await wrapper.get('button').trigger('click')
    const value = wrapper.emitted('update:modelValue')?.[0]?.[0] as XNodeSettings
    expect(Object.keys(value.certificates ?? {})).toEqual(['certificate-1'])
    await wrapper.setProps({ modelValue: value })
    expect(wrapper.find('[data-lifecycle]').exists()).toBe(false)
    expect(wrapper.emitted('valid')?.at(-1)).toEqual([false])
    wrapper.unmount()
  })

  it('permits certificate recovery after failed activation without treating cached progress as newer terminal evidence', async () => {
    const metric = { nodeId: 'node', connected: true, ready: false, synchronized: false, stale: false, telemetryAvailable: false, machine: {}, logs: [],
      capabilities: ['certificate_lifecycle_v1'], certificates: { primary: { phase: 'failed', active: true, command_id: 'operation' } },
      certificateProgress: { command_id: 'operation', profile_id: 'primary', action: 'renew' as const, phase: 'applying' as const, received_at: new Date().toISOString() } }
    const wrapper = mount(XNodeCertificateSettings, { props: { modelValue: { certificates: { primary: { config: { cert_mode: 'http', domain: 'proxy.example' } } } },
      providers: {}, nodeId: 'node', revision: 3, saved: true, savedProfileIds: ['primary'], metric }, global: { stubs } })
    expect(wrapper.get('[data-lifecycle]').attributes('data-can-run')).toBe('true')
    expect(wrapper.get('[data-status-phase]').attributes('data-status-phase')).toBe('failed')
    await wrapper.setProps({ metric: { ...metric, stale: true } })
    expect(wrapper.get('[data-lifecycle]').attributes('data-can-run')).toBe('false')
    wrapper.unmount()
  })
})
