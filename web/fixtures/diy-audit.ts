import { xnodeMemberApi, type DIYOverrides, type DIYPreview, type DIYView, type DIYWrite, type RoutingView } from '@/api/xnodeMember'
import { memberOperationsApi } from '@/api/memberOperations'
import { ApiError } from '@/api/http'
import type { OperationReceipt } from '@/api/types'

export function installDIYAudit(params: URLSearchParams, trace: { calls: string[]; keys: string[]; writes: unknown[] }, routing: () => Promise<RoutingView>) {
  let revision = 0, overrides: DIYOverrides = params.has('populated') ? { fingerprint: 'firefox', tcpFastOpen: false, tcpKeepAliveIdle: 0, muxConcurrency: -1 } : {}
  let current: RoutingView, failed = false, committed = false, polls = 0
  const operations = new Map<string, { receipt: OperationReceipt; routingChanged: boolean }>()
  const pause = () => new Promise(resolve => setTimeout(resolve, params.has('slow') ? 1800 : 30))
  async function preview(value: DIYOverrides, signal?: AbortSignal): Promise<DIYPreview & { fields: DIYView['fields'] }> {
    const response = await fetch('http://127.0.0.1:5175/diy', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value), signal })
    if (!response.ok) throw new ApiError(response.status, { code: 'XNODE_INVALID', message: '' })
    return response.json()
  }
  xnodeMemberApi.diy = async signal => {
    trace.calls.push('diy-load'); await pause()
    if (params.has('load-error') && !failed) { failed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    current ??= await routing()
    current.plan.choices = current.plan.choices.map(choice => ({ ...choice, inboundEndpointId: choice.inboundEndpointId ?? `${choice.inboundNodeId}-primary`, outboundEndpointId: choice.outboundEndpointId ?? `${choice.outboundNodeId}-primary` }))
    const metadata = await preview(overrides, signal)
    return structuredClone({ settingsRevision: revision, overrides, routing: current, fields: metadata.fields, compatibility: metadata.compatibility })
  }
  xnodeMemberApi.previewDIY = async (body, signal) => { trace.calls.push('diy-preview'); return preview(body.overrides, signal) }
  xnodeMemberApi.saveDIY = async (body: DIYWrite, key: string) => {
    trace.calls.push('diy-save'); trace.keys.push(key); await pause()
    const replay = operations.get(key)
    if (replay) return { settingsRevision: body.settingsRevision + 1, ...replay }
    if (params.has('save-error')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
    if (body.settingsRevision !== revision || body.routingRevision !== current.plan.revision) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
    const routingChanged = JSON.stringify(current.plan.choices) !== JSON.stringify(body.choices)
    overrides = structuredClone(body.overrides); revision++
    const receipt: OperationReceipt = { id: `diy-${revision}`, kind: 'xnode_diy', status: routingChanged ? 'queued' : 'succeeded', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
    operations.set(key, { receipt, routingChanged }); trace.writes.push(structuredClone(body))
    if (routingChanged) current = { ...current, plan: { ...current.plan, choices: structuredClone(body.choices), revision: current.plan.revision + 1, phase: 'revoking', operationId: receipt.id } }
    if (params.has('lost-reply') && !committed) { committed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    return { settingsRevision: revision, receipt, routingChanged }
  }
  memberOperationsApi.getOperation = async id => {
    trace.calls.push('diy-operation'); await pause(); polls++
    const receipt = [...operations.values()].find(op => op.receipt.id === id)?.receipt
    if (!receipt) throw new ApiError(404, { code: 'XNODE_NOT_FOUND', message: '' })
    if (params.has('deployment-error') && polls < 3) return { ...receipt, status: 'pending_review' }
    current = { ...current, plan: { ...current.plan, phase: 'active' } }
    return { ...receipt, status: 'succeeded' }
  }
}
