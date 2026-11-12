import { onScopeDispose, shallowRef, watch } from 'vue'

import { api } from '@/api/client'
import type { OperationReceipt, PaymentOperation } from '@/api/types'
import { createUuid } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'
import { operationIsActive, useOperationReceipt } from './useOperationReceipt'

export type PaymentCommandKind = 'create' | 'cancel'

export interface PaymentCommand {
  kind: PaymentCommandKind
  paymentOrderId: string
}

interface PaymentOrderOperationOptions {
  onTerminal: (command: PaymentCommand, receipt: OperationReceipt) => void | Promise<void>
}

export function usePaymentOrderOperations(options: PaymentOrderOperationOptions) {
  const command = shallowRef<PaymentCommand | null>(null)
  const tracker = useOperationReceipt()
  const createKeys = new Map<string, string>()
  const cancelKeys = new Map<string, string>()
  const latest = createLatestRequest()

  function accept(kind: PaymentCommandKind, result: PaymentOperation): void {
    command.value = { kind, paymentOrderId: result.paymentOrderId }
    tracker.track(result.operation)
  }

  async function queueCreate(methodId: string, txbMinor: string): Promise<void> {
    const token = latest.begin()
    const fingerprint = JSON.stringify([methodId, txbMinor])
    const key = createKeys.get(fingerprint) ?? createUuid()
    createKeys.set(fingerprint, key)
    const result = await api.createPaymentOrder(methodId, txbMinor, key)
    if (!latest.isCurrent(token)) return
    createKeys.delete(fingerprint)
    accept('create', result)
  }

  async function queueCancellation(paymentOrderId: string): Promise<void> {
    const token = latest.begin()
    const key = cancelKeys.get(paymentOrderId) ?? createUuid()
    cancelKeys.set(paymentOrderId, key)
    const result = await api.cancelPaymentOrder(paymentOrderId, key)
    if (!latest.isCurrent(token)) return
    cancelKeys.delete(paymentOrderId)
    accept('cancel', result)
  }

  watch(tracker.receipt, (receipt) => {
    const current = command.value
    if (!receipt || !current || operationIsActive(receipt.status)) return
    command.value = null
    void options.onTerminal(current, receipt)
  })

  function reset(): void {
    latest.invalidate()
    command.value = null
    tracker.reset()
  }

  onScopeDispose(() => { reset(); latest.dispose() })

  return {
    receipt: tracker.receipt,
    error: tracker.error,
    queueCreate,
    queueCancellation,
    retry: tracker.refresh,
    reset,
  }
}
