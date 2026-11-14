import { computed, onScopeDispose, shallowRef, watch } from 'vue'
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
  const data = useSubscriptionData(hourlyApi.get, 15_000, 'hourly.loadFailed', undefined, '/api/v1/hourly')
  const operation = useOperationReceipt()
  const submitting = shallowRef(false)
  const actionError = shallowRef<string | null>(null)
  const keys = new Map<string, string>()
  let active = true
  const busy = computed(() => !data.liveReady.value || submitting.value || operation.polling.value)
  watch(operation.terminal, terminal => { if (terminal) void data.refresh() })

  async function run(identity: string, action: (key: string) => Promise<OperationReceipt>): Promise<boolean> {
    if (!active || !data.liveReady.value || busy.value) return false
    submitting.value = true
    actionError.value = null
    const key = keys.get(identity) ?? createUuid()
    keys.set(identity, key)
    try {
      const receipt = await action(key)
      if (!active) return false
      operation.track(receipt)
      keys.delete(identity)
      await data.refresh()
      return active
    } catch (caught) {
      if (active) actionError.value = hourlyError(caught)
      return false
    } finally { if (active) submitting.value = false }
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
    if (!active || !snapshot || !data.liveReady.value || busy.value) return false
    submitting.value = true
    actionError.value = null
    try {
      await hourlyApi.extra(enabled, snapshot.version, snapshot.policy.extraGBPriceTXBMinor)
      if (!active) return false
      await data.refresh()
      return active
    } catch (caught) { if (active) actionError.value = hourlyError(caught); return false }
    finally { if (active) submitting.value = false }
  }
  onScopeDispose(() => { active = false; keys.clear() })
  return { ...data, operation, busy, submitting, actionError, activate, disable, extra }
}
