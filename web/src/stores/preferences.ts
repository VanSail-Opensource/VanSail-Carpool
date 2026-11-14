import { computed, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import { preferencesApi, type UserPreferences, type UserPreferencesPatch } from '@/api/preferences'
import { restoreRef } from '@/api/cache/restore'
import { localizedError } from '@/i18n'

export const usePreferencesStore = defineStore('preferences', () => {
  const value = shallowRef<UserPreferences | null>(null)
  const loading = shallowRef(false)
  const saving = shallowRef(false)
  const pendingKey = shallowRef<string | null>(null)
  const error = shallowRef<string | null>(null)
  const confirmed = shallowRef(false)
  const blocked = computed(() => !confirmed.value || loading.value || saving.value)
  let principal: string | null = null
  let revision = 0

  async function refresh(): Promise<void> {
    if (!principal || saving.value) return
    const version = ++revision
    if (!value.value) restoreRef('/api/v1/me/preferences', value, {}, 60_000)
    loading.value = true
    try {
      const next = await preferencesApi.get()
      if (version !== revision) return
      value.value = next
      confirmed.value = true
      error.value = null
    } catch (caught) {
      if (version === revision) {
        error.value = localizedError(caught, 'settings.loadFailed')
        confirmed.value = false
        // A failed eligibility check must not expose optional entrances.
        if (value.value) value.value = { ...value.value, activeCombo: false }
      }
    } finally {
      if (version === revision) loading.value = false
    }
  }

  function bind(userId: string | null): void {
    if (principal === userId) return
    principal = userId
    revision += 1
    value.value = null
    confirmed.value = false
    error.value = null
    loading.value = false
    saving.value = false
    pendingKey.value = null
    if (userId) void refresh()
  }

  async function save(patch: UserPreferencesPatch): Promise<void> {
    if (blocked.value || !principal) return
    const version = ++revision
    loading.value = false
    saving.value = true
    pendingKey.value = patch.notifications ? `notifications.${Object.keys(patch.notifications)[0]}` : Object.keys(patch)[0] ?? null
    error.value = null
    try {
      const next = await preferencesApi.update(patch)
      if (version === revision) value.value = next
    } catch (caught) {
      if (version === revision) error.value = localizedError(caught, 'settings.saveFailed')
    } finally {
      if (version === revision) { saving.value = false; pendingKey.value = null }
    }
  }

  const showAroundTX = computed(() => Boolean(confirmed.value && value.value?.activeCombo && value.value.showAroundTx))
  const showActivity = computed(() => Boolean(confirmed.value && value.value?.activeCombo && value.value.showActivity))
  const includeNodePrices = computed(() => value.value?.includeNodePrices ?? true)
  return { value, loading, saving, blocked, pendingKey, error, showAroundTX, showActivity, includeNodePrices, bind, refresh, save }
})
