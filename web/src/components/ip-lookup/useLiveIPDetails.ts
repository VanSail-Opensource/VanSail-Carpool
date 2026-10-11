import { onScopeDispose, shallowRef, watch } from 'vue'
import { ipLookupApi, type IPLookupLiveDetails, type IPLookupProgress } from '@/api/ipLookup'
import { ApiError } from '@/api/http'
import { useSessionStore } from '@/stores/session'

export function useLiveIPDetails(address: () => string, reportKey: () => string) {
  const session = useSessionStore()
  const details = shallowRef<Partial<IPLookupLiveDetails> | null>(null)
  const progress = shallowRef<IPLookupProgress | null>(null)
  const loading = shallowRef(false)
  const errorCode = shallowRef('')
  let controller: AbortController | undefined
  let revision = 0

  async function refresh(force = true): Promise<void> {
    const ip = address()
    if (!reportKey() || !session.user?.id || loading.value) return
    controller?.abort()
    const active = controller = new AbortController()
    const current = ++revision
    loading.value = true
    progress.value = { phase: 'topology', completed: 0, total: 8 }
    errorCode.value = ''
    try {
      const result = await ipLookupApi.details(ip, active.signal, force, event => {
        if (current !== revision || active.signal.aborted || event.ip !== ip) return
        if ('progress' in event) progress.value = event.progress
        if (event.type === 'section') details.value = { ...details.value, ip, [event.section]: event.data }
      })
      if (current === revision && !active.signal.aborted && result.ip === ip) {
        details.value = result
        progress.value = { phase: 'complete', completed: 8, total: 8 }
      }
    } catch (error) {
      if (current === revision && !active.signal.aborted) errorCode.value = error instanceof ApiError ? error.code : 'IP_DETAILS_UNAVAILABLE'
    } finally { if (current === revision) loading.value = false }
  }

  watch(() => `${session.user?.id ?? ''}|${address()}|${reportKey()}`, () => {
    revision++
    controller?.abort()
    details.value = null
    progress.value = null
    loading.value = false
    errorCode.value = ''
    void refresh(false)
  }, { immediate: true })
  onScopeDispose(() => { revision++; controller?.abort() })
  return { details, progress, loading, errorCode, refresh }
}
