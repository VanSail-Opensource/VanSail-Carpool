import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import USelectMenu from '@nuxt/ui/runtime/components/SelectMenu.vue'
import { adminOperationsApi } from '@/api/adminOperations'
import { featuresApi, type CouponDefinition } from '@/api/features'
import { ApiError } from '@/api/http'
import { t } from '@/i18n'
import AdminUserCouponActions from './AdminUserCouponActions.vue'

afterEach(() => vi.restoreAllMocks())

describe('administrator coupon grant recovery', () => {
  it('retries a possibly committed grant with the same key and distinguishes changed reasons', async () => {
    vi.spyOn(featuresApi, 'getAdminCoupons').mockResolvedValue({ items: [{ id: 'coupon', kind: 'purchase_once', active: true, code: 'SUPPORT', name: 'Support' } as CouponDefinition] })
    const grant = vi.spyOn(adminOperationsApi, 'grantUserCoupon').mockRejectedValue(new ApiError(503, { code: 'TEMPORARY_FAILURE', message: '' }))
    const wrapper = mount(AdminUserCouponActions, {
      props: { userId: 'member', grants: [] },
      global: { stubs: {
        Modal: { props: ['open'], template: '<section v-if="open"><slot name="body" /><slot name="footer" :close="() => {}" /></section>' },
        FormField: { template: '<div><slot /></div>' }, SelectMenu: true, Table: true, InlineNotice: true,
      } },
    })
    await wrapper.findAll('button').find(button => button.text() === t('adminUserProfile.addCoupon'))!.trigger('click')
    await flushPromises()
    wrapper.getComponent(USelectMenu).vm.$emit('update:modelValue', 'coupon')
    await wrapper.get('textarea').setValue('Support correction')
    const submit = async () => {
      await wrapper.findAll('button').find(button => button.text() === t('common.confirm'))!.trigger('click')
      await flushPromises()
    }
    await submit(); await submit()
    expect(grant.mock.calls[0]).toEqual(['member', 'coupon', 'Support correction', expect.any(String)])
    expect(grant.mock.calls[1]![3]).toBe(grant.mock.calls[0]![3])
    await wrapper.get('textarea').setValue('A different correction')
    await submit()
    expect(grant.mock.calls[2]![3]).not.toBe(grant.mock.calls[0]![3])
    wrapper.unmount()
  })
})
