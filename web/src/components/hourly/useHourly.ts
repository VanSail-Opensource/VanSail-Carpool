import { computed, shallowRef, watch } from 'vue'
import { hourlyApi, type HourlySquad } from '@/api/hourly'
import type { OperationReceipt } from '@/api/types'
import { ApiError } from '@/api/http'
import { useOperationReceipt } from '@/composables/useOperationReceipt'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

export function hourlyError(caught: unknown): string {
  if (caught instanceof ApiError && caught.status === 409) return t('hourly.conflict')
  return localizedError(caught, 'hourly.actionFailed')
}

export function useHourly() {
  const data = useSubscriptionData(hourlyApi.get, 15_000, 'hourly.loadFailed')
  const operation = useOperationReceipt()
  const submitting = shallowRef(false)
  const actionError = shallowRef<string | null>(null)
  const keys = new Map<string, string>()
  const busy = computed(() => submitting.value || operation.polling.value)
  watch(operation.terminal, terminal => { if (terminal) void data.refresh() })

  async function run(identity: string, action: (key: string) => Promise<OperationReceipt>): Promise<boolean> {
    if (busy.value) return false
    submitting.value = true
    actionError.value = null
    const key = keys.get(identity) ?? createUuid()
    keys.set(identity, key)
    try {
      operation.track(await action(key))
      keys.delete(identity)
      await data.refresh()
      return true
    } catch (caught) {
      actionError.value = hourlyError(caught)
      return false
    } finally { submitting.value = false }
  }
  function activate(squad: HourlySquad, code?: string): Promise<boolean> {
    const version = data.value.value?.policy.version ?? ''
    return run(`activate:${squad.offer.groupId}:${version}:${squad.offer.revision}`, key =>
      hourlyApi.activate(squad.offer.groupId, version, squad.offer.revision, key, code))
  }
  function disable(squad: HourlySquad): Promise<boolean> {
    const id = squad.enrollment?.id
    if (!id) return Promise.resolve(false)
    const version = data.value.value?.policy.version ?? ''
    return run(`disable:${id}:${version}`, key => hourlyApi.disable(id, version, key))
  }
  async function extra(enabled: boolean): Promise<boolean> {
    const snapshot = data.value.value?.policy
    if (!snapshot || busy.value) return false
    submitting.value = true
    actionError.value = null
    try {
      await hourlyApi.extra(enabled, snapshot.version, snapshot.policy.extraGBPriceTXBMinor)
      await data.refresh()
      return true
    } catch (caught) { actionError.value = hourlyError(caught); return false }
    finally { submitting.value = false }
  }
  return { ...data, operation, busy, submitting, actionError, activate, disable, extra }
}
