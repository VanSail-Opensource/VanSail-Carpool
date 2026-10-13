import { computed, onScopeDispose, readonly, shallowRef } from 'vue'

import { api } from '@/api/client'
import type { FeaturePaymentMethod, FeaturePaymentOrder } from '@/api/features'
import type { OperationReceipt } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { notifyHaptic } from '@/utils/telegram'
import { resolveOrderSelection, resolveReissueCandidate } from './paymentOrderHelpers'
import { usePaymentConfiguration } from './usePaymentConfiguration'
import { type PaymentCommand, usePaymentOrderOperations } from './usePaymentOrderOperations'
import { usePaymentTarget } from './usePaymentTarget'

export { isTerminalPaymentStatus } from './paymentOrderHelpers'
export type PaymentStage = 'configure' | 'creating' | 'pending' | 'cancelling' | 'cancelled' | 'paid' | 'review'

interface PaymentOrderOptions {
  onPaid: () => void
  minimumMinor?: () => string
  maximumMinor?: () => string
}

export function usePaymentOrder(options: PaymentOrderOptions) {
  const stage = shallowRef<PaymentStage>('configure')
  const localError = shallowRef<string | null>(null)
  const canReissue = shallowRef(false)
  const canRecreate = shallowRef(false)
  let closeTimer: ReturnType<typeof setTimeout> | undefined
  const configuration = usePaymentConfiguration(options, stage)
  const target = usePaymentTarget({ onPaid: handlePaid, onTerminal: handleOrderTerminal })
  const operations = usePaymentOrderOperations({ onTerminal: handleOperationTerminal })
  const error = computed(() => operations.error.value ?? localError.value)

  function reset(methods: readonly FeaturePaymentMethod[]): void {
    if (closeTimer !== undefined) clearTimeout(closeTimer)
    operations.reset()
    target.reset()
    localError.value = null
    canReissue.value = false
    canRecreate.value = false
    stage.value = 'configure'
    configuration.reset(methods)
  }
  function hydrateReissueOrder(candidate: FeaturePaymentOrder, methods: readonly FeaturePaymentMethod[]): boolean {
    const restored = resolveReissueCandidate(candidate, methods, configuration.minimumMinor.value, configuration.maximumMinor.value)
    if (!restored) {
      localError.value = t('payment.reissueUnavailable')
      return false
    }
    configuration.amount.value = restored.amount
    configuration.selectedMethodId.value = restored.methodId
    void target.remember(candidate)
    canReissue.value = true
    localError.value = candidate.status === 'expired'
      ? t('payment.orderExpired')
      : t('payment.terminalStatus', { status: t(`payment.status.${candidate.status}`) })
    return true
  }
  async function hydratePendingOrder(candidate: FeaturePaymentOrder, methods: readonly FeaturePaymentMethod[]): Promise<boolean> {
    if (candidate.status !== 'pending') return false
    const restored = resolveOrderSelection(candidate, methods, configuration.minimumMinor.value, configuration.maximumMinor.value)
    if (restored) {
      configuration.amount.value = restored.amount
      configuration.selectedMethodId.value = restored.methodId
    }
    canRecreate.value = (candidate.provider === 'bepusdt' || candidate.provider === 'builtin_usdt') && restored !== null
    stage.value = 'pending'
    const result = await target.present(candidate)
    if (result === 'pending') stage.value = 'pending'
    return result === 'pending'
  }
  async function createOrder(): Promise<void> {
    const methodId = configuration.selectedMethodId.value
    if (!configuration.canCreate.value || !methodId) return
    stage.value = 'creating'
    localError.value = null
    canReissue.value = false
    try {
      await operations.queueCreate(methodId, configuration.amountMinor.value)
    } catch (caught) {
      stage.value = 'configure'
      canReissue.value = true
      localError.value = localizedError(caught, 'errors.paymentCreate')
      notifyHaptic('error')
    }
  }
  async function cancelOrder(): Promise<void> {
    if (!target.order.value || stage.value !== 'pending') return
    target.stop()
    stage.value = 'cancelling'
    localError.value = null
    try {
      await operations.queueCancellation(target.order.value.id)
    } catch (caught) {
      stage.value = 'pending'
      localError.value = localizedError(caught, 'errors.paymentCancel')
      void target.present(target.order.value)
    }
  }
  async function recreateOrder(): Promise<void> {
    const current = target.order.value
    const methodId = configuration.selectedMethodId.value
    if (!current || (current.provider !== 'bepusdt' && current.provider !== 'builtin_usdt') || !canRecreate.value || !methodId
      || Date.now() < new Date(current.expiresAt).getTime()) return
    target.stop()
    stage.value = 'creating'
    localError.value = null
    try {
      await operations.queueCreate(methodId, configuration.amountMinor.value)
    } catch (caught) {
      stage.value = 'pending'
      localError.value = localizedError(caught, 'errors.paymentCreate')
      notifyHaptic('error')
    }
  }
  async function handleOperationTerminal(command: PaymentCommand, receipt: OperationReceipt): Promise<void> {
    let current: FeaturePaymentOrder
    try {
      current = await api.getPaymentOrder(command.paymentOrderId)
    } catch (caught) {
      stage.value = 'review'
      localError.value = localizedError(caught, 'operations.statusUnavailable')
      return
    }
    if (receipt.status === 'succeeded') {
      if (command.kind === 'create') await presentCreatedOrder(current)
      else finishCancellation(current)
      return
    }
    await target.remember(current)
    localError.value = receipt.errorCode?.startsWith('USDT_')
      ? localizedError({ code: receipt.errorCode }, 'errors.paymentCreate')
      : receipt.errorMessage
      ? t('cryptoPayment.rejected', { message: receipt.errorMessage })
      : t('payment.operationStatus', { status: t(`operations.status.${receipt.status}`) })
    if (current.status === 'paid') handlePaid()
    else if (command.kind === 'create' && ['failed', 'compensated'].includes(receipt.status)) {
      stage.value = 'configure'
      canReissue.value = true
    } else stage.value = receipt.status === 'failed' && command.kind === 'cancel' ? 'cancelled' : 'review'
  }
  async function presentCreatedOrder(current: FeaturePaymentOrder): Promise<void> {
    canRecreate.value = (current.provider === 'bepusdt' || current.provider === 'builtin_usdt') && configuration.canCreate.value
    const result = await target.present(current)
    if (result === 'pending') stage.value = 'pending'
    else if (result === 'missing') {
      stage.value = 'review'
      localError.value = t('payment.targetMissing')
      notifyHaptic('error')
    }
  }
  function finishCancellation(current: FeaturePaymentOrder): void {
    void target.remember(current)
    if (current.status === 'paid') handlePaid()
    else {
      stage.value = 'cancelled'
      target.cancelled()
    }
  }
  function handleOrderTerminal(current: FeaturePaymentOrder): void {
    canRecreate.value = false
    canReissue.value = current.status === 'expired' || current.status === 'failed'
    localError.value = current.status === 'expired'
      ? t('payment.orderExpired')
      : t('payment.terminalStatus', { status: t(`payment.status.${current.status}`) })
    stage.value = current.status === 'cancelled' ? 'cancelled' : 'configure'
  }

  function handlePaid(): void {
    target.stop()
    canRecreate.value = false
    stage.value = 'paid'
    notifyHaptic('success')
    closeTimer = setTimeout(options.onPaid, 700)
  }

  onScopeDispose(() => closeTimer !== undefined && clearTimeout(closeTimer))
  return {
    amount: configuration.amount,
    selectedMethodId: readonly(configuration.selectedMethodId),
    stage: readonly(stage),
    order: target.order,
    qrDataUrl: target.qrDataUrl,
    error,
    canReissue: readonly(canReissue),
    canRecreate: readonly(canRecreate),
    amountMinor: configuration.amountMinor,
    amountValid: configuration.amountValid,
    canCreate: configuration.canCreate,
    reset,
    hydrateReissueOrder,
    hydratePendingOrder,
    chooseMethod: configuration.chooseMethod,
    createOrder,
    cancelOrder,
    recreateOrder,
    retryOperation: operations.retry,
    openPaymentTarget: target.open,
    stopPolling: target.stop,
  }
}
