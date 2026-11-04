import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { Purchase } from '@/api/types'
import EntitlementSummary from './EntitlementSummary.vue'
import RideSummaryFace from './RideSummaryFace.vue'

const purchase = (status: 'active' | 'queued'): Purchase => ({
  id: 'purchase', comboName: 'Tokyo', squadUuids: [], status,
  validFrom: new Date(Date.now() + (status === 'queued' ? 60_000 : -60_000)).toISOString(),
  validUntil: new Date(Date.now() + 120_000).toISOString(),
} as unknown as Purchase)
const options = { global: { mocks: { $t: (key: string) => key } } }

describe('entitlement summary', () => {
  it('hides the combo summary for a member without a combo', () => {
    const wrapper = shallowMount(EntitlementSummary, options)
    expect(wrapper.find('.home-ride').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows the active combo directly', () => {
    const wrapper = shallowMount(EntitlementSummary, { ...options, props: { active: purchase('active') } })
    expect(wrapper.findComponent(RideSummaryFace).props('active').comboName).toBe('Tokyo')
    expect(wrapper.find('.home-ride__queued').exists()).toBe(false)
    wrapper.unmount()
  })

  it('retains a queued combo without an active combo', () => {
    const wrapper = shallowMount(EntitlementSummary, { ...options, props: { queued: purchase('queued') } })
    expect(wrapper.find('.home-ride__queued').exists()).toBe(true)
    expect(wrapper.findComponent(RideSummaryFace).exists()).toBe(false)
    wrapper.unmount()
  })
})
