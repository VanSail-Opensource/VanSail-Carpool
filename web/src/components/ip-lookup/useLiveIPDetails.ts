import { onScopeDispose, shallowRef, watch } from 'vue'
import { ipLookupApi, type IPLookupLiveDetails, type IPLookupProgress } from '@/api/ipLookup'
import { ApiError } from '@/api/http'
import { useSessionStore } from '@/stores/session'

export function useLiveIPDetails(address: () => string, reportKey: () => string, snapshot: () => IPLookupLiveDetails | null = () => null) {
  const session = useSessionStore()
  const details = shallowRef<Partial<IPLookupLiveDetails> | null>(null)
  const progress = shallowRef<IPLookupProgress | null>(null)
  const loading = shallowRef(false)
  const errorCode = shallowRef('')
  let controller: AbortController | undefined
  let revision = 0
  let disposed = false
  let owner = session.user?.id
  let retiredSnapshot: IPLookupLiveDetails | null = null

  async function refresh(): Promise<void> {
    const ip = address()
    if (disposed || !reportKey() || !session.user?.id || loading.value) return
    controller?.abort()
    const active = controller = new AbortController()
    const current = ++revision
    loading.value = true
    progress.value = { phase: 'topology', completed: 0, total: 8 }
    errorCode.value = ''
    try {
      const result = await ipLookupApi.details(ip, active.signal, false, event => {
        if (current !== revision || active.signal.aborted || event.ip !== ip) return
        if ('progress' in event) progress.value = event.progress
        if (event.type === 'section') details.value = { ...details.value, ip, [event.section]: event.data }
      })
      if (current === revision && !active.signal.aborted && result.ip === ip) {
        details.value = result
        progress.value = { phase: 'complete', completed: progress.value?.total ?? 8, total: progress.value?.total ?? 8 }
      }
    } catch (error) {
      if (current === revision && !active.signal.aborted) errorCode.value = error instanceof ApiError ? error.code : 'IP_DETAILS_UNAVAILABLE'
    } finally { if (current === revision) loading.value = false }
  }

  let scope = ''
  watch(() => [session.user?.id, address(), reportKey(), snapshot()], () => {
    revision++
    controller?.abort()
    if (owner !== session.user?.id) {
      // A parent may still hold the previous owner's paid report for this IP.
      // Only a new snapshot or an owner-scoped read may populate the new view.
      retiredSnapshot = snapshot()
      owner = session.user?.id
    }
    const nextScope = `${session.user?.id ?? ''}|${address()}`
    if (nextScope !== scope) details.value = null
    scope = nextScope
    progress.value = null
    loading.value = false
    errorCode.value = ''
    if (!session.user?.id || !reportKey()) { details.value = null; return }
    const supplied = snapshot()
    if (supplied && supplied !== retiredSnapshot && supplied.ip === address()) details.value = supplied
    else void refresh()
  }, { immediate: true, flush: 'sync' })
  onScopeDispose(() => { disposed = true; revision++; controller?.abort(); details.value = null; progress.value = null; loading.value = false; errorCode.value = '' })
  return { details, progress, loading, errorCode, refresh }
}
