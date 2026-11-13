import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { BillingAmountLimits } from '@/api/types'
import { adminBillingApi } from '@/api/adminBilling'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { t } from '@/i18n'
import AdminBillingAmountLimits from './AdminBillingAmountLimits.vue'

const cache = vi.hoisted(() => ({ restoreCached: vi.fn() }))
vi.mock('@/api/cache/restore', () => cache)
const money = (minor: string) => ({ currency: 'TXB' as const, minor, display: '' })
const limits = (minimum: string, maximum: string): BillingAmountLimits => ({
  minimum: money(minimum), maximum: money(maximum), updatedAt: '2026-11-13T00:00:00Z',
})
beforeEach(() => { cache.restoreCached.mockReset().mockReturnValue(false) })
afterEach(() => vi.restoreAllMocks())

describe('billing amount write and refresh ordering', () => {
  it.each(['success', 'failure'])('retires an older live read %s after saving the reviewed exact bounds', async outcome => {
    cache.restoreCached.mockImplementation((_path: string, apply: (value: BillingAmountLimits) => void) => { apply(limits('100', '10000')); return true })
    let resolve!: (value: BillingAmountLimits) => void
    let reject!: (error: Error) => void
    vi.spyOn(adminBillingApi, 'getAmountLimits').mockReturnValue(new Promise((done, fail) => { resolve = done; reject = fail }))
    const update = vi.spyOn(adminBillingApi, 'updateAmountLimits').mockResolvedValue(limits('200', '10000'))
    const wrapper = mount(AdminBillingAmountLimits)
    await flushPromises()
    wrapper.findAllComponents(TxbAmountField)[0]!.vm.$emit('update:modelValue', '2.00')
    await flushPromises()
    expect(await wrapper.vm.save()).toBe(true)
    expect(update).toHaveBeenCalledExactlyOnceWith('200', '10000')
    if (outcome === 'success') resolve(limits('150', '20000'))
    else reject(new Error('retired read failure'))
    await flushPromises()
    expect(wrapper.findAllComponents(TxbAmountField).map(field => field.props('modelValue'))).toEqual(['2.00', '100.00'])
    expect(wrapper.text()).not.toContain(t('errors.adminLoad'))
    wrapper.unmount()
  })
})
