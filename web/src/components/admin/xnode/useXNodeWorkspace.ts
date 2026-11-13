import { computed, onMounted, onScopeDispose, shallowRef, watch, type ShallowRef } from 'vue'
import { xnodeApi, type XNode, type XNodeConfiguration, type XNodeSnippet, type XNodeGroup, type XNodeEndpoint, type XNodeMetrics } from '@/api/xnode'
import { localizedError, t } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'
import { workspaceNeedsMetrics, workspaceResources, type WorkspaceTab } from './workspaceTabs'

export function useXNodeWorkspace(tab: () => WorkspaceTab = () => 'nodes') {
  const nodes = shallowRef<XNode[]>([])
  const configurations = shallowRef<XNodeConfiguration[]>([])
  const snippets = shallowRef<XNodeSnippet[]>([])
  const groups = shallowRef<XNodeGroup[]>([])
  const endpoints = shallowRef<XNodeEndpoint[]>([])
  const metrics = shallowRef<XNodeMetrics | null>(null)
  const loading = shallowRef(true)
  const error = shallowRef<string | null>(null)
  const metricError = shallowRef<string | null>(null)
  const actionError = shallowRef<string | null>(null)
  const busy = shallowRef(false)
  const canRefresh = computed(() => workspaceResources(tab()).length > 0)
  let active = true
  let metricInFlight = false
  let metricFailures = 0
  const latest = createLatestRequest()
  const metricLatest = createLatestRequest()
  let controller: AbortController | undefined
  let metricController: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined

  function stopMetrics(): void {
    clearTimeout(timer); metricController?.abort(); metricLatest.invalidate(); metricInFlight = false
    if (metrics.value) metrics.value = { ...metrics.value, nodes: metrics.value.nodes.map(node => ({ ...node, stale: true })) }
  }
  async function loadMetrics(): Promise<void> {
    if (!active || metricInFlight || document.hidden || !workspaceNeedsMetrics(tab())) return
    clearTimeout(timer)
    metricController = new AbortController()
    const ticket = metricLatest.begin()
    metricInFlight = true
    try {
      const value = await xnodeApi.metrics(metricController.signal)
      if (!metricLatest.isCurrent(ticket)) return
      metrics.value = value; metricError.value = null; metricFailures = 0
    } catch {
      if (!metricLatest.isCurrent(ticket)) return
      metricError.value = t('xnode.metricsLoadFailed')
      metricFailures = Math.min(metricFailures + 1, 4)
      if (metrics.value) metrics.value = { ...metrics.value, nodes: metrics.value.nodes.map(node => ({ ...node, stale: true })) }
    } finally {
      if (metricLatest.isCurrent(ticket)) {
        metricInFlight = false
        if (active && !document.hidden && workspaceNeedsMetrics(tab())) timer = setTimeout(() => void loadMetrics(), Math.min(5000 * 2 ** metricFailures, 60000))
      }
    }
  }
  async function load(): Promise<void> {
    if (!active) return
    controller?.abort(); stopMetrics()
    const abort = new AbortController(); controller = abort
    const ticket = latest.begin()
    loading.value = true; error.value = null
    // Live observations are optional. Their failure must not disable recovery or configuration.
    void loadMetrics()
    async function read<T>(fetch: (signal: AbortSignal) => Promise<{ items: T[] }>, target: ShallowRef<T[]>): Promise<void> {
      const result = await fetch(abort.signal)
      if (latest.isCurrent(ticket)) target.value = result.items
    }
    const reads = {
      nodes: () => read(xnodeApi.nodes, nodes), configurations: () => read(xnodeApi.configurations, configurations),
      snippets: () => read(xnodeApi.snippets, snippets), groups: () => read(xnodeApi.groups, groups), endpoints: () => read(xnodeApi.endpoints, endpoints),
    }
    const results = await Promise.allSettled(workspaceResources(tab()).map(resource => reads[resource]()))
    if (!latest.isCurrent(ticket)) return
    const failure = results.find(result => result.status === 'rejected')
    if (failure?.status === 'rejected') error.value = localizedError(failure.reason, 'xnode.loadFailed')
    loading.value = false
  }
  async function perform(action: () => Promise<unknown>): Promise<boolean> {
    if (busy.value || !active) return false
    if (loading.value || error.value) { actionError.value = error.value ?? localizedError(null, 'xnode.loadFailed'); return false }
    busy.value = true; actionError.value = null
    try { await action(); if (!active) return false; await load(); return active }
    catch (caught) { if (active) actionError.value = localizedError(caught, 'xnode.saveFailed'); return false }
    finally { if (active) busy.value = false }
  }
  function visibility(): void { if (document.hidden) stopMetrics(); else void loadMetrics() }
  function dispose(): void {
    active = false; controller?.abort(); stopMetrics(); latest.dispose(); metricLatest.dispose()
    metrics.value = null; nodes.value = []; configurations.value = []; snippets.value = []; groups.value = []; endpoints.value = []
  }
  watch(tab, () => { actionError.value = null; void load() }, { flush: 'sync' })
  onMounted(() => {
    document.addEventListener('visibilitychange', visibility)
    window.addEventListener('txc:session-cleared', dispose)
    void load()
  })
  onScopeDispose(() => {
    dispose(); document.removeEventListener('visibilitychange', visibility); window.removeEventListener('txc:session-cleared', dispose)
  })
  return { nodes, configurations, snippets, groups, endpoints, metrics, loading, error, metricError, actionError, busy, canRefresh, load, perform }
}
