import { computed, onMounted, onScopeDispose, readonly, shallowRef } from 'vue'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

// Live member data is never restored from persistent rendering snapshots.
export function useSubscriptionData<T>(fetcher: (signal: AbortSignal) => Promise<T>, interval = 60_000, errorKey = 'subscription.loadFailed', nextInterval?: (value: T | null) => number) {
  const value = shallowRef<T | null>(null)
  const loading = shallowRef(true)
  const refreshing = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false

  function schedule(): void {
    if (timer) clearTimeout(timer)
    if (!disposed && !document.hidden) timer = setTimeout(() => void refresh(), Math.max(1000, nextInterval ? nextInterval(value.value as T | null) : interval))
  }

  async function refresh(clear = false): Promise<void> {
    if (disposed) return
    if (timer) clearTimeout(timer)
    controller?.abort()
    controller = new AbortController()
    const signal = controller.signal
    const token = latest.begin()
    if (clear) value.value = null
    loading.value = value.value == null
    refreshing.value = true
    error.value = null
    try {
      const result = await fetcher(signal)
      if (latest.isCurrent(token) && !disposed) value.value = result
    } catch (caught) {
      if (latest.isCurrent(token) && !signal.aborted && !disposed) error.value = localizedError(caught, errorKey)
    } finally {
      if (latest.isCurrent(token) && !disposed) {
        loading.value = false
        refreshing.value = false
        schedule()
      }
    }
  }

  function visibilityChanged(): void {
    if (document.hidden) { if (timer) clearTimeout(timer); controller?.abort(); latest.begin(); refreshing.value = false }
    else void refresh()
  }
  function focused(): void { if (!document.hidden) void refresh() }

  onMounted(() => {
    document.addEventListener('visibilitychange', visibilityChanged)
    window.addEventListener('focus', focused)
    void refresh()
  })
  onScopeDispose(() => {
    disposed = true
    latest.dispose()
    controller?.abort()
    if (timer) clearTimeout(timer)
    document.removeEventListener('visibilitychange', visibilityChanged)
    window.removeEventListener('focus', focused)
    value.value = null
  })
  return { value: readonly(value), loading: readonly(loading), refreshing: readonly(refreshing), error: readonly(error), hasData: computed(() => value.value != null), refresh }
}
