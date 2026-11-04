import { onMounted, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { memberConnectionsApi, type MemberConnectionPage } from '@/api/memberConnections'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useMemberConnections() {
  const data = shallowRef<MemberConnectionPage | null>(null)
  const search = ref(''), page = ref(1)
  const loading = ref(true), saving = ref(false)
  const error = ref(''), grantError = ref('')
  const latest = createLatestRequest()
  let controller: AbortController | undefined
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  async function load() {
    controller?.abort(); controller = new AbortController()
    const ticket = latest.begin()
    loading.value = true; error.value = ''
    try {
      const next = await memberConnectionsApi.list(search.value, page.value, controller.signal)
      if (!latest.isCurrent(ticket)) return
      data.value = next
      // The server clamps pages after expiry. Do not discard pager metadata during refresh.
      page.value = next.page
    } catch (caught) {
      if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'connections.loadFailed')
    } finally { if (latest.isCurrent(ticket)) loading.value = false }
  }
  async function grant(ip: string) {
    if (saving.value || !data.value?.canGrant) return
    saving.value = true; grantError.value = ''
    try { await memberConnectionsApi.grant(ip.trim()); if (!disposed) await load() }
    catch (caught) { if (!disposed) grantError.value = localizedError(caught, 'connections.grantFailed') }
    finally { if (!disposed) saving.value = false }
  }
  watch(search, () => { latest.invalidate(); controller?.abort(); page.value = 1; if (timer) clearTimeout(timer); timer = setTimeout(() => void load(), 250) })
  watch(page, () => { if (!loading.value) void load() })
  onMounted(() => void load())
  onScopeDispose(() => { disposed = true; latest.dispose(); controller?.abort(); if (timer) clearTimeout(timer) })
  return { data, search, page, loading, saving, error, grantError, load, grant }
}
