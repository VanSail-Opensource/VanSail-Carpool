import { mount } from '@vue/test-utils'
import { defineComponent, h, shallowRef } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { PurchaseValidity } from '@/utils/purchaseValidity'
import { usePurchaseValidity } from './usePurchaseValidity'

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

describe('purchase boundary lifecycle', () => {
  it('expires without a successful refresh, resumes, and clears its timer on disposal', async () => {
    vi.useFakeTimers()
    const schedule = vi.spyOn(globalThis, 'setTimeout')
    vi.setSystemTime(new Date('2026-10-25T12:00:00Z'))
    const items = shallowRef<PurchaseValidity[]>([{
      status: 'active', validFrom: '2026-10-24T12:00:00Z', validUntil: '2026-10-25T12:00:01Z',
    }])
    const refresh = vi.fn(async () => { /* Offline refresh leaves the cached record intact. */ })
    const wrapper = mount(defineComponent({ setup() {
      const validity = usePurchaseValidity(items, refresh)
      return () => h('div', validity.status(items.value[0]!))
    } }))
    expect(wrapper.text()).toBe('active')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.text()).toBe('expired')
    expect(items.value[0]!.status).toBe('active')
    expect(refresh).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new Event('focus'))
    expect(refresh).toHaveBeenCalledTimes(2)
    items.value = [{ ...items.value[0]!, validUntil: '2026-11-25T12:00:00Z' }]
    await wrapper.vm.$nextTick()
    const ownedIndex = schedule.mock.calls.findIndex(([, delay]) => delay === 2_147_483_647)
    const ownedTimer = schedule.mock.results[ownedIndex]!.value
    const clear = vi.spyOn(globalThis, 'clearTimeout')
    wrapper.unmount()
    expect(clear).toHaveBeenCalledWith(ownedTimer)
    window.dispatchEvent(new Event('focus'))
    expect(refresh).toHaveBeenCalledTimes(2)
  })
})
