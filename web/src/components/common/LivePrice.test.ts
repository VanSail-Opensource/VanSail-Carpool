import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { components } from '@/api/generated'
import LivePrice from './LivePrice.vue'

function decision(validUntil: string): components['schemas']['PricingDecision'] {
  const money = { currency: 'TXB' as const, minor: '100', display: '1.00 TXB' }
  return { dynamicEnabled: true, version: '1', status: 'ADJUSTED', reason: '', publicPrice: money, personalPrice: money, netPrice: money, modelVersion: '1', calibration: '', generatedAt: '2026-11-01T00:00:00Z', validUntil }
}

afterEach(() => vi.useRealTimers())

describe('live price validity', () => {
  it('never presents an invalid expiry as ready and retires superseded timers', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-01T00:00:00Z'))
    const wrapper = mount(LivePrice, { props: { decision: decision('invalid') } })
    expect(wrapper.get('small').attributes('data-state')).toBe('stale')
    await wrapper.setProps({ decision: decision('2026-11-01T00:00:01Z') })
    expect(wrapper.get('small').attributes('data-state')).toBe('ready')
    await wrapper.setProps({ decision: decision('2026-11-01T00:00:03Z') })
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.get('small').attributes('data-state')).toBe('ready')
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.get('small').attributes('data-state')).toBe('stale')
    wrapper.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('rechecks a long validity window instead of expiring at the timer limit', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-01T00:00:00Z'))
    const lifetime = 30 * 24 * 60 * 60 * 1000
    const wrapper = mount(LivePrice, { props: { decision: decision(new Date(Date.now() + lifetime).toISOString()) } })
    await vi.advanceTimersByTimeAsync(2_147_483_647)
    expect(wrapper.get('small').attributes('data-state')).toBe('ready')
    await vi.advanceTimersByTimeAsync(lifetime - 2_147_483_647)
    expect(wrapper.get('small').attributes('data-state')).toBe('stale')
    wrapper.unmount()
  })
})
