import type { Ref } from 'vue'
import { computed, onScopeDispose, readonly, shallowRef, watch } from 'vue'

import type { FeaturePaymentOrder, FeaturePaymentReturnStatus, PaymentReturnProvider } from '@/api/features'
import { api } from '@/api/client'
import { ApiError } from '@/api/http'
import { createLatestRequest } from '@/utils/latestRequest'

export type PaymentReturnState = 'checking' | 'pending' | 'confirmed' | 'terminal' | 'missing' | 'unavailable'

const terminalStatuses = new Set<FeaturePaymentOrder['status']>(['cancelled', 'expired', 'failed', 'refunded'])

interface PaymentReturnOptions {
  browserStatus?: boolean
  provider?: Ref<PaymentReturnProvider | null>
  capability?: Ref<string>
}

export function usePaymentReturn(orderId: Ref<string>, options: PaymentReturnOptions = {}) {
  const state = shallowRef<PaymentReturnState>('checking')
  const order = shallowRef<FeaturePaymentOrder | null>(null)
  const returnDetails = shallowRef<FeaturePaymentReturnStatus | null>(null)
  const orderStatus = shallowRef<FeaturePaymentOrder['status'] | null>(null)
  const authenticationRequired = shallowRef(false)
  let pollTimer: ReturnType<typeof setTimeout> | undefined
  const latest = createLatestRequest()
  let pollingActive = false
  let pollingToken = 0
  let checkingToken: number | undefined
  let failures = 0

  const isConfirmed = computed(() => state.value === 'confirmed')

  function stopPolling(): void {
    pollingActive = false
    latest.invalidate()
    clearPollTimer()
  }

  function clearPollTimer(): void {
    if (pollTimer !== undefined) clearTimeout(pollTimer)
    pollTimer = undefined
  }

  function schedulePoll(token: number): void {
    clearPollTimer()
    pollingActive = true; pollingToken = token
    if (!document.hidden) pollTimer = setTimeout(() => void refresh(token), Math.min(2000 * 2 ** failures, 30_000))
  }

  function applyStatus(status: FeaturePaymentOrder['status'], token: number): void {
    pollingActive = false
    if (status === 'paid') {
      state.value = 'confirmed'
    } else if (terminalStatuses.has(status)) {
      state.value = 'terminal'
    } else {
      state.value = 'pending'
      schedulePoll(token)
    }
  }

  async function refresh(expectedToken?: number): Promise<void> {
    const token = expectedToken ?? latest.begin()
    if (checkingToken === token || !latest.isCurrent(token)) return
    clearPollTimer()
    pollingActive = false
    if (!orderId.value) {
      state.value = 'missing'
      return
    }
    state.value = order.value || returnDetails.value ? 'pending' : 'checking'
    checkingToken = token
    try {
      if (options.browserStatus) {
        const provider = options.provider?.value
        const capability = options.capability?.value ?? ''
        if (!provider) {
          state.value = 'unavailable'
          return
        }
        const next = await api.getPaymentReturnStatus(provider, orderId.value, capability)
        if (!latest.isCurrent(token)) return
        failures = 0
        authenticationRequired.value = false
        returnDetails.value = next
        orderStatus.value = next.status
        applyStatus(next.status, token)
      } else {
        returnDetails.value = null
        const next = await api.getPaymentOrder(orderId.value)
        if (!latest.isCurrent(token)) return
        failures = 0
        authenticationRequired.value = false
        order.value = next
        orderStatus.value = next.status
        applyStatus(next.status, token)
      }
    } catch (caught) {
      if (!latest.isCurrent(token)) return
      if (caught instanceof ApiError && [401, 403, 404].includes(caught.status)) {
        authenticationRequired.value = caught.status !== 404
        order.value = null
        returnDetails.value = null
        orderStatus.value = null
        state.value = authenticationRequired.value ? 'unavailable' : 'missing'
        return
      }
      if (order.value || returnDetails.value) {
        failures = Math.min(failures + 1, 4)
        state.value = 'pending'
        schedulePoll(token)
      } else {
        state.value = 'unavailable'
      }
    } finally {
      if (checkingToken === token) checkingToken = undefined
    }
  }

  watch(() => [orderId.value, options.provider?.value ?? null, options.capability?.value ?? ''], () => {
    order.value = null
    returnDetails.value = null
    orderStatus.value = null
    authenticationRequired.value = false
    failures = 0
    void refresh()
  }, { immediate: true })
  function visibility(): void {
    if (document.hidden) clearPollTimer()
    else if (pollingActive) void refresh(pollingToken)
  }
  document.addEventListener('visibilitychange', visibility)
  onScopeDispose(() => {
    pollingActive = false
    document.removeEventListener('visibilitychange', visibility)
    clearPollTimer()
    latest.dispose()
  })

  const details = computed(() => order.value ?? returnDetails.value)

  return { state: readonly(state), order: readonly(order), details, orderStatus: readonly(orderStatus), authenticationRequired: readonly(authenticationRequired), isConfirmed, refresh, stopPolling }
}
