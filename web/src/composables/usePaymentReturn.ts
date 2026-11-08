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

  const isConfirmed = computed(() => state.value === 'confirmed')

  function stopPolling(): void {
    latest.invalidate()
    clearPollTimer()
  }

  function clearPollTimer(): void {
    if (pollTimer !== undefined) clearTimeout(pollTimer)
    pollTimer = undefined
  }

  function schedulePoll(token: number): void {
    clearPollTimer()
    pollTimer = setTimeout(() => void refresh(token), 2000)
  }

  function applyStatus(status: FeaturePaymentOrder['status'], token: number): void {
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
    clearPollTimer()
    if (!latest.isCurrent(token)) return
    if (!orderId.value) {
      state.value = 'missing'
      return
    }
    state.value = order.value || returnDetails.value ? 'pending' : 'checking'
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
        authenticationRequired.value = false
        returnDetails.value = next
        orderStatus.value = next.status
        applyStatus(next.status, token)
      } else {
        returnDetails.value = null
        const next = await api.getPaymentOrder(orderId.value)
        if (!latest.isCurrent(token)) return
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
        state.value = 'pending'
        schedulePoll(token)
      } else {
        state.value = 'unavailable'
      }
    }
  }

  watch(() => [orderId.value, options.provider?.value ?? null, options.capability?.value ?? ''], () => {
    order.value = null
    returnDetails.value = null
    orderStatus.value = null
    authenticationRequired.value = false
    void refresh()
  }, { immediate: true })
  onScopeDispose(() => {
    clearPollTimer()
    latest.dispose()
  })

  const details = computed(() => order.value ?? returnDetails.value)

  return { state: readonly(state), order: readonly(order), details, orderStatus: readonly(orderStatus), authenticationRequired: readonly(authenticationRequired), isConfirmed, refresh, stopPolling }
}
