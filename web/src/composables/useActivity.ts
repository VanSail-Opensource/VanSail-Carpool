import { onMounted, onScopeDispose, readonly, shallowRef } from 'vue'

import { featuresApi } from '@/api/features'
import type { ActivityOverview, ActivityResult } from '@/api/features'
import { activityNotification } from '@/components/activity/feedback'
import { localizedError } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { notifyBetOutcome, notifyHaptic } from '@/utils/telegram'
import { createLatestRequest } from '@/utils/latestRequest'

export function useActivity() {
  const overview = shallowRef<ActivityOverview | null>(null)
  const result = shallowRef<ActivityResult | null>(null)
  const loading = shallowRef(true)
  const refreshing = shallowRef(false)
  const busy = shallowRef<'check-in' | 'bet' | 'draw' | null>(null)
  const error = shallowRef<string | null>(null)
  const actionKeys = new Map<string, string>()
  const latestLoad = createLatestRequest()
  let disposed = false

  function idempotencyKey(action: string): string {
    const existing = actionKeys.get(action)
    if (existing) return existing
    const created = createUuid()
    actionKeys.set(action, created)
    return created
  }

  async function load(options: { quiet?: boolean } = {}): Promise<void> {
    if (disposed) return
    const token = latestLoad.begin()
    if (!options.quiet) loading.value = true
    refreshing.value = true
    error.value = null
    try {
      const response = await featuresApi.getActivity()
      if (latestLoad.isCurrent(token)) overview.value = response
    } catch (caught) {
      if (!latestLoad.isCurrent(token)) return
      error.value = localizedError(caught, 'errors.activityUnavailable')
      if (overview.value) overview.value = {
        ...overview.value,
        groupBoost: { state: 'unavailable', count: null, boostUrl: overview.value.groupBoost.boostUrl },
      }
    } finally {
      if (latestLoad.isCurrent(token)) {
        loading.value = false
        refreshing.value = false
      }
    }
  }

  async function run(kind: NonNullable<typeof busy.value>, actionId: string, action: (key: string) => Promise<ActivityResult>): Promise<void> {
    if (disposed || busy.value) return
    busy.value = kind
    error.value = null
    try {
      const received = await action(idempotencyKey(actionId))
      if (disposed) return
      result.value = received
      actionKeys.delete(actionId)
      if (result.value.kind === 'bet') notifyBetOutcome(result.value.outcome === 'win' ? 'win' : 'loss')
      else notifyHaptic(activityNotification(result.value))
      await load({ quiet: true })
    } catch (caught) {
      if (disposed) return
      error.value = localizedError(caught, 'errors.activityFailed')
      if (kind === 'check-in') {
        const failure = error.value
        await load({ quiet: true })
        if (disposed) return
        error.value = failure
      }
      notifyHaptic('error')
    } finally {
      if (!disposed) busy.value = null
    }
  }

  function checkIn(): Promise<void> {
    return run('check-in', 'check-in', featuresApi.checkIn)
  }

  function placeBet(payload: { gameId: string; stakeTxbMinor: string }): Promise<void> {
    const actionId = `bet:${payload.gameId}:${payload.stakeTxbMinor}`
    return run('bet', actionId, (key) => featuresApi.placeBet(payload.gameId, payload.stakeTxbMinor, key))
  }

  async function draw(drawId: string): Promise<ActivityResult> {
    if (disposed || busy.value) throw new Error()
    const actionId = 'draw:' + drawId
    busy.value = 'draw'
    error.value = null
    try {
      const drawResult = await featuresApi.drawLuckyPrize(drawId, idempotencyKey(actionId))
      if (disposed) throw new globalThis.DOMException('', 'AbortError')
      actionKeys.delete(actionId)
      notifyHaptic(activityNotification(drawResult))
      await load({ quiet: true })
      if (disposed) throw new globalThis.DOMException('', 'AbortError')
      return drawResult
    } catch (caught) {
      if (disposed) throw caught
      error.value = localizedError(caught, 'errors.activityFailed')
      notifyHaptic('error')
      throw caught
    } finally {
      if (!disposed) busy.value = null
    }
  }

  function clearResult(): void {
    result.value = null
  }

  onMounted(() => void load())
  onScopeDispose(() => {
    disposed = true
    latestLoad.dispose()
    actionKeys.clear()
    overview.value = null
    result.value = null
    busy.value = null
  })

  return {
    overview: readonly(overview),
    result: readonly(result),
    loading: readonly(loading),
    refreshing: readonly(refreshing),
    busy: readonly(busy),
    error: readonly(error),
    load,
    checkIn,
    placeBet,
    draw,
    clearResult,
  }
}
