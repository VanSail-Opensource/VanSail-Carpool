import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, type AdminPaymentProfile } from '@/api/client'
import AdminPaymentProfiles from './AdminPaymentProfiles.vue'

afterEach(() => vi.restoreAllMocks())

describe('payment profile batch persistence', () => {
  it('stops after the first failed profile and reports the failure to Settings', async () => {
    const profile: AdminPaymentProfile = { id: 'one', provider: 'ezpay', providerName: 'First', enabledChannels: ['alipay'], endpoint: 'https://payments.example.test', merchantId: 'merchant', credential: '', acknowledgement: 'ok', enabled: true, configured: true }
    vi.spyOn(api, 'getAdminPaymentProfiles').mockResolvedValue({ items: [profile, { ...profile, id: 'two', providerName: 'Second' }] })
    const update = vi.spyOn(api, 'updateAdminPaymentProfile').mockRejectedValueOnce(new Error('credential rotation rejected')).mockResolvedValue(profile)
    const wrapper = mount(AdminPaymentProfiles)
    await flushPromises()
    expect(await wrapper.vm.saveAll()).toBe(false)
    expect(update).toHaveBeenCalledOnce()
    expect(update.mock.calls[0]?.[0]).toBe('one')
    wrapper.unmount()
  })
})
