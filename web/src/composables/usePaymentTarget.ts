import { onScopeDispose, readonly, shallowRef } from 'vue'

import { api } from '@/api/client'
import type { FeaturePaymentOrder } from '@/api/features'
import { notifyHaptic, openExternalLink, openTelegramInvoice } from '@/utils/telegram'
import { createLatestRequest } from '@/utils/latestRequest'
import { isTerminalPaymentStatus, paymentQrDataUrl } from './paymentOrderHelpers'

export type PaymentTargetResult = 'paid' | 'pending' | 'terminal' | 'missing'

interface PaymentTargetOptions {
  onPaid: () => void
  onTerminal: (order: FeaturePaymentOrder) => void
}

export function usePaymentTarget(options: PaymentTargetOptions) {
  const order = shallowRef<FeaturePaymentOrder | null>(null)
  const qrDataUrl = shallowRef<string | null>(null)
  let timer: ReturnType<typeof setTimeout> | undefined
  const latest = createLatestRequest()
  let tracking = false
  let trackingToken = 0
  let failures = 0
  let pollingToken: number | undefined

  function clearTimer(): void {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }

  function stop(): void {
    tracking = false
    latest.invalidate()
    clearTimer()
  }

  async function remember(next: FeaturePaymentOrder, expectedToken?: number): Promise<void> {
    const token = expectedToken ?? latest.begin()
    if (!latest.isCurrent(token)) return
    order.value = next
    const nextQrDataUrl = await paymentQrDataUrl(next)
    if (latest.isCurrent(token)) qrDataUrl.value = nextQrDataUrl
  }

  async function present(next: FeaturePaymentOrder): Promise<PaymentTargetResult> {
    const token = latest.begin()
    tracking = false; failures = 0
    clearTimer()
    await remember(next, token)
    if (!latest.isCurrent(token)) return 'pending'
    if (next.status === 'paid') {
      options.onPaid()
      return 'paid'
    }
    if (isTerminalPaymentStatus(next.status)) {
      options.onTerminal(next)
      return 'terminal'
    }
    if (!next.paymentUrl && !next.qrPayload && !next.receivingAddress) return 'missing'
    if (next.provider === 'stars' && next.paymentUrl
      && !openTelegramInvoice(next.paymentUrl)) openExternalLink(next.paymentUrl)
    schedule(token)
    return 'pending'
  }

  function schedule(token: number): void {
    clearTimer()
    tracking = true; trackingToken = token
    if (!document.hidden) timer = setTimeout(() => void poll(token), Math.min(2000 * 2 ** failures, 30_000))
  }

  async function poll(token: number): Promise<void> {
    if (!tracking || !latest.isCurrent(token) || !order.value || pollingToken === token) return
    clearTimer()
    pollingToken = token
    try {
      const refreshed = await api.getPaymentOrder(order.value.id)
      if (!latest.isCurrent(token)) return
      failures = 0
      order.value = refreshed
      if (refreshed.status === 'paid') {
        tracking = false
        options.onPaid()
        return
      }
      if (isTerminalPaymentStatus(refreshed.status)) {
        tracking = false
        options.onTerminal(refreshed)
        return
      }
    } catch {
      // Retain the accepted checkout while reducing pressure during an outage.
      if (latest.isCurrent(token)) failures = Math.min(failures + 1, 4)
    } finally {
      if (pollingToken === token) pollingToken = undefined
    }
    if (latest.isCurrent(token)) schedule(token)
  }

  function reset(): void {
    stop()
    order.value = null
    qrDataUrl.value = null
  }

  function open(): void {
    if (order.value?.paymentUrl) openExternalLink(order.value.paymentUrl)
  }

  function cancelled(): void {
    notifyHaptic('success')
  }

  function visibility(): void {
    if (document.hidden) clearTimer()
    else if (tracking) void poll(trackingToken)
  }
  document.addEventListener('visibilitychange', visibility)
  onScopeDispose(() => {
    tracking = false
    document.removeEventListener('visibilitychange', visibility)
    clearTimer()
    latest.dispose()
  })
  return { order: readonly(order), qrDataUrl: readonly(qrDataUrl), remember, present, reset, stop, open, cancelled }
}
