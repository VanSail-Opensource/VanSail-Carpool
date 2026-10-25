import { computed, onMounted, onScopeDispose, readonly, shallowRef } from 'vue'
import { comboControlsApi } from '@/api/comboControls'
import { clearResponseCache } from '@/api/cache/session'
import type { ComboControls, EarlyActivationQuote } from '@/api/types'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useComboControls() {
  const value = shallowRef<ComboControls | null>(null)
  const quote = shallowRef<EarlyActivationQuote | null>(null)
  const loading = shallowRef(true)
  const refreshing = shallowRef(false)
  let blockedTimer: ReturnType<typeof globalThis.setTimeout> | undefined
  const loadError = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  const command = useDurableCommand({ errorKey: 'settings.combo.operationFailed', onTerminal: async () => { clearResponseCache(); await load() } })
  const error = computed(() => command.error.value ?? loadError.value)
  const blocked = computed(() => command.busy.value || command.blocksMutations.value || loading.value || !value.value?.mutable)

  async function load(): Promise<void> {
    if (blockedTimer) globalThis.clearTimeout(blockedTimer)
    const token = latest.begin()
    loading.value = value.value === null
    refreshing.value = true
    loadError.value = null
    try {
      const controls = await comboControlsApi.get()
      if (!latest.isCurrent(token)) return
      value.value = controls
      quote.value = null
      if (controls.operation) command.resume(controls.operation, controls.operation.kind)
      if (controls.activePurchase?.status === 'active' && controls.queuedPurchase) {
        const next = await comboControlsApi.quoteActivation(controls.queuedPurchase.id)
        if (latest.isCurrent(token)) quote.value = next
      }
    } catch (caught) {
      if (latest.isCurrent(token)) loadError.value = localizedError(caught, 'settings.combo.loadFailed')
    } finally {
      if (latest.isCurrent(token)) {
        loading.value = false; refreshing.value = false
        if (value.value?.activePurchase && !value.value.mutable && !command.blocksMutations.value && !loadError.value && !document.hidden) blockedTimer = globalThis.setTimeout(() => void load(), 5000)
      }
    }
  }

  async function switchSquad(uuid: string, enabled: boolean): Promise<void> {
    const purchase = value.value?.activePurchase
    if (!purchase || blocked.value) return
    const accepted = await command.execute(uuid, `switch:${purchase.id}:${uuid}:${enabled}`, key => comboControlsApi.switchSquad(purchase.id, uuid, enabled, key))
    if (accepted) await load()
  }

  async function activate(confirmation: string): Promise<boolean> {
    const current = quote.value
    if (!current?.eligible || blocked.value) return false
    const accepted = await command.execute('activate', `activate:${current.currentPurchaseId}:${current.queuedPurchaseId}`, key => comboControlsApi.activate(current.queuedPurchaseId, current.currentPurchaseId, confirmation, key))
    if (accepted) await load()
    return accepted
  }

  onMounted(() => void load())
  function visibilityChanged(): void { if (blockedTimer) globalThis.clearTimeout(blockedTimer); if (!document.hidden && value.value?.activePurchase && !value.value.mutable) void load() }
  onMounted(() => document.addEventListener('visibilitychange', visibilityChanged))
  onScopeDispose(() => { latest.dispose(); if (blockedTimer) globalThis.clearTimeout(blockedTimer); document.removeEventListener('visibilitychange', visibilityChanged) })
  return { value: readonly(value), quote: readonly(quote), loading: readonly(loading), refreshing: readonly(refreshing), error, blocked, command, load, switchSquad, activate }
}
