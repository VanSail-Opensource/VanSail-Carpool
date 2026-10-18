import { onMounted, onUnmounted, readonly, shallowRef } from 'vue'

import { api } from '@/api/client'
import type { StatisticsNodesSnapshot, StatisticsSnapshot } from '@/api/types'
import { localizedError } from '@/i18n'

const nodeRefreshMilliseconds = 10_000

export function useStatistics() {
  const snapshot = shallowRef<StatisticsSnapshot | null>(null)
  const nodeSnapshot = shallowRef<StatisticsNodesSnapshot | null>(null)
  const loading = shallowRef(true)
  const refreshing = shallowRef(false)
  const nodesLoading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const nodesError = shallowRef<string | null>(null)
  let timer: number | undefined
  let disposed = false
  let requestVersion = 0
  let controller: AbortController | undefined
  let nodeController: AbortController | undefined
  let nodeVersion = 0

  async function loadNodes(): Promise<void> {
    if (nodesLoading.value || disposed) return
    nodeController?.abort(); nodeController = new AbortController()
    const version = ++nodeVersion
    nodesLoading.value = true
    nodesError.value = null
    try {
      const response = await api.getStatisticsNodes(nodeController.signal)
      if (!disposed && version === nodeVersion) nodeSnapshot.value = response
    } catch (caught) {
      if (!disposed && version === nodeVersion) nodesError.value = localizedError(caught, 'statistics.nodesLoadFailed')
    } finally {
      if (!disposed && version === nodeVersion) nodesLoading.value = false
    }
  }

  async function load(options: { quiet?: boolean } = {}): Promise<void> {
    controller?.abort(); nodeController?.abort(); controller = new AbortController()
    const nodesTicket = ++nodeVersion; nodesLoading.value = false
    const version = ++requestVersion
    if (options.quiet) refreshing.value = true
    else loading.value = snapshot.value === null
    error.value = null
    const [statisticsResult, nodesResult] = await Promise.allSettled([
      api.getStatistics(controller.signal),
      api.getStatisticsNodes(controller.signal),
    ])
    if (disposed || version !== requestVersion) return
    if (statisticsResult.status === 'fulfilled') snapshot.value = statisticsResult.value
    else error.value = localizedError(statisticsResult.reason, 'statistics.loadFailed')
    if (nodesTicket === nodeVersion) {
      if (nodesResult.status === 'fulfilled') {
        nodeSnapshot.value = nodesResult.value
        nodesError.value = null
      } else {
        nodesError.value = localizedError(nodesResult.reason, 'statistics.nodesLoadFailed')
      }
    }
    loading.value = false
    refreshing.value = false
  }

  onMounted(() => {
    void load()
    timer = window.setInterval(() => {
      if (!document.hidden) void loadNodes()
    }, nodeRefreshMilliseconds)
  })

  onUnmounted(() => {
    disposed = true
    requestVersion += 1; nodeVersion++; controller?.abort(); nodeController?.abort(); snapshot.value = null; nodeSnapshot.value = null
    if (timer !== undefined) window.clearInterval(timer)
  })

  return {
    snapshot: readonly(snapshot),
    nodeSnapshot: readonly(nodeSnapshot),
    loading: readonly(loading),
    refreshing: readonly(refreshing),
    nodesLoading: readonly(nodesLoading),
    error: readonly(error),
    nodesError: readonly(nodesError),
    load,
  }
}
