import { effectScope } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { FeaturePaymentOrder } from '@/api/features'
import type { OperationReceipt } from '@/api/types'

const api = vi.hoisted(() => ({ getPaymentOrder: vi.fn() }))
vi.mock('@/api/client', () => ({ api }))
vi.mock('./paymentOrderHelpers', () => ({
  paymentQrDataUrl: vi.fn().mockResolvedValue(null),
  isTerminalPaymentStatus: (status: string) => ['paid', 'cancelled', 'expired', 'failed', 'refunded'].includes(status),
}))
import { usePaymentTarget } from './usePaymentTarget'
import { useOperationReceipt } from './useOperationReceipt'

const pending = { id: 'accepted-order', status: 'pending', provider: 'ezpay', paymentUrl: 'https://pay.example/order' } as FeaturePaymentOrder
const receipt: OperationReceipt = { id: 'accepted-operation', kind: 'xnode_command', status: 'queued', createdAt: '2026-11-13T00:00:00Z', updatedAt: '2026-11-13T00:00:00Z' }
function visibility(hidden: boolean): void {
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(hidden)
  document.dispatchEvent(new Event('visibilitychange'))
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('read-only foreground receipt polling', () => {
  it('holds an accepted operation while hidden, resumes it and stops at terminal success', async () => {
    visibility(true)
    const read = vi.fn().mockResolvedValue({ ...receipt, status: 'succeeded' })
    const scope = effectScope()
    const state = scope.run(() => useOperationReceipt(1500, read))!
    state.track(receipt)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(read).not.toHaveBeenCalled()
    expect(state.receipt.value?.id).toBe(receipt.id)
    visibility(false)
    await flushPromises()
    expect(read).toHaveBeenCalledExactlyOnceWith(receipt.id)
    expect(state.receipt.value?.status).toBe('succeeded')
    visibility(true); visibility(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(read).toHaveBeenCalledOnce()
    scope.stop()
  })

  it('does not duplicate a slow checkout read or restart a completed checkout on visibility events', async () => {
    visibility(true)
    let finish!: (order: FeaturePaymentOrder) => void
    api.getPaymentOrder.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const onPaid = vi.fn(), scope = effectScope()
    const target = scope.run(() => usePaymentTarget({ onPaid, onTerminal: vi.fn() }))!
    expect(await target.present(pending)).toBe('pending')
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getPaymentOrder).not.toHaveBeenCalled()
    visibility(false); visibility(true); visibility(false)
    expect(api.getPaymentOrder).toHaveBeenCalledOnce()
    finish({ ...pending, status: 'paid' }); await flushPromises()
    expect(onPaid).toHaveBeenCalledOnce()
    visibility(true); visibility(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getPaymentOrder).toHaveBeenCalledOnce()
    scope.stop()
  })

  it('backs off unavailable checkout reads without forgetting its order and recovers normal cadence', async () => {
    api.getPaymentOrder.mockRejectedValue(new Error('unavailable'))
    const scope = effectScope()
    const target = scope.run(() => usePaymentTarget({ onPaid: vi.fn(), onTerminal: vi.fn() }))!
    await target.present(pending)
    await vi.advanceTimersByTimeAsync(2000)
    expect(api.getPaymentOrder).toHaveBeenCalledTimes(1)
    expect(target.order.value?.id).toBe(pending.id)
    await vi.advanceTimersByTimeAsync(3999)
    expect(api.getPaymentOrder).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(api.getPaymentOrder).toHaveBeenCalledTimes(2)
    api.getPaymentOrder.mockResolvedValue(pending)
    await vi.advanceTimersByTimeAsync(8000)
    expect(api.getPaymentOrder).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(2000)
    expect(api.getPaymentOrder).toHaveBeenCalledTimes(4)
    target.stop()
    visibility(true); visibility(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getPaymentOrder).toHaveBeenCalledTimes(4)
    scope.stop()
  })
})
