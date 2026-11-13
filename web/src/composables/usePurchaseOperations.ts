import { computed, onScopeDispose, readonly, shallowRef } from 'vue'

import { ApiError } from '@/api/client'
import { memberOperationsApi } from '@/api/memberOperations'
import type { MemberRefundQuote } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { notifyHaptic } from '@/utils/telegram'
import { createLatestRequest } from '@/utils/latestRequest'
import { operationIsActive, useOperationReceipt } from './useOperationReceipt'

export type PurchaseOperationKind = 'refund'

export function usePurchaseOperations(purchaseId: () => string) {
  const refundQuote = shallowRef<MemberRefundQuote | null>(null)
  const refundEligibilityLoading = shallowRef(true)
  const quoteLoading = shallowRef(false)
  const mutating = shallowRef(false)
  const mutationError = shallowRef<string | null>(null)
  const activeKind = shallowRef<PurchaseOperationKind | null>(null)
  const keys = new Map<string, string>()
  const operation = useOperationReceipt()
  const quotes = createLatestRequest()
  const mutations = createLatestRequest()

  const error = computed(() => mutationError.value ?? operation.error.value)
  const blocksMutations = computed(() => {
    const status = operation.receipt.value?.status
    return operationIsActive(status) || status === 'pending_review' || status === 'partial'
  })

  async function loadRefundEligibility(): Promise<void> {
    const id = purchaseId()
    const ticket = quotes.begin()
    refundEligibilityLoading.value = true
    quoteLoading.value = false
    try {
      const quote = await memberOperationsApi.getPurchaseRefundQuote(id)
      if (quotes.isCurrent(ticket) && purchaseId() === id) refundQuote.value = quote
    } catch {
      if (quotes.isCurrent(ticket) && purchaseId() === id) refundQuote.value = null
    } finally {
      if (quotes.isCurrent(ticket)) refundEligibilityLoading.value = false
    }
  }

  async function loadQuote(): Promise<void> {
    const id = purchaseId()
    const ticket = quotes.begin()
    refundEligibilityLoading.value = false
    quoteLoading.value = true
    mutationError.value = null
    refundQuote.value = null
    try {
      const quote = await memberOperationsApi.getPurchaseRefundQuote(id)
      if (quotes.isCurrent(ticket) && purchaseId() === id) refundQuote.value = quote
    } catch (caught) {
      if (quotes.isCurrent(ticket) && purchaseId() === id) mutationError.value = localizedError(caught, 'purchaseOperations.errors.quoteUnavailable')
    } finally {
      if (quotes.isCurrent(ticket)) quoteLoading.value = false
    }
  }

  async function refreshAfterConflict(id: string): Promise<void> {
    const ticket = quotes.begin()
    refundQuote.value = null
    try {
      const quote = await memberOperationsApi.getPurchaseRefundQuote(id)
      if (quotes.isCurrent(ticket) && purchaseId() === id) refundQuote.value = quote
    } catch {
      // Preserve the mutation error; the member can explicitly request a new quote.
    }
  }

  async function start(kind: PurchaseOperationKind): Promise<boolean> {
    if (mutating.value || blocksMutations.value) return false
    const id = purchaseId()
    const ticket = mutations.begin()
    quotes.invalidate()
    quoteLoading.value = false
    refundEligibilityLoading.value = false
    mutating.value = true
    mutationError.value = null
    const actionId = `${kind}:${id}`
    try {
      const key = keys.get(actionId) ?? createUuid()
      keys.set(actionId, key)
      const receipt = await memberOperationsApi.refundPurchase(id, key)
      if (!mutations.isCurrent(ticket) || purchaseId() !== id) return false
      keys.delete(actionId)
      activeKind.value = kind
      operation.track(receipt)
      notifyHaptic('success')
      return true
    } catch (caught) {
      if (!mutations.isCurrent(ticket) || purchaseId() !== id) return false
      if (caught instanceof ApiError && (caught.status === 409 || caught.status === 422)) {
        keys.delete(actionId)
        await refreshAfterConflict(id)
      }
      if (!mutations.isCurrent(ticket) || purchaseId() !== id) return false
      mutationError.value = caught instanceof ApiError && ['OPERATION_CONFLICT', 'PURCHASE_OPERATION_INELIGIBLE'].includes(caught.code)
        ? t('purchaseOperations.quoteChanged')
        : localizedError(caught, 'purchaseOperations.errors.operationFailed')
      notifyHaptic('error')
      return false
    } finally {
      if (mutations.isCurrent(ticket)) mutating.value = false
    }
  }

  function dismissOperation(): void {
    if (blocksMutations.value) return
    activeKind.value = null
    mutationError.value = null
    operation.reset()
  }

  function reset(): void {
    quotes.invalidate()
    mutations.invalidate()
    refundQuote.value = null
    mutationError.value = null
    activeKind.value = null
    refundEligibilityLoading.value = false
    quoteLoading.value = false
    mutating.value = false
    operation.reset()
  }

  onScopeDispose(() => { quotes.dispose(); mutations.dispose(); keys.clear() })

  return {
    refundQuote: readonly(refundQuote),
    refundEligibilityLoading: readonly(refundEligibilityLoading),
    quoteLoading: readonly(quoteLoading),
    mutating: readonly(mutating),
    activeKind: readonly(activeKind),
    receipt: operation.receipt,
    polling: operation.polling,
    checking: operation.checking,
    terminal: operation.terminal,
    error,
    blocksMutations,
    loadRefundEligibility,
    loadQuote,
    start,
    refresh: operation.refresh,
    dismissOperation,
    reset,
  }
}
