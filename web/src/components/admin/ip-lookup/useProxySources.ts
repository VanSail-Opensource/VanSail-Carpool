import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { ApiError } from '@/api/http'
import { ipLookupApi, type IPLookupProxySourcesStatus } from '@/api/ipLookup'
import { useSessionStore } from '@/stores/session'

/** One visible-page poll supplies all source rows, including startup refreshes. */
export function useProxySources() {
  const session = useSessionStore()
  const status = shallowRef<IPLookupProxySourcesStatus | null>(null)
  const errorCode = shallowRef('')
  const starting = shallowRef<string[]>([])
  const byId = computed(() => new Map(status.value?.sources.map(source => [source.id, source]) ?? []))
  let disposed = false
  let version = 0
  let cycle = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller = new AbortController()
  let loading = false
  const fail = (error: unknown) => { errorCode.value = error instanceof ApiError ? error.code : 'IP_LOOKUP_FAILED' }

  async function load(): Promise<void> {
    if (disposed || loading) return
    const current = version
    loading = true
    try {
      const result = await ipLookupApi.sourcesStatus(controller.signal)
      if (disposed || current !== version) return
      status.value = result
      errorCode.value = ''
    } catch (error) { if (!disposed && current === version) fail(error) }
    finally { if (current === version) loading = false }
  }

  async function poll(ticket: number): Promise<void> {
    if (disposed || ticket !== cycle) return
    const current = version
    if (!document.hidden) await load()
    if (disposed || current !== version || ticket !== cycle) return
    const active = status.value?.sources.some(source => source.refreshing) || starting.value.length > 0
    timer = setTimeout(() => { void poll(ticket) }, active ? 2000 : 15000)
  }

  async function refreshSource(id: string): Promise<void> {
    if (starting.value.includes(id) || byId.value.get(id)?.refreshing) return
    const current = version
    starting.value = [...starting.value, id]
    try {
      await ipLookupApi.testSource(id)
      if (disposed || current !== version) return
      const ticket = ++cycle
      clearTimeout(timer)
      await load()
      if (disposed || current !== version || ticket !== cycle) return
      timer = setTimeout(() => { void poll(ticket) }, 2000)
    } catch (error) { if (!disposed && current === version) fail(error) }
    finally { if (current === version) starting.value = starting.value.filter(value => value !== id) }
  }

  watch(() => session.user?.id, () => {
    version++
    controller.abort()
    controller = new AbortController()
    clearTimeout(timer)
    status.value = null
    starting.value = []
    loading = false
    void poll(++cycle)
  }, { immediate: true })
  const onVisibility = () => { if (!document.hidden) { clearTimeout(timer); void poll(++cycle) } }
  document.addEventListener('visibilitychange', onVisibility)
  onScopeDispose(() => { disposed = true; version++; cycle++; clearTimeout(timer); controller.abort(); document.removeEventListener('visibilitychange', onVisibility) })
  return { status, byId, errorCode, starting, load, refreshSource }
}
