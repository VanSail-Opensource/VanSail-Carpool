import { h, type ShallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, type AdminPaymentProfile } from '@/api/client'
import { t } from '@/i18n'
import AdminPaymentProfiles from './AdminPaymentProfiles.vue'

const cache = vi.hoisted(() => ({ restoreItems: vi.fn() }))
vi.mock('@/api/cache/restore', () => cache)
const profile: AdminPaymentProfile = { id: 'one', provider: 'ezpay', providerName: 'First', enabledChannels: ['alipay'], endpoint: 'https://payments.example.test', merchantId: 'merchant', credential: '', acknowledgement: 'ok', enabled: true, configured: true }
const Input = { name: 'InputStub', props: ['modelValue'], emits: ['update:modelValue'], render(this: { modelValue: string; $emit: (event: string, value: string) => void }) {
  return h('input', { value: this.modelValue, onInput: (event: Event) => this.$emit('update:modelValue', (event.target as HTMLInputElement).value) })
} }
function render() {
  const base = {
    Table: { props: ['data'], template: '<div><section v-for="profile in data" :key="profile.id"><p>{{ profile.providerName }}</p><slot name="actions-cell" :row="{ original: profile }" /></section></div>' },
    Slideover: { props: ['open'], template: '<section v-if="open"><slot name="body" /><slot name="footer" /></section>' },
    FormField: { template: '<label><slot /></label>' }, Input, SwitchField: true,
    Button: { props: ['label', 'disabled'], render(this: { label: string; disabled: boolean }) { return h('button', { disabled: this.disabled }, this.label) } },
    ConfirmDialog: { name: 'ConfirmDialogStub', props: ['open'], emits: ['confirm'], template: '<div />' },
    InlineNotice: { template: '<p><slot /></p>' },
  }
  return mount(AdminPaymentProfiles, { global: { stubs: { ...base, ...Object.fromEntries(Object.entries(base).map(([name, value]) => ['U' + name, value])) } } })
}
function cachedProfile(): void {
  cache.restoreItems.mockImplementation((_path: string, target: ShallowRef<AdminPaymentProfile[]>) => { target.value = [profile]; return true })
}
beforeEach(() => { cache.restoreItems.mockReset().mockReturnValue(false) })
afterEach(() => vi.restoreAllMocks())

describe('payment profile batch persistence', () => {
  it('stops after the first failed profile and reports the failure to Settings', async () => {
    vi.spyOn(api, 'getAdminPaymentProfiles').mockResolvedValue({ items: [profile, { ...profile, id: 'two', providerName: 'Second' }] })
    const update = vi.spyOn(api, 'updateAdminPaymentProfile').mockRejectedValueOnce(new Error('credential rotation rejected')).mockResolvedValue(profile)
    const wrapper = render()
    await flushPromises()
    expect(await wrapper.vm.saveAll()).toBe(false)
    expect(update).toHaveBeenCalledOnce()
    expect(update.mock.calls[0]?.[0]).toBe('one')
    wrapper.unmount()
  })

  it.each(['success', 'failure'])('does not let an older cached refresh %s supersede a confirmed edit', async outcome => {
    cachedProfile()
    let resolve!: (value: { items: AdminPaymentProfile[] }) => void
    let reject!: (error: Error) => void
    vi.spyOn(api, 'getAdminPaymentProfiles').mockReturnValue(new Promise((done, fail) => { resolve = done; reject = fail }))
    const update = vi.spyOn(api, 'updateAdminPaymentProfile').mockResolvedValue({ ...profile, providerName: 'Updated' })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[aria-label="' + t('adminWorkspace.details') + '"]').trigger('click')
    const input = wrapper.findAllComponents(Input).find(field => field.props('modelValue') === 'First')!
    input.vm.$emit('update:modelValue', 'Updated')
    await flushPromises()
    expect(await wrapper.vm.saveAll()).toBe(true)
    expect(update.mock.calls[0]?.[1].providerName).toBe('Updated')
    if (outcome === 'success') resolve({ items: [profile] })
    else reject(new Error('retired read failure'))
    await flushPromises()
    expect(wrapper.text()).toContain('Updated')
    expect(wrapper.text()).not.toContain(t('errors.adminLoad'))
    await wrapper.vm.saveAll()
    expect(update.mock.calls[1]?.[1].providerName).toBe('Updated')
    wrapper.unmount()
  })

  it('does not resurrect a deleted profile from a delayed inventory read', async () => {
    cachedProfile()
    let resolve!: (value: { items: AdminPaymentProfile[] }) => void
    vi.spyOn(api, 'getAdminPaymentProfiles').mockReturnValue(new Promise(done => { resolve = done }))
    const remove = vi.spyOn(api, 'deleteAdminPaymentProfile').mockResolvedValue(undefined)
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[aria-label="' + t('adminPaymentProfiles.delete') + '"]').trigger('click')
    wrapper.findComponent({ name: 'ConfirmDialogStub' }).vm.$emit('confirm')
    await flushPromises()
    expect(remove).toHaveBeenCalledExactlyOnceWith('one')
    resolve({ items: [profile] })
    await flushPromises()
    expect(wrapper.text()).not.toContain('First')
    wrapper.unmount()
  })
})
