import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { AdminEntitlement } from '@/api/adminOperations'
import AdminEntitlementEditor from './AdminEntitlementEditor.vue'

describe('entitlement edit review lock', () => {
  it('retains the reviewed draft and rejects close/resubmission while its write is pending', async () => {
    const price = { currency: 'TXB', minor: '1000', display: '10.00 TXB' } as const
    const item: AdminEntitlement = { id: 'purchase', userId: 'member', comboId: 'combo', comboName: 'Plan', price, grossPrice: price, couponDiscount: { ...price, minor: '0', display: '0.00 TXB' }, couponGrantId: null, status: 'active', validFrom: '2026-11-01T00:00:00Z', validUntil: '2026-12-01T00:00:00Z', trafficLimitBytes: '1000', resetStrategy: 'MONTH_ROLLING', squadUuids: [], createdAt: '2026-11-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z' }
    const wrapper = shallowMount(AdminEntitlementEditor, {
      props: { open: true, item, combos: [], squads: [], busy: false, optionsLoading: false },
      global: { renderStubDefaultSlot: true, stubs: {
        Slideover: { name: 'Slideover', props: ['open', 'close'], emits: ['update:open'], template: '<section><slot name="body" /><slot name="footer" /></section>' },
      } },
    })
    wrapper.getComponent({ name: 'Textarea' }).vm.$emit('update:modelValue', 'Reviewed entitlement correction')
    wrapper.getComponent({ name: 'Form' }).vm.$emit('submit')
    expect(wrapper.emitted('save')).toHaveLength(1)
    expect(wrapper.emitted('save')![0]![0]).toMatchObject({ version: item.updatedAt, trafficLimitBytes: '1000', reason: 'Reviewed entitlement correction' })
    await wrapper.setProps({ busy: true })
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    expect(wrapper.getComponent({ name: 'Slideover' }).props('close')).toBe(false)
    wrapper.getComponent({ name: 'Form' }).vm.$emit('submit')
    wrapper.getComponent({ name: 'Slideover' }).vm.$emit('update:open', false)
    expect(wrapper.emitted('save')).toHaveLength(1)
    expect(wrapper.emitted('update:open')).toBeUndefined()
    wrapper.unmount()
  })
})
