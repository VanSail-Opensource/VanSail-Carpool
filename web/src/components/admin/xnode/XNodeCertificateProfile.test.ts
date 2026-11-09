import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import XNodeCertificateProfile from './XNodeCertificateProfile.vue'
import type { CertificateProfile } from './certificateForm'

vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }), localizedError: (_caught: unknown, key: string) => key }))
const stubs = {
  FormField: { template: '<label><slot /></label>' }, Input: { render: () => h('input') }, Textarea: { render: () => h('textarea') }, Switch: { template: '<span />' },
  Select: defineComponent({ props: { items: { type: Array, default: () => [] } }, setup: props => () => h('select', props.items.map(item => { const option = item as { value: string; label: string }; return h('option', { value: option.value }, option.label) })) }),
  Button: { render: () => h('button') },
}
function editor(profile: CertificateProfile) {
  return mount(XNodeCertificateProfile, { props: { id: 'primary', modelValue: profile, providers: { saved: { type: 'cloudflare', credentials: { token: '' } } } }, global: { stubs } })
}

describe('certificate material and provider controls', () => {
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
