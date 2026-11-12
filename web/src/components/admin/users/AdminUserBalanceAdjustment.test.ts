import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import AdminUserBalanceAdjustment from './AdminUserBalanceAdjustment.vue'

afterEach(() => vi.restoreAllMocks())

describe('administrator balance adjustment recovery', () => {
  it('reuses an ambiguous attempt only for the same member, delta and reason', async () => {
    const adjust = vi.spyOn(api, 'adjustBalance').mockRejectedValue(new Error('connection closed after commit'))
    const wrapper = mount(AdminUserBalanceAdjustment, {
      props: { userId: 'member-1' },
      global: { stubs: {
        USlideover: { template: '<section><slot name="body" /></section>' },
        UFormField: { template: '<div><slot /></div>' },
        InlineNotice: true,
      } },
    })
    const amountInput = () => wrapper.get('#profile-balance-adjustment')
    const reasonInput = () => wrapper.get('input[maxlength="300"]')
    await amountInput().setValue('12.50')
    await reasonInput().setValue('Manual correction')
    const submit = async (index: number) => {
      await wrapper.findAll('.row-actions button')[index].trigger('click')
      await flushPromises()
    }
    await submit(0)
    await submit(0)
    await submit(1)
    expect(adjust.mock.calls[0]).toEqual(['member-1', '1250', 'Manual correction', expect.any(String)])
    expect(adjust.mock.calls[1][3]).toBe(adjust.mock.calls[0][3])
    expect(adjust.mock.calls[2][3]).not.toBe(adjust.mock.calls[0][3])
    await reasonInput().setValue('Second correction')
    await submit(0)
    expect(adjust.mock.calls[3][3]).not.toBe(adjust.mock.calls[0][3])
    await reasonInput().setValue('Manual correction')
    adjust.mockResolvedValue({} as Awaited<ReturnType<typeof api.adjustBalance>>)
    await submit(0)
    expect(adjust.mock.calls[4][3]).toBe(adjust.mock.calls[0][3])
    await amountInput().setValue('12.50')
    await reasonInput().setValue('Manual correction')
    await submit(0)
    expect(adjust.mock.calls[5][3]).not.toBe(adjust.mock.calls[0][3])
    wrapper.unmount()
  })
})
