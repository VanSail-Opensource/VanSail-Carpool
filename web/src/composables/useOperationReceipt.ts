import { computed, getCurrentInstance, inject, onScopeDispose, readonly, shallowRef } from 'vue'

import { memberOperationsApi } from '@/api/memberOperations'
import type { OperationReceipt, OperationStatus } from '@/api/types'
import { localizedError } from '@/i18n'
import { operationReceiptLoaderKey, type OperationReceiptLoader } from './operationReceiptLoader'

const activeStatuses = new Set<OperationStatus>(['queued', 'processing'])

export function operationIsActive(status: OperationStatus | undefined): boolean {
  return status !== undefined && activeStatuses.has(status)
}

export function useOperationReceipt(intervalMilliseconds = 1500, loader?: OperationReceiptLoader) {
  const loadReceipt = loader ?? (getCurrentInstance() ? inject(operationReceiptLoaderKey, memberOperationsApi.getOperation) : memberOperationsApi.getOperation)
  const receipt = shallowRef<OperationReceipt | null>(null)
  const polling = shallowRef(false)
  const checking = shallowRef(false)
  const error = shallowRef<string | null>(null)
  let timer: number | undefined
  let version = 0
  let failures = 0

  const terminal = computed(() => receipt.value !== null && !operationIsActive(receipt.value.status))

  function clearTimer(): void {
    if (timer !== undefined) window.clearTimeout(timer)
    timer = undefined
  }

  function schedule(expectedVersion: number): void {
    clearTimer()
    const delay = Math.min(intervalMilliseconds * (2 ** failures), 10_000)
    timer = window.setTimeout(() => void poll(expectedVersion), delay)
  }

  async function poll(expectedVersion = version): Promise<void> {
    const operationId = receipt.value?.id
    if (!operationId || checking.value || expectedVersion !== version) return
    checking.value = true
    try {
      const next = await loadReceipt(operationId)
      if (expectedVersion !== version) return
      receipt.value = next
      failures = 0
      error.value = null
      polling.value = operationIsActive(next.status)
      if (polling.value) schedule(expectedVersion)
    } catch (caught) {
      if (expectedVersion !== version) return
      failures += 1
      error.value = localizedError(caught, 'operations.statusUnavailable')
      if (failures < 5) schedule(expectedVersion)
      else polling.value = false
    } finally {
      if (expectedVersion === version) checking.value = false
    }
  }

  function track(next: OperationReceipt): void {
    version += 1
    clearTimer()
    failures = 0
    checking.value = false
    receipt.value = next
    error.value = null
    polling.value = operationIsActive(next.status)
    if (polling.value) schedule(version)
  }

  function reset(): void {
    version += 1
    clearTimer()
    receipt.value = null
    error.value = null
    polling.value = false
    checking.value = false
    failures = 0
  }

  onScopeDispose(reset)

  return {
    receipt: readonly(receipt),
    polling: readonly(polling),
    checking: readonly(checking),
    error: readonly(error),
    terminal,
    track,
    refresh: () => poll(version),
    reset,
  }
}
