/* eslint-disable vue/one-component-per-file */
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import XNodeCertificateProfile from './XNodeCertificateProfile.vue'
import { certificateErrors, type CertificateProfile } from './certificateForm'

vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }), localizedError: (_caught: unknown, key: string) => key }))
const stubs = {
  FormField: { props: { label: { type: String, default: '' }, error: { type: String, default: '' }, required: Boolean }, template: '<label :data-field="label" :data-required="required" :data-error="error"><slot /></label>' },
  Input: defineComponent({ props: { modelValue: { type: String, default: '' } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('input', { value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value) }) }),
  Textarea: defineComponent({ props: { modelValue: { type: String, default: '' } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('textarea', { value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLTextAreaElement).value) }) }),
  Switch: { template: '<span />' },
  Select: defineComponent({ props: { modelValue: { type: String, default: '' }, items: { type: Array, default: () => [] } }, emits: ['update:modelValue'], setup: (props, { emit }) => () => h('select', { value: props.modelValue, onChange: (event: Event) => emit('update:modelValue', (event.target as HTMLSelectElement).value) }, props.items.map(item => { const option = item as { value: string; label: string }; return h('option', { value: option.value }, option.label) })) }),
  Button: { render: () => h('button') },
}
function editor(profile: CertificateProfile) {
  return mount(XNodeCertificateProfile, { props: { id: 'primary', modelValue: profile, providers: { saved: { type: 'cloudflare', credentials: { token: '' } } } }, global: { stubs } })
}

describe('certificate material and provider controls', () => {
  it.each(['file', 'content'] as const)('keeps inferred %s controls visible and invalid when one material field is cleared', async mode => {
    const certificate = '-----BEGIN CERTIFICATE-----\nQUJD\n-----END CERTIFICATE-----'
    const key = '-----BEGIN PRIVATE KEY-----\nQUJD\n-----END PRIVATE KEY-----'
    const profile: CertificateProfile = { config: mode === 'file' ? { cert_mode: '', cert_file: './cert.pem', key_file: './key.pem' } : { cert_mode: '', cert_content: certificate, key_content: key } }
    const field = mode === 'file' ? 'xnode.certificateFile' : 'xnode.certificateContent', keyName = mode === 'file' ? 'cert_file' : 'cert_content'
    const control = mode === 'file' ? 'input' : 'textarea', original = mode === 'file' ? './cert.pem' : certificate
    const wrapper = editor(profile)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.get(`[data-field="${field}"] ${control}`).setValue('')
    const cleared = wrapper.emitted('update:modelValue')?.at(-1)?.[0] as CertificateProfile
    expect(cleared.config.cert_mode).toBe(mode)
    expect(certificateErrors(cleared, {})).toHaveProperty(keyName)
    expect(profile.config.cert_mode).toBe('')
    await wrapper.setProps({ modelValue: cleared })
    expect(wrapper.find(`[data-field="${field}"]`).exists()).toBe(true)
    expect(wrapper.get(`[data-field="${field}"]`).attributes('data-error')).not.toBe('')
    await wrapper.get(`[data-field="${field}"] ${control}`).setValue(original)
    const restored = wrapper.emitted('update:modelValue')?.at(-1)?.[0] as CertificateProfile
    expect(certificateErrors(restored, {})).toEqual({})
    await wrapper.setProps({ modelValue: restored })
    expect(wrapper.get(`[data-field="${field}"]`).attributes('data-error')).toBe('')
    wrapper.unmount()
  })
  it('renders inferred file controls without rewriting mode until the user selects one', async () => {
    const profile = { config: { cert_mode: '', cert_file: './cert.pem', key_file: '../key.pem' } }
    const wrapper = editor(profile)
    expect(wrapper.get('select').element).toHaveProperty('value', 'file')
    expect(wrapper.find('[data-field="xnode.certificateFile"]').exists()).toBe(true)
    expect(wrapper.find('[data-field="xnode.domain"]').exists()).toBe(false)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.get('select').setValue('content')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ config: { cert_mode: 'content', cert_file: './cert.pem', key_file: '../key.pem' } })
    expect(profile.config.cert_mode).toBe('')
    wrapper.unmount()
  })
  it('renders supplied legacy PEM with an optional/wildcard name and no ACME identity fields', async () => {
    const profile = { config: { cert_mode: '', cert_content: '-----BEGIN CERTIFICATE-----\nQUJD\n-----END CERTIFICATE-----', key_content: '-----BEGIN PRIVATE KEY-----\nQUJD\n-----END PRIVATE KEY-----' } }
    const wrapper = editor(profile)
    expect(wrapper.get('select').element).toHaveProperty('value', 'content')
    expect(wrapper.get('[data-field="xnode.domain"]').attributes('data-required')).toBe('false')
    expect(wrapper.get('[data-field="xnode.domain"]').attributes('data-error')).toBe('')
    expect(wrapper.find('[data-field="xnode.email"]').exists()).toBe(false)
    expect(wrapper.findAll('textarea')).toHaveLength(2)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.setProps({ modelValue: { config: { ...profile.config, domain: '*.example.com' } } })
    expect(wrapper.get('[data-field="xnode.domain"]').attributes('data-error')).toBe('')
    wrapper.unmount()
  })
  it('selects configured providers and retains legacy inline references without a secret editor', () => {
    const wrapper = editor({ config: { cert_mode: 'dns', domain: 'proxy.example', dns_provider: 'cloudflare', dns_env: { token: 'retained-provider-secret' } } })
    expect(wrapper.text()).toContain('saved (cloudflare)')
    expect(wrapper.text()).toContain('certificates.legacyProvider')
    expect(wrapper.html()).not.toContain('retained-provider-secret')
    expect(wrapper.find('textarea').exists()).toBe(false)
    wrapper.unmount()
  })

  it('imports PEM through native file controls and preserves the matching field type', async () => {
    const wrapper = editor({ config: { cert_mode: 'content', domain: 'proxy.example', cert_content: '', key_content: '' } })
    const files = wrapper.findAll('input[type="file"]')
    const pem = '-----BEGIN CERTIFICATE-----\nQUJD\n-----END CERTIFICATE-----'
    Object.defineProperty(files[0]!.element, 'files', { value: [{ size: pem.length, text: async () => pem }] })
    await files[0]!.trigger('change'); await flushPromises()
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toMatchObject({ config: { cert_content: pem, key_content: '' } })
    wrapper.unmount()
  })

  it('rejects oversized imports before reading their content', async () => {
    const wrapper = editor({ config: { cert_mode: 'content', domain: 'proxy.example' } })
    const read = vi.fn()
    const file = wrapper.findAll('input[type="file"]')[1]!
    Object.defineProperty(file.element, 'files', { value: [{ size: 1024 * 1024 + 1, text: read }] })
    await file.trigger('change'); await flushPromises()
    expect(read).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('certificates.fileTooLarge')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })
})
