import { getCurrentInstance, onMounted, onScopeDispose, readonly, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { effectivePurchaseStatus, isCurrentPurchase, type PurchaseValidity } from '@/utils/purchaseValidity'

// One boundary timer per consumer; no polling or mutation of purchase records.
export function usePurchaseValidity(
  purchases: MaybeRefOrGetter<readonly PurchaseValidity[]>,
  refresh?: () => void | Promise<void>,
) {
  const now = shallowRef(Date.now())
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false

  function schedule(): void {
    if (timer) clearTimeout(timer)
    now.value = Date.now()
    const boundaries = toValue(purchases)
      .filter(item => ['active', 'activating', 'queued'].includes(item.status))
      .flatMap(item => [Date.parse(item.validFrom), Date.parse(item.validUntil)])
      .filter(boundary => Number.isFinite(boundary) && boundary > now.value)
    if (!boundaries.length) return
    const delay = Math.min(2_147_483_647, Math.min(...boundaries) - now.value)
    timer = setTimeout(() => {
      schedule()
      if (!disposed) void refresh?.()
    }, delay)
  }

  function resume(): void {
    if (document.hidden || disposed) return
    schedule()
    void refresh?.()
  }

  watch(() => toValue(purchases), schedule, { immediate: true })
  if (getCurrentInstance()) onMounted(() => {
    document.addEventListener('visibilitychange', resume)
    window.addEventListener('focus', resume)
  })
  onScopeDispose(() => {
    disposed = true
    if (timer) clearTimeout(timer)
    document.removeEventListener('visibilitychange', resume)
    window.removeEventListener('focus', resume)
  })

  return {
    now: readonly(now),
    status: (item: PurchaseValidity) => effectivePurchaseStatus(item, now.value),
    current: (item: PurchaseValidity | null | undefined) => isCurrentPurchase(item, now.value),
  }
}
