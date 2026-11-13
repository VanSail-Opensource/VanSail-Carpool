import { computed, onMounted, onScopeDispose, readonly, shallowRef } from 'vue'

import type { EmbyOverview } from '@/api/features'
import { featuresApi } from '@/api/features'
import { restoreRef } from '@/api/cache/restore'
import type { OperationReceipt } from '@/api/types'
import { localizedError, t } from '@/i18n'
import { notifyHaptic } from '@/utils/telegram'
import { createLatestRequest } from '@/utils/latestRequest'
import { useDurableCommand } from './useDurableCommand'

type EmbyCommand = 'setup' | 'preferences' | 'password'

const successKeys: Record<EmbyCommand, string> = {
  setup: 'emby.provisioningStarted',
  preferences: 'emby.preferencesUpdated',
  password: 'emby.passwordChanged',
}

export function useEmby() {
  const overview = shallowRef<EmbyOverview | null>(null)
  const loading = shallowRef(true)
  const loadError = shallowRef<string | null>(null)
  const message = shallowRef<string | null>(null)
  const latestLoad = createLatestRequest()
  let disposed = false
  const command = useDurableCommand({
    errorKey: 'errors.embyUpdate',
    onTerminal: async (receipt, commandId) => {
      if (disposed) return
      if (receipt.status === 'succeeded') {
        await load()
        if (disposed) return
        message.value = t(successKeys[commandId as EmbyCommand])
        notifyHaptic('success')
      } else {
        message.value = null
        notifyHaptic('error')
      }
    },
  })
  const busy = computed<EmbyCommand | null>(() => command.busy.value
    ? command.activeCommandId.value as EmbyCommand
    : null)
  const error = computed(() => loadError.value ?? command.error.value)

  async function load(): Promise<void> {
    if (disposed) return
    const token = latestLoad.begin()
    loading.value = !restoreRef('/api/v1/emby/account', overview)
    loadError.value = null
    try {
      const next = await featuresApi.getEmby()
      if (latestLoad.isCurrent(token)) overview.value = next
    } catch (caught) {
      if (latestLoad.isCurrent(token)) loadError.value = localizedError(caught, 'errors.embyUnavailable')
    } finally {
      if (latestLoad.isCurrent(token)) loading.value = false
    }
  }

  async function perform(kind: EmbyCommand, fingerprint: string, action: (key: string) => Promise<OperationReceipt>): Promise<boolean> {
    if (disposed) return false
    message.value = null
    const accepted = await command.execute(kind, `${kind}:${fingerprint}`, action)
    if (disposed) return false
    if (accepted && kind === 'setup') message.value = t('emby.provisioningStarted')
    if (!accepted) notifyHaptic('error')
    return accepted
  }

  function setup(payload: { password: string; maxParentalRating: number | null; disabledLibraryIds: string[] }): Promise<boolean> {
    return perform('setup', JSON.stringify(payload), (key) => featuresApi.setupEmby(payload, key))
  }

  function updatePreferences(payload: { maxParentalRating: number | null; disabledLibraryIds: string[] }): Promise<boolean> {
    return perform('preferences', JSON.stringify(payload), (key) => featuresApi.updateEmbyPreferences(payload, key))
  }

  function changePassword(password: string): Promise<boolean> {
    return perform('password', password, (key) => featuresApi.changeEmbyPassword(password, key))
  }

  onMounted(() => void load())
  onScopeDispose(() => { disposed = true; latestLoad.dispose(); overview.value = null; message.value = null })

  return {
    overview: readonly(overview),
    loading: readonly(loading),
    busy: readonly(busy),
    blocked: command.blocksMutations,
    receipt: command.receipt,
    checking: command.checking,
    error: readonly(error),
    message: readonly(message),
    load,
    setup,
    updatePreferences,
    changePassword,
    refreshOperation: command.refresh,
  }
}
