import { computed, onScopeDispose, readonly, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

import { adminOperationsApi, type AdminHourlySquad, type AdminHourlyView, type OperationReceipt } from '@/api/adminOperations'
import { ApiError } from '@/api/http'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

// Owner-scoped hourly projection for the administrator workspace. It reuses
// the shared request client, never exposes member controls, and keeps the
// policy version that the disable command must quote.
export function useAdminUserHourly(userId: MaybeRefOrGetter<string>) {
  const view = shallowRef<AdminHourlyView | null>(null)
  const loading = shallowRef(true)
  const error = shallowRef<string | null>(null)
  const actionError = shallowRef<string | null>(null)
  const conflict = shallowRef(false)
  const busyEnrollment = shallowRef<string | null>(null)
  const queued = shallowRef<OperationReceipt | null>(null)
  const squads = computed(() => view.value?.squads ?? [])
  const policyVersion = computed(() => view.value?.policy.version ?? '')
  const keys = new Map<string, string>()
  let controller: AbortController | undefined
  let generation = 0
  let identityVersion = 0
  let active = true

  async function load(quiet = false): Promise<void> {
    if (!active) return
    controller?.abort(); controller = new AbortController()
    const ticket = ++generation
    const id = toValue(userId)
    if (!quiet) loading.value = true
    error.value = null
    try {
      const value = await adminOperationsApi.getAdminHourly(id, controller.signal)
      if (!active || ticket !== generation || id !== toValue(userId)) return
      view.value = value
    } catch (caught) {
      if (active && ticket === generation) error.value = localizedError(caught, 'hourly.admin.user.loadFailed')
    } finally {
      if (active && ticket === generation) loading.value = false
    }
  }

  async function disable(squad: AdminHourlySquad): Promise<boolean> {
    const version = view.value?.policy.version
    if (!active || busyEnrollment.value || !version) return false
    const target = toValue(userId)
    const identity = identityVersion
    const actionKey = JSON.stringify([target, squad.enrollmentId, version])
    busyEnrollment.value = squad.enrollmentId
    actionError.value = null
    conflict.value = false
    queued.value = null
    const key = keys.get(actionKey) ?? createUuid()
    keys.set(actionKey, key)
    try {
      const receipt = await adminOperationsApi.disableAdminHourly(target, squad.enrollmentId, version, key)
      if (!active || identity !== identityVersion || target !== toValue(userId)) return false
      keys.delete(actionKey)
      queued.value = receipt
      await load(true)
      return true
    } catch (caught) {
      if (!active || identity !== identityVersion || target !== toValue(userId)) return false
      conflict.value = caught instanceof ApiError && caught.status === 409
      actionError.value = conflict.value
        ? t('hourly.conflict')
        : localizedError(caught, 'hourly.admin.user.actionFailed')
      return false
    } finally {
      if (active && identity === identityVersion) busyEnrollment.value = null
    }
  }

  function clearActionError(): void {
    actionError.value = null
    conflict.value = false
    queued.value = null
  }

  watch(() => toValue(userId), () => {
    identityVersion++
    busyEnrollment.value = null
    view.value = null
    error.value = null
    clearActionError()
    void load()
  }, { immediate: true })
  onScopeDispose(() => { active = false; generation++; identityVersion++; controller?.abort(); view.value = null; keys.clear() })

  return {
    view: readonly(view), squads, policyVersion, loading: readonly(loading), error: readonly(error),
    actionError: readonly(actionError), conflict: readonly(conflict), busyEnrollment: readonly(busyEnrollment),
    queued: readonly(queued), load, disable, clearActionError,
  }
}
