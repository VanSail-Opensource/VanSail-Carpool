import { onMounted, onScopeDispose, shallowRef } from 'vue'
import { xnodeApi, type XNode, type XNodeConfiguration, type XNodeSnippet, type XNodeGroup, type XNodeEndpoint, type XNodeMetrics } from '@/api/xnode'
import { localizedError } from '@/i18n'

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
  let timer: ReturnType<typeof setInterval> | undefined
  async function load(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const [nodeList, configList, snippetList, groupList, endpointList, current] = await Promise.all([
        xnodeApi.nodes(), xnodeApi.configurations(), xnodeApi.snippets(), xnodeApi.groups(), xnodeApi.endpoints(), xnodeApi.metrics(),
      ])
      if (!active) return
      nodes.value = nodeList.items; configurations.value = configList.items; snippets.value = snippetList.items
      groups.value = groupList.items; endpoints.value = endpointList.items; metrics.value = current
    } catch (caught) { if (active) error.value = localizedError(caught, 'xnode.loadFailed') }
    finally { if (active) loading.value = false }
  }
  async function perform(action: () => Promise<unknown>): Promise<boolean> {
    if (busy.value) return false
    busy.value = true; actionError.value = null
    try { await action(); await load(); return true }
    catch (caught) { actionError.value = localizedError(caught, 'xnode.saveFailed'); return false }
    finally { busy.value = false }
  }
  onMounted(() => {
    void load()
    timer = setInterval(() => {
      if (document.visibilityState !== 'visible' || loading.value || busy.value) return
      void xnodeApi.metrics().then(value => { if (active) metrics.value = value }).catch(() => { if (metrics.value) metrics.value = { ...metrics.value, nodes: metrics.value.nodes.map(node => ({ ...node, stale: true })) } })
    }, 5000)
  })
  onScopeDispose(() => { active = false; if (timer) clearInterval(timer) })
  return { nodes, configurations, snippets, groups, endpoints, metrics, loading, error, actionError, busy, load, perform }
}
