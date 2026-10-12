import { onScopeDispose, shallowRef } from 'vue'
import { ApiError } from '@/api/http'
import { ipLookupApi, type IPLookupProxySourceTest } from '@/api/ipLookup'

export function useProxySourceTest(sourceId: () => string) {
  const test = shallowRef<IPLookupProxySourceTest | null>(null)
  const busy = shallowRef(false)
  const errorCode = shallowRef('')
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  const controller = new AbortController()
  const fail = (error: unknown) => { errorCode.value = error instanceof ApiError ? error.code : 'IP_LOOKUP_FAILED'; busy.value = false }

  async function poll(): Promise<void> {
    if (disposed || !test.value) return
    if (document.hidden) { timer = setTimeout(() => { void poll() }, 2000); return }
    try {
      const value = await ipLookupApi.sourceTest(test.value.id, controller.signal)
      if (disposed) return
      test.value = value
      busy.value = value.status !== 'completed'
      if (busy.value) timer = setTimeout(() => { void poll() }, 2000)
    } catch (error) { if (!disposed) fail(error) }
  }

  async function start(): Promise<void> {
    if (busy.value) return
    busy.value = true
    errorCode.value = ''
    try {
      const value = await ipLookupApi.testSource(sourceId())
      if (disposed) return
      test.value = value
      timer = setTimeout(() => { void poll() }, 2000)
    } catch (error) { if (!disposed) fail(error) }
  }
  onScopeDispose(() => { disposed = true; clearTimeout(timer); controller.abort() })
  return { test, busy, errorCode, start }
}
