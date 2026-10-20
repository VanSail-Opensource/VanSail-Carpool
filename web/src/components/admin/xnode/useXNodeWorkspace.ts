import { onMounted, onScopeDispose, shallowRef } from 'vue'
import { xnodeApi, type XNode, type XNodeConfiguration, type XNodeSnippet, type XNodeGroup, type XNodeEndpoint, type XNodeMetrics } from '@/api/xnode'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useXNodeWorkspace() {
  const nodes = shallowRef<XNode[]>([])
  const configurations = shallowRef<XNodeConfiguration[]>([])
  const snippets = shallowRef<XNodeSnippet[]>([])
  const groups = shallowRef<XNodeGroup[]>([])
  const endpoints = shallowRef<XNodeEndpoint[]>([])
  const metrics = shallowRef<XNodeMetrics | null>(null)
  const loading = shallowRef(true)
  const error = shallowRef<string | null>(null)
  const actionError = shallowRef<string | null>(null)
  const busy = shallowRef(false)
  let active = true
  let metricInFlight = false
  const latest = createLatestRequest()
  const metricLatest = createLatestRequest()
  let controller: InstanceType<typeof globalThis.AbortController> | undefined
  let metricController: InstanceType<typeof globalThis.AbortController> | undefined
  let timer: ReturnType<typeof setInterval> | undefined
  async function load(): Promise<void> {
    controller?.abort(); metricController?.abort(); metricLatest.invalidate(); metricInFlight = false
    controller = new globalThis.AbortController()
    const ticket = latest.begin()
    loading.value = true
    error.value = null
    try {
      const [nodeList, configList, snippetList, groupList, endpointList, current] = await Promise.all([
        xnodeApi.nodes(controller.signal), xnodeApi.configurations(controller.signal), xnodeApi.snippets(controller.signal), xnodeApi.groups(controller.signal), xnodeApi.endpoints(controller.signal), xnodeApi.metrics(controller.signal),
      ])
      if (!active || !latest.isCurrent(ticket)) return
      nodes.value = nodeList.items; configurations.value = configList.items; snippets.value = snippetList.items
      groups.value = groupList.items; endpoints.value = endpointList.items; metrics.value = current
    } catch (caught) { if (active && latest.isCurrent(ticket)) error.value = localizedError(caught, 'xnode.loadFailed') }
    finally { if (active && latest.isCurrent(ticket)) loading.value = false }
  }
  async function perform(action: () => Promise<unknown>): Promise<boolean> {
    if (busy.value || loading.value || error.value || !active) return false
    busy.value = true; actionError.value = null
    try { await action(); await load(); return true }
    catch (caught) { actionError.value = localizedError(caught, 'xnode.saveFailed'); return false }
    finally { busy.value = false }
  }
  onMounted(() => {
    void load()
    timer = setInterval(() => {
      if (document.visibilityState !== 'visible' || loading.value || busy.value || metricInFlight) return
      metricController?.abort(); metricController = new globalThis.AbortController()
      const ticket = metricLatest.begin(); metricInFlight = true
      void xnodeApi.metrics(metricController.signal).then(value => { if (active && metricLatest.isCurrent(ticket)) metrics.value = value }).catch(() => { if (active && metricLatest.isCurrent(ticket) && metrics.value) metrics.value = { ...metrics.value, nodes: metrics.value.nodes.map(node => ({ ...node, stale: true })) } }).finally(() => { if (metricLatest.isCurrent(ticket)) metricInFlight = false })
    }, 5000)
  })
  onScopeDispose(() => { active = false; controller?.abort(); metricController?.abort(); latest.dispose(); metricLatest.dispose(); metrics.value = null; if (timer) clearInterval(timer) })
  return { nodes, configurations, snippets, groups, endpoints, metrics, loading, error, actionError, busy, load, perform }
}
