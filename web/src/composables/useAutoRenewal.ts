import { computed, onScopeDispose, readonly, shallowRef } from 'vue'

import { api } from '@/api/client'
import type { AutoRenewal } from '@/api/types'
import { localizedError } from '@/i18n'
import { notifyHaptic } from '@/utils/telegram'
import { createLatestRequest } from '@/utils/latestRequest'

export function useAutoRenewal(purchaseID: () => string, initialEnabled: () => boolean) {
  const renewal = shallowRef<AutoRenewal | null>(null)
  const loading = shallowRef(false)
  const updating = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const reads = createLatestRequest()
  const writes = createLatestRequest()

  const enabled = computed(() => renewal.value?.enabled ?? initialEnabled())
  const canEnable = computed(() => renewal.value?.canEnable ?? false)
  const ineligibleReason = computed(() => renewal.value?.ineligibleReason ?? null)

  function reset(): void {
    reads.invalidate()
    writes.invalidate()
    renewal.value = null
    error.value = null
    loading.value = false
    updating.value = false
  }

  async function load(): Promise<void> {
    if (updating.value) return
    const purchaseId = purchaseID()
    const version = reads.begin()
    loading.value = true
    error.value = null
    try {
      const response = await api.getAutoRenewal(purchaseId)
      if (reads.isCurrent(version) && purchaseID() === purchaseId) renewal.value = response
    } catch (caught) {
      if (reads.isCurrent(version) && purchaseID() === purchaseId) error.value = localizedError(caught, 'errors.autoRenewalFailed')
    } finally {
      if (reads.isCurrent(version)) loading.value = false
    }
  }

  async function setEnabled(next: boolean): Promise<boolean> {
    if (updating.value || (next && !canEnable.value)) return false
    const purchaseId = purchaseID()
    const version = writes.begin()
    reads.invalidate()
    loading.value = false
    updating.value = true
    error.value = null
    try {
      const response = await api.setAutoRenewal(purchaseId, next)
      if (!writes.isCurrent(version) || purchaseID() !== purchaseId) return false
      renewal.value = response
      notifyHaptic('success')
      return true
    } catch (caught) {
      if (!writes.isCurrent(version) || purchaseID() !== purchaseId) return false
      error.value = localizedError(caught, 'errors.autoRenewalFailed')
      notifyHaptic('error')
      return false
    } finally {
      if (writes.isCurrent(version)) updating.value = false
    }
  }

  onScopeDispose(() => { reads.dispose(); writes.dispose() })

  return {
    renewal: readonly(renewal),
    loading: readonly(loading),
    updating: readonly(updating),
    error: readonly(error),
    enabled,
    canEnable,
    ineligibleReason,
    reset,
    load,
    setEnabled,
  }
}
