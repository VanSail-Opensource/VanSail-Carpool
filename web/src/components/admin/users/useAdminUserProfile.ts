import { restoreCatalogOptions } from '@/api/cache/catalogOptions'
import { restoreRef } from '@/api/cache/restore'
import { computed, shallowReadonly, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

import { adminOperationsApi, type AdminCatalogOptions, type AdminUserDetail } from '@/api/adminOperations'
import { ApiError } from '@/api/http'
import { localizedError } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

export function useAdminUserProfile(userId: MaybeRefOrGetter<string>) {
  const detail = shallowRef<AdminUserDetail | null>(null)
  const options = shallowRef<AdminCatalogOptions>({ combos: [], squads: [] })
  const loading = shallowRef(true)
  const optionsLoading = shallowRef(false)
  const busyAction = shallowRef<string | null>(null)
  const loadError = shallowRef<string | null>(null)
  const actionError = shallowRef<string | null>(null)
  const optionsError = shallowRef<string | null>(null)
  const conflict = shallowRef(false)
  const keys = new Map<string, string>()

  const error = computed(() => loadError.value ?? actionError.value)

  async function load(): Promise<void> {
    const expectedId = toValue(userId)
    detail.value = null
    loading.value = !restoreRef(`/api/v1/admin/users/${encodeURIComponent(expectedId)}`, detail)
    loadError.value = null
    try {
      const result = await adminOperationsApi.getUser(expectedId)
      if (toValue(userId) === expectedId) detail.value = result
    } catch (caught) {
      loadError.value = localizedError(caught, 'adminUserProfile.loadFailed')
    } finally {
      if (toValue(userId) === expectedId) loading.value = false
    }
  }

  async function loadOptions(): Promise<void> {
    if (optionsLoading.value || (options.value.combos.length && options.value.squads.length)) return
    optionsLoading.value = !restoreCatalogOptions(options)
    optionsError.value = null
    try {
      options.value = await adminOperationsApi.getCatalogOptions()
    } catch (caught) {
      optionsError.value = localizedError(caught, 'adminUserProfile.optionsFailed')
    } finally {
      optionsLoading.value = false
    }
  }

  async function perform(action: string, task: (key: string) => Promise<unknown>): Promise<boolean> {
    if (busyAction.value) return false
    busyAction.value = action
    actionError.value = null
    conflict.value = false
    const key = keys.get(action) ?? createUuid()
    keys.set(action, key)
    try {
      await task(key)
      keys.delete(action)
      await load()
      return true
    } catch (caught) {
      if (caught instanceof ApiError) keys.delete(action)
      conflict.value = caught instanceof ApiError && caught.status === 409
      actionError.value = localizedError(caught, conflict.value ? 'adminUserProfile.conflict' : 'adminUserProfile.actionFailed')
      return false
    } finally {
      busyAction.value = null
    }
  }

  function clearActionError(): void {
    actionError.value = null
    conflict.value = false
  }

  watch(() => toValue(userId), () => void load(), { immediate: true })

  return {
    detail: shallowReadonly(detail), options: shallowReadonly(options), loading: shallowReadonly(loading),
    optionsLoading: shallowReadonly(optionsLoading), busyAction: shallowReadonly(busyAction), error,
    optionsError: shallowReadonly(optionsError), conflict: shallowReadonly(conflict), load, loadOptions, perform, clearActionError,
  }
}
