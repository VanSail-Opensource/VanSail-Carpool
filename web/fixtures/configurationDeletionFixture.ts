import { ApiError } from '@/api/http'
import { xnodeApi, type XNode, type XNodeConfiguration } from '@/api/xnode'

type Trace = { calls: string[]; writes: unknown[] }

export function configurationDeletionFixture(params: URLSearchParams, trace: Trace, nodes: XNode[], configurations: XNodeConfiguration[]): void {
  if (params.has('configuration-deletion') && configurations[0]) {
    const unused: XNodeConfiguration = { ...structuredClone(configurations[0]), id: 'unused-configuration', name: 'Unused regional fallback configuration' }
    if (params.has('configuration-only-unused')) {
      configurations.splice(0, configurations.length, unused)
      nodes.splice(0)
    } else configurations.push(unused)
  }
  let failed = false
  xnodeApi.deleteConfiguration = async id => {
    trace.calls.push('delete-configuration:' + id)
    if (params.has('configuration-delete-slow')) await new Promise(resolve => setTimeout(resolve, 1500))
    if (nodes.some(node => node.configId === id)) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
    if (params.has('configuration-delete-error') && !failed) {
      failed = true
      throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
    }
    const index = configurations.findIndex(item => item.id === id)
    if (index < 0) throw new ApiError(404, { code: 'XNODE_NOT_FOUND', message: '' })
    configurations.splice(index, 1)
    trace.writes.push({ deleteConfiguration: id })
  }
}
