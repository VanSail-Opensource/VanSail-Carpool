import { defineComponent, h, type ShallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { featuresApi, type CouponDefinition } from '@/api/features'
import { t } from '@/i18n'
import { useAdminCoupons } from './useAdminCoupons'

const cache = vi.hoisted(() => ({ restoreItems: vi.fn() }))
vi.mock('@/api/cache/restore', () => cache)
const coupon: CouponDefinition = {
  id: 'coupon-one', code: 'RETAINED', name: 'Reviewed coupon', kind: 'purchase_recurring', discountMode: 'percent',
  valueMinorOrBps: '1250', percentCapMinor: null, eligibleComboIds: ['combo-one'], eligibleSquadIds: [],
  expiresAt: '2026-12-01T08:15:30.123Z', globalUseLimit: null, perUserUseLimit: 1,
  active: true, usageCount: 2, createdAt: '2026-11-13T00:00:00Z', updatedAt: '2026-11-13T00:00:00Z',
}
function setup() {
  let state!: ReturnType<typeof useAdminCoupons>
  const wrapper = mount(defineComponent({ setup() { state = useAdminCoupons(); return () => h('div') } }))
  return { state, wrapper }
}
beforeEach(() => {
  cache.restoreItems.mockReset().mockReturnValue(false)
  vi.spyOn(featuresApi, 'getAdminCoupons').mockResolvedValue({ items: [coupon] })
  vi.spyOn(api, 'getAdminResource').mockResolvedValue({ items: [] })
})
afterEach(() => vi.restoreAllMocks())

describe('administrator coupon recovery', () => {
  it('keeps a confirmed save after a failed refresh and ignores the older cached inventory', async () => {
    cache.restoreItems.mockImplementation((path: string, target: ShallowRef<CouponDefinition[]>) => {
      if (path !== '/api/v1/admin/coupons') return false
      target.value = [coupon]
      return true
    })
    let release!: (value: { items: CouponDefinition[] }) => void
    vi.mocked(featuresApi.getAdminCoupons).mockReturnValueOnce(new Promise(done => { release = done })).mockRejectedValueOnce(new Error('refresh unavailable'))
    const saved = { ...coupon, name: 'Confirmed coupon', valueMinorOrBps: '1500' }
    const save = vi.spyOn(featuresApi, 'saveAdminCoupon').mockResolvedValue(saved)
    const { state, wrapper } = setup()
    state.edit(coupon)
    state.draft.name = 'Confirmed coupon'
    await state.save()
    expect(save).toHaveBeenCalledExactlyOnceWith(coupon.id, { name: 'Confirmed coupon' })
    expect(state.items.value).toEqual([saved])
    expect(state.error.value).toBe(t('adminCoupons.loadFailed'))
    expect(state.editingId.value).toBeUndefined()
    release({ items: [coupon] })
    await flushPromises()
    expect(state.items.value).toEqual([saved])
    expect(state.error.value).toBe(t('adminCoupons.loadFailed'))
    wrapper.unmount()
  })

  it('locks the captured editor and repeated writes until failure, then permits explicit retry', async () => {
    let reject!: (error: Error) => void
    const save = vi.spyOn(featuresApi, 'saveAdminCoupon').mockReturnValueOnce(new Promise((_done, fail) => { reject = fail })).mockResolvedValue({ ...coupon, name: 'Confirmed coupon' })
    const { state, wrapper } = setup()
    await flushPromises()
    state.edit(coupon)
    state.draft.name = 'Confirmed coupon'
    const pending = state.save()
    await state.save()
    state.edit({ ...coupon, id: 'replacement' })
    state.close()
    state.requestDeactivation(coupon)
    expect(state.editingId.value).toBe(coupon.id)
    expect(state.deactivating.value).toBeNull()
    expect(save).toHaveBeenCalledOnce()
    reject(new Error('constructed refusal'))
    await pending
    expect(state.error.value).toBe(t('adminCoupons.saveFailed'))
    expect(state.draft.name).toBe('Confirmed coupon')
    expect(state.busy.value).toBe(false)
    await state.save()
    expect(save.mock.calls).toEqual([[coupon.id, { name: 'Confirmed coupon' }], [coupon.id, { name: 'Confirmed coupon' }]])
    wrapper.unmount()
  })

  it('retains successful deactivation during a read failure and prevents obsolete repeated actions', async () => {
    let release!: () => void
    const deactivate = vi.spyOn(featuresApi, 'deactivateAdminCoupon').mockReturnValue(new Promise<void>(done => { release = done }))
    const { state, wrapper } = setup()
    await flushPromises()
    state.requestDeactivation(coupon)
    const pending = state.deactivate()
    await state.deactivate()
    state.closeDeactivation()
    expect(state.deactivating.value?.id).toBe(coupon.id)
    vi.mocked(featuresApi.getAdminCoupons).mockRejectedValueOnce(new Error('read unavailable'))
    release()
    await pending
    expect(deactivate).toHaveBeenCalledExactlyOnceWith(coupon.id)
    expect(state.items.value[0]?.active).toBe(false)
    expect(state.deactivating.value).toBeNull()
    expect(state.error.value).toBe(t('adminCoupons.loadFailed'))
    state.requestDeactivation(coupon)
    expect(state.deactivating.value).toBeNull()
    wrapper.unmount()
  })

  it('preserves the original exact expiry and sends only changed fields', async () => {
    const save = vi.spyOn(featuresApi, 'saveAdminCoupon').mockResolvedValue(coupon)
    const { state, wrapper } = setup()
    await flushPromises()
    state.edit(coupon)
    state.draft.name = 'Only name changed'
    await state.save()
    expect(save.mock.calls[0]).toEqual([coupon.id, { name: 'Only name changed' }])
    state.edit(coupon)
    state.draft.expiresAt = '2026-12-02'
    await state.save()
    expect(save.mock.calls[1]).toEqual([coupon.id, { expiresAt: '2026-12-02T23:59:59.000Z' }])
    state.edit(coupon)
    state.draft.expiresAt = ''
    await state.save()
    expect(save.mock.calls[2]).toEqual([coupon.id, { expiresAt: null }])
    wrapper.unmount()
  })

  it('keeps exact multiplier precision and rejects rounded percentages or unsafe use limits', async () => {
    const save = vi.spyOn(featuresApi, 'saveAdminCoupon').mockResolvedValue(coupon)
    const { state, wrapper } = setup()
    await flushPromises()
    state.edit()
    Object.assign(state.draft, { code: 'EXACT', name: 'Exact multiplier', kind: 'balance_multiply', factor: '1.0001' })
    await state.save()
    expect(save.mock.calls[0]?.[1]).toMatchObject({ valueMinorOrBps: '10001' })
    state.edit(coupon)
    state.draft.percent = '12.345'
    await state.save()
    expect(save).toHaveBeenCalledOnce()
    expect(state.error.value).toBe(t('adminCoupons.invalidFields'))
    state.draft.percent = '12.50'
    state.draft.globalLimit = '9007199254740993'
    await state.save()
    expect(save).toHaveBeenCalledOnce()
    expect(state.editingId.value).toBe(coupon.id)
    wrapper.unmount()
  })

  it('keeps eligibility selections through an option outage and retries options explicitly', async () => {
    vi.mocked(api.getAdminResource).mockRejectedValueOnce(new Error('options unavailable'))
    const { state, wrapper } = setup()
    await flushPromises()
    state.edit(coupon)
    expect(state.optionsError.value).toBe(t('adminCoupons.optionsFailed'))
    expect(state.draft.eligibleComboIds).toEqual(['combo-one'])
    await state.loadOptions()
    expect(state.optionsError.value).toBeNull()
    expect(state.draft.eligibleComboIds).toEqual(['combo-one'])
    wrapper.unmount()
  })

  it('retires a pending write without refreshing or changing the disposed workspace', async () => {
    let release!: (value: CouponDefinition) => void
    vi.spyOn(featuresApi, 'saveAdminCoupon').mockReturnValue(new Promise(done => { release = done }))
    const { state, wrapper } = setup()
    await flushPromises()
    state.edit(coupon)
    state.draft.name = 'Retired result'
    const pending = state.save()
    wrapper.unmount()
    release({ ...coupon, name: 'Retired result' })
    await pending
    await state.load()
    await state.loadOptions()
    expect(state.items.value).toEqual([coupon])
    expect(featuresApi.getAdminCoupons).toHaveBeenCalledOnce()
    expect(api.getAdminResource).toHaveBeenCalledTimes(2)
  })
})
