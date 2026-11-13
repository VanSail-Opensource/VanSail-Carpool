import { effectScope } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { PaymentOperation } from '@/api/types'

const { createPaymentOrder, cancelPaymentOrder } = vi.hoisted(() => ({ createPaymentOrder: vi.fn(), cancelPaymentOrder: vi.fn() }))
vi.mock('@/api/client', () => ({ api: { createPaymentOrder, cancelPaymentOrder } }))
import { usePaymentOrderOperations } from './usePaymentOrderOperations'

afterEach(() => { vi.clearAllMocks(); vi.useRealTimers() })

describe('payment operation recovery', () => {
  it('retains cancellation keys per order across ambiguous failures', async () => {
    cancelPaymentOrder.mockRejectedValue(new Error('lost response'))
    const scope = effectScope()
    const operations = scope.run(() => usePaymentOrderOperations({ onTerminal: vi.fn() }))!
    for (const id of ['order-1', 'order-2', 'order-1']) {
      await expect(operations.queueCancellation(id)).rejects.toThrow('lost response')
    }
    const calls = cancelPaymentOrder.mock.calls
    expect(calls[0][1]).not.toBe(calls[1][1])
    expect(calls[2][1]).toBe(calls[0][1])
    scope.stop()
  })

  it('does not track an accepted response after reset and preserves its retry key', async () => {
    vi.useFakeTimers()
    let resolve!: (result: PaymentOperation) => void
    createPaymentOrder.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    const scope = effectScope()
    const operations = scope.run(() => usePaymentOrderOperations({ onTerminal: vi.fn() }))!
    const pending = operations.queueCreate('ezpay:alipay', '2000')
    operations.reset()
    resolve({ paymentOrderId: 'order-1', operation: { id: 'operation-1', kind: 'payment_create', status: 'queued', createdAt: '', updatedAt: '' } })
    await pending
    expect(operations.receipt.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
    createPaymentOrder.mockRejectedValueOnce(new Error('offline'))
    await expect(operations.queueCreate('ezpay:alipay', '2000')).rejects.toThrow('offline')
    expect(createPaymentOrder.mock.calls[1][2]).toBe(createPaymentOrder.mock.calls[0][2])
    scope.stop()
  })
})
