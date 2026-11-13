import { effectScope, shallowRef } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { FeaturePaymentOrder } from '@/api/features'
import { ApiError } from '@/api/http'

const { getPaymentOrder, getPaymentReturnStatus } = vi.hoisted(() => ({ getPaymentOrder: vi.fn(), getPaymentReturnStatus: vi.fn() }))

vi.mock('@/api/client', () => ({ api: { getPaymentOrder, getPaymentReturnStatus } }))

import { usePaymentReturn } from './usePaymentReturn'

const pendingOrder: FeaturePaymentOrder = {
  id: 'payment-1', methodId: 'ezpay:alipay', provider: 'ezpay', providerRail: 'alipay', status: 'pending',
  txb: { currency: 'TXB', minor: '2000', display: '20.00 TXB' }, payableAmount: '20.00', payableCurrency: 'CNY',
  rateSnapshot: '1', rateDirection: 'txb_per_currency', paymentUrl: null, qrPayload: null, receivingAddress: null,
  actualCryptoAmount: null, actualCryptoCurrency: null, expiresAt: '2026-08-11T01:00:00Z', paidAt: null,
  refundedAt: null, cancelledAt: null, cancelReason: '', providerCancelStatus: '', createdAt: '2026-08-11T00:00:00Z', updatedAt: '2026-08-11T00:00:00Z',
}

describe('payment return confirmation', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
    vi.restoreAllMocks()
  })

  it('keeps the signed browser receipt projection alongside its status', async () => {
    getPaymentReturnStatus.mockResolvedValue({
      id: 'payment-1', provider: 'ezpay', providerRail: 'alipay', status: 'paid',
      txb: { currency: 'TXB', minor: '2000', display: '20.00 TXB' }, payableAmount: '20.00', payableCurrency: 'CNY',
      actualCryptoAmount: null, actualCryptoCurrency: null, createdAt: '2026-08-11T00:00:00Z', paidAt: '2026-08-11T00:01:00Z',
    })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('payment-1'), {
      browserStatus: true,
      provider: shallowRef('ezpay'),
      capability: shallowRef('capability'),
    }))!

    await payment.refresh()

    expect(payment.details.value?.id).toBe('payment-1')
    expect(payment.details.value?.payableAmount).toBe('20.00')
    expect(payment.state.value).toBe('confirmed')
    scope.stop()
  })

  it('confirms only an authoritative paid order', async () => {
    getPaymentOrder.mockResolvedValue({ ...pendingOrder, status: 'paid', paidAt: '2026-08-11T00:01:00Z' })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('payment-1')))!

    await payment.refresh()

    expect(payment.state.value).toBe('confirmed')
    expect(payment.isConfirmed.value).toBe(true)
    scope.stop()
  })

  it('attempts authenticated owner lookup for a legacy return without a capability', async () => {
    getPaymentReturnStatus.mockResolvedValue({ ...pendingOrder, status: 'paid' })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('payment-1'), {
      browserStatus: true, provider: shallowRef('ezpay'),
    }))!
    await payment.refresh()
    expect(getPaymentReturnStatus).toHaveBeenCalledWith('ezpay', 'payment-1', '')
    expect(payment.state.value).toBe('confirmed')
    scope.stop()
  })

  it.each([401, 403, 404])('clears a previously visible receipt and stops polling after %s', async status => {
    vi.useFakeTimers()
    getPaymentReturnStatus.mockResolvedValue(pendingOrder)
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('payment-1'), {
      browserStatus: true, provider: shallowRef('ezpay'), capability: shallowRef('expired'),
    }))!
    await payment.refresh()
    expect(payment.details.value).not.toBeNull()
    getPaymentReturnStatus.mockRejectedValue(new ApiError(status, { code: 'DENIED', message: '' }))
    await vi.advanceTimersByTimeAsync(2000)
    expect(payment.details.value).toBeNull()
    expect(payment.authenticationRequired.value).toBe(status !== 404)
    const calls = getPaymentReturnStatus.mock.calls.length
    await vi.advanceTimersByTimeAsync(10_000)
    expect(getPaymentReturnStatus.mock.calls.length).toBe(calls)
    scope.stop()
  })

  it('polls a pending return until the durable order is paid', async () => {
    vi.useFakeTimers()
    getPaymentOrder.mockResolvedValue(pendingOrder)
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('payment-1')))!

    await payment.refresh()
    expect(payment.state.value).toBe('pending')
    getPaymentOrder.mockResolvedValue({ ...pendingOrder, status: 'paid', paidAt: '2026-08-11T00:01:00Z' })
    await vi.advanceTimersByTimeAsync(2000)

    expect(payment.state.value).toBe('confirmed')
    scope.stop()
  })

  it('keeps a provider-declared terminal failure out of the success state', async () => {
    getPaymentOrder.mockResolvedValue({ ...pendingOrder, status: 'expired' })
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('payment-1')))!

    await payment.refresh()

    expect(payment.state.value).toBe('terminal')
    expect(payment.isConfirmed.value).toBe(false)
    scope.stop()
  })

  it('does not request a payment when the return URL omitted its order id', async () => {
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('')))!

    await payment.refresh()

    expect(payment.state.value).toBe('missing')
    expect(getPaymentOrder).not.toHaveBeenCalled()
    scope.stop()
  })
  it('pauses hidden status reads and backs off outages without losing the retained receipt', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    getPaymentOrder.mockResolvedValue(pendingOrder)
    const scope = effectScope()
    const payment = scope.run(() => usePaymentReturn(shallowRef('payment-1')))!
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(getPaymentOrder).toHaveBeenCalledTimes(1)
    expect(payment.state.value).toBe('pending')
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    getPaymentOrder.mockRejectedValue(new Error('unavailable'))
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(getPaymentOrder).toHaveBeenCalledTimes(2)
    expect(payment.details.value?.id).toBe('payment-1')
    await vi.advanceTimersByTimeAsync(3999)
    expect(getPaymentOrder).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1)
    expect(getPaymentOrder).toHaveBeenCalledTimes(3)
    payment.stopPolling()
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(60_000)
    expect(getPaymentOrder).toHaveBeenCalledTimes(3)
    scope.stop()
  })

})
