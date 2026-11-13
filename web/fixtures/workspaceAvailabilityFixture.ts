import { xnodeApi, type XNode } from '@/api/xnode'
import { ApiError } from '@/api/http'

export function workspaceAvailabilityFixture(params: URLSearchParams, nodes: XNode[]): void {
  if (!params.has('availability')) return
  const state = { calls: [] as string[], recovered: false }
  Object.assign(window, { __xnodeAvailability: state })
  if (params.has('long-names')) for (const node of nodes) node.name += ' · International transit and recovery node / ' + 'longlocation'.repeat(12)
  function track<T>(resource: string, read: (signal?: AbortSignal) => Promise<T>): (signal?: AbortSignal) => Promise<T> {
    return async signal => {
      state.calls.push(resource)
      if (params.get('slow-read') === resource) await new Promise(resolve => setTimeout(resolve, Number(params.get('delay')) || 1500))
      if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
      const attempts = state.calls.filter(value => value === resource).length
      if (!state.recovered && (params.get('unavailable') === resource || params.get('fail-after-first') === resource && attempts > 1)) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
      return read(signal)
    }
  }
  xnodeApi.nodes = track('nodes', xnodeApi.nodes)
  xnodeApi.configurations = track('configurations', xnodeApi.configurations)
  xnodeApi.snippets = track('snippets', xnodeApi.snippets)
  xnodeApi.groups = track('groups', xnodeApi.groups)
  xnodeApi.endpoints = track('endpoints', xnodeApi.endpoints)
  xnodeApi.global = track('global', xnodeApi.global)
  const metrics = xnodeApi.metrics
  xnodeApi.metrics = track('metrics', async signal => {
    const value = await metrics(signal)
    return params.has('missing-metrics') ? { ...value, nodes: [] } : value
  })
}
