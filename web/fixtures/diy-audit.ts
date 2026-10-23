import { xnodeMemberApi, type DIYOverrides, type DIYPreview, type DIYView, type DIYWhitelist, type DIYWrite, type DIYSplitProfile, type DIYBillingQuote, type RoutingView } from '@/api/xnodeMember'
import { memberOperationsApi } from '@/api/memberOperations'
import { ApiError } from '@/api/http'
import type { OperationReceipt } from '@/api/types'

export function installDIYAudit(params: URLSearchParams, trace: { calls: string[]; keys: string[]; writes: unknown[] }, routing: () => Promise<RoutingView>) {
  let revision = 0, overrides: DIYOverrides = params.has('populated') ? { fingerprint: 'firefox', tcpFastOpen: false, muxXudpConcurrency: -1 } : {}
  let whitelist: DIYWhitelist = { enabled: params.has('whitelist'), ips: params.has('whitelist') ? ['8.8.8.8'] : [] }, pendingWhitelist = false
  let splitProfiles: DIYSplitProfile[] = params.has('split') ? [{ id: 'split-one', uploadEndpointId: 'a-primary', downloadEndpointId: 'a-secondary', mode: 'auto' }] : []
  let remaining = params.has('paid') ? 0 : 5, chargeCount = 0
  function billing(changed = true): DIYBillingQuote { return { available: true, changed, useQuota: changed && remaining > 0, remaining, charge: { currency: 'TXB', minor: changed && !remaining ? '250' : '0', display: changed && !remaining ? '2.50 TXB' : '0.00 TXB' }, token: `quote-${remaining}`, expiresAt: 9999999999 } }
  const splitHosts: DIYView['splitHosts'] = params.has('empty') ? [] : [
    { id: 'a-primary', name: 'Amsterdam primary', nodeId: 'a', inboundTag: 'xhttp', downloadIds: ['a-secondary', 'a-tertiary'], modes: ['auto', 'packet-up'], downloadOptions: [{ downloadEndpointId: 'a-secondary', modes: ['auto'] }, { downloadEndpointId: 'a-tertiary', modes: ['packet-up'] }] },
    { id: 'a-secondary', name: 'Amsterdam alternate', nodeId: 'a', inboundTag: 'xhttp', downloadIds: ['a-primary'], modes: ['stream-up'], downloadOptions: [{ downloadEndpointId: 'a-primary', modes: ['stream-up'] }] },
    { id: 'a-tertiary', name: 'Amsterdam reserve', nodeId: 'a', inboundTag: 'xhttp', downloadIds: ['a-primary'], modes: ['stream-up'], downloadOptions: [{ downloadEndpointId: 'a-primary', modes: ['stream-up'] }] },
  ]
  let current: RoutingView, failed = false, committed = false, polls = 0
  const operations = new Map<string, { receipt: OperationReceipt; routingChanged: boolean; deploymentChanged: boolean }>()
  const pause = () => new Promise(resolve => setTimeout(resolve, params.has('slow') ? 1800 : 30))
  async function preview(value: DIYOverrides, signal?: AbortSignal): Promise<DIYPreview & { fields: DIYView['fields'] }> {
    const query = new URLSearchParams()
    if (params.has('inherited-stream-one')) query.set('mode', 'stream-one')
    if (params.has('empty')) query.set('empty', '1')
    const response = await fetch(`http://127.0.0.1:5175/diy?${query}`,  { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value), signal })
    if (!response.ok) throw new ApiError(response.status, { code: 'XNODE_INVALID', message: '' })
    return { ...await response.json(), billing: billing() }
  }
  xnodeMemberApi.diy = async signal => {
    trace.calls.push('diy-load'); await pause()
    if (params.has('load-error') && !failed) { failed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    current ??= await routing()
    if (params.has('recovered') && !current.plan.operationId) {
      const receipt: OperationReceipt = { id: 'recovered-diy', kind: 'xnode_diy', status: params.get('recovered') === 'failed' ? 'failed' : params.get('recovered') === 'review' ? 'pending_review' : 'processing', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
      current.plan = { ...current.plan, phase: 'revoking', operationId: receipt.id }
      operations.set('recovered', { receipt, routingChanged: true, deploymentChanged: true })
    }
    current.plan.choices = current.plan.choices.map(choice => ({ ...choice, inboundEndpointId: choice.inboundEndpointId ?? `${choice.inboundNodeId}-primary`, outboundEndpointId: choice.outboundEndpointId ?? `${choice.outboundNodeId}-primary` }))
    const metadata = await preview(overrides, signal)
    return structuredClone({ splitProfiles, splitHosts, billing: billing(), settingsRevision: revision, overrides, whitelist: { ...whitelist, quota: params.has('small-quota') ? 1 : 5, phase: pendingWhitelist ? 'pending' : whitelist.enabled ? 'active' : 'disabled', pendingNodes: pendingWhitelist ? ['a'] : [] }, routing: current, fields: metadata.fields, compatibility: metadata.compatibility })
  }
  xnodeMemberApi.previewDIY = async (body, signal) => { trace.calls.push('diy-preview'); const result = await preview(body.overrides, signal); return { ...result, billing: billing(JSON.stringify(body.overrides) !== JSON.stringify(overrides) || JSON.stringify(body.choices) !== JSON.stringify(current.plan.choices) || JSON.stringify(body.whitelist ?? whitelist) !== JSON.stringify(whitelist) || JSON.stringify(body.splitProfiles ?? splitProfiles) !== JSON.stringify(splitProfiles)) } }
  xnodeMemberApi.saveDIY = async (body: DIYWrite, key: string) => {
    trace.calls.push('diy-save'); trace.keys.push(key); await pause()
    const replay = operations.get(key)
    if (replay) return { changed: true, settingsRevision: body.settingsRevision + 1, ...replay }
    if (params.has('save-error')) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
    if (body.settingsRevision !== revision || body.routingRevision !== current.plan.revision) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
    if (params.has('insufficient')) throw new ApiError(409, { code: 'INSUFFICIENT_BALANCE', message: '' })
    if (params.has('stale-quote')) throw new ApiError(409, { code: 'DIY_QUOTE_CHANGED', message: '' })
    const changed = JSON.stringify(body.overrides) !== JSON.stringify(overrides) || JSON.stringify(body.choices) !== JSON.stringify(current.plan.choices) || JSON.stringify(body.whitelist ?? whitelist) !== JSON.stringify(whitelist) || JSON.stringify(body.splitProfiles ?? splitProfiles) !== JSON.stringify(splitProfiles)
    if (changed) { chargeCount++; remaining = Math.max(0, remaining - 1); trace.calls.push(`diy-charge:${chargeCount}`) }
    splitProfiles = structuredClone(body.splitProfiles ?? splitProfiles)
    const routingChanged = JSON.stringify(current.plan.choices) !== JSON.stringify(body.choices)
    const deploymentChanged = routingChanged || JSON.stringify(whitelist) !== JSON.stringify(body.whitelist ?? whitelist)
    whitelist = structuredClone(body.whitelist ?? whitelist); pendingWhitelist = deploymentChanged
    overrides = structuredClone(body.overrides); if (changed) revision++
    const receipt: OperationReceipt = { id: `diy-${revision}`, kind: 'xnode_diy', status: deploymentChanged ? 'queued' : 'succeeded', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
    operations.set(key, { receipt, routingChanged, deploymentChanged }); trace.writes.push(structuredClone(body))
    if (routingChanged) current = { ...current, plan: { ...current.plan, choices: structuredClone(body.choices), revision: current.plan.revision + 1, phase: 'revoking', operationId: receipt.id } }
    if (params.has('lost-reply') && !committed) { committed = true; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    return { changed, settingsRevision: revision, receipt, routingChanged, deploymentChanged }
  }
  memberOperationsApi.getOperation = async id => {
    trace.calls.push('diy-operation'); await pause(); polls++
    const receipt = [...operations.values()].find(op => op.receipt.id === id)?.receipt
    if (!receipt) throw new ApiError(404, { code: 'XNODE_NOT_FOUND', message: '' })
    if (params.has('poll-error') && polls === 1) throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' })
    if (receipt.status === 'failed' || receipt.status === 'pending_review' || params.has('recovered') && polls < 3) return { ...receipt }
    if (params.has('deployment-error') && polls < 3) return { ...receipt, status: 'pending_review' }
    pendingWhitelist = false
    current = { ...current, plan: { ...current.plan, phase: 'active' } }
    return { ...receipt, status: 'succeeded' }
  }
}
