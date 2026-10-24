import { restoreCatalogOptions } from '@/api/cache/catalogOptions'
import { restoreRef } from '@/api/cache/restore'
import { computed, onScopeDispose, shallowReadonly, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

import { adminOperationsApi, type AdminCatalogOptions, type AdminUserDetail } from '@/api/adminOperations'
import { ApiError } from '@/api/http'
import { localizedError } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { usePurchaseValidity } from '@/composables/usePurchaseValidity'

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
  let active = true
  let generation = 0
  let identityVersion = 0
  let controller: AbortController | undefined

  const error = computed(() => loadError.value ?? actionError.value)
  const validity = usePurchaseValidity(() => detail.value?.entitlements ?? [], () => load(true))
  const currentDetail = computed(() => detail.value ? {
    ...detail.value,
    activeCombo: validity.current(detail.value.activeCombo) ? detail.value.activeCombo : null,
  } : null)

  async function load(quiet = false): Promise<void> {
    controller?.abort(); controller = new AbortController()
    const ticket = ++generation
    const expectedId = toValue(userId)
    if (!quiet) {
      detail.value = null
      loading.value = !restoreRef(`/api/v1/admin/users/${encodeURIComponent(expectedId)}`, detail)
    }
    loadError.value = null
    try {
      const result = await adminOperationsApi.getUser(expectedId, controller.signal)
      if (active && ticket === generation && toValue(userId) === expectedId) detail.value = result
    } catch (caught) {
      if (active && ticket === generation) { detail.value = null; loadError.value = localizedError(caught, 'adminUserProfile.loadFailed') }
    } finally {
      if (active && ticket === generation && toValue(userId) === expectedId) loading.value = false
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
    if (busyAction.value || loading.value || !detail.value || loadError.value) return false
    const version = identityVersion
    const target = toValue(userId)
    const actionKey = `${target}:${action}`
    busyAction.value = action
    actionError.value = null
    conflict.value = false
    const key = keys.get(actionKey) ?? createUuid()
    keys.set(actionKey, key)
    try {
      await task(key)
      if (!active || version !== identityVersion || target !== toValue(userId)) return false
      keys.delete(actionKey)
      await load()
      return true
    } catch (caught) {
      if (!active || version !== identityVersion) return false
      if (caught instanceof ApiError) keys.delete(actionKey)
      conflict.value = caught instanceof ApiError && caught.status === 409
      actionError.value = localizedError(caught, conflict.value ? 'adminUserProfile.conflict' : 'adminUserProfile.actionFailed')
      return false
    } finally {
      if (active && version === identityVersion) busyAction.value = null
    }
  }

  function clearActionError(): void {
    actionError.value = null
    conflict.value = false
  }

  watch(() => toValue(userId), () => { identityVersion++; busyAction.value = null; actionError.value = null; void load() }, { immediate: true })
  onScopeDispose(() => { active = false; generation++; identityVersion++; controller?.abort(); detail.value = null })

  return {
    detail: currentDetail, options: shallowReadonly(options), loading: shallowReadonly(loading),
    optionsLoading: shallowReadonly(optionsLoading), busyAction: shallowReadonly(busyAction), error,
    optionsError: shallowReadonly(optionsError), conflict: shallowReadonly(conflict), load, loadOptions, perform, clearActionError,
  }
}
