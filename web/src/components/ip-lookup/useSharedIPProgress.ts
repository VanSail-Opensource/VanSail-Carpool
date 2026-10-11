import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { ipLookupApi, type IPLookupSharedProgress } from '@/api/ipLookup'
import { useSessionStore } from '@/stores/session'

export function useSharedIPProgress(address: () => string, completed: () => void) {
  const session = useSessionStore()
  const activeRun = shallowRef<IPLookupSharedProgress | null>(null)
  const loading = shallowRef(false)
  const unavailable = shallowRef(false)
  let revision = 0
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  async function read(): Promise<void> {
    const ip = address()
    if (!ip || !session.user?.id || disposed || document.hidden) return
    controller?.abort()
    const request = controller = new AbortController()
    const current = revision
    try {
      const result = await ipLookupApi.progress(ip, request.signal)
      if (disposed || current !== revision || request.signal.aborted) return
      const wasRunning = !!activeRun.value
      activeRun.value = result
      unavailable.value = false
      if (wasRunning && !result) completed()
    } catch { if (current === revision && !request.signal.aborted) unavailable.value = true }
    finally {
      if (current === revision && !disposed) {
        loading.value = false
        clearTimeout(timer)
        if (activeRun.value || unavailable.value) timer = setTimeout(() => void read(), 5000)
      }
    }
  }
  watch(() => [session.user?.id, address()], () => {
    revision++
    controller?.abort()
    clearTimeout(timer)
    activeRun.value = null
    unavailable.value = false
    loading.value = !!address()
    void read()
  }, { immediate: true })
  const onVisibility = () => { clearTimeout(timer); if (!document.hidden) void read() }
  document.addEventListener('visibilitychange', onVisibility)
  onScopeDispose(() => { disposed = true; revision++; controller?.abort(); clearTimeout(timer); document.removeEventListener('visibilitychange', onVisibility) })
  return { activeRun, progressLoading: loading, progressUnavailable: unavailable, checking: computed(() => !!activeRun.value || loading.value || unavailable.value), reloadProgress: read }
}
