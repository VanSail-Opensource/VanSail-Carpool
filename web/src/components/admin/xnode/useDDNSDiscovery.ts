import { computed, onScopeDispose, shallowRef } from 'vue'
import { xnodeApi, type XNodeGlobal } from '@/api/xnode'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useDDNSDiscovery() {
  const providers = shallowRef<XNodeGlobal['providers']>({})
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const latest = createLatestRequest()
  let controller: AbortController | undefined
  async function load() {
    controller?.abort(); controller = new AbortController()
    const ticket = latest.begin(); loading.value = true; error.value = null
    try {
      const value = await xnodeApi.global(controller.signal)
      if (latest.isCurrent(ticket)) providers.value = value.providers ?? {}
    } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'nodeForms.providersFailed') }
    finally { if (latest.isCurrent(ticket)) loading.value = false }
  }
  onScopeDispose(() => { controller?.abort(); latest.dispose() })
  void load()
  const items = computed(() => Object.keys(providers.value).sort().map(value => ({ value, label: value })))
  return { providers, items, loading, error, load }
}
