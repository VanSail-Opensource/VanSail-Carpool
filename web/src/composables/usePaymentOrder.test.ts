import { effectScope, nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { FeaturePaymentOrder } from '@/api/features'
import type { OperationReceipt, PaymentOperation } from '@/api/types'
import { setLocale, t } from '@/i18n'

const { createPaymentOrder, getPaymentOrder, cancelPaymentOrder, getOperation } = vi.hoisted(() => ({
  createPaymentOrder: vi.fn(),
  getPaymentOrder: vi.fn(),
  cancelPaymentOrder: vi.fn(),
  getOperation: vi.fn(),
}))

vi.mock('@/api/client', () => ({ api: { createPaymentOrder, getPaymentOrder, cancelPaymentOrder } }))
vi.mock('@/api/memberOperations', () => ({ memberOperationsApi: { getOperation } }))
vi.mock('qrcode', () => ({
  default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,qr') },
}))

import { isTerminalPaymentStatus, usePaymentOrder } from './usePaymentOrder'

const timestamp = '2026-08-07T00:00:00Z'
const pendingOrder: FeaturePaymentOrder = {
  id: 'payment-1', methodId: 'ezpay:alipay', provider: 'ezpay', providerRail: 'alipay', status: 'pending',
  txb: { currency: 'TXB', minor: '2000', display: '20.00 TXB' }, payableAmount: '20.00', payableCurrency: 'CNY',
  rateSnapshot: '1', rateDirection: 'txb_per_currency', paymentUrl: 'https://pay.example/order-1',
  qrPayload: 'https://pay.example/order-1', receivingAddress: null, actualCryptoAmount: null,
  actualCryptoCurrency: null, expiresAt: '2026-08-07T01:00:00Z', paidAt: null, refundedAt: null,
  cancelledAt: null, cancelReason: '', providerCancelStatus: '', createdAt: timestamp, updatedAt: timestamp,
}

function receipt(kind: 'payment_create' | 'payment_cancel', status: OperationReceipt['status']): OperationReceipt {
  return { id: `${kind}-1`, kind, status, createdAt: timestamp, updatedAt: timestamp }
}

function accepted(kind: 'payment_create' | 'payment_cancel'): PaymentOperation {
  return { paymentOrderId: pendingOrder.id, operation: receipt(kind, 'queued') }
}

async function finishOperation(kind: 'payment_create' | 'payment_cancel'): Promise<void> {
  getOperation.mockResolvedValueOnce(receipt(kind, 'succeeded'))
  await vi.advanceTimersByTimeAsync(1500)
}

describe('payment order polling', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
    setLocale('en')
  })

  it('recognizes authoritative terminal provider states', () => {
    expect(isTerminalPaymentStatus('paid')).toBe(true)
    expect(isTerminalPaymentStatus('refunded')).toBe(true)
    expect(isTerminalPaymentStatus('pending')).toBe(false)
  })

  it('opens a checkout only after its durable create operation succeeds', async () => {
    vi.useFakeTimers()
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    getPaymentOrder
      .mockResolvedValueOnce(pendingOrder)
      .mockResolvedValueOnce({ ...pendingOrder, status: 'paid', paidAt: '2026-08-07T00:01:00Z' })
    const onPaid = vi.fn()
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid }))!

    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    expect(payment.stage.value).toBe('creating')
    await finishOperation('payment_create')
    expect(payment.stage.value).toBe('pending')

    await vi.advanceTimersByTimeAsync(2000)
    expect(payment.stage.value).toBe('paid')
    await vi.advanceTimersByTimeAsync(700)
    expect(onPaid).toHaveBeenCalledOnce()
    scope.stop()
  })

  it('reuses one create key after an ambiguous request failure', async () => {
    createPaymentOrder.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(accepted('payment_create'))
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!
    payment.chooseMethod('ezpay:alipay')

    await payment.createOrder()
    await payment.createOrder()

    const firstKey = createPaymentOrder.mock.calls[0]?.[2]
    expect(firstKey).toMatch(/^[0-9a-f-]{36}$/)
    expect(createPaymentOrder.mock.calls[1]?.[2]).toBe(firstKey)
    scope.stop()
  })

  it('keeps ambiguous create keys scoped to the reviewed method and amount', async () => {
    createPaymentOrder.mockRejectedValue(new Error('network'))
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!
    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    payment.amount.value = '30'
    await payment.createOrder()
    payment.amount.value = '20'
    await payment.createOrder()
    const keys = createPaymentOrder.mock.calls.map(call => call[2])
    expect(keys[0]).not.toBe(keys[1])
    expect(keys[2]).toBe(keys[0])
    scope.stop()
  })

  it('allows a newly created crypto order to be replaced after its payment window expires', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(timestamp))
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    getPaymentOrder.mockResolvedValue({ ...pendingOrder, provider: 'builtin_usdt', methodId: 'builtin_usdt:usdt.trc20' })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!
    payment.chooseMethod('builtin_usdt:usdt.trc20')
    await payment.createOrder()
    expect(payment.stage.value).toBe('creating')
    await finishOperation('payment_create')
    expect(payment.stage.value).toBe('pending')
    expect(payment.canRecreate.value).toBe(true)
    await payment.recreateOrder()
    expect(createPaymentOrder).toHaveBeenCalledTimes(1)
    vi.setSystemTime(new Date(pendingOrder.expiresAt))
    await payment.recreateOrder()
    expect(createPaymentOrder).toHaveBeenCalledTimes(2)
    expect(payment.stage.value).toBe('creating')
    scope.stop()
  })

  it('ignores an older order lookup after the payment sheet is reset', async () => {
    vi.useFakeTimers()
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    let finishLookup!: (value: FeaturePaymentOrder) => void
    getPaymentOrder.mockImplementation(() => new Promise(resolve => { finishLookup = resolve }))
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!
    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    await finishOperation('payment_create')
    expect(getPaymentOrder).toHaveBeenCalledTimes(1)
    payment.reset([])
    finishLookup(pendingOrder)
    await nextTick()
    await nextTick()
    expect(payment.stage.value).toBe('configure')
    expect(payment.order.value).toBeNull()
    scope.stop()
  })

  it('queues cancellation and waits for its receipt', async () => {
    vi.useFakeTimers()
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    cancelPaymentOrder.mockResolvedValue(accepted('payment_cancel'))
    getPaymentOrder
      .mockResolvedValueOnce(pendingOrder)
      .mockResolvedValueOnce({ ...pendingOrder, status: 'cancelled', cancelledAt: timestamp })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!

    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    await finishOperation('payment_create')
    await payment.cancelOrder()
    expect(payment.stage.value).toBe('cancelling')
    await finishOperation('payment_cancel')

    expect(payment.stage.value).toBe('cancelled')
    expect(cancelPaymentOrder.mock.calls[0]?.[0]).toBe('payment-1')
    expect(cancelPaymentOrder.mock.calls[0]?.[1]).toMatch(/^[0-9a-f-]{36}$/)
    scope.stop()
  })

  it.each(['failed', 'succeeded'] as const)('keeps a pending checkout after a %s cancellation receipt', async (status) => {
    vi.useFakeTimers()
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    cancelPaymentOrder.mockResolvedValue(accepted('payment_cancel'))
    getPaymentOrder.mockResolvedValue(pendingOrder)
    const onPaid = vi.fn()
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid }))!

    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    await finishOperation('payment_create')
    await payment.cancelOrder()
    getOperation.mockResolvedValueOnce(receipt('payment_cancel', status))
    await vi.advanceTimersByTimeAsync(1500)

    expect(payment.order.value?.status).toBe('pending')
    expect(payment.stage.value).toBe('pending')
    expect(payment.canCreate.value).toBe(false)
    if (status === 'failed') expect(payment.error.value).toBeTruthy()

    getPaymentOrder.mockResolvedValue({ ...pendingOrder, status: 'paid', paidAt: timestamp })
    await vi.advanceTimersByTimeAsync(2700)
    expect(payment.stage.value).toBe('paid')
    expect(onPaid).toHaveBeenCalledOnce()
    scope.stop()
  })

  it('uses the refunded order state after a completed cancellation receipt', async () => {
    vi.useFakeTimers()
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    cancelPaymentOrder.mockResolvedValue(accepted('payment_cancel'))
    getPaymentOrder.mockResolvedValueOnce(pendingOrder)
      .mockResolvedValueOnce({ ...pendingOrder, status: 'refunded', refundedAt: timestamp })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!

    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    await finishOperation('payment_create')
    await payment.cancelOrder()
    await finishOperation('payment_cancel')

    expect(payment.order.value?.status).toBe('refunded')
    expect(payment.stage.value).toBe('configure')
    expect(payment.error.value).toBe(t('payment.terminalStatus', { status: t('payment.status.refunded') }))
    scope.stop()
  })

  it('blocks duplicate payment when provider outcome needs review', async () => {
    vi.useFakeTimers()
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    getOperation.mockResolvedValueOnce(receipt('payment_create', 'pending_review'))
    getPaymentOrder.mockResolvedValue({ ...pendingOrder, status: 'creating', paymentUrl: null, qrPayload: null })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!

    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    await vi.advanceTimersByTimeAsync(1500)

    expect(payment.stage.value).toBe('review')
    expect(payment.canCreate.value).toBe(false)
    expect(payment.error.value).toBe(t('payment.operationStatus', { status: t('operations.status.pending_review') }))
    scope.stop()
  })

  it('uses localized copy when a succeeded operation has no payment target', async () => {
    vi.useFakeTimers()
    setLocale('zh-CN')
    createPaymentOrder.mockResolvedValue(accepted('payment_create'))
    getPaymentOrder.mockResolvedValue({ ...pendingOrder, paymentUrl: null, qrPayload: null, receivingAddress: null })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentOrder({ onPaid: vi.fn() }))!

    payment.chooseMethod('ezpay:alipay')
    await payment.createOrder()
    await finishOperation('payment_create')

    expect(payment.stage.value).toBe('review')
    expect(payment.error.value).toBe(t('payment.targetMissing'))
    scope.stop()
  })
})
