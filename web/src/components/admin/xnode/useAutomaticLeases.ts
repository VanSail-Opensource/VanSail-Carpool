import { onMounted, onScopeDispose, shallowRef, watch } from 'vue'
import { firewallApi, type AutomaticLeasePage } from '@/api/firewall'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

/** Visible-page polling owns a bounded page; no IP data enters browser storage. */
export function useAutomaticLeases() {
  const data = shallowRef<AutomaticLeasePage | null>(null)
  const query = shallowRef(''), page = shallowRef(1)
  const loading = shallowRef(true), error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let active = true

  async function load() {
    if (!active) return
    clearTimeout(timer); controller?.abort(); controller = new AbortController()
    const ticket = latest.begin()
    loading.value = true; error.value = null
    try {
      const value = await firewallApi.leases(query.value.trim(), page.value, controller.signal)
      if (latest.isCurrent(ticket)) {
        const lastPage = Math.max(1, Math.ceil(value.total / value.pageSize))
        if (page.value > lastPage) page.value = lastPage
        else data.value = value
      }
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'automaticLeases.loadFailed') }
    finally {
      if (latest.isCurrent(ticket)) {
        loading.value = false
        if (!document.hidden) timer = setTimeout(() => void load(), 10000)
      }
    }
  }
  function changed() {
    latest.invalidate(); controller?.abort(); clearTimeout(timer)
    // Search/page changes must not show rows from the previous query.
    data.value = null; loading.value = true
    timer = setTimeout(() => void load(), 300)
  }
  watch(query, () => { if (page.value !== 1) page.value = 1; else changed() })
  watch(page, changed)
  function visibility() {
    clearTimeout(timer)
    if (!document.hidden) void load()
  }
  function dispose() {
    active = false; latest.dispose(); controller?.abort(); clearTimeout(timer); data.value = null
  }
  onMounted(() => {
    document.addEventListener('visibilitychange', visibility)
    window.addEventListener('txc:session-cleared', dispose)
    void load()
  })
  onScopeDispose(() => {
    dispose(); document.removeEventListener('visibilitychange', visibility)
    window.removeEventListener('txc:session-cleared', dispose)
  })
  return { data, query, page, loading, error, load }
}
