import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { hourlyFixture } from '../../../fixtures/hourly-data'
import HourlyAllowanceGraph from './HourlyAllowanceGraph.vue'

describe('observed hourly traffic budgets', () => {
  it('retains purchased and reserved credit after automatic purchases are disabled', async () => {
    const { view } = hourlyFixture(new URLSearchParams('out-of-allowance&extra-off'))
    const usage = { ...view.usage!, extraReservedBytes: '1000000000' }
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage, policy: view.policy.policy, extraEnabled: false } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(false)
    expect(wrapper.find('[data-hourly-extra]').exists()).toBe(true)
    expect(wrapper.find('.hourly-allowance__segment--extra-left').exists()).toBe(true)
    await wrapper.setProps({ usage: { ...usage, extraUsedBytes: usage.extraFundedBytes, extraReservedBytes: '0' } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('treats a zero free allowance as exhausted even before any observed traffic', () => {
    const { view } = hourlyFixture(new URLSearchParams())
    const usage = { ...view.usage!, freeAllowanceBytes: '0', freeUsedBytes: '0', freeReservedBytes: '0', extraFundedBytes: '0', extraUsedBytes: '0', extraReservedBytes: '0' }
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage, policy: { ...view.policy.policy, dailyAllowanceBytes: '0' }, extraEnabled: false } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
