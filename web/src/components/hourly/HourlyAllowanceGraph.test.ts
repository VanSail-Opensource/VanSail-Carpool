import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { hourlyFixture } from '../../../fixtures/hourly-data'
import HourlyAllowanceGraph from './HourlyAllowanceGraph.vue'
import HourlyThrottleMeter from './HourlyThrottleMeter.vue'
import { formatBytes } from '@/utils/format'

describe('observed hourly traffic budgets', () => {
  it('retains purchased and reserved credit after automatic purchases are disabled', async () => {
    const { view } = hourlyFixture(new URLSearchParams('out-of-allowance&extra-off'))
    const usage = { ...view.usage!, extraReservedBytes: '1000000000' }
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage, policy: view.policy.policy, extraEnabled: false } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(false)
    expect(wrapper.findComponent(HourlyThrottleMeter).exists()).toBe(false)
    expect(wrapper.find('[data-hourly-extra]').exists()).toBe(true)
    expect(wrapper.find('.hourly-allowance__segment--extra-left').exists()).toBe(true)
    expect(wrapper.get('.hourly-allowance__legend').text()).toContain(formatBytes('1000000000'))
    await wrapper.setProps({ usage: { ...usage, extraUsedBytes: usage.extraFundedBytes, extraReservedBytes: '0' } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    wrapper.getComponent(HourlyThrottleMeter).vm.$emit('extra', true)
    expect(wrapper.emitted('extra')).toEqual([[true]])
    wrapper.unmount()
  })

  it('treats a zero free allowance as exhausted even before any observed traffic', () => {
    const { view } = hourlyFixture(new URLSearchParams())
    const usage = { ...view.usage!, freeAllowanceBytes: '0', freeUsedBytes: '0', freeReservedBytes: '0', extraFundedBytes: '0', extraUsedBytes: '0', extraReservedBytes: '0' }
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage, policy: { ...view.policy.policy, dailyAllowanceBytes: '0' }, extraEnabled: false } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    expect(wrapper.getComponent(HourlyThrottleMeter).props('enabled')).toBe(false)
    wrapper.unmount()
  })

  it('passes busy and purchase availability to the inline switch without changing consent', async () => {
    const { view } = hourlyFixture(new URLSearchParams('credit-exhausted&no-extra'))
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage: view.usage, policy: view.policy.policy, extraEnabled: false, busy: true } })
    const throttle = wrapper.getComponent(HourlyThrottleMeter)
    expect(throttle.props('extraAvailable')).toBe(false)
    expect(throttle.props('busy')).toBe(true)
    expect(wrapper.emitted('extra')).toBeUndefined()
    await wrapper.setProps({ busy: false, policy: { ...view.policy.policy, extraGBPriceTXBMinor: '125' } })
    expect(throttle.props('extraAvailable')).toBe(true)
    expect(throttle.props('busy')).toBe(false)
    expect(throttle.props('extraPrice')).toBe('125')
    expect(wrapper.emitted('extra')).toBeUndefined()
    wrapper.unmount()
  })

  it('shows reduced speed for recovery exhaustion even with automatic purchases enabled', () => {
    const { view } = hourlyFixture(new URLSearchParams('recovery-exhausted&extra'))
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage: view.usage, policy: view.policy.policy, extraEnabled: true } })
    expect(wrapper.getComponent(HourlyThrottleMeter).props('enabled')).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('109.81 MiB')
    expect(wrapper.find('[data-hourly-accounting]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not boost on consent; it waits for funded usable traffic', async () => {
    const { view } = hourlyFixture(new URLSearchParams('credit-exhausted'))
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage: view.usage, policy: view.policy.policy, extraEnabled: false } })
    await wrapper.setProps({ extraEnabled: true })
    expect(wrapper.getComponent(HourlyThrottleMeter).props('enabled')).toBe(true)
    expect(wrapper.find('[data-hourly-boost]').exists()).toBe(false)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    await wrapper.setProps({ usage: { ...view.usage!, extraFundedBytes: '3000000000' } })
    // The hosted component environment prefers reduced motion: funding swaps
    // immediately and keeps only the static confirmation flash.
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(false)
    expect(wrapper.find('[data-hourly-boost]').exists()).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('keeps current open credit spendable and unknown credit separate from Used', () => {
    const { view } = hourlyFixture(new URLSearchParams('open-credit'))
    const usage = { ...view.usage!, accounting: { ...view.usage!.accounting!, openReservedBytes: '114103556', recoveryUnknownBytes: '1048576', freeRecoveryUnknownBytes: '1048576' } }
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage, policy: view.policy.policy, extraEnabled: false } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(false)
    expect(wrapper.get('.hourly-allowance__legend').text()).toContain('108.81 MiB')
    expect(wrapper.get('.hourly-allowance__legend').text()).toContain(formatBytes('2032331516'))
    expect(wrapper.find('.hourly-allowance__segment--unknown').exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not celebrate a UTC-day reset as a purchase', async () => {
    const { view } = hourlyFixture(new URLSearchParams('credit-exhausted'))
    const wrapper = mount(HourlyAllowanceGraph, { props: { usage: view.usage, policy: view.policy.policy, extraEnabled: true } })
    await wrapper.setProps({ usage: { ...view.usage!, dayUTC: '2099-01-01', freeUsedBytes: '0', extraFundedBytes: '3000000000' } })
    expect(wrapper.find('[data-hourly-throttle]').exists()).toBe(false)
    expect(wrapper.find('[data-hourly-boost]').exists()).toBe(false)
    wrapper.unmount()
  })
})
