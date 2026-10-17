import { computed, onScopeDispose, readonly, shallowRef, watch } from 'vue'

import type { OperationReceipt } from '@/api/types'
import { localizedError } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { createLatestRequest } from '@/utils/latestRequest'
import { operationIsActive, useOperationReceipt } from './useOperationReceipt'
import type { OperationReceiptLoader } from './operationReceiptLoader'

interface DurableCommandOptions {
  loadReceipt?: OperationReceiptLoader
  errorKey: string
  onTerminal?: (receipt: OperationReceipt, commandId: string) => void | Promise<void>
}

type QueueCommand = (idempotencyKey: string) => Promise<OperationReceipt>

export function useDurableCommand(options: DurableCommandOptions) {
  const submitting = shallowRef(false)
  const activeCommandId = shallowRef<string | null>(null)
  const commandError = shallowRef<string | null>(null)
  const attempts = new Map<string, string>()
  const commandByOperation = new Map<string, string>()
  const notifiedOperations = new Set<string>()
  const tracker = useOperationReceipt(1500, options.loadReceipt)
  const latestExecution = createLatestRequest()

  const busy = computed(() => submitting.value || operationIsActive(tracker.receipt.value?.status))
  const blocksMutations = computed(() => {
    const status = tracker.receipt.value?.status
    return operationIsActive(status) || status === 'pending_review' || status === 'partial'
  })
  const error = computed(() => commandError.value ?? tracker.error.value)

  watch(tracker.receipt, (receipt) => {
    if (!receipt || operationIsActive(receipt.status) || notifiedOperations.has(receipt.id)) return
    const commandId = commandByOperation.get(receipt.id) ?? activeCommandId.value
    if (!commandId || !options.onTerminal) return
    commandByOperation.delete(receipt.id)
    notifiedOperations.add(receipt.id)
    void Promise.resolve(options.onTerminal(receipt, commandId)).catch(() => undefined)
  })

  async function execute(commandId: string, fingerprint: string, queue: QueueCommand): Promise<boolean> {
    if (submitting.value || blocksMutations.value) return false
    const token = latestExecution.begin()
    submitting.value = true
    activeCommandId.value = commandId
    commandError.value = null
    tracker.reset()
    try {
      const key = attempts.get(fingerprint) ?? createUuid()
      attempts.set(fingerprint, key)
      const receipt = await queue(key)
      if (!latestExecution.isCurrent(token)) return false
      attempts.delete(fingerprint)
      commandByOperation.set(receipt.id, commandId)
      tracker.track(receipt)
      return true
    } catch (caught) {
      if (!latestExecution.isCurrent(token)) return false
      commandError.value = localizedError(caught, options.errorKey)
      return false
    } finally {
      if (latestExecution.isCurrent(token)) submitting.value = false
    }
  }

  function reset(): void {
    latestExecution.invalidate()
    attempts.clear()
    commandByOperation.clear()
    notifiedOperations.clear()
    activeCommandId.value = null
    commandError.value = null
    submitting.value = false
    tracker.reset()
  }

  function resume(receipt: OperationReceipt, commandId: string): void {
    if (submitting.value || tracker.receipt.value?.id === receipt.id) return
    activeCommandId.value = commandId
    commandByOperation.set(receipt.id, commandId)
    tracker.track(receipt)
  }

  onScopeDispose(latestExecution.dispose)

  return {
    receipt: tracker.receipt,
    checking: tracker.checking,
    polling: tracker.polling,
    terminal: tracker.terminal,
    submitting: readonly(submitting),
    activeCommandId: readonly(activeCommandId),
    busy,
    blocksMutations,
    error,
    execute,
    refresh: tracker.refresh,
    reset,
    resume,
  }
}
